//go:build integration

package repositories

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/esc-chula/intania-shop-api/internal/models"
	"github.com/esc-chula/intania-shop-api/internal/usecases"
	"github.com/jackc/pgx/v5/pgxpool"
)

type orderIntegrationFixture struct {
	ProjectID      int64
	OtherProjectID int64
	StaffID        int64
	OrderOneID     int64
	OrderTwoID     int64
}

func TestPOSOperatorNameSnapshotAcrossCheckoutHistoryAndRetry(t *testing.T) {
	database := openPromotionIntegrationDatabase(t)
	fixture := newPromotionIntegrationFixture(t, database)
	staffID := insertCheckoutStaff(t, database)
	setCheckoutStock(t, database, fixture, 30, 30, 30)
	ctx := context.Background()
	checkout := NewPOSRepository(database)
	service := usecases.NewPOSService(checkout)
	history := NewOrderRepository(database)
	cases := []struct {
		name   string
		input  *string
		stored *string
		qr     bool
	}{
		{name: "omitted"},
		{name: "empty", input: strPtr("")},
		{name: "blank", input: strPtr(" \t ")},
		{name: "explicit cash", input: strPtr("  มายด์  "), stored: strPtr("มายด์")},
		{name: "explicit QR", input: strPtr("กิตติ"), stored: strPtr("กิตติ"), qr: true},
	}
	type attempt struct {
		key     string
		request models.POSCheckoutRequest
		order   models.POSOrder
	}
	attempts := make([]attempt, 0, len(cases))
	for index, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			command := cashCheckoutCommand(t, fixture, staffID, "unused", "200.00", false)
			if test.qr {
				slip := models.PaymentSlip{
					ObjectKey: fmt.Sprintf("payment-slips/operator-%d.png", staffID),
					URL:       "https://example.com/operator-slip.png", ContentType: "image/png", Size: 2048,
				}
				if err := checkout.RecordPaymentSlip(ctx, slip, staffID); err != nil {
					t.Fatal(err)
				}
				command = qrCheckoutCommand(t, fixture, staffID, "unused", slip.ObjectKey)
			}
			request := models.POSCheckoutRequest{
				Buyer: command.Buyer, Items: command.Items, Payment: command.Payment, OperatorName: test.input,
			}
			key := fmt.Sprintf("operator-%d-%d", staffID, index)
			order, replayed, err := service.Checkout(ctx, fixture.ProjectID, staffID, key, request)
			if err != nil || replayed {
				t.Fatalf("create = %+v, replayed = %t, error = %v", order, replayed, err)
			}
			want := "POS staff"
			if test.stored != nil {
				want = *test.stored
			}
			if order.OperatorName != want || order.Staff.StaffID != staffID || order.Staff.FullName != "POS staff" {
				t.Fatalf("operator/staff snapshot = %+v", order)
			}
			var raw *string
			if err := database.QueryRow(ctx, `SELECT operator_name FROM orders WHERE order_id = $1`, order.OrderID).Scan(&raw); err != nil {
				t.Fatal(err)
			}
			if (raw == nil) != (test.stored == nil) || (raw != nil && *raw != *test.stored) {
				t.Fatalf("stored name = %v, want %v", raw, test.stored)
			}
			for _, item := range order.Items {
				var actor string
				if err := database.QueryRow(ctx, `SELECT created_by FROM stock_transactions WHERE transaction_id = $1`, item.InventoryTransactionID).Scan(&actor); err != nil {
					t.Fatal(err)
				}
				if actor != fmt.Sprint(staffID) {
					t.Fatalf("stock actor = %q, want authenticated user %d", actor, staffID)
				}
			}
			attempts = append(attempts, attempt{key, request, order})
		})
	}
	if len(attempts) != len(cases) {
		t.Fatal("checkout setup failed")
	}
	// Fallback and retries must use the order's snapshot, even after a rename.
	if _, err := database.Exec(ctx, `UPDATE users SET full_name = 'Renamed account' WHERE user_id = $1`, staffID); err != nil {
		t.Fatal(err)
	}
	for _, attempt := range attempts {
		order, replayed, err := service.Checkout(ctx, fixture.ProjectID, staffID, attempt.key, attempt.request)
		if err != nil || !replayed || order.OrderID != attempt.order.OrderID || order.OperatorName != attempt.order.OperatorName || order.Staff != attempt.order.Staff {
			t.Fatalf("retry = %+v, replayed = %t, error = %v", order, replayed, err)
		}
		detail, err := history.Detail(ctx, fixture.ProjectID, order.OrderID)
		if err != nil || detail.OperatorName != order.OperatorName || detail.Staff != order.Staff {
			t.Fatalf("detail = %+v, error = %v", detail, err)
		}
		changed := attempt.request
		changed.OperatorName = strPtr("คนอื่น")
		if _, _, err := service.Checkout(ctx, fixture.ProjectID, staffID, attempt.key, changed); !errors.Is(err, ErrIdempotencyKeyReused) {
			t.Fatalf("retry with changed operator = %v, want idempotency conflict", err)
		}
	}
	filter := models.OrderFilter{
		StaffID: staffID, SortBy: models.OrderSortByOrderNumber, SortOrder: models.OrderSortAsc,
		CreatedFrom: time.Now().Add(-time.Hour), CreatedTo: time.Now().Add(time.Hour),
	}
	listed, total, err := history.List(ctx, fixture.ProjectID, filter, 0, 10)
	if err != nil || total != int64(len(attempts)) || len(listed) != len(attempts) {
		t.Fatalf("list = %+v, total = %d, error = %v", listed, total, err)
	}
	exported, err := history.Export(ctx, fixture.ProjectID, filter)
	if err != nil || len(exported) != len(attempts) {
		t.Fatalf("export = %+v, error = %v", exported, err)
	}
	for index, attempt := range attempts {
		for _, got := range []models.POSOrder{listed[index], exported[index]} {
			if got.OrderID != attempt.order.OrderID || got.OperatorName != attempt.order.OperatorName || got.Staff != attempt.order.Staff {
				t.Fatalf("history/export snapshot = %+v, want %+v", got, attempt.order)
			}
		}
	}
	if got := countProjectOrders(t, database, fixture.ProjectID); got != len(attempts) {
		t.Fatalf("order count = %d, want %d", got, len(attempts))
	}
	if got := readProductStock(t, database, fixture.ProductAID); got != 30-int32(len(attempts)) {
		t.Fatalf("product stock = %d after retries", got)
	}
	if got := readVariantStock(t, database, fixture.VariantOneID); got != 30-2*int32(len(attempts)) {
		t.Fatalf("variant stock = %d after retries", got)
	}
}

