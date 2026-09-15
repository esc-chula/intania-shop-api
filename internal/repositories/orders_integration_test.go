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

type orderIntegrationFixture struct {
	ProjectID      int64
	OtherProjectID int64
	StaffID        int64
	OrderOneID     int64
	OrderTwoID     int64
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
