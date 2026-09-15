//go:build integration

package repositories

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/esc-chula/intania-shop-api/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPOSRepositoryCheckoutWritesTheWholeOrderAtomically(t *testing.T) {
	database := openPromotionIntegrationDatabase(t)
	fixture := newPromotionIntegrationFixture(t, database)
	staffID := insertCheckoutStaff(t, database)
	ctx := context.Background()
	setCheckoutStock(t, database, fixture, 7, 3, 5)

	repository := NewPOSRepository(database)
	command := cashCheckoutCommand(t, fixture, staffID, "checkout-key-1", "200.00", true)

	order, replayed, err := repository.Checkout(ctx, fixture.Today, command, checkoutPlanner(t, command, nil))
	if err != nil {
		t.Fatal(err)
	}
	if replayed {
		t.Fatal("a new order was reported as a replay")
	}

	if order.OrderNumber != fmt.Sprintf("ORD-%08d", order.OrderID) {
		t.Fatalf("order number = %q for order %d", order.OrderNumber, order.OrderID)
	}
	if order.ProjectID != fixture.ProjectID || order.Status != models.POSOrderStatusCompleted {
		t.Fatalf("order identity = %+v", order)
	}
	if order.Staff.StaffID != staffID || order.Staff.Email == "" {
		t.Fatalf("staff snapshot = %+v", order.Staff)
	}
	if order.Buyer.Gender != models.POSGenderFemale || order.Buyer.Age == nil || *order.Buyer.Age != 21 {
		t.Fatalf("buyer snapshot = %+v", order.Buyer)
	}

	// Two of variant one at 40.00 plus one of product A at 100.00.
	assertOrderAmount(t, order.Subtotal, "180.00")
	assertOrderAmount(t, order.Discount, "0.00")
	assertOrderAmount(t, order.NetTotal, "180.00")
	if order.AppliedPromotion != nil {
		t.Fatalf("applied promotion = %+v", order.AppliedPromotion)
	}
	if order.Payment.Method != models.POSPaymentRealMoney || order.Payment.SlipURL != nil {
		t.Fatalf("payment = %+v", order.Payment)
	}
	assertOrderAmount(t, *order.Payment.ReceivedAmount, "200.00")
	assertOrderAmount(t, *order.Payment.ChangeAmount, "0.00")
	assertOrderAmount(t, *order.Payment.RetainedAmount, "20.00")
	if order.Payment.NoChange == nil || !*order.Payment.NoChange {
		t.Fatalf("no_change = %+v, want true", order.Payment.NoChange)
	}

	if len(order.Items) != 2 {
		t.Fatalf("order items = %+v", order.Items)
	}
	if order.Items[0].ProductID != fixture.ProductBID || order.Items[0].VariantID == nil ||
		*order.Items[0].VariantID != fixture.VariantOneID || order.Items[0].Quantity != 2 {
		t.Fatalf("first item = %+v", order.Items[0])
	}
	assertOrderAmount(t, order.Items[0].UnitPrice, "40.00")
	assertOrderAmount(t, order.Items[0].LineTotal, "80.00")
	if order.Items[1].ProductID != fixture.ProductAID || order.Items[1].Quantity != 1 {
		t.Fatalf("second item = %+v", order.Items[1])
	}

	// Stock is reduced exactly once for the sellable items that were bought.
	if got := readProductStock(t, database, fixture.ProductAID); got != 6 {
		t.Fatalf("product stock = %d, want 6", got)
	}
	if got := readVariantStock(t, database, fixture.VariantOneID); got != 1 {
		t.Fatalf("variant stock = %d, want 1", got)
	}
	if got := readVariantStock(t, database, fixture.VariantTwoID); got != 5 {
		t.Fatalf("untouched variant stock = %d, want 5", got)
	}

	for _, item := range order.Items {
		var (
			transactionType string
			change          int32
			before          int32
			after           int32
			referenceType   *string
			referenceID     *int64
		)
		if err := database.QueryRow(ctx, `
			SELECT transaction_type::text, quantity_change, quantity_before, quantity_after,
			       reference_type, reference_id
			FROM stock_transactions
			WHERE transaction_id = $1`, item.InventoryTransactionID).Scan(
			&transactionType, &change, &before, &after, &referenceType, &referenceID); err != nil {
			t.Fatalf("read ORDER stock transaction: %v", err)
		}
		if transactionType != "ORDER" || change != -item.Quantity || after != before-item.Quantity {
			t.Fatalf("stock transaction = %s %d (%d -> %d)", transactionType, change, before, after)
		}
		if referenceType == nil || *referenceType != "ORDER" || referenceID == nil || *referenceID != order.OrderID {
			t.Fatalf("stock transaction reference = %+v/%+v", referenceType, referenceID)
		}
	}

	// Later master data edits must not rewrite a paid order.
	if _, err := database.Exec(ctx, `UPDATE products SET name = 'Renamed product' WHERE id = $1`, fixture.ProductAID); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(ctx, `
		UPDATE project_products SET project_price = 999.00
		WHERE project_id = $1 AND product_id = $2`, fixture.ProjectID, fixture.ProductAID); err != nil {
		t.Fatal(err)
	}
	stored, _, err := repository.Checkout(ctx, fixture.Today, command, failingCheckoutPlanner(t))
	if err != nil {
		t.Fatal(err)
	}
	if stored.Items[1].ProductName != fixture.ProductAName {
		t.Fatalf("item name after master data edit = %q, want %q", stored.Items[1].ProductName, fixture.ProductAName)
	}
	assertOrderAmount(t, stored.Items[1].UnitPrice, "100.00")
	assertOrderAmount(t, stored.NetTotal, "180.00")
}

