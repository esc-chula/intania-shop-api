package usecases

import (
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/esc-chula/intania-shop-api/internal/models"
)

func TestPricingServiceCalculatesSubtotalWithoutMatchingPromotion(t *testing.T) {
	service := NewPricingService()
	cart := []models.PricingCartLine{
		{ProductID: 1, Quantity: 2, UnitPrice: pricingTestAmount(t, "100.00")},
		{ProductID: 2, Quantity: 1, UnitPrice: pricingTestAmount(t, "50.00")},
	}

	result, err := service.Calculate(cart, []models.PricingPromotion{
		{
			PromotionID:    1,
			Name:           "Three items",
			PromotionPrice: pricingTestAmount(t, "200.00"),
			Items: []models.PricingPromotionItem{
				{ProductID: 1, Quantity: 3},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	assertPricingAmount(t, result.Subtotal, "250.00")
	assertPricingAmount(t, result.Discount, "0.00")
	assertPricingAmount(t, result.NetTotal, "250.00")
	if result.AppliedPromotion != nil {
		t.Fatalf("AppliedPromotion = %+v, want nil", result.AppliedPromotion)
	}
}

func TestPricingServiceAppliesEligiblePromotionOnce(t *testing.T) {
	service := NewPricingService()
	result, err := service.Calculate(
		[]models.PricingCartLine{{ProductID: 1, Quantity: 3, UnitPrice: pricingTestAmount(t, "100.00")}},
		[]models.PricingPromotion{{
			PromotionID:    12,
			Name:           "Single bundle",
			PromotionPrice: pricingTestAmount(t, "70.00"),
			Items:          []models.PricingPromotionItem{{ProductID: 1, Quantity: 1}},
		}},
	)
	if err != nil {
		t.Fatal(err)
	}

	assertPricingAmount(t, result.Subtotal, "300.00")
	assertPricingAmount(t, result.Discount, "30.00")
	assertPricingAmount(t, result.NetTotal, "270.00")
	if result.AppliedPromotion == nil {
		t.Fatal("AppliedPromotion is nil")
	}
	if result.AppliedPromotion.PromotionID != 12 {
		t.Fatalf("PromotionID = %d, want 12", result.AppliedPromotion.PromotionID)
	}
	assertPricingAmount(t, result.AppliedPromotion.OriginalBundlePrice, "100.00")
	assertPricingAmount(t, result.AppliedPromotion.PromotionPrice, "70.00")
}

func TestPricingServiceSelectsHighestDiscountAndLowestIDOnTie(t *testing.T) {
	service := NewPricingService()
	cart := []models.PricingCartLine{
		{ProductID: 1, Quantity: 3, UnitPrice: pricingTestAmount(t, "100.00")},
		{ProductID: 2, Quantity: 1, UnitPrice: pricingTestAmount(t, "50.00")},
	}

	highest := models.PricingPromotion{
		PromotionID:    20,
		Name:           "Two shirts",
		PromotionPrice: pricingTestAmount(t, "150.00"),
		Items:          []models.PricingPromotionItem{{ProductID: 1, Quantity: 2}},
	}
	tieHigherID := models.PricingPromotion{
		PromotionID:    12,
		Name:           "Shirt and pin",
		PromotionPrice: pricingTestAmount(t, "130.00"),
		Items: []models.PricingPromotionItem{
			{ProductID: 1, Quantity: 1},
			{ProductID: 2, Quantity: 1},
		},
	}
	tieLowerID := tieHigherID
	tieLowerID.PromotionID = 7

	result, err := service.Calculate(cart, []models.PricingPromotion{tieHigherID, highest, tieLowerID})
	if err != nil {
		t.Fatal(err)
	}
	if result.AppliedPromotion == nil || result.AppliedPromotion.PromotionID != 20 {
		t.Fatalf("AppliedPromotion = %+v, want highest-discount Promotion 20", result.AppliedPromotion)
	}
	assertPricingAmount(t, result.Discount, "50.00")

	for _, promotions := range [][]models.PricingPromotion{
		{tieHigherID, tieLowerID},
		{tieLowerID, tieHigherID},
	} {
		result, err := service.Calculate(cart, promotions)
		if err != nil {
			t.Fatal(err)
		}
		if result.AppliedPromotion == nil || result.AppliedPromotion.PromotionID != 7 {
			t.Fatalf("AppliedPromotion = %+v, want lower-ID tie winner 7", result.AppliedPromotion)
		}
		assertPricingAmount(t, result.Discount, "20.00")
	}
}

func TestPricingServiceMatchesRequiredQuantitiesAndVariantIdentity(t *testing.T) {
	variantID := int64(2)
	cart := []models.PricingCartLine{
		{ProductID: 1, Quantity: 2, UnitPrice: pricingTestAmount(t, "10.00")},
		{ProductID: 1, VariantID: &variantID, Quantity: 1, UnitPrice: pricingTestAmount(t, "20.00")},
	}

	result, err := NewPricingService().Calculate(cart, []models.PricingPromotion{
		{
			PromotionID:    1,
			PromotionPrice: pricingTestAmount(t, "15.00"),
			Items:          []models.PricingPromotionItem{{ProductID: 1, Quantity: 2}},
		},
		{
			PromotionID:    2,
			PromotionPrice: pricingTestAmount(t, "15.00"),
			Items:          []models.PricingPromotionItem{{ProductID: 1, VariantID: &variantID, Quantity: 2}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.AppliedPromotion == nil || result.AppliedPromotion.PromotionID != 1 {
		t.Fatalf("AppliedPromotion = %+v, want variantless Promotion 1", result.AppliedPromotion)
	}
	assertPricingAmount(t, result.AppliedPromotion.OriginalBundlePrice, "20.00")
	assertPricingAmount(t, result.Discount, "5.00")
}

func TestPricingServiceSelectsEligibleZeroDiscountPromotion(t *testing.T) {
	result, err := NewPricingService().Calculate(
		[]models.PricingCartLine{{ProductID: 1, Quantity: 1, UnitPrice: pricingTestAmount(t, "10.00")}},
		[]models.PricingPromotion{{
			PromotionID:    4,
			Name:           "No savings bundle",
			PromotionPrice: pricingTestAmount(t, "10.00"),
			Items:          []models.PricingPromotionItem{{ProductID: 1, Quantity: 1}},
		}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.AppliedPromotion == nil || result.AppliedPromotion.PromotionID != 4 {
		t.Fatalf("AppliedPromotion = %+v, want eligible zero-discount Promotion", result.AppliedPromotion)
	}
	assertPricingAmount(t, result.Discount, "0.00")
	assertPricingAmount(t, result.NetTotal, "10.00")
}

func TestPricingServiceRejectsInvalidInput(t *testing.T) {
	variantID := int64(2)
	validCart := []models.PricingCartLine{{ProductID: 1, Quantity: 1, UnitPrice: pricingTestAmount(t, "10.00")}}
	validPromotion := models.PricingPromotion{
		PromotionID:    1,
		PromotionPrice: pricingTestAmount(t, "5.00"),
		Items:          []models.PricingPromotionItem{{ProductID: 1, Quantity: 1}},
	}

	tests := []struct {
		name       string
		cart       []models.PricingCartLine
		promotions []models.PricingPromotion
		want       error
	}{
		{name: "empty cart", cart: nil, promotions: nil, want: ErrPricingInvalidCart},
		{name: "invalid cart ID", cart: []models.PricingCartLine{{ProductID: 0, Quantity: 1}}, want: ErrPricingInvalidCart},
		{name: "invalid cart variant ID", cart: []models.PricingCartLine{{ProductID: 1, VariantID: &variantID, Quantity: 0}}, want: ErrPricingInvalidCart},
		{name: "duplicate cart identity", cart: []models.PricingCartLine{{ProductID: 1, Quantity: 1}, {ProductID: 1, Quantity: 2}}, want: ErrPricingDuplicateCartItem},
		{name: "invalid promotion ID", cart: validCart, promotions: []models.PricingPromotion{{PromotionID: 0, Items: validPromotion.Items}}, want: ErrPricingInvalidPromotion},
		{name: "empty promotion", cart: validCart, promotions: []models.PricingPromotion{{PromotionID: 1}}, want: ErrPricingInvalidPromotion},
		{name: "invalid promotion item", cart: validCart, promotions: []models.PricingPromotion{{PromotionID: 1, Items: []models.PricingPromotionItem{{ProductID: 1, Quantity: 0}}}}, want: ErrPricingInvalidPromotion},
		{name: "duplicate promotion item", cart: validCart, promotions: []models.PricingPromotion{{PromotionID: 1, Items: []models.PricingPromotionItem{{ProductID: 1, Quantity: 1}, {ProductID: 1, Quantity: 2}}}}, want: ErrPricingDuplicatePromotionItem},
		{name: "duplicate promotion ID", cart: validCart, promotions: []models.PricingPromotion{validPromotion, validPromotion}, want: ErrPricingDuplicatePromotion},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewPricingService().Calculate(test.cart, test.promotions)
			if !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestPricingServiceRejectsPromotionPriceAboveBundle(t *testing.T) {
	result, err := NewPricingService().Calculate(
		[]models.PricingCartLine{{ProductID: 1, Quantity: 1, UnitPrice: pricingTestAmount(t, "10.00")}},
		[]models.PricingPromotion{{
			PromotionID:    1,
			PromotionPrice: pricingTestAmount(t, "11.00"),
			Items:          []models.PricingPromotionItem{{ProductID: 1, Quantity: 1}},
		}},
	)
	if !errors.Is(err, ErrPricingInvalidPromotion) {
		t.Fatalf("error = %v, want ErrPricingInvalidPromotion", err)
	}
	if !errors.Is(err, models.ErrTHBAmountUnderflow) {
		t.Fatalf("error = %v, want THB underflow to be preserved", err)
	}
	if result != (models.PricingResult{}) {
		t.Fatalf("result on error = %+v, want zero result", result)
	}
}

func TestPricingServicePropagatesArithmeticOverflow(t *testing.T) {
	maximum, err := models.NewTHBAmountFromSatang(math.MaxInt64)
	if err != nil {
		t.Fatal(err)
	}

	result, err := NewPricingService().Calculate(
		[]models.PricingCartLine{{ProductID: 1, Quantity: 2, UnitPrice: maximum}},
		nil,
	)
	if !errors.Is(err, models.ErrTHBAmountOverflow) {
		t.Fatalf("error = %v, want THB overflow", err)
	}
	if result != (models.PricingResult{}) {
		t.Fatalf("result on error = %+v, want zero result", result)
	}
}

func TestPricingServiceDoesNotMutateInputs(t *testing.T) {
	variantID := int64(2)
	cart := []models.PricingCartLine{{ProductID: 1, VariantID: &variantID, Quantity: 2, UnitPrice: pricingTestAmount(t, "10.00")}}
	promotions := []models.PricingPromotion{{
		PromotionID:    7,
		Name:           "Bundle",
		PromotionPrice: pricingTestAmount(t, "5.00"),
		Items:          []models.PricingPromotionItem{{ProductID: 1, VariantID: &variantID, Quantity: 1}},
	}}
	cartBefore := append([]models.PricingCartLine(nil), cart...)
	promotionsBefore := append([]models.PricingPromotion(nil), promotions...)
	promotionsBefore[0].Items = append([]models.PricingPromotionItem(nil), promotions[0].Items...)

	if _, err := NewPricingService().Calculate(cart, promotions); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cart, cartBefore) {
		t.Fatalf("cart mutated: before=%+v after=%+v", cartBefore, cart)
	}
	if !reflect.DeepEqual(promotions, promotionsBefore) {
		t.Fatalf("promotions mutated: before=%+v after=%+v", promotionsBefore, promotions)
	}
}

func pricingTestAmount(t *testing.T, value string) models.THBAmount {
	t.Helper()
	amount, err := models.ParseTHBAmount(value)
	if err != nil {
		t.Fatalf("parse amount %q: %v", value, err)
	}
	return amount
}

func assertPricingAmount(t *testing.T, amount models.THBAmount, want string) {
	t.Helper()
	if got := amount.String(); got != want {
		t.Fatalf("amount = %s, want %s", got, want)
	}
}
