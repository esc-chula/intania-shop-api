package repositories

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/esc-chula/intania-shop-api/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrOrderNotFound reports an order ID that does not exist in the project.
var ErrOrderNotFound = errors.New("order not found")

// OrderRepository reads the immutable POS order history recorded at checkout.
// It never joins back to products, users, or promotions for display data, so
// a later edit to any of those never changes a returned order.
type OrderRepository struct {
	pool          *pgxpool.Pool
	storageBucket string
}

// NewOrderRepository constructs an order repository. storageBucket is used to
// rebuild a payment slip's public URL from its stored object key.
func NewOrderRepository(pool *pgxpool.Pool, storageBucket string) *OrderRepository {
	return &OrderRepository{pool: pool, storageBucket: storageBucket}
}

const orderColumns = `o.order_id, o.order_number, o.project_id,
	o.staff_id, o.staff_full_name, o.staff_email,
	o.buyer_gender, o.buyer_age, o.buyer_student_alumni_year,
	o.payment_method, o.payment_slip_object_key,
	o.payment_received_amount::text, o.payment_change_amount::text, o.payment_retained_amount::text,
	o.payment_no_change, o.payment_note,
	o.subtotal::text, o.discount::text, o.net_total::text,
	o.promotion_id, o.promotion_name, o.promotion_original_bundle_price::text,
	o.promotion_price::text, o.promotion_discount::text,
	o.created_at`

const orderFilterClause = ` WHERE o.project_id = $1
  AND ($2::text = '' OR o.order_number ILIKE '%' || $2 || '%' ESCAPE '\')
  AND ($3::text = '' OR o.payment_method::text = $3)
  AND ($4::bigint = 0 OR o.staff_id = $4)
  AND o.created_at >= $5 AND o.created_at <= $6`

// List returns one filtered, sorted page of orders and the filtered total.
func (repository *OrderRepository) List(ctx context.Context, projectID int64, filter models.OrderFilter, offset, limit int32) ([]models.Order, int64, error) {
	if err := repository.requireProject(ctx, projectID); err != nil {
		return nil, 0, err
	}

	query := fmt.Sprintf(`SELECT %s, COUNT(*) OVER () AS total
FROM orders o
%s
ORDER BY %s
OFFSET $7 LIMIT $8`, orderColumns, orderFilterClause, orderByClause(filter.SortBy, filter.SortOrder))

	rows, err := repository.pool.Query(ctx, query,
		projectID, escapeLikePattern(filter.OrderNumber), string(filter.PaymentMethod), filter.StaffID,
		filter.CreatedFrom, filter.CreatedTo, offset, limit)
	if err != nil {
		return nil, 0, fmt.Errorf("list orders: %w", err)
	}
	defer rows.Close()

	orders := make([]models.Order, 0)
	ids := make([]int64, 0)
	var total int64
	for rows.Next() {
		order, err := repository.scanOrder(rows, &total)
		if err != nil {
			return nil, 0, err
		}
		orders = append(orders, order)
		ids = append(ids, order.OrderID)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate orders: %w", err)
	}

	if len(orders) == 0 {
		if total, err = repository.count(ctx, projectID, filter); err != nil {
			return nil, 0, err
		}
		return orders, total, nil
	}

	if err := repository.attachItems(ctx, orders, ids); err != nil {
		return nil, 0, err
	}
	return orders, total, nil
}

