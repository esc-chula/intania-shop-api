//go:build integration

package repositories

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/esc-chula/intania-shop-api/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

type orderIntegrationFixture struct {
	ProjectID  int64
	ProductID  int64
	StaffID    int64
	StockTxID  int64
	OrderOneID int64
	OrderTwoID int64
}

func TestOrderRepositoryListFiltersSortsAndPaginates(t *testing.T) {
	database := openPromotionIntegrationDatabase(t)
	fixture := newOrderIntegrationFixture(t, database)
	ctx := context.Background()
	repository := NewOrderRepository(database, "test-bucket")

	orders, total, err := repository.List(ctx, fixture.ProjectID, models.OrderFilter{
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
	if len(orders[0].Items) != 1 {
		t.Fatalf("order one items = %d, want 1", len(orders[0].Items))
	}
	if orders[1].AppliedPromotion == nil || orders[1].AppliedPromotion.Name != "Bundle" {
		t.Fatalf("order two promotion snapshot = %+v", orders[1].AppliedPromotion)
	}

	filtered, filteredTotal, err := repository.List(ctx, fixture.ProjectID, models.OrderFilter{
		PaymentMethod: models.OrderPaymentMethodQR,
		SortBy:        models.OrderSortByOrderNumber, SortOrder: models.OrderSortAsc,
		CreatedFrom: time.Now().Add(-24 * time.Hour), CreatedTo: time.Now().Add(24 * time.Hour),
	}, 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if filteredTotal != 1 || len(filtered) != 1 || filtered[0].OrderID != fixture.OrderOneID {
		t.Fatalf("payment method filter = %+v (total %d)", filtered, filteredTotal)
	}

	page, pageTotal, err := repository.List(ctx, fixture.ProjectID, models.OrderFilter{
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
	otherProjectID := insertPromotionProject(t, database, "Other order project", "2026-01-01", "2026-01-02")
	t.Cleanup(func() {
		if _, err := database.Exec(context.Background(), `DELETE FROM projects WHERE project_id = $1`, otherProjectID); err != nil {
			t.Errorf("cleanup other project: %v", err)
		}
	})
	ctx := context.Background()
	repository := NewOrderRepository(database, "test-bucket")

	order, err := repository.Detail(ctx, fixture.ProjectID, fixture.OrderOneID)
	if err != nil {
		t.Fatal(err)
	}
	if order.OrderID != fixture.OrderOneID || len(order.Items) != 1 {
		t.Fatalf("order detail = %+v", order)
	}
	if order.Staff.StaffID != fixture.StaffID {
		t.Fatalf("staff snapshot = %+v", order.Staff)
	}

	if _, err := repository.Detail(ctx, otherProjectID, fixture.OrderOneID); !errors.Is(err, ErrOrderNotFound) {
		t.Fatalf("cross-project detail err = %v, want ErrOrderNotFound", err)
	}
	if _, err := repository.Detail(ctx, fixture.ProjectID, 9223372036854770000); !errors.Is(err, ErrOrderNotFound) {
		t.Fatalf("missing order err = %v, want ErrOrderNotFound", err)
	}
}

func TestOrderRepositoryExportReturnsOneRowPerItemUnpaginated(t *testing.T) {
	database := openPromotionIntegrationDatabase(t)
	fixture := newOrderIntegrationFixture(t, database)
	ctx := context.Background()
	repository := NewOrderRepository(database, "test-bucket")

	orders, err := repository.Export(ctx, fixture.ProjectID, models.OrderFilter{
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
	if totalItems != 2 {
		t.Fatalf("exported items = %d, want 2", totalItems)
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
	ctx := context.Background()

	projectID := insertPromotionProject(t, database, "Order history project", "2026-01-01", "2026-12-31")
	productID := insertPromotionProduct(t, database, "Order history product", "100.00")

	var staffID int64
	if err := database.QueryRow(ctx, `
		INSERT INTO users (full_name, email, role) VALUES ('Staff One', 'staff-one@example.com', 'ADMIN')
		RETURNING user_id`).Scan(&staffID); err != nil {
		t.Fatalf("insert staff fixture: %v", err)
	}

	var stockTxID int64
	if err := database.QueryRow(ctx, `
		INSERT INTO stock_transactions (product_id, transaction_type, quantity_change, quantity_before, quantity_after)
		VALUES ($1, 'ORDER', -1, 10, 9)
		RETURNING transaction_id`, productID).Scan(&stockTxID); err != nil {
		t.Fatalf("insert stock transaction fixture: %v", err)
	}

	var orderOneID int64
	if err := database.QueryRow(ctx, `
		INSERT INTO orders (project_id, order_number, staff_id, staff_full_name, staff_email,
			buyer_gender, buyer_age, buyer_student_alumni_year,
			payment_method, subtotal, discount, net_total)
		VALUES ($1, 'ORD-0001', $2, 'Staff One', 'staff-one@example.com',
			'FEMALE', 21, 'Intania 105',
			'QR_CODE', '100.00', '0.00', '100.00')
		RETURNING order_id`, projectID, staffID).Scan(&orderOneID); err != nil {
		t.Fatalf("insert order one fixture: %v", err)
	}
	if _, err := database.Exec(ctx, `
		INSERT INTO order_items (order_id, product_id, product_name, quantity, unit_price, line_total, inventory_transaction_id)
		VALUES ($1, $2, 'Order history product', 1, '100.00', '100.00', $3)`,
		orderOneID, productID, stockTxID); err != nil {
		t.Fatalf("insert order one item fixture: %v", err)
	}

	var orderTwoID int64
	if err := database.QueryRow(ctx, `
		INSERT INTO orders (project_id, order_number, staff_id, staff_full_name, staff_email,
			buyer_gender, payment_method, payment_received_amount, payment_change_amount, payment_no_change,
			subtotal, discount, net_total,
			promotion_id, promotion_name, promotion_original_bundle_price, promotion_price, promotion_discount)
		VALUES ($1, 'ORD-0002', $2, 'Staff One', 'staff-one@example.com',
			'MALE', 'REAL_MONEY', '200.00', '0.00', true,
			'100.00', '20.00', '80.00',
			NULL, 'Bundle', '100.00', '80.00', '20.00')
		RETURNING order_id`, projectID, staffID).Scan(&orderTwoID); err != nil {
		t.Fatalf("insert order two fixture: %v", err)
	}
	if _, err := database.Exec(ctx, `
		INSERT INTO order_items (order_id, product_id, product_name, quantity, unit_price, line_total, inventory_transaction_id)
		VALUES ($1, $2, 'Order history product', 1, '100.00', '100.00', $3)`,
		orderTwoID, productID, stockTxID); err != nil {
		t.Fatalf("insert order two item fixture: %v", err)
	}

	fixture := orderIntegrationFixture{
		ProjectID: projectID, ProductID: productID, StaffID: staffID, StockTxID: stockTxID,
		OrderOneID: orderOneID, OrderTwoID: orderTwoID,
	}

	t.Cleanup(func() {
		cleanupOrderIntegrationFixture(t, database, fixture)
	})
	return fixture
}

func cleanupOrderIntegrationFixture(t *testing.T, database *pgxpool.Pool, fixture orderIntegrationFixture) {
	t.Helper()
	ctx := context.Background()
	if _, err := database.Exec(ctx, `DELETE FROM order_items WHERE order_id IN ($1, $2)`, fixture.OrderOneID, fixture.OrderTwoID); err != nil {
		t.Errorf("cleanup order items: %v", err)
	}
	if _, err := database.Exec(ctx, `DELETE FROM orders WHERE order_id IN ($1, $2)`, fixture.OrderOneID, fixture.OrderTwoID); err != nil {
		t.Errorf("cleanup orders: %v", err)
	}
	if _, err := database.Exec(ctx, `DELETE FROM stock_transactions WHERE transaction_id = $1`, fixture.StockTxID); err != nil {
		t.Errorf("cleanup stock transaction: %v", err)
	}
	if _, err := database.Exec(ctx, `DELETE FROM users WHERE user_id = $1`, fixture.StaffID); err != nil {
		t.Errorf("cleanup staff: %v", err)
	}
	if _, err := database.Exec(ctx, `DELETE FROM products WHERE id = $1`, fixture.ProductID); err != nil {
		t.Errorf("cleanup product: %v", err)
	}
	if _, err := database.Exec(ctx, `DELETE FROM projects WHERE project_id = $1`, fixture.ProjectID); err != nil {
		t.Errorf("cleanup project: %v", err)
	}
}
