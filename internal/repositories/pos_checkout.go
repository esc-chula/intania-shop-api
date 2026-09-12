package repositories

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/esc-chula/intania-shop-api/internal/models"
	"github.com/jackc/pgx/v5"
)

var (
	// ErrPaymentSlipNotTrusted reports a slip object key this server did not
	// issue, or one that already backs another order.
	ErrPaymentSlipNotTrusted = errors.New("payment slip is not trusted")
	// ErrIdempotencyKeyReused reports an Idempotency-Key replayed with a
	// different payload.
	ErrIdempotencyKeyReused = errors.New("idempotency key was reused with a different request")
	// ErrPOSStaffNotFound reports an authenticated staff user that no longer
	// exists.
	ErrPOSStaffNotFound = errors.New("POS staff user not found")
)

// RecordPaymentSlip stores one uploaded slip so a later checkout can prove the
// object key it received came from this server.
func (repository *POSRepository) RecordPaymentSlip(ctx context.Context, slip models.PaymentSlip, uploadedBy int64) error {
	if _, err := repository.pool.Exec(ctx, `
		INSERT INTO payment_slips (object_key, url, content_type, size_bytes, uploaded_by)
		VALUES ($1, $2, $3, $4, $5)`,
		slip.ObjectKey, slip.URL, slip.ContentType, slip.Size, uploadedBy); err != nil {
		return fmt.Errorf("insert payment slip: %w", err)
	}

	return nil
}

// Checkout creates one paid order, its snapshots, and its ORDER stock
// transactions in a single transaction. The planner is called with the locked
// snapshot and decides every monetary value; an error it returns rolls the
// whole checkout back, leaving no order, payment, or stock change behind.
//
// The boolean reports an idempotent replay of a previously stored order.
func (repository *POSRepository) Checkout(ctx context.Context, today models.Date, command models.POSCheckoutCommand,
	planner func(models.POSCheckoutSnapshot) (models.POSCheckoutPlan, error)) (models.POSOrder, bool, error) {
	if err := validatePOSCartIdentities(command.Items); err != nil {
		return models.POSOrder{}, false, err
	}

	order, replayed, err := repository.replayOrder(ctx, repository.pool, command)
	if err != nil || replayed {
		return order, replayed, err
	}

	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return models.POSOrder{}, false, fmt.Errorf("begin POS checkout: %w", err)
	}
	defer rollback(ctx, tx)

	status, err := lockPOSProject(ctx, tx, today, command.ProjectID)
	if err != nil {
		return models.POSOrder{}, false, err
	}
	if err := lockPOSStock(ctx, tx, command.Items); err != nil {
		return models.POSOrder{}, false, err
	}

	// A duplicate that passed the pre-check above blocks on the stock locks
	// and resumes only once the winner has committed. Replaying here returns
	// the stored order rather than planning against decremented stock or a
	// slip that now backs the winner's order.
	order, replayed, err = repository.replayOrder(ctx, tx, command)
	if err != nil || replayed {
		return order, replayed, err
	}

	snapshot, err := repository.readCheckoutSnapshot(ctx, tx, status, command)
	if err != nil {
		return models.POSOrder{}, false, err
	}

	plan, err := planner(snapshot)
	if err != nil {
		return models.POSOrder{}, false, err
	}

	orderID, orderNumber, inserted, err := insertPOSOrder(ctx, tx, command, snapshot, plan)
	if err != nil {
		return models.POSOrder{}, false, err
	}
	if !inserted {
		// A concurrent request committed this Idempotency-Key first. Release
		// the locks before replaying its order.
		rollback(ctx, tx)
		return repository.replayOrder(ctx, repository.pool, command)
	}

	if err := writePOSOrderLines(ctx, tx, orderID, orderNumber, command, snapshot, plan); err != nil {
		return models.POSOrder{}, false, err
	}
	if err := insertPOSOrderPayment(ctx, tx, orderID, snapshot, plan); err != nil {
		return models.POSOrder{}, false, err
	}
	if err := insertPOSOrderPromotion(ctx, tx, orderID, plan); err != nil {
		return models.POSOrder{}, false, err
	}

	created, err := readPOSOrder(ctx, tx, orderID)
	if err != nil {
		return models.POSOrder{}, false, err
	}

	if err := tx.Commit(ctx); err != nil {
		return models.POSOrder{}, false, fmt.Errorf("commit POS checkout: %w", err)
	}

	return created, false, nil
}

