package repositories

import (
	"errors"
	"math"
	"testing"

	"github.com/esc-chula/intania-shop-api/internal/models"
)

func TestValidatePromotionPriceReportsArithmeticOverflowAsValidation(t *testing.T) {
	maximum, err := models.NewTHBAmountFromSatang(math.MaxInt64)
	if err != nil {
		t.Fatal(err)
	}

	err = validatePromotionPrice(
		models.ProjectPromotionMutation{
			Items: []models.ProjectPromotionItemInput{{ProductID: 1, Quantity: 2}},
		},
		[]ProjectProductAssignment{{ProductID: 1, ProjectPrice: maximum}},
	)
	if !errors.Is(err, models.ErrTHBAmountOverflow) {
		t.Fatalf("overflow error = %v, want models.ErrTHBAmountOverflow", err)
	}
	if errors.Is(err, ErrPromotionPricingCorrupt) {
		t.Fatalf("overflow error = %v, should not be classified as persisted pricing corruption", err)
	}
}

func TestValidatePromotionGroupPriceUsesMinimumAlternativeBundle(t *testing.T) {
	promotionPrice, err := models.ParseTHBAmount("310.00")
	if err != nil {
		t.Fatal(err)
	}
	cheap, err := models.ParseTHBAmount("300.00")
	if err != nil {
		t.Fatal(err)
	}
	expensive, err := models.ParseTHBAmount("350.00")
	if err != nil {
		t.Fatal(err)
	}
	mutation := models.ProjectPromotionMutation{
		PromotionPrice: promotionPrice,
		ItemGroups: []models.ProjectPromotionItemGroupInput{{Options: []models.ProjectPromotionItemInput{
			{ProductID: 1, Quantity: 1},
			{ProductID: 2, Quantity: 1},
		}}},
	}
	references := flattenPromotionItemGroups(mutation.ItemGroups)
	err = validatePromotionGroupPrice(mutation, references, []ProjectProductAssignment{
		{ProductID: 1, ProjectPrice: cheap},
		{ProductID: 2, ProjectPrice: expensive},
	})
	if !errors.Is(err, ErrPromotionPriceExceedsBundle) {
		t.Fatalf("error = %v, want ErrPromotionPriceExceedsBundle", err)
	}
}

func TestPooledVariantPromotionPriceRangeAndGuard(t *testing.T) {
	amount := func(value string) models.THBAmount {
		parsed, err := models.ParseTHBAmount(value)
		if err != nil {
			t.Fatal(err)
		}
		return parsed
	}
	small, medium := int64(21), int64(22)
	groups := []models.ProjectPromotionItemGroupInput{{Options: []models.ProjectPromotionItemInput{
		{ProductID: 10, VariantID: &small, Quantity: 2},
		{ProductID: 10, VariantID: &medium, Quantity: 2},
	}}}
	for _, price := range []string{"400.00", "800.00", "800.01"} {
		t.Run(price, func(t *testing.T) {
			err := validatePromotionPrice(models.ProjectPromotionMutation{PromotionPrice: amount(price), ItemGroups: groups}, []ProjectProductAssignment{
				{ProductID: 10, VariantID: &small, ProjectPrice: amount("400.00")},
				{ProductID: 10, VariantID: &medium, ProjectPrice: amount("450.00")},
			})
			if price == "800.01" {
				if !errors.Is(err, ErrPromotionPriceExceedsBundle) {
					t.Fatalf("error=%v, want price above cheapest pair rejected", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
		})
	}
	promotions := []models.ProjectPromotion{{PromotionID: 1, PromotionPrice: amount("400.00"), ItemGroups: []models.ProjectPromotionItemGroup{{Options: []models.ProjectPromotionItem{
		{ProductID: 10, VariantID: &small, Quantity: 2, UnitPrice: amount("400.00")},
		{ProductID: 10, VariantID: &medium, Quantity: 2, UnitPrice: amount("450.00")},
	}}}}}
	if err := calculatePromotionTotals(promotions); err != nil {
		t.Fatal(err)
	}
	got := promotions[0]
	if got.OriginalBundlePriceMin.String() != "800.00" || got.OriginalBundlePriceMax.String() != "900.00" || got.DiscountMin.String() != "400.00" || got.DiscountMax.String() != "500.00" {
		t.Fatalf("pooled variants changed per-set price ranges: %+v", got)
	}
}