func TestPOSRepositoryCheckoutReplaysAnIdenticalIdempotentRetry(t *testing.T) {
	database := openPromotionIntegrationDatabase(t)
	fixture := newPromotionIntegrationFixture(t, database)
	staffID := insertCheckoutStaff(t, database)
	ctx := context.Background()
	setCheckoutStock(t, database, fixture, 7, 3, 5)

	repository := NewPOSRepository(database)
	command := cashCheckoutCommand(t, fixture, staffID, "checkout-key-2", "200.00", false)

	created, _, err := repository.Checkout(ctx, fixture.Today, command, checkoutPlanner(t, command, nil))
	if err != nil {
		t.Fatal(err)
	}

	// The retry must not reduce stock again, so its planner must never run.
	replayedOrder, replayed, err := repository.Checkout(ctx, fixture.Today, command, failingCheckoutPlanner(t))
	if err != nil {
		t.Fatal(err)
	}
	if !replayed || replayedOrder.OrderID != created.OrderID {
		t.Fatalf("replay = %v for order %d, want the original %d", replayed, replayedOrder.OrderID, created.OrderID)
	}

	if got := readVariantStock(t, database, fixture.VariantOneID); got != 1 {
		t.Fatalf("variant stock after replay = %d, want 1", got)
	}
	if got := countProjectOrders(t, database, fixture.ProjectID); got != 1 {
		t.Fatalf("order count after replay = %d, want 1", got)
	}
	if got := countOrderStockTransactions(t, database, created.OrderID); got != 2 {
		t.Fatalf("ORDER stock transactions after replay = %d, want 2", got)
	}

	different := cashCheckoutCommand(t, fixture, staffID, "checkout-key-2", "500.00", false)
	if _, _, err := repository.Checkout(ctx, fixture.Today, different, failingCheckoutPlanner(t)); !errors.Is(err, ErrIdempotencyKeyReused) {
		t.Fatalf("reused key error = %v, want ErrIdempotencyKeyReused", err)
	}
	if got := countProjectOrders(t, database, fixture.ProjectID); got != 1 {
		t.Fatalf("order count after a reused key = %d, want 1", got)
	}
}

func TestPOSRepositoryCheckoutRollsBackEverythingWhenPlanningFails(t *testing.T) {
	database := openPromotionIntegrationDatabase(t)
	fixture := newPromotionIntegrationFixture(t, database)
	staffID := insertCheckoutStaff(t, database)
	ctx := context.Background()
	setCheckoutStock(t, database, fixture, 7, 3, 5)

	repository := NewPOSRepository(database)
	command := cashCheckoutCommand(t, fixture, staffID, "checkout-key-3", "200.00", false)
	rejection := errors.New("insufficient stock")

	_, _, err := repository.Checkout(ctx, fixture.Today, command,
		func(models.POSCheckoutSnapshot) (models.POSCheckoutPlan, error) {
			return models.POSCheckoutPlan{}, rejection
		})
	if !errors.Is(err, rejection) {
		t.Fatalf("planner error = %v", err)
	}

	if got := countProjectOrders(t, database, fixture.ProjectID); got != 0 {
		t.Fatalf("order count = %d, want 0", got)
	}
	if got := readProductStock(t, database, fixture.ProductAID); got != 7 {
		t.Fatalf("product stock = %d, want the original 7", got)
	}
	if got := readVariantStock(t, database, fixture.VariantOneID); got != 3 {
		t.Fatalf("variant stock = %d, want the original 3", got)
	}

	var stockTransactions int
	if err := database.QueryRow(ctx, `
		SELECT count(*) FROM stock_transactions
		WHERE product_id = $1 OR variant_id = $2`, fixture.ProductAID, fixture.VariantOneID).Scan(&stockTransactions); err != nil {
		t.Fatal(err)
	}
	if stockTransactions != 0 {
		t.Fatalf("stock transactions after a rejected checkout = %d, want 0", stockTransactions)
	}

	// The Idempotency-Key of a failed checkout stays reusable.
	order, _, err := repository.Checkout(ctx, fixture.Today, command, checkoutPlanner(t, command, nil))
	if err != nil {
		t.Fatal(err)
	}
	if order.OrderID == 0 {
		t.Fatal("retry after a rejected checkout did not create an order")
	}
}