// Export returns every order matching the filter in the requested sort
// order, unpaginated, for the Excel export.
func (repository *OrderRepository) Export(ctx context.Context, projectID int64, filter models.OrderFilter) ([]models.Order, error) {
	if err := repository.requireProject(ctx, projectID); err != nil {
		return nil, err
	}

	query := fmt.Sprintf(`SELECT %s
FROM orders o
%s
ORDER BY %s`, orderColumns, orderFilterClause, orderByClause(filter.SortBy, filter.SortOrder))

	rows, err := repository.pool.Query(ctx, query,
		projectID, escapeLikePattern(filter.OrderNumber), string(filter.PaymentMethod), filter.StaffID,
		filter.CreatedFrom, filter.CreatedTo)
	if err != nil {
		return nil, fmt.Errorf("export orders: %w", err)
	}
	defer rows.Close()

	orders := make([]models.Order, 0)
	ids := make([]int64, 0)
	for rows.Next() {
		order, err := repository.scanOrder(rows)
		if err != nil {
			return nil, err
		}
		orders = append(orders, order)
		ids = append(ids, order.OrderID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate orders: %w", err)
	}

	if len(orders) == 0 {
		return orders, nil
	}
	if err := repository.attachItems(ctx, orders, ids); err != nil {
		return nil, err
	}
	return orders, nil
}

// Detail returns one order, scoped to its project so an order from another
// project or a missing ID both report ErrOrderNotFound.
func (repository *OrderRepository) Detail(ctx context.Context, projectID, orderID int64) (models.Order, error) {
	query := fmt.Sprintf(`SELECT %s FROM orders o WHERE o.project_id = $1 AND o.order_id = $2`, orderColumns)

	order, err := repository.scanOrder(repository.pool.QueryRow(ctx, query, projectID, orderID))
	if errors.Is(err, pgx.ErrNoRows) {
		return models.Order{}, ErrOrderNotFound
	}
	if err != nil {
		return models.Order{}, fmt.Errorf("get order: %w", err)
	}

	items, err := repository.itemsFor(ctx, []int64{order.OrderID})
	if err != nil {
		return models.Order{}, err
	}
	order.Items = items[order.OrderID]
	return order, nil
}

func (repository *OrderRepository) count(ctx context.Context, projectID int64, filter models.OrderFilter) (int64, error) {
	var total int64
	query := `SELECT COUNT(*) FROM orders o` + orderFilterClause
	if err := repository.pool.QueryRow(ctx, query,
		projectID, escapeLikePattern(filter.OrderNumber), string(filter.PaymentMethod), filter.StaffID,
		filter.CreatedFrom, filter.CreatedTo).Scan(&total); err != nil {
		return 0, fmt.Errorf("count orders: %w", err)
	}
	return total, nil
}

func (repository *OrderRepository) requireProject(ctx context.Context, projectID int64) error {
	var exists int
	err := repository.pool.QueryRow(ctx, `SELECT 1 FROM projects WHERE project_id = $1`, projectID).Scan(&exists)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrProjectNotFound
	}
	if err != nil {
		return fmt.Errorf("check project: %w", err)
	}
	return nil
}

func (repository *OrderRepository) attachItems(ctx context.Context, orders []models.Order, orderIDs []int64) error {
	itemsByOrder, err := repository.itemsFor(ctx, orderIDs)
	if err != nil {
		return err
	}
	for index := range orders {
		orders[index].Items = itemsByOrder[orders[index].OrderID]
	}
	return nil
}

const orderItemColumns = `order_item_id, order_id, product_id, variant_id, product_name,
	size, color, image_url, quantity, unit_price::text, line_total::text, inventory_transaction_id`

