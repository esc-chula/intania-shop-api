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