func TestOrderRepositoryListFiltersSortsAndPaginates(t *testing.T) {
	database := openPromotionIntegrationDatabase(t)
	fixture := newOrderIntegrationFixture(t, database)
	repository := NewOrderRepository(database)

	orders, total, err := repository.List(context.Background(), fixture.ProjectID, models.OrderFilter{
		SortBy: models.OrderSortByOrderNumber, SortOrder: models.OrderSortAsc,
		CreatedFrom: time.Now().Add(-24 * time.Hour), CreatedTo: time.Now().Add(24 * time.Hour),
	}, 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(orders) != 2 {
		t.Fatalf("orders = %d, total = %d, want 2/2", len(orders), total)
	}
	if orders[0].OrderID != fixture.OrderOneID || orders[1].OrderID != fixture.OrderTwoID {
		t.Fatalf("orders not sorted by order_number asc: %+v", orders)
	}
	if len(orders[0].Items) != 2 {
		t.Fatalf("order one items = %d, want 2", len(orders[0].Items))
	}
	if orders[1].AppliedPromotion == nil || orders[1].AppliedPromotion.Name != "Bundle" {
		t.Fatalf("order two promotion snapshot = %+v", orders[1].AppliedPromotion)
	}

	filtered, filteredTotal, err := repository.List(context.Background(), fixture.ProjectID, models.OrderFilter{
		PaymentMethod: models.POSPaymentQRCode,
		SortBy:        models.OrderSortByOrderNumber, SortOrder: models.OrderSortAsc,
		CreatedFrom: time.Now().Add(-24 * time.Hour), CreatedTo: time.Now().Add(24 * time.Hour),
	}, 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if filteredTotal != 1 || len(filtered) != 1 || filtered[0].OrderID != fixture.OrderOneID {
		t.Fatalf("payment method filter = %+v (total %d)", filtered, filteredTotal)
	}

	page, pageTotal, err := repository.List(context.Background(), fixture.ProjectID, models.OrderFilter{
		SortBy: models.OrderSortByOrderNumber, SortOrder: models.OrderSortAsc,
		CreatedFrom: time.Now().Add(-24 * time.Hour), CreatedTo: time.Now().Add(24 * time.Hour),
	}, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if pageTotal != 2 || len(page) != 1 || page[0].OrderID != fixture.OrderTwoID {
		t.Fatalf("page 2 = %+v (total %d)", page, pageTotal)
	}
}

func TestOrderRepositoryDetailIsScopedToProject(t *testing.T) {
	database := openPromotionIntegrationDatabase(t)
	fixture := newOrderIntegrationFixture(t, database)
	repository := NewOrderRepository(database)

	order, err := repository.Detail(context.Background(), fixture.ProjectID, fixture.OrderOneID)
	if err != nil {
		t.Fatal(err)
	}
	if order.OrderID != fixture.OrderOneID || len(order.Items) != 2 {
		t.Fatalf("order detail = %+v", order)
	}
	if order.Staff.StaffID != fixture.StaffID {
		t.Fatalf("staff snapshot = %+v", order.Staff)
	}

	if _, err := repository.Detail(context.Background(), fixture.OtherProjectID, fixture.OrderOneID); !errors.Is(err, ErrOrderNotFound) {
		t.Fatalf("cross-project detail err = %v, want ErrOrderNotFound", err)
	}
	if _, err := repository.Detail(context.Background(), fixture.ProjectID, 9223372036854770000); !errors.Is(err, ErrOrderNotFound) {
		t.Fatalf("missing order err = %v, want ErrOrderNotFound", err)
	}
}

func TestOrderRepositoryExportReturnsAllItemsUnpaginated(t *testing.T) {
	database := openPromotionIntegrationDatabase(t)
	fixture := newOrderIntegrationFixture(t, database)
	repository := NewOrderRepository(database)

	orders, err := repository.Export(context.Background(), fixture.ProjectID, models.OrderFilter{
		SortBy: models.OrderSortByOrderNumber, SortOrder: models.OrderSortAsc,
		CreatedFrom: time.Now().Add(-24 * time.Hour), CreatedTo: time.Now().Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(orders) != 2 {
		t.Fatalf("exported orders = %d, want 2", len(orders))
	}
	totalItems := 0
	for _, order := range orders {
		totalItems += len(order.Items)
	}
	if totalItems != 4 {
		t.Fatalf("exported items = %d, want 4", totalItems)
	}
}

func TestProjectRepositoryUpdateRejectsSaleDatesExcludingExistingOrder(t *testing.T) {
	database := openPromotionIntegrationDatabase(t)
	fixture := newOrderIntegrationFixture(t, database)
	ctx := context.Background()
	repository := NewProjectRepository(database)

	var createdAt time.Time
	if err := database.QueryRow(ctx, `SELECT created_at FROM orders WHERE order_id = $1`, fixture.OrderOneID).Scan(&createdAt); err != nil {
		t.Fatal(err)
	}
	orderDate := models.TodayInBangkok(createdAt)

	narrowedStart := models.NewDate(orderDate.Time.AddDate(0, 0, 1))
	narrowedEnd := models.NewDate(orderDate.Time.AddDate(0, 0, 2))
	_, err := repository.Update(ctx, orderDate, fixture.ProjectID, models.ProjectInput{
		Name: strPtr("Narrowed"), StartDate: &narrowedStart, EndDate: &narrowedEnd,
	})
	if !errors.Is(err, ErrProjectSaleDatesConflict) {
		t.Fatalf("err = %v, want ErrProjectSaleDatesConflict", err)
	}

	widerStart := models.NewDate(orderDate.Time.AddDate(0, 0, -1))
	widerEnd := models.NewDate(orderDate.Time.AddDate(0, 0, 1))
	updated, err := repository.Update(ctx, orderDate, fixture.ProjectID, models.ProjectInput{
		Name: strPtr("Widened"), StartDate: &widerStart, EndDate: &widerEnd,
	})
	if err != nil {
		t.Fatalf("update with compatible dates: %v", err)
	}
	if updated.Name != "Widened" {
		t.Fatalf("updated project name = %q", updated.Name)
	}
}

func strPtr(value string) *string { return &value }

func newOrderIntegrationFixture(t *testing.T, database *pgxpool.Pool) orderIntegrationFixture {
	t.Helper()
	base := newPromotionIntegrationFixture(t, database)
	staffID := insertCheckoutStaff(t, database)
	setCheckoutStock(t, database, base, 10, 10, 10)

	checkout := NewPOSRepository(database)
	slip := models.PaymentSlip{
		ObjectKey:   fmt.Sprintf("payment-slips/order-history-%d.png", time.Now().UnixNano()),
		URL:         "https://storage.googleapis.com/test-bucket/order-history.png",
		ContentType: "image/png",
		Size:        2048,
	}
	if err := checkout.RecordPaymentSlip(context.Background(), slip, staffID); err != nil {
		t.Fatalf("record payment slip: %v", err)
	}

	firstCommand := qrCheckoutCommand(t, base, staffID,
		fmt.Sprintf("order-history-qr-%d", time.Now().UnixNano()), slip.ObjectKey)
	first, _, err := checkout.Checkout(context.Background(), base.Today, firstCommand,
		checkoutPlanner(t, firstCommand, nil))
	if err != nil {
		t.Fatalf("create QR order fixture: %v", err)
	}

	promotionRepository := NewPromotionRepository(database)
	promotion, err := promotionRepository.Create(context.Background(), base.Today, base.ProjectID, models.ProjectPromotionMutation{
		Name:           "Bundle",
		PromotionPrice: mustPromotionAmount(t, "150.00"),
		Items: []models.ProjectPromotionItemInput{
			{ProductID: base.ProductAID, Quantity: 1},
			{ProductID: base.ProductBID, VariantID: &base.VariantOneID, Quantity: 2},
		},
	})
	if err != nil {
		t.Fatalf("create promotion fixture: %v", err)
	}
	applied := &models.AppliedProjectPromotion{
		PromotionID:         promotion.PromotionID,
		Name:                promotion.Name,
		OriginalBundlePrice: mustPromotionAmount(t, "180.00"),
		PromotionPrice:      mustPromotionAmount(t, "150.00"),
		Discount:            mustPromotionAmount(t, "30.00"),
	}
	secondCommand := cashCheckoutCommand(t, base, staffID,
		fmt.Sprintf("order-history-cash-%d", time.Now().UnixNano()), "200.00", true)
	second, _, err := checkout.Checkout(context.Background(), base.Today, secondCommand,
		checkoutPlanner(t, secondCommand, applied))
	if err != nil {
		t.Fatalf("create promoted cash order fixture: %v", err)
	}

	return orderIntegrationFixture{
		ProjectID:      base.ProjectID,
		OtherProjectID: base.OtherProjectID,
		StaffID:        staffID,
		OrderOneID:     first.OrderID,
		OrderTwoID:     second.OrderID,
	}
}