func TestPOSRepositoryCheckoutRejectsUnsellableItemsAndUnknownProjects(t *testing.T) {
	database := openPromotionIntegrationDatabase(t)
	fixture := newPromotionIntegrationFixture(t, database)
	staffID := insertCheckoutStaff(t, database)
	ctx := context.Background()
	repository := NewPOSRepository(database)

	unsellable := cashCheckoutCommand(t, fixture, staffID, "checkout-key-4", "200.00", false)
	unsellable.Items = []models.POSCartItemRequest{{ProductID: fixture.OtherProductID, Quantity: 1}}
	if _, _, err := repository.Checkout(ctx, fixture.Today, unsellable, failingCheckoutPlanner(t)); !errors.Is(err, ErrProductNotSellable) {
		t.Fatalf("unsellable item error = %v, want ErrProductNotSellable", err)
	}

	missingProject := cashCheckoutCommand(t, fixture, staffID, "checkout-key-5", "200.00", false)
	missingProject.ProjectID = 9223372036854770000
	if _, _, err := repository.Checkout(ctx, fixture.Today, missingProject, failingCheckoutPlanner(t)); !errors.Is(err, ErrProjectNotFound) {
		t.Fatalf("missing project error = %v, want ErrProjectNotFound", err)
	}

	missingStaff := cashCheckoutCommand(t, fixture, 9223372036854770000, "checkout-key-6", "200.00", false)
	if _, _, err := repository.Checkout(ctx, fixture.Today, missingStaff, failingCheckoutPlanner(t)); !errors.Is(err, ErrPOSStaffNotFound) {
		t.Fatalf("missing staff error = %v, want ErrPOSStaffNotFound", err)
	}

	// A duplicate identity is named here rather than left to trip
	// uq_order_items_identity halfway through the write.
	duplicate := cashCheckoutCommand(t, fixture, staffID, "checkout-key-11", "200.00", false)
	duplicate.Items = append(duplicate.Items, duplicate.Items[0])
	if _, _, err := repository.Checkout(ctx, fixture.Today, duplicate, failingCheckoutPlanner(t)); !errors.Is(err, ErrPOSDuplicateCartItem) {
		t.Fatalf("duplicate cart item error = %v, want ErrPOSDuplicateCartItem", err)
	}
	if got := countProjectOrders(t, database, fixture.ProjectID); got != 0 {
		t.Fatalf("order count after a duplicate Cart = %d, want 0", got)
	}
}