// replayOrder returns the order already stored under the Idempotency-Key. A
// key stored for a different project or payload is a reuse conflict rather
// than a replay.
func (repository *POSRepository) replayOrder(ctx context.Context, queryer promotionQueryer, command models.POSCheckoutCommand) (models.POSOrder, bool, error) {
	var (
		orderID     int64
		projectID   int64
		fingerprint *string
	)
	err := queryer.QueryRow(ctx, `
		SELECT order_id, project_id, request_fingerprint
		FROM orders
		WHERE idempotency_key = $1`, command.IdempotencyKey).Scan(&orderID, &projectID, &fingerprint)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.POSOrder{}, false, nil
	}
	if err != nil {
		return models.POSOrder{}, false, fmt.Errorf("read idempotent POS order: %w", err)
	}

	if projectID != command.ProjectID || fingerprint == nil || *fingerprint != command.RequestFingerprint {
		return models.POSOrder{}, false, ErrIdempotencyKeyReused
	}

	order, err := readPOSOrder(ctx, queryer, orderID)
	if err != nil {
		return models.POSOrder{}, false, err
	}

	return order, true, nil
}

// readCheckoutSnapshot reads the catalogue, Promotion, and staff data the
// planner needs, and locks the referenced payment slip. The project and stock
// rows are already locked by the caller, so everything it returns is protected
// by a lock held until the transaction ends.
func (repository *POSRepository) readCheckoutSnapshot(ctx context.Context, tx pgx.Tx, status models.ProjectStatus, command models.POSCheckoutCommand) (models.POSCheckoutSnapshot, error) {
	items, err := queryPOSCartItems(ctx, tx, command.ProjectID, command.Items)
	if err != nil {
		return models.POSCheckoutSnapshot{}, err
	}

	hydratedPromotions, err := repository.promotions.queryPromotions(ctx, tx, command.ProjectID, nil)
	if err != nil {
		return models.POSCheckoutSnapshot{}, err
	}

	staff, err := readPOSStaff(ctx, tx, command.StaffUserID)
	if err != nil {
		return models.POSCheckoutSnapshot{}, err
	}

	slip, err := lockTrustedPaymentSlip(ctx, tx, command.Payment)
	if err != nil {
		return models.POSCheckoutSnapshot{}, err
	}

	return models.POSCheckoutSnapshot{
		ProjectStatus: status,
		Staff:         staff,
		Items:         items,
		Promotions:    pricingPromotionsFromHydrated(hydratedPromotions),
		Slip:          slip,
	}, nil
}

// lockPOSProject holds a share lock for the rest of the checkout, so the
// project dates and its product selection cannot be changed while the order is
// priced and written.
func lockPOSProject(ctx context.Context, tx pgx.Tx, today models.Date, projectID int64) (models.ProjectStatus, error) {
	var startDate, endDate time.Time
	err := tx.QueryRow(ctx, `
		SELECT start_date, end_date
		FROM projects
		WHERE project_id = $1
		FOR SHARE`, projectID).Scan(&startDate, &endDate)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrProjectNotFound
	}
	if err != nil {
		return "", fmt.Errorf("lock POS checkout project: %w", err)
	}

	return models.ProjectStatusFor(models.NewDate(startDate), models.NewDate(endDate), today), nil
}

