package repositories

import (
	"errors"
	"reflect"
	"testing"

	"github.com/esc-chula/intania-shop-api/internal/models"
)

func TestBuildPOSRequestedTablePreservesOrderAndNullVariants(t *testing.T) {
	variantID := int64(22)
	table, arguments := buildPOSRequestedTable(9, []models.POSCartItemRequest{
		{ProductID: 4, VariantID: nil, Quantity: 2},
		{ProductID: 8, VariantID: &variantID, Quantity: 1},
	})

	wantTable := `(VALUES ($2::bigint, $3::bigint, $4::bigint, $5::integer), ($6::bigint, $7::bigint, $8::bigint, $9::integer)) AS requested(ordinal, product_id, variant_id, quantity)`
	if table != wantTable {
		t.Fatalf("requested table = %q, want %q", table, wantTable)
	}
	wantArguments := []any{int64(9), 0, int64(4), (*int64)(nil), int32(2), 1, int64(8), &variantID, int32(1)}
	if !reflect.DeepEqual(arguments, wantArguments) {
		t.Fatalf("arguments = %#v, want %#v", arguments, wantArguments)
	}
}

func TestValidatePOSCartIdentitiesUsesNullSafeVariantIdentity(t *testing.T) {
	variantID := int64(22)
	if err := validatePOSCartIdentities([]models.POSCartItemRequest{
		{ProductID: 4, VariantID: nil, Quantity: 1},
		{ProductID: 4, VariantID: &variantID, Quantity: 1},
	}); err != nil {
		t.Fatalf("distinct variant identities rejected: %v", err)
	}

	err := validatePOSCartIdentities([]models.POSCartItemRequest{
		{ProductID: 4, VariantID: &variantID, Quantity: 1},
		{ProductID: 4, VariantID: &variantID, Quantity: 3},
	})
	if !errors.Is(err, ErrPOSDuplicateCartItem) {
		t.Fatalf("duplicate identity error = %v, want ErrPOSDuplicateCartItem", err)
	}
}

func TestPricingPromotionsFromHydratedCopiesOnlyPricingFields(t *testing.T) {
	variantID := int64(22)
	promotionPrice, err := models.ParseTHBAmount("25.00")
	if err != nil {
		t.Fatal(err)
	}
	hydrated := []models.ProjectPromotion{{
		PromotionID:    7,
		ProjectID:      9,
		Name:           "Bundle",
		PromotionPrice: promotionPrice,
		Items: []models.ProjectPromotionItem{{
			ProductID:   4,
			VariantID:   &variantID,
			Quantity:    2,
			ProductName: "Ignored by pricing",
			UnitPrice:   promotionPrice,
		}},
	}}

	converted := pricingPromotionsFromHydrated(hydrated)
	want := []models.PricingPromotion{{
		PromotionID:    7,
		Name:           "Bundle",
		PromotionPrice: promotionPrice,
		Items: []models.PricingPromotionItem{{
			ProductID: 4,
			VariantID: &variantID,
			Quantity:  2,
		}},
	}}
	if !reflect.DeepEqual(converted, want) {
		t.Fatalf("converted promotions = %#v, want %#v", converted, want)
	}
	if converted[0].Items[0].VariantID == hydrated[0].Items[0].VariantID {
		t.Fatal("converted variant pointer aliases hydrated promotion")
	}
}