func (repository *OrderRepository) itemsFor(ctx context.Context, orderIDs []int64) (map[int64][]models.OrderItemSnapshot, error) {
	rows, err := repository.pool.Query(ctx,
		`SELECT `+orderItemColumns+` FROM order_items WHERE order_id = ANY($1) ORDER BY order_id, order_item_id`, orderIDs)
	if err != nil {
		return nil, fmt.Errorf("list order items: %w", err)
	}
	defer rows.Close()

	byOrder := make(map[int64][]models.OrderItemSnapshot)
	for rows.Next() {
		var (
			item         models.OrderItemSnapshot
			orderID      int64
			unitPriceRaw string
			lineTotalRaw string
		)
		if err := rows.Scan(&item.OrderItemID, &orderID, &item.ProductID, &item.VariantID, &item.ProductName,
			&item.Size, &item.Color, &item.ImageURL, &item.Quantity, &unitPriceRaw, &lineTotalRaw,
			&item.InventoryTransactionID); err != nil {
			return nil, fmt.Errorf("scan order item: %w", err)
		}
		if item.UnitPrice, err = models.ParseTHBAmount(unitPriceRaw); err != nil {
			return nil, fmt.Errorf("parse order item unit price: %w", err)
		}
		if item.LineTotal, err = models.ParseTHBAmount(lineTotalRaw); err != nil {
			return nil, fmt.Errorf("parse order item line total: %w", err)
		}
		byOrder[orderID] = append(byOrder[orderID], item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate order items: %w", err)
	}
	return byOrder, nil
}

// scanner is the Scan surface shared by pgx.Row and pgx.Rows.
type scanner interface{ Scan(dest ...any) error }

// scanOrder reads one order header row, appending any extra scan
// destinations (the window total) the caller selected after those columns,
// and rebuilds the payment slip URL from its stored object key.
func (repository *OrderRepository) scanOrder(row scanner, extra ...any) (models.Order, error) {
	var (
		order                   models.Order
		staffID                 *int64
		buyerGender             string
		paymentMethod           string
		slipObjectKey           *string
		receivedAmountRaw       *string
		changeAmountRaw         *string
		retainedAmountRaw       *string
		subtotalRaw             string
		discountRaw             string
		netTotalRaw             string
		promotionID             *int64
		promotionName           *string
		promotionBundlePriceRaw *string
		promotionPriceRaw       *string
		promotionDiscountRaw    *string
	)

	destinations := append([]any{
		&order.OrderID, &order.OrderNumber, &order.ProjectID,
		&staffID, &order.Staff.FullName, &order.Staff.Email,
		&buyerGender, &order.Buyer.Age, &order.Buyer.StudentAlumniYear,
		&paymentMethod, &slipObjectKey,
		&receivedAmountRaw, &changeAmountRaw, &retainedAmountRaw,
		&order.Payment.NoChange, &order.Payment.Note,
		&subtotalRaw, &discountRaw, &netTotalRaw,
		&promotionID, &promotionName, &promotionBundlePriceRaw,
		&promotionPriceRaw, &promotionDiscountRaw,
		&order.CreatedAt,
	}, extra...)

	if err := row.Scan(destinations...); err != nil {
		return models.Order{}, fmt.Errorf("scan order: %w", err)
	}

	order.Status = models.OrderStatusCompleted
	order.Buyer.Gender = models.OrderBuyerGender(buyerGender)
	order.Payment.Method = models.OrderPaymentMethod(paymentMethod)
	order.Payment.SlipURL = repository.slipURL(slipObjectKey)
	if staffID != nil {
		order.Staff.StaffID = *staffID
	}

	var err error
	if order.Subtotal, err = models.ParseTHBAmount(subtotalRaw); err != nil {
		return models.Order{}, fmt.Errorf("parse order subtotal: %w", err)
	}
	if order.Discount, err = models.ParseTHBAmount(discountRaw); err != nil {
		return models.Order{}, fmt.Errorf("parse order discount: %w", err)
	}
	if order.NetTotal, err = models.ParseTHBAmount(netTotalRaw); err != nil {
		return models.Order{}, fmt.Errorf("parse order net total: %w", err)
	}

	if order.Payment.ReceivedAmount, err = parseOptionalTHBAmount(receivedAmountRaw); err != nil {
		return models.Order{}, fmt.Errorf("parse order received amount: %w", err)
	}
	if order.Payment.ChangeAmount, err = parseOptionalTHBAmount(changeAmountRaw); err != nil {
		return models.Order{}, fmt.Errorf("parse order change amount: %w", err)
	}
	if order.Payment.RetainedAmount, err = parseOptionalTHBAmount(retainedAmountRaw); err != nil {
		return models.Order{}, fmt.Errorf("parse order retained amount: %w", err)
	}

	if promotionID != nil && promotionName != nil && promotionBundlePriceRaw != nil &&
		promotionPriceRaw != nil && promotionDiscountRaw != nil {
		bundlePrice, err := models.ParseTHBAmount(*promotionBundlePriceRaw)
		if err != nil {
			return models.Order{}, fmt.Errorf("parse order promotion bundle price: %w", err)
		}
		promotionPrice, err := models.ParseTHBAmount(*promotionPriceRaw)
		if err != nil {
			return models.Order{}, fmt.Errorf("parse order promotion price: %w", err)
		}
		promotionDiscount, err := models.ParseTHBAmount(*promotionDiscountRaw)
		if err != nil {
			return models.Order{}, fmt.Errorf("parse order promotion discount: %w", err)
		}
		order.AppliedPromotion = &models.AppliedProjectPromotion{
			PromotionID:         *promotionID,
			Name:                *promotionName,
			OriginalBundlePrice: bundlePrice,
			PromotionPrice:      promotionPrice,
			Discount:            promotionDiscount,
		}
	}

	return order, nil
}

func parseOptionalTHBAmount(raw *string) (*models.THBAmount, error) {
	if raw == nil {
		return nil, nil
	}
	amount, err := models.ParseTHBAmount(*raw)
	if err != nil {
		return nil, err
	}
	return &amount, nil
}

// orderByClause builds a literal ORDER BY expression from whitelisted enum
// values, which the use case validates before this ever runs. order_id is
// always the final tie-break so a stable sort never reorders across pages.
func orderByClause(sortBy models.OrderSortField, sortOrder models.OrderSortOrder) string {
	direction := "asc"
	if sortOrder == models.OrderSortDesc {
		direction = "desc"
	}

	column := "o.order_number"
	if sortBy == models.OrderSortByNetTotal {
		column = "o.net_total"
	}

	return fmt.Sprintf("%s %s, o.order_id %s", column, direction, direction)
}

// slipURL rebuilds a payment slip's public URL from its stored object key.
func (repository *OrderRepository) slipURL(objectKey *string) *string {
	if objectKey == nil || strings.TrimSpace(*objectKey) == "" {
		return nil
	}
	url := "https://storage.googleapis.com/" + repository.storageBucket + "/" + *objectKey
	return &url
}