// lockPOSStock locks every row whose stock the checkout reduces. Products are
// locked before variants and each group in ascending ID order, so concurrent
// checkouts over overlapping Carts queue instead of deadlocking.
//
// FOR NO KEY UPDATE is the mode the stock UPDATE takes anyway. It still
// excludes a concurrent checkout, but unlike FOR UPDATE it does not conflict
// with the FOR KEY SHARE that a later order_items insert takes on the parent
// product of a variant line.
func lockPOSStock(ctx context.Context, tx pgx.Tx, items []models.POSCartItemRequest) error {
	productIDs := make([]int64, 0, len(items))
	variantIDs := make([]int64, 0, len(items))
	for _, item := range items {
		if item.VariantID == nil {
			productIDs = append(productIDs, item.ProductID)
			continue
		}
		variantIDs = append(variantIDs, *item.VariantID)
	}

	if len(productIDs) != 0 {
		if _, err := tx.Exec(ctx, `
			SELECT 1 FROM products
			WHERE id = ANY($1::bigint[])
			ORDER BY id
			FOR NO KEY UPDATE`, productIDs); err != nil {
			return fmt.Errorf("lock POS product stock: %w", err)
		}
	}

	if len(variantIDs) != 0 {
		if _, err := tx.Exec(ctx, `
			SELECT 1 FROM variants
			WHERE variant_id = ANY($1::bigint[])
			ORDER BY variant_id
			FOR NO KEY UPDATE`, variantIDs); err != nil {
			return fmt.Errorf("lock POS variant stock: %w", err)
		}
	}

	return nil
}

func readPOSStaff(ctx context.Context, tx pgx.Tx, staffUserID int64) (models.POSStaffSnapshot, error) {
	var staff models.POSStaffSnapshot
	err := tx.QueryRow(ctx, `
		SELECT user_id, COALESCE(NULLIF(btrim(full_name), ''), email), email
		FROM users
		WHERE user_id = $1`, staffUserID).Scan(&staff.StaffID, &staff.FullName, &staff.Email)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.POSStaffSnapshot{}, ErrPOSStaffNotFound
	}
	if err != nil {
		return models.POSStaffSnapshot{}, fmt.Errorf("read POS staff: %w", err)
	}

	return staff, nil
}