func TestPOSRepositoryCheckoutAcceptsOnlyATrustedUnusedSlip(t *testing.T) {
	database := openPromotionIntegrationDatabase(t)
	fixture := newPromotionIntegrationFixture(t, database)
	staffID := insertCheckoutStaff(t, database)
	ctx := context.Background()
	setCheckoutStock(t, database, fixture, 7, 3, 5)
	repository := NewPOSRepository(database)

	slip := models.PaymentSlip{
		ObjectKey:   fmt.Sprintf("payment-slips/%d-slip.png", time.Now().UnixNano()),
		URL:         "https://storage.googleapis.com/bucket/slip.png",
		ContentType: "image/png",
		Size:        2048,
	}
	if err := repository.RecordPaymentSlip(ctx, slip, staffID); err != nil {
		t.Fatal(err)
	}
	forged := qrCheckoutCommand(t, fixture, staffID, "checkout-key-7", "payment-slips/forged.png")
	if _, _, err := repository.Checkout(ctx, fixture.Today, forged, failingCheckoutPlanner(t)); !errors.Is(err, ErrPaymentSlipNotTrusted) {
		t.Fatalf("forged slip error = %v, want ErrPaymentSlipNotTrusted", err)
	}

	first := qrCheckoutCommand(t, fixture, staffID, "checkout-key-8", slip.ObjectKey)
	order, _, err := repository.Checkout(ctx, fixture.Today, first, checkoutPlanner(t, first, nil))
	if err != nil {
		t.Fatal(err)
	}
	if order.Payment.Method != models.POSPaymentQRCode || order.Payment.SlipURL == nil || *order.Payment.SlipURL != slip.URL {
		t.Fatalf("QR payment snapshot = %+v", order.Payment)
	}
	if order.Payment.ReceivedAmount != nil || order.Payment.NoChange != nil {
		t.Fatalf("QR payment carries cash fields = %+v", order.Payment)
	}

	reused := qrCheckoutCommand(t, fixture, staffID, "checkout-key-9", slip.ObjectKey)
	if _, _, err := repository.Checkout(ctx, fixture.Today, reused, failingCheckoutPlanner(t)); !errors.Is(err, ErrPaymentSlipNotTrusted) {
		t.Fatalf("reused slip error = %v, want ErrPaymentSlipNotTrusted", err)
	}
	if got := countProjectOrders(t, database, fixture.ProjectID); got != 1 {
		t.Fatalf("order count after a reused slip = %d, want 1", got)
	}
}

