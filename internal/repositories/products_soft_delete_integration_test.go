//go:build integration

package repositories

import (
	"context"
	"errors"
	"testing"

	"github.com/esc-chula/intania-shop-api/internal/models"
)

func TestArchiveProductRejectsLiveProjectsAndPreservesHistoricalCheckout(t *testing.T) {
	database := openPromotionIntegrationDatabase(t)
	fixture := newPromotionIntegrationFixture(t, database)
	ctx := context.Background()
	productRepository := NewProductRepository(database)

	// A current project assignment, including a variant assignment, makes the
	// archive unsafe because it could invalidate checkout and promotion rules.
	if err := productRepository.Archive(ctx, fixture.Today, fixture.ProductBID); !errors.Is(err, ErrProductInUse) {
		t.Fatalf("archive active project product error = %v, want ErrProductInUse", err)
	}

	imageURL := "https://storage.googleapis.com/test-bucket/products/archive-proof.png"
	if _, err := database.Exec(ctx, `UPDATE products SET images = ARRAY[$2] WHERE id = $1`, fixture.ProductBID, imageURL); err != nil {
		t.Fatalf("set product image: %v", err)
	}
	setCheckoutStock(t, database, fixture, 7, 3, 5)
	staffID := insertCheckoutStaff(t, database)
	checkoutRepository := NewPOSRepository(database)
	command := cashCheckoutCommand(t, fixture, staffID, "archive-history", "200.00", true)
	order, _, err := checkoutRepository.Checkout(ctx, fixture.Today, command, checkoutPlanner(t, command, nil))
	if err != nil {
		t.Fatalf("write historical order: %v", err)
	}

	// Once the project is completed, the same assignment is historical only and
	// product archival is allowed. This is the date predicate used by Archive.
	if _, err := database.Exec(ctx, `
		UPDATE projects
		SET start_date = $2::date, end_date = $3::date
		WHERE project_id = $1`,
		fixture.ProjectID,
		fixture.Today.Time.AddDate(0, 0, -3),
		fixture.Today.Time.AddDate(0, 0, -2)); err != nil {
		t.Fatalf("complete project: %v", err)
	}
	if err := productRepository.Archive(ctx, fixture.Today, fixture.ProductBID); err != nil {
		t.Fatalf("archive completed-project product: %v", err)
	}
	// Repeating DELETE is intentionally idempotent for an already archived ID.
	if err := productRepository.Archive(ctx, fixture.Today, fixture.ProductBID); err != nil {
		t.Fatalf("repeat archive: %v", err)
	}
	if _, err := NewInventoryRepository(database).Adjust(ctx, fixture.ProductBID,
		models.AdjustStockRequest{VariantID: &fixture.VariantOneID, QuantityChange: 1, Reason: "must reject archived product"}, "archive-test"); !errors.Is(err, ErrProductArchived) {
		t.Fatalf("adjust archived variant error = %v, want ErrProductArchived", err)
	}
	staleCommand := cashCheckoutCommand(t, fixture, staffID, "archive-stale-cart", "200.00", true)
	if _, _, err := checkoutRepository.Checkout(ctx, fixture.Today, staleCommand, failingCheckoutPlanner(t)); !errors.Is(err, ErrProductNotSellable) {
		t.Fatalf("checkout archived product error = %v, want ErrProductNotSellable", err)
	}

	var archived bool
	if err := database.QueryRow(ctx, `SELECT deleted_at IS NOT NULL FROM products WHERE id = $1`, fixture.ProductBID).Scan(&archived); err != nil || !archived {
		t.Fatalf("product archive state = %t, err = %v", archived, err)
	}
	var variantCount, stockTransactionCount int
	if err := database.QueryRow(ctx, `SELECT COUNT(*) FROM variants WHERE product_id = $1`, fixture.ProductBID).Scan(&variantCount); err != nil || variantCount != 2 {
		t.Fatalf("variants after archive = %d, err = %v; want 2", variantCount, err)
	}
	if err := database.QueryRow(ctx, `SELECT COUNT(*) FROM stock_transactions WHERE product_id = $1`, fixture.ProductBID).Scan(&stockTransactionCount); err != nil || stockTransactionCount == 0 {
		t.Fatalf("stock transactions after archive = %d, err = %v; want non-zero", stockTransactionCount, err)
	}

	// The order is read entirely from order_items snapshots, so its display data
	// (including the GCS URL) survives the master-product lifecycle change.
	historical, err := readPOSOrder(ctx, database, order.OrderID)
	if err != nil {
		t.Fatalf("read archived product order: %v", err)
	}
	var found bool
	for _, item := range historical.Items {
		if item.ProductID != fixture.ProductBID {
			continue
		}
		found = true
		if item.ProductName != fixture.ProductBName || item.ImageURL == nil || *item.ImageURL != imageURL || item.Size == nil || *item.Size != "M" || item.Color == nil || *item.Color != "Blue" {
			t.Fatalf("historical archived product snapshot = %+v", item)
		}
	}
	if !found {
		t.Fatalf("order %d did not retain product %d snapshot", order.OrderID, fixture.ProductBID)
	}

	if _, err := productRepository.Detail(ctx, fixture.ProductBID); !errors.Is(err, ErrProductNotFound) {
		t.Fatalf("archived detail error = %v, want ErrProductNotFound", err)
	}
	if results, err := productRepository.Search(ctx, fixture.ProductBName, 0, 10); err != nil || len(results) != 0 {
		t.Fatalf("archived search results = %+v, err = %v", results, err)
	}
	projectRepository := NewProjectRepository(database)
	projectName := "must not reactivate archived catalogue"
	activeStart := fixture.Today
	activeEnd := models.NewDate(fixture.Today.Time.AddDate(0, 0, 1))
	if _, err := projectRepository.Update(ctx, fixture.Today, fixture.ProjectID, models.ProjectInput{
		Name: &projectName, StartDate: &activeStart, EndDate: &activeEnd,
	}); !errors.Is(err, ErrProjectArchivedProduct) {
		t.Fatalf("reactivate project with archived product error = %v, want ErrProjectArchivedProduct", err)
	}
	if candidates, total, err := projectRepository.ListProductCandidates(ctx, fixture.ProjectID, structFilter(), 0, 10); err != nil || candidateContains(candidates, fixture.ProductBID) || total < 0 {
		t.Fatalf("archived candidate lookup = %+v total=%d err=%v", candidates, total, err)
	}
	if catalogue, err := checkoutRepository.Catalog(ctx, fixture.Today, fixture.ProjectID); err != nil || catalogueContains(catalogue, fixture.ProductBID) {
		t.Fatalf("archived POS catalogue = %+v, err = %v", catalogue, err)
	}
	// No uniqueness rule is introduced by archival. A later catalogue item with
	// identical business fields is a new identity, preserving the old product's
	// references and snapshots.
	var replacementID int64
	if err := database.QueryRow(ctx, `
		INSERT INTO products (name, price, status, category)
		VALUES ($1, 40.00, 'IN_STOCK', 'Replacement')
		RETURNING id`, fixture.ProductBName).Scan(&replacementID); err != nil || replacementID == fixture.ProductBID {
		t.Fatalf("create replacement product id = %d, err = %v", replacementID, err)
	}
	t.Cleanup(func() {
		if _, err := database.Exec(context.Background(), `DELETE FROM products WHERE id = $1`, replacementID); err != nil {
			t.Errorf("cleanup replacement product: %v", err)
		}
	})
}

// Keep the archive assertions readable without importing model aliases into
// every call site in this integration test.
func structFilter() models.ProjectProductFilter { return models.ProjectProductFilter{} }

func candidateContains(items []models.ProjectProductCandidate, productID int64) bool {
	for _, item := range items {
		if item.ProductID == productID {
			return true
		}
	}
	return false
}

func catalogueContains(catalogue models.POSCatalogSnapshot, productID int64) bool {
	for _, item := range catalogue.Items {
		if item.ProductID == productID {
			return true
		}
	}
	return false
}