// lockTrustedPaymentSlip resolves a QR slip reference to the object this
// server stored, and rejects one that already backs another order. The row
// lock serializes two checkouts racing for the same slip.
func lockTrustedPaymentSlip(ctx context.Context, tx pgx.Tx, payment models.POSPaymentRequest) (*models.PaymentSlip, error) {
	if payment.Method != models.POSPaymentQRCode || payment.SlipObjectKey == nil {
		return nil, nil
	}

	var slip models.PaymentSlip
	err := tx.QueryRow(ctx, `
		SELECT object_key, url, content_type, size_bytes
		FROM payment_slips
		WHERE object_key = $1
		FOR UPDATE`, *payment.SlipObjectKey).Scan(&slip.ObjectKey, &slip.URL, &slip.ContentType, &slip.Size)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("%w: unknown slip object key", ErrPaymentSlipNotTrusted)
	}
	if err != nil {
		return nil, fmt.Errorf("lock payment slip: %w", err)
	}

	var used bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM order_payments WHERE slip_object_key = $1)`,
		slip.ObjectKey).Scan(&used); err != nil {
		return nil, fmt.Errorf("check payment slip use: %w", err)
	}
	if used {
		return nil, fmt.Errorf("%w: slip already backs another order", ErrPaymentSlipNotTrusted)
	}

	return &slip, nil
}

// insertPOSOrder reserves the order ID so the opaque order number can be
// derived from it in the same statement. A false result means another
// transaction already committed this Idempotency-Key.
func insertPOSOrder(ctx context.Context, tx pgx.Tx, command models.POSCheckoutCommand,
	snapshot models.POSCheckoutSnapshot, plan models.POSCheckoutPlan) (int64, string, bool, error) {
	var (
		orderID     int64
		orderNumber string
	)
	err := tx.QueryRow(ctx, `
		WITH reserved AS (
		    SELECT nextval(pg_get_serial_sequence('orders', 'order_id')) AS order_id
		)
		INSERT INTO orders (
		    order_id, order_number, project_id,
		    staff_user_id, staff_full_name, staff_email,
		    buyer_gender, buyer_age, buyer_student_alumni_year,
		    subtotal, discount, net_total, idempotency_key, request_fingerprint)
		SELECT reserved.order_id, 'ORD-' || lpad(reserved.order_id::text, 8, '0'), $1,
		       $2, $3, $4,
		       $5::pos_buyer_gender, $6, $7,
		       $8::numeric, $9::numeric, $10::numeric, $11, $12
		FROM reserved
		ON CONFLICT (idempotency_key) DO NOTHING
		RETURNING order_id, order_number`,
		command.ProjectID,
		snapshot.Staff.StaffID, snapshot.Staff.FullName, snapshot.Staff.Email,
		string(command.Buyer.Gender), command.Buyer.Age, command.Buyer.StudentAlumniYear,
		plan.Subtotal.String(), plan.Discount.String(), plan.NetTotal.String(),
		command.IdempotencyKey, command.RequestFingerprint).Scan(&orderID, &orderNumber)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, "", false, nil
	}
	if err != nil {
		return 0, "", false, fmt.Errorf("insert POS order: %w", err)
	}

	return orderID, orderNumber, true, nil
}

// writePOSOrderLines reduces stock, records one ORDER stock transaction per
// line, and snapshots the line itself, in request order.
func writePOSOrderLines(ctx context.Context, tx pgx.Tx, orderID int64, orderNumber string,
	command models.POSCheckoutCommand, snapshot models.POSCheckoutSnapshot, plan models.POSCheckoutPlan) error {
	if len(plan.LineTotals) != len(snapshot.Items) {
		return fmt.Errorf("%w: planned line totals do not match the Cart", ErrPOSDataCorrupt)
	}

	actor := strconv.FormatInt(command.StaffUserID, 10)
	reason := "POS order " + orderNumber

	for index, item := range snapshot.Items {
		before, after, err := reducePOSStock(ctx, tx, item)
		if err != nil {
			return err
		}

		var transactionID int64
		if err := tx.QueryRow(ctx, `
			INSERT INTO stock_transactions (
			    product_id, variant_id, transaction_type, quantity_change,
			    quantity_before, quantity_after, reason, reference_type, reference_id, created_by)
			VALUES ($1, $2, 'ORDER', $3, $4, $5, $6, 'ORDER', $7, $8)
			RETURNING transaction_id`,
			item.Item.ProductID, item.Item.VariantID, -item.Quantity,
			before, after, reason, orderID, actor).Scan(&transactionID); err != nil {
			return fmt.Errorf("record ORDER stock transaction for product %d: %w", item.Item.ProductID, err)
		}

		if _, err := tx.Exec(ctx, `
			INSERT INTO order_items (
			    order_id, product_id, variant_id, product_name, size, color, image_url,
			    quantity, unit_price, line_total, inventory_transaction_id)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::numeric, $10::numeric, $11)`,
			orderID, item.Item.ProductID, item.Item.VariantID, item.Item.ProductName,
			item.Item.Size, item.Item.Color, item.Item.ImageURL, item.Quantity,
			item.Item.ProjectPrice.String(), plan.LineTotals[index].String(), transactionID); err != nil {
			return fmt.Errorf("insert POS order item for product %d: %w", item.Item.ProductID, err)
		}
	}

	return nil
}

// reducePOSStock decrements the locked stock row and reports the quantities
// before and after the sale.
func reducePOSStock(ctx context.Context, tx pgx.Tx, item models.POSResolvedCartItem) (int32, int32, error) {
	var before, after int32
	var err error

	if item.Item.VariantID != nil {
		err = tx.QueryRow(ctx, `
			UPDATE variants
			SET stock_quantity = COALESCE(stock_quantity, 0) - $2
			WHERE variant_id = $1
			RETURNING stock_quantity + $2, stock_quantity`,
			*item.Item.VariantID, item.Quantity).Scan(&before, &after)
	} else {
		err = tx.QueryRow(ctx, `
			UPDATE products
			SET stock_quantity = COALESCE(stock_quantity, 0) - $2, updated_at = CURRENT_TIMESTAMP
			WHERE id = $1
			RETURNING stock_quantity + $2, stock_quantity`,
			item.Item.ProductID, item.Quantity).Scan(&before, &after)
	}
	if err != nil {
		return 0, 0, fmt.Errorf("reduce stock for product %d: %w", item.Item.ProductID, err)
	}

	return before, after, nil
}

func insertPOSOrderPayment(ctx context.Context, tx pgx.Tx, orderID int64,
	snapshot models.POSCheckoutSnapshot, plan models.POSCheckoutPlan) error {
	var slipObjectKey *string
	if snapshot.Slip != nil {
		objectKey := snapshot.Slip.ObjectKey
		slipObjectKey = &objectKey
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO order_payments (
		    order_id, method, slip_object_key, slip_url,
		    received_amount, change_amount, retained_amount, no_change, note)
		VALUES ($1, $2::pos_payment_method, $3, $4, $5::numeric, $6::numeric, $7::numeric, $8, $9)`,
		orderID, string(plan.Payment.Method), slipObjectKey, plan.Payment.SlipURL,
		posAmountText(plan.Payment.ReceivedAmount), posAmountText(plan.Payment.ChangeAmount),
		posAmountText(plan.Payment.RetainedAmount), plan.Payment.NoChange, plan.Payment.Note); err != nil {
		return fmt.Errorf("insert POS order payment: %w", err)
	}

	return nil
}