func TestPOSRepositoryCheckoutSnapshotsTheAppliedPromotion(t *testing.T) {
	database := openPromotionIntegrationDatabase(t)
	fixture := newPromotionIntegrationFixture(t, database)
	staffID := insertCheckoutStaff(t, database)
	ctx := context.Background()
	setCheckoutStock(t, database, fixture, 7, 3, 5)

	promotionRepository := NewPromotionRepository(database)
	promotion, err := promotionRepository.Create(ctx, fixture.Today, fixture.ProjectID, models.ProjectPromotionMutation{
		Name:           "Checkout bundle",
		PromotionPrice: mustPromotionAmount(t, "150.00"),
		Items: []models.ProjectPromotionItemInput{
			{ProductID: fixture.ProductAID, Quantity: 1},
			{ProductID: fixture.ProductBID, VariantID: &fixture.VariantOneID, Quantity: 2},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	repository := NewPOSRepository(database)
	command := cashCheckoutCommand(t, fixture, staffID, "checkout-key-10", "150.00", true)

	applied := &models.AppliedProjectPromotion{
		PromotionID:         promotion.PromotionID,
		Name:                promotion.Name,
		OriginalBundlePrice: mustPromotionAmount(t, "180.00"),
		PromotionPrice:      mustPromotionAmount(t, "150.00"),
		Discount:            mustPromotionAmount(t, "30.00"),
	}
	plan := checkoutPlanner(t, command, applied)

	order, _, err := repository.Checkout(ctx, fixture.Today, command,
		func(snapshot models.POSCheckoutSnapshot) (models.POSCheckoutPlan, error) {
			if len(snapshot.Promotions) != 1 || snapshot.Promotions[0].PromotionID != promotion.PromotionID {
				return models.POSCheckoutPlan{}, fmt.Errorf("snapshot promotions = %+v", snapshot.Promotions)
			}
			return plan(snapshot)
		})
	if err != nil {
		t.Fatal(err)
	}

	if order.AppliedPromotion == nil || order.AppliedPromotion.PromotionID != promotion.PromotionID {
		t.Fatalf("applied promotion = %+v", order.AppliedPromotion)
	}
	assertOrderAmount(t, order.AppliedPromotion.Discount, "30.00")
	assertOrderAmount(t, order.NetTotal, "150.00")

	// Deleting the promotion afterwards must leave the paid order untouched.
	if err := promotionRepository.Delete(ctx, fixture.Today, fixture.ProjectID, promotion.PromotionID); err != nil {
		t.Fatal(err)
	}
	stored, _, err := repository.Checkout(ctx, fixture.Today, command, failingCheckoutPlanner(t))
	if err != nil {
		t.Fatal(err)
	}
	if stored.AppliedPromotion == nil || stored.AppliedPromotion.Name != promotion.Name {
		t.Fatalf("promotion snapshot after deletion = %+v", stored.AppliedPromotion)
	}
}

// checkoutPlanner prices the snapshot the way the POS use case does: line
// totals from the snapshot, then the promotion discount, then the payment
// settled against the resulting net total.
func checkoutPlanner(t *testing.T, command models.POSCheckoutCommand, promotion *models.AppliedProjectPromotion) func(models.POSCheckoutSnapshot) (models.POSCheckoutPlan, error) {
	t.Helper()
	return func(snapshot models.POSCheckoutSnapshot) (models.POSCheckoutPlan, error) {
		plan := models.POSCheckoutPlan{LineTotals: make([]models.THBAmount, len(snapshot.Items))}
		for index, item := range snapshot.Items {
			lineTotal, err := item.Item.ProjectPrice.Mul(int64(item.Quantity))
			if err != nil {
				return models.POSCheckoutPlan{}, err
			}
			plan.LineTotals[index] = lineTotal
			if plan.Subtotal, err = plan.Subtotal.Add(lineTotal); err != nil {
				return models.POSCheckoutPlan{}, err
			}
		}

		plan.NetTotal = plan.Subtotal
		if promotion != nil {
			netTotal, err := plan.Subtotal.Sub(promotion.Discount)
			if err != nil {
				return models.POSCheckoutPlan{}, err
			}
			plan.AppliedPromotion = promotion
			plan.Discount = promotion.Discount
			plan.NetTotal = netTotal
		}

		payment, err := settleCheckoutPayment(command.Payment, snapshot.Slip, plan.NetTotal)
		if err != nil {
			return models.POSCheckoutPlan{}, err
		}
		plan.Payment = payment

		return plan, nil
	}
}

func settleCheckoutPayment(request models.POSPaymentRequest, slip *models.PaymentSlip, netTotal models.THBAmount) (models.POSPaymentSnapshot, error) {
	if request.Method == models.POSPaymentQRCode {
		if slip == nil || request.SlipObjectKey == nil || slip.ObjectKey != *request.SlipObjectKey {
			return models.POSPaymentSnapshot{}, fmt.Errorf("QR checkout resolved slip %+v", slip)
		}
		slipURL := slip.URL
		return models.POSPaymentSnapshot{Method: models.POSPaymentQRCode, SlipURL: &slipURL}, nil
	}

	received := *request.ReceivedAmount
	excess, err := received.Sub(netTotal)
	if err != nil {
		return models.POSPaymentSnapshot{}, err
	}
	change := excess
	retained := models.THBAmount{}
	noChange := *request.NoChange
	if noChange {
		change = models.THBAmount{}
		retained = excess
	}

	return models.POSPaymentSnapshot{
		Method:         models.POSPaymentRealMoney,
		ReceivedAmount: &received,
		ChangeAmount:   &change,
		RetainedAmount: &retained,
		NoChange:       &noChange,
		Note:           request.Note,
	}, nil
}

// failingCheckoutPlanner fails the test when persistence prices a checkout it
// should have replayed or rejected before planning.
func failingCheckoutPlanner(t *testing.T) func(models.POSCheckoutSnapshot) (models.POSCheckoutPlan, error) {
	t.Helper()
	return func(models.POSCheckoutSnapshot) (models.POSCheckoutPlan, error) {
		t.Error("checkout planning ran when it should not have")
		return models.POSCheckoutPlan{}, errors.New("planner must not run")
	}
}

func cashCheckoutCommand(t *testing.T, fixture promotionIntegrationFixture, staffID int64, key, received string, noChange bool) models.POSCheckoutCommand {
	t.Helper()
	amount := mustPromotionAmount(t, received)
	age := int32(21)

	return models.POSCheckoutCommand{
		ProjectID:          fixture.ProjectID,
		StaffUserID:        staffID,
		IdempotencyKey:     key,
		RequestFingerprint: "fingerprint-" + key + "-" + received,
		Buyer:              models.POSBuyer{Gender: models.POSGenderFemale, Age: &age},
		Items: []models.POSCartItemRequest{
			{ProductID: fixture.ProductBID, VariantID: &fixture.VariantOneID, Quantity: 2},
			{ProductID: fixture.ProductAID, Quantity: 1},
		},
		Payment: models.POSPaymentRequest{
			Method:         models.POSPaymentRealMoney,
			ReceivedAmount: &amount,
			NoChange:       &noChange,
		},
	}
}

func qrCheckoutCommand(t *testing.T, fixture promotionIntegrationFixture, staffID int64, key, slipObjectKey string) models.POSCheckoutCommand {
	t.Helper()
	command := models.POSCheckoutCommand{
		ProjectID:          fixture.ProjectID,
		StaffUserID:        staffID,
		IdempotencyKey:     key,
		RequestFingerprint: "fingerprint-" + key,
		Buyer:              models.POSBuyer{Gender: models.POSGenderPreferNotToSay},
		Items: []models.POSCartItemRequest{
			{ProductID: fixture.ProductBID, VariantID: &fixture.VariantOneID, Quantity: 2},
			{ProductID: fixture.ProductAID, Quantity: 1},
		},
		Payment: models.POSPaymentRequest{
			Method:        models.POSPaymentQRCode,
			SlipObjectKey: &slipObjectKey,
		},
	}

	return command
}

func insertCheckoutStaff(t *testing.T, database *pgxpool.Pool) int64 {
	t.Helper()
	email := fmt.Sprintf("pos-staff-%d@example.com", time.Now().UnixNano())

	var staffID int64
	if err := database.QueryRow(context.Background(), `
		INSERT INTO users (full_name, email, role)
		VALUES ('POS staff', $1, 'ADMIN')
		RETURNING user_id`, email).Scan(&staffID); err != nil {
		t.Fatalf("insert checkout staff: %v", err)
	}

	t.Cleanup(func() {
		ctx := context.Background()
		if _, err := database.Exec(ctx, `DELETE FROM orders WHERE staff_user_id = $1`, staffID); err != nil {
			t.Errorf("cleanup staff orders: %v", err)
		}
		if _, err := database.Exec(ctx, `DELETE FROM payment_slips WHERE uploaded_by = $1`, staffID); err != nil {
			t.Errorf("cleanup staff payment slips: %v", err)
		}
		if _, err := database.Exec(ctx, `DELETE FROM users WHERE user_id = $1`, staffID); err != nil {
			t.Errorf("cleanup checkout staff: %v", err)
		}
	})

	return staffID
}

func setCheckoutStock(t *testing.T, database *pgxpool.Pool, fixture promotionIntegrationFixture, product, variantOne, variantTwo int32) {
	t.Helper()
	ctx := context.Background()
	if _, err := database.Exec(ctx, `UPDATE products SET stock_quantity = $2 WHERE id = $1`, fixture.ProductAID, product); err != nil {
		t.Fatalf("set product stock: %v", err)
	}
	if _, err := database.Exec(ctx, `
		UPDATE variants SET stock_quantity = CASE variant_id
			WHEN $1 THEN $3::integer
			WHEN $2 THEN $4::integer
		END
		WHERE variant_id IN ($1, $2)`, fixture.VariantOneID, fixture.VariantTwoID, variantOne, variantTwo); err != nil {
		t.Fatalf("set variant stock: %v", err)
	}
}

func readProductStock(t *testing.T, database *pgxpool.Pool, productID int64) int32 {
	t.Helper()
	var stock int32
	if err := database.QueryRow(context.Background(),
		`SELECT COALESCE(stock_quantity, 0) FROM products WHERE id = $1`, productID).Scan(&stock); err != nil {
		t.Fatalf("read product stock: %v", err)
	}
	return stock
}

func readVariantStock(t *testing.T, database *pgxpool.Pool, variantID int64) int32 {
	t.Helper()
	var stock int32
	if err := database.QueryRow(context.Background(),
		`SELECT COALESCE(stock_quantity, 0) FROM variants WHERE variant_id = $1`, variantID).Scan(&stock); err != nil {
		t.Fatalf("read variant stock: %v", err)
	}
	return stock
}

func countProjectOrders(t *testing.T, database *pgxpool.Pool, projectID int64) int {
	t.Helper()
	var count int
	if err := database.QueryRow(context.Background(),
		`SELECT count(*) FROM orders WHERE project_id = $1`, projectID).Scan(&count); err != nil {
		t.Fatalf("count project orders: %v", err)
	}
	return count
}

func countOrderStockTransactions(t *testing.T, database *pgxpool.Pool, orderID int64) int {
	t.Helper()
	var count int
	if err := database.QueryRow(context.Background(),
		`SELECT count(*) FROM stock_transactions WHERE reference_type = 'ORDER' AND reference_id = $1`,
		orderID).Scan(&count); err != nil {
		t.Fatalf("count ORDER stock transactions: %v", err)
	}
	return count
}

func assertOrderAmount(t *testing.T, amount models.THBAmount, want string) {
	t.Helper()
	if amount.String() != want {
		t.Fatalf("amount = %s, want %s", amount, want)
	}
}

// A racing duplicate passes the pre-transaction replay check, then blocks on
// the stock locks and resumes only after the winner commits. Both fixtures
// below make the winner's committed effects fatal to a replan, so an attempt
// that reaches the planner fails the test instead of silently succeeding on
// surplus stock.
func TestPOSRepositoryCheckoutReplaysARacingIdempotentRetry(t *testing.T) {
	type result struct {
		order    models.POSOrder
		replayed bool
		err      error
	}

	race := func(t *testing.T, repository *POSRepository, today models.Date, command models.POSCheckoutCommand,
		planner func(models.POSCheckoutSnapshot) (models.POSCheckoutPlan, error)) (result, result) {
		t.Helper()
		results := make(chan result, 2)
		start := make(chan struct{})
		for attempt := 0; attempt < 2; attempt++ {
			go func() {
				<-start
				order, replayed, err := repository.Checkout(context.Background(), today, command, planner)
				results <- result{order: order, replayed: replayed, err: err}
			}()
		}
		close(start)

		return <-results, <-results
	}

	assertOneOrder := func(t *testing.T, first, second result) {
		t.Helper()
		if first.err != nil || second.err != nil {
			t.Fatalf("concurrent retry errors = %v and %v", first.err, second.err)
		}
		if first.order.OrderID != second.order.OrderID {
			t.Fatalf("the retry returned order %d, want the original %d", second.order.OrderID, first.order.OrderID)
		}
		if first.replayed == second.replayed {
			t.Fatalf("exactly one attempt should have created the order; replayed = %v and %v",
				first.replayed, second.replayed)
		}
	}

	// Stock for exactly one cart. A replan would see the winner's decremented
	// stock and reject the retry instead of replaying its order.
	t.Run("cash, with only one cart's worth of stock", func(t *testing.T) {
		database := openPromotionIntegrationDatabase(t)
		fixture := newPromotionIntegrationFixture(t, database)
		staffID := insertCheckoutStaff(t, database)
		setCheckoutStock(t, database, fixture, 1, 2, 5)

		repository := NewPOSRepository(database)
		command := cashCheckoutCommand(t, fixture, staffID, "racing-key-cash", "200.00", false)

		first, second := race(t, repository, fixture.Today, command,
			stockCheckingCheckoutPlanner(t, command, errors.New("insufficient stock")))
		assertOneOrder(t, first, second)

		if got := countProjectOrders(t, database, fixture.ProjectID); got != 1 {
			t.Fatalf("order count = %d, want 1", got)
		}
		if got := readProductStock(t, database, fixture.ProductAID); got != 0 {
			t.Fatalf("product stock = %d, want 0", got)
		}
		if got := readVariantStock(t, database, fixture.VariantOneID); got != 0 {
			t.Fatalf("variant stock = %d, want 0", got)
		}
		if got := countOrderStockTransactions(t, database, first.order.OrderID); got != 2 {
			t.Fatalf("ORDER stock transactions = %d, want 2", got)
		}
	})

	// A slip backs at most one order, so a replan would reject the retry with
	// ErrPaymentSlipNotTrusted for an order that already exists.
	t.Run("QR, where the slip is spent by the winner", func(t *testing.T) {
		database := openPromotionIntegrationDatabase(t)
		fixture := newPromotionIntegrationFixture(t, database)
		staffID := insertCheckoutStaff(t, database)
		setCheckoutStock(t, database, fixture, 7, 3, 5)

		repository := NewPOSRepository(database)
		slip := models.PaymentSlip{
			ObjectKey:   fmt.Sprintf("payment-slips/%d-racing.png", time.Now().UnixNano()),
			URL:         "https://storage.googleapis.com/bucket/racing.png",
			ContentType: "image/png",
			Size:        2048,
		}
		if err := repository.RecordPaymentSlip(context.Background(), slip, staffID); err != nil {
			t.Fatal(err)
		}
		command := qrCheckoutCommand(t, fixture, staffID, "racing-key-qr", slip.ObjectKey)

		first, second := race(t, repository, fixture.Today, command, checkoutPlanner(t, command, nil))
		assertOneOrder(t, first, second)

		if got := countProjectOrders(t, database, fixture.ProjectID); got != 1 {
			t.Fatalf("order count = %d, want 1", got)
		}
		if first.order.Payment.SlipURL == nil || *first.order.Payment.SlipURL != slip.URL {
			t.Fatalf("QR payment snapshot = %+v", first.order.Payment)
		}
		if got := readVariantStock(t, database, fixture.VariantOneID); got != 1 {
			t.Fatalf("variant stock = %d, want 1", got)
		}
	})
}

// stockCheckingCheckoutPlanner prices the snapshot only when it still covers
// the Cart, the way the POS use case does.
func stockCheckingCheckoutPlanner(t *testing.T, command models.POSCheckoutCommand,
	soldOut error) func(models.POSCheckoutSnapshot) (models.POSCheckoutPlan, error) {
	t.Helper()
	price := checkoutPlanner(t, command, nil)

	return func(snapshot models.POSCheckoutSnapshot) (models.POSCheckoutPlan, error) {
		for _, item := range snapshot.Items {
			if item.Quantity > item.Item.StockQuantity {
				return models.POSCheckoutPlan{}, soldOut
			}
		}
		return price(snapshot)
	}
}

func TestPOSRepositoryCheckoutSerializesConcurrentRequests(t *testing.T) {
	t.Run("the same idempotency key creates one order", func(t *testing.T) {
		database := openPromotionIntegrationDatabase(t)
		fixture := newPromotionIntegrationFixture(t, database)
		staffID := insertCheckoutStaff(t, database)
		setCheckoutStock(t, database, fixture, 7, 3, 5)

		repository := NewPOSRepository(database)
		command := cashCheckoutCommand(t, fixture, staffID, "concurrent-key-1", "200.00", false)

		type result struct {
			order    models.POSOrder
			replayed bool
			err      error
		}
		results := make(chan result, 2)
		start := make(chan struct{})
		for attempt := 0; attempt < 2; attempt++ {
			go func() {
				<-start
				order, replayed, err := repository.Checkout(context.Background(), fixture.Today, command,
					checkoutPlanner(t, command, nil))
				results <- result{order: order, replayed: replayed, err: err}
			}()
		}
		close(start)

		first, second := <-results, <-results
		if first.err != nil || second.err != nil {
			t.Fatalf("concurrent checkout errors = %v and %v", first.err, second.err)
		}
		if first.order.OrderID != second.order.OrderID {
			t.Fatalf("concurrent checkouts created orders %d and %d", first.order.OrderID, second.order.OrderID)
		}
		if first.replayed == second.replayed {
			t.Fatalf("exactly one attempt should have created the order; replayed = %v and %v", first.replayed, second.replayed)
		}
		if got := countProjectOrders(t, database, fixture.ProjectID); got != 1 {
			t.Fatalf("order count = %d, want 1", got)
		}
		// Stock is reduced once even though both requests ran together.
		if got := readVariantStock(t, database, fixture.VariantOneID); got != 1 {
			t.Fatalf("variant stock = %d, want 1", got)
		}
		if got := countOrderStockTransactions(t, database, first.order.OrderID); got != 2 {
			t.Fatalf("ORDER stock transactions = %d, want 2", got)
		}
	})

	t.Run("the last unit is sold once", func(t *testing.T) {
		database := openPromotionIntegrationDatabase(t)
		fixture := newPromotionIntegrationFixture(t, database)
		staffID := insertCheckoutStaff(t, database)
		// Exactly one cart's worth of stock for two competing checkouts.
		setCheckoutStock(t, database, fixture, 1, 2, 5)

		repository := NewPOSRepository(database)
		soldOut := errors.New("insufficient stock")

		errs := make(chan error, 2)
		start := make(chan struct{})
		for attempt := 0; attempt < 2; attempt++ {
			key := fmt.Sprintf("concurrent-key-stock-%d", attempt)
			command := cashCheckoutCommand(t, fixture, staffID, key, "200.00", false)
			go func() {
				<-start
				_, _, err := repository.Checkout(context.Background(), fixture.Today, command,
					stockCheckingCheckoutPlanner(t, command, soldOut))
				errs <- err
			}()
		}
		close(start)

		first, second := <-errs, <-errs
		sold, rejected := first, second
		if sold != nil {
			sold, rejected = second, first
		}
		if sold != nil {
			t.Fatalf("both concurrent checkouts failed: %v and %v", first, second)
		}
		if !errors.Is(rejected, soldOut) {
			t.Fatalf("second checkout error = %v, want the stock rejection", rejected)
		}

		if got := countProjectOrders(t, database, fixture.ProjectID); got != 1 {
			t.Fatalf("order count = %d, want 1", got)
		}
		if got := readProductStock(t, database, fixture.ProductAID); got != 0 {
			t.Fatalf("product stock = %d, want 0", got)
		}
		if got := readVariantStock(t, database, fixture.VariantOneID); got != 0 {
			t.Fatalf("variant stock = %d, want 0", got)
		}
	})
}
