//go:build integration

package repositories

import (
	"context"
	"errors"
	"testing"

	"github.com/esc-chula/intania-shop-api/internal/models"
)

func TestPOSRepositoryReadsCurrentCatalogueAndQuoteSnapshot(t *testing.T) {
	database := openPromotionIntegrationDatabase(t)
	fixture := newPromotionIntegrationFixture(t, database)
	ctx := context.Background()

	if _, err := database.Exec(ctx, `UPDATE products SET stock_quantity = 7 WHERE id = $1`, fixture.ProductAID); err != nil {
		t.Fatalf("set product fixture stock: %v", err)
	}
	if _, err := database.Exec(ctx, `
		UPDATE variants SET stock_quantity = CASE variant_id
			WHEN $1 THEN 3
			WHEN $2 THEN 5
		END
		WHERE variant_id IN ($1, $2)`, fixture.VariantOneID, fixture.VariantTwoID); err != nil {
		t.Fatalf("set fixture stock: %v", err)
	}

	promotionRepository := NewPromotionRepository(database)
	promotionPrice, err := models.ParseTHBAmount("120.00")
	if err != nil {
		t.Fatal(err)
	}
	createdPromotion, err := promotionRepository.Create(ctx, fixture.Today, fixture.ProjectID, models.ProjectPromotionMutation{
		Name:           "POS bundle",
		PromotionPrice: promotionPrice,
		Items: []models.ProjectPromotionItemInput{
			{ProductID: fixture.ProductAID, Quantity: 1},
			{ProductID: fixture.ProductBID, VariantID: &fixture.VariantOneID, Quantity: 2},
		},
	})
	if err != nil {
		t.Fatalf("create fixture promotion: %v", err)
	}

	repository := NewPOSRepository(database)
	catalogue, err := repository.Catalog(ctx, fixture.Today, fixture.ProjectID)
	if err != nil {
		t.Fatal(err)
	}
	if catalogue.Project.ProjectID != fixture.ProjectID || catalogue.Project.Status != models.ProjectStatusActive {
		t.Fatalf("catalogue project = %+v", catalogue.Project)
	}
	if len(catalogue.Items) != 3 {
		t.Fatalf("catalogue item count = %d, want 3", len(catalogue.Items))
	}
	assertPOSResolvedItem(t, catalogue.Items[0], fixture.ProductAID, nil, "100.00", 7)
	assertPOSResolvedItem(t, catalogue.Items[1], fixture.ProductBID, &fixture.VariantOneID, "40.00", 3)
	assertPOSResolvedItem(t, catalogue.Items[2], fixture.ProductBID, &fixture.VariantTwoID, "40.00", 5)

	quote, err := repository.QuoteSnapshot(ctx, fixture.Today, fixture.ProjectID, []models.POSCartItemRequest{
		{ProductID: fixture.ProductBID, VariantID: &fixture.VariantOneID, Quantity: 1},
		{ProductID: fixture.ProductAID, Quantity: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(quote.Items) != 2 {
		t.Fatalf("quote item count = %d, want 2", len(quote.Items))
	}
	assertPOSResolvedItem(t, quote.Items[0].Item, fixture.ProductBID, &fixture.VariantOneID, "40.00", 3)
	assertPOSResolvedItem(t, quote.Items[1].Item, fixture.ProductAID, nil, "100.00", 7)
	if quote.Items[0].Quantity != 1 || quote.Items[1].Quantity != 1 {
		t.Fatalf("quote quantities = %+v", quote.Items)
	}
	if len(quote.Promotions) != 1 || quote.Promotions[0].PromotionID != createdPromotion.PromotionID {
		t.Fatalf("quote promotions = %+v", quote.Promotions)
	}
	if len(quote.Promotions[0].Items) != 2 || quote.Promotions[0].Items[0].ProductID != fixture.ProductAID || quote.Promotions[0].Items[1].ProductID != fixture.ProductBID {
		t.Fatalf("quote promotion items = %+v", quote.Promotions[0].Items)
	}

	_, err = repository.QuoteSnapshot(ctx, fixture.Today, fixture.ProjectID, []models.POSCartItemRequest{
		{ProductID: fixture.OtherProductID, Quantity: 1},
	})
	if !errors.Is(err, ErrProductNotSellable) {
		t.Fatalf("unassigned quote item error = %v, want ErrProductNotSellable", err)
	}

	_, err = repository.QuoteSnapshot(ctx, fixture.Today, fixture.ProjectID, []models.POSCartItemRequest{
		{ProductID: fixture.ProductAID, Quantity: 1},
		{ProductID: fixture.ProductAID, Quantity: 2},
	})
	if !errors.Is(err, ErrPOSDuplicateCartItem) {
		t.Fatalf("duplicate quote item error = %v, want ErrPOSDuplicateCartItem", err)
	}
}

func TestPOSRepositoryCatalogueAllowsPreviewProjectStatuses(t *testing.T) {
	database := openPromotionIntegrationDatabase(t)
	fixture := newPromotionIntegrationFixture(t, database)
	ctx := context.Background()
	repository := NewPOSRepository(database)

	if _, err := database.Exec(ctx, `
		UPDATE projects
		SET start_date = $2::date, end_date = $3::date
		WHERE project_id = $1`, fixture.ProjectID,
		fixture.Today.Time.AddDate(0, 0, 2), fixture.Today.Time.AddDate(0, 0, 3)); err != nil {
		t.Fatalf("set not-started project dates: %v", err)
	}
	notStarted, err := repository.Catalog(ctx, fixture.Today, fixture.ProjectID)
	if err != nil {
		t.Fatal(err)
	}
	if notStarted.Project.Status != models.ProjectStatusNotStarted {
		t.Fatalf("not-started catalogue status = %s", notStarted.Project.Status)
	}

	if _, err := database.Exec(ctx, `
		UPDATE projects
		SET start_date = $2::date, end_date = $3::date
		WHERE project_id = $1`, fixture.ProjectID,
		fixture.Today.Time.AddDate(0, 0, -3), fixture.Today.Time.AddDate(0, 0, -2)); err != nil {
		t.Fatalf("set completed project dates: %v", err)
	}
	completed, err := repository.Catalog(ctx, fixture.Today, fixture.ProjectID)
	if err != nil {
		t.Fatal(err)
	}
	if completed.Project.Status != models.ProjectStatusCompleted {
		t.Fatalf("completed catalogue status = %s", completed.Project.Status)
	}
}

func TestPOSRepositoryMapsMissingProject(t *testing.T) {
	database := openPromotionIntegrationDatabase(t)
	fixture := newPromotionIntegrationFixture(t, database)
	repository := NewPOSRepository(database)
	missingID := int64(9223372036854770000)

	if _, err := repository.Catalog(context.Background(), fixture.Today, missingID); !errors.Is(err, ErrProjectNotFound) {
		t.Fatalf("missing catalogue project error = %v, want ErrProjectNotFound", err)
	}
	if _, err := repository.QuoteSnapshot(context.Background(), fixture.Today, missingID, []models.POSCartItemRequest{{ProductID: fixture.ProductAID, Quantity: 1}}); !errors.Is(err, ErrProjectNotFound) {
		t.Fatalf("missing quote project error = %v, want ErrProjectNotFound", err)
	}
}

func assertPOSResolvedItem(t *testing.T, item models.POSResolvedItem, productID int64, variantID *int64, price string, stock int32) {
	t.Helper()
	if item.ProductID != productID || item.ProjectPrice.String() != price || item.StockQuantity != stock {
		t.Fatalf("POS item = %+v, want product=%d price=%s stock=%d", item, productID, price, stock)
	}
	if (item.VariantID == nil) != (variantID == nil) {
		t.Fatalf("POS item variant = %v, want %v", item.VariantID, variantID)
	}
	if item.VariantID != nil && *item.VariantID != *variantID {
		t.Fatalf("POS item variant = %d, want %d", *item.VariantID, *variantID)
	}
}
