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