func insertPOSOrderPromotion(ctx context.Context, tx pgx.Tx, orderID int64, plan models.POSCheckoutPlan) error {
	promotion := plan.AppliedPromotion
	if promotion == nil {
		return nil
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO order_promotions (
		    order_id, promotion_id, name, original_bundle_price, promotion_price, discount)
		VALUES ($1, $2, $3, $4::numeric, $5::numeric, $6::numeric)`,
		orderID, promotion.PromotionID, promotion.Name,
		promotion.OriginalBundlePrice.String(), promotion.PromotionPrice.String(),
		promotion.Discount.String()); err != nil {
		return fmt.Errorf("insert POS order promotion: %w", err)
	}

	return nil
}

// readPOSOrder hydrates one stored order. Joining the payment row restricts
// the result to orders written by checkout, which always carry a complete
// staff and buyer snapshot.
func readPOSOrder(ctx context.Context, queryer promotionQueryer, orderID int64) (models.POSOrder, error) {
	var (
		order          models.POSOrder
		gender         string
		subtotal       string
		discount       string
		netTotal       string
		method         string
		receivedAmount *string
		changeAmount   *string
		retainedAmount *string
		promotionID    *int64
		promotionName  *string
		bundlePrice    *string
		promotionPrice *string
		promotionSaved *string
	)

	err := queryer.QueryRow(ctx, `
		SELECT o.order_id, o.order_number, o.project_id, o.status::text,
		       o.staff_user_id, o.staff_full_name, o.staff_email,
		       o.buyer_gender::text, o.buyer_age, o.buyer_student_alumni_year,
		       o.subtotal::text, o.discount::text, o.net_total::text, o.created_at,
		       pay.method::text, pay.slip_url, pay.received_amount::text,
		       pay.change_amount::text, pay.retained_amount::text, pay.no_change, pay.note,
		       promo.promotion_id, promo.name, promo.original_bundle_price::text,
		       promo.promotion_price::text, promo.discount::text
		FROM orders o
		JOIN order_payments pay ON pay.order_id = o.order_id
		LEFT JOIN order_promotions promo ON promo.order_id = o.order_id
		WHERE o.order_id = $1`, orderID).Scan(
		&order.OrderID, &order.OrderNumber, &order.ProjectID, &order.Status,
		&order.Staff.StaffID, &order.Staff.FullName, &order.Staff.Email,
		&gender, &order.Buyer.Age, &order.Buyer.StudentAlumniYear,
		&subtotal, &discount, &netTotal, &order.CreatedAt,
		&method, &order.Payment.SlipURL, &receivedAmount,
		&changeAmount, &retainedAmount, &order.Payment.NoChange, &order.Payment.Note,
		&promotionID, &promotionName, &bundlePrice, &promotionPrice, &promotionSaved)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.POSOrder{}, fmt.Errorf("%w: order %d has no payment snapshot", ErrPOSDataCorrupt, orderID)
	}
	if err != nil {
		return models.POSOrder{}, fmt.Errorf("read POS order: %w", err)
	}

	order.Buyer.Gender = models.POSGender(gender)
	order.Payment.Method = models.POSPaymentMethod(method)

	if order.Subtotal, err = parsePOSOrderAmount(orderID, subtotal); err != nil {
		return models.POSOrder{}, err
	}
	if order.Discount, err = parsePOSOrderAmount(orderID, discount); err != nil {
		return models.POSOrder{}, err
	}
	if order.NetTotal, err = parsePOSOrderAmount(orderID, netTotal); err != nil {
		return models.POSOrder{}, err
	}

	if order.Payment.ReceivedAmount, err = parseOptionalPOSOrderAmount(orderID, receivedAmount); err != nil {
		return models.POSOrder{}, err
	}
	if order.Payment.ChangeAmount, err = parseOptionalPOSOrderAmount(orderID, changeAmount); err != nil {
		return models.POSOrder{}, err
	}
	if order.Payment.RetainedAmount, err = parseOptionalPOSOrderAmount(orderID, retainedAmount); err != nil {
		return models.POSOrder{}, err
	}

	if promotionID != nil {
		if promotionName == nil || bundlePrice == nil || promotionPrice == nil || promotionSaved == nil {
			return models.POSOrder{}, fmt.Errorf("%w: order %d has an incomplete promotion snapshot", ErrPOSDataCorrupt, orderID)
		}
		applied := models.AppliedProjectPromotion{PromotionID: *promotionID, Name: *promotionName}
		if applied.OriginalBundlePrice, err = parsePOSOrderAmount(orderID, *bundlePrice); err != nil {
			return models.POSOrder{}, err
		}
		if applied.PromotionPrice, err = parsePOSOrderAmount(orderID, *promotionPrice); err != nil {
			return models.POSOrder{}, err
		}
		if applied.Discount, err = parsePOSOrderAmount(orderID, *promotionSaved); err != nil {
			return models.POSOrder{}, err
		}
		order.AppliedPromotion = &applied
	}

	items, err := readPOSOrderItems(ctx, queryer, orderID)
	if err != nil {
		return models.POSOrder{}, err
	}
	order.Items = items

	return order, nil
}

func readPOSOrderItems(ctx context.Context, queryer promotionQueryer, orderID int64) ([]models.POSOrderItemSnapshot, error) {
	rows, err := queryer.Query(ctx, `
		SELECT order_item_id, product_id, variant_id, product_name, size, color, image_url,
		       quantity, unit_price::text, line_total::text, inventory_transaction_id
		FROM order_items
		WHERE order_id = $1
		ORDER BY order_item_id`, orderID)
	if err != nil {
		return nil, fmt.Errorf("read POS order items: %w", err)
	}
	defer rows.Close()

	items := make([]models.POSOrderItemSnapshot, 0)
	for rows.Next() {
		var (
			item      models.POSOrderItemSnapshot
			unitPrice string
			lineTotal string
		)
		if err := rows.Scan(&item.OrderItemID, &item.ProductID, &item.VariantID, &item.ProductName,
			&item.Size, &item.Color, &item.ImageURL, &item.Quantity,
			&unitPrice, &lineTotal, &item.InventoryTransactionID); err != nil {
			return nil, fmt.Errorf("scan POS order item: %w", err)
		}

		if item.UnitPrice, err = parsePOSOrderAmount(orderID, unitPrice); err != nil {
			return nil, err
		}
		if item.LineTotal, err = parsePOSOrderAmount(orderID, lineTotal); err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate POS order items: %w", err)
	}

	return items, nil
}

func parsePOSOrderAmount(orderID int64, value string) (models.THBAmount, error) {
	amount, err := models.ParseTHBAmount(value)
	if err != nil {
		return models.THBAmount{}, fmt.Errorf("%w: order %d amount %q: %v", ErrPOSDataCorrupt, orderID, value, err)
	}

	return amount, nil
}

func parseOptionalPOSOrderAmount(orderID int64, value *string) (*models.THBAmount, error) {
	if value == nil {
		return nil, nil
	}

	amount, err := parsePOSOrderAmount(orderID, *value)
	if err != nil {
		return nil, err
	}

	return &amount, nil
}

func posAmountText(amount *models.THBAmount) *string {
	if amount == nil {
		return nil
	}

	text := amount.String()
	return &text
}
