package usecases

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"testing"

	"github.com/esc-chula/intania-shop-api/internal/models"
)

func assertPricingApplicationCount(t *testing.T, promotion *models.AppliedProjectPromotion, want int64) {
	t.Helper()
	if promotion == nil || promotion.ApplicationCount == nil {
		t.Fatalf("promotion application count is missing: %+v", promotion)
	}
	if *promotion.ApplicationCount != want {
		t.Fatalf("application count = %d, want %d", *promotion.ApplicationCount, want)
	}
}

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

func TestPricingServiceRepeatsEligiblePromotion(t *testing.T) {
	tests := []struct {
		name                                string
		quantity, required                  int32
		price                               string
		subtotal, original, total, discount string
		net                                 string
	}{
		{"one set", 1, 1, "70.00", "100.00", "100.00", "70.00", "30.00", "70.00"},
		{"three sets", 3, 1, "70.00", "300.00", "300.00", "210.00", "90.00", "210.00"},
		{"incomplete remainder", 5, 2, "170.00", "500.00", "400.00", "340.00", "60.00", "440.00"},
		{"free sets", 3, 1, "0.00", "300.00", "300.00", "0.00", "300.00", "0.00"},
		{"zero discount", 3, 1, "100.00", "300.00", "300.00", "300.00", "0.00", "300.00"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := NewPricingService().Calculate(
				[]models.PricingCartLine{{ProductID: 1, Quantity: test.quantity, UnitPrice: pricingTestAmount(t, "100.00")}},
				[]models.PricingPromotion{{
					PromotionID: 12, Name: "Repeated bundle", PromotionPrice: pricingTestAmount(t, test.price),
					Items: []models.PricingPromotionItem{{ProductID: 1, Quantity: test.required}},
				}},
			)
			if err != nil {
				t.Fatal(err)
			}
			assertPricingAmount(t, result.Subtotal, test.subtotal)
			assertPricingAmount(t, result.Discount, test.discount)
			assertPricingAmount(t, result.NetTotal, test.net)
			if result.AppliedPromotion == nil || result.AppliedPromotion.PromotionID != 12 {
				t.Fatalf("AppliedPromotion = %+v, want Promotion 12", result.AppliedPromotion)
			}
			assertPricingAmount(t, result.AppliedPromotion.OriginalBundlePrice, test.original)
			assertPricingAmount(t, result.AppliedPromotion.PromotionPrice, test.total)
			assertPricingAmount(t, result.AppliedPromotion.Discount, test.discount)
			assertPricingApplicationCount(t, result.AppliedPromotion, int64(test.quantity/test.required))
		})
	}
}

func TestPricingServiceLimitsRepeatedPromotionByEveryGroup(t *testing.T) {
	for _, quantity := range []int32{2, 5} {
		t.Run(fmt.Sprintf("first group quantity %d", quantity), func(t *testing.T) {
			result, err := NewPricingService().Calculate(
				[]models.PricingCartLine{
					{ProductID: 1, Quantity: quantity, UnitPrice: pricingTestAmount(t, "100.00")},
					{ProductID: 2, Quantity: 2, UnitPrice: pricingTestAmount(t, "50.00")},
				},
				[]models.PricingPromotion{{
					PromotionID: 1, PromotionPrice: pricingTestAmount(t, "120.00"),
					ItemGroups: []models.PricingPromotionItemGroup{
						{Options: []models.PricingPromotionItem{{ProductID: 1, Quantity: 1}}},
						{Options: []models.PricingPromotionItem{{ProductID: 2, Quantity: 1}}},
					},
				}},
			)
			if err != nil {
				t.Fatal(err)
			}
			if result.AppliedPromotion == nil {
				t.Fatal("AppliedPromotion is nil")
			}
			assertPricingAmount(t, result.AppliedPromotion.OriginalBundlePrice, "300.00")
			assertPricingAmount(t, result.AppliedPromotion.PromotionPrice, "240.00")
			assertPricingApplicationCount(t, result.AppliedPromotion, 2)
			assertPricingAmount(t, result.Discount, "60.00")
			wantNet := "240.00"
			if quantity == 5 {
				wantNet = "540.00"
			}
			assertPricingAmount(t, result.NetTotal, wantNet)
		})
	}
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

func TestPricingServiceMatchesORGroupsAndSelectsHighestEligibleOption(t *testing.T) {
	largeID := int64(22)
	xlargeID := int64(23)
	result, err := NewPricingService().Calculate(
		[]models.PricingCartLine{
			{ProductID: 10, VariantID: &largeID, Quantity: 1, UnitPrice: pricingTestAmount(t, "300.00")},
			{ProductID: 10, VariantID: &xlargeID, Quantity: 1, UnitPrice: pricingTestAmount(t, "350.00")},
			{ProductID: 11, Quantity: 1, UnitPrice: pricingTestAmount(t, "50.00")},
		},
		[]models.PricingPromotion{{
			PromotionID:    1,
			Name:           "Shirt or shirt plus pin",
			PromotionPrice: pricingTestAmount(t, "300.00"),
			ItemGroups: []models.PricingPromotionItemGroup{
				{Options: []models.PricingPromotionItem{
					{ProductID: 10, VariantID: &largeID, Quantity: 1},
					{ProductID: 10, VariantID: &xlargeID, Quantity: 1},
				}},
				{Options: []models.PricingPromotionItem{{ProductID: 11, Quantity: 1}}},
			},
		}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.AppliedPromotion == nil {
		t.Fatal("AppliedPromotion is nil")
	}
	assertPricingAmount(t, result.AppliedPromotion.OriginalBundlePrice, "400.00")
	assertPricingAmount(t, result.Discount, "100.00")
	assertPricingAmount(t, result.NetTotal, "600.00")
}

func TestPricingServiceRequiresEveryORGroup(t *testing.T) {
	largeID := int64(22)
	xlargeID := int64(23)
	result, err := NewPricingService().Calculate(
		[]models.PricingCartLine{{ProductID: 10, VariantID: &largeID, Quantity: 1, UnitPrice: pricingTestAmount(t, "300.00")}},
		[]models.PricingPromotion{{
			PromotionID:    1,
			PromotionPrice: pricingTestAmount(t, "250.00"),
			ItemGroups: []models.PricingPromotionItemGroup{
				{Options: []models.PricingPromotionItem{{ProductID: 10, VariantID: &largeID, Quantity: 1}, {ProductID: 10, VariantID: &xlargeID, Quantity: 1}}},
				{Options: []models.PricingPromotionItem{{ProductID: 11, Quantity: 1}}},
			},
		}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.AppliedPromotion != nil {
		t.Fatalf("AppliedPromotion = %+v, want nil", result.AppliedPromotion)
	}
}

func TestPricingServiceRepeatsORGroups(t *testing.T) {
	largeID, xlargeID := int64(22), int64(23)
	tests := []struct {
		name                    string
		pins                    int32
		original, discount, net string
	}{
		{"one set prefers XL", 1, "400.00", "100.00", "600.00"},
		{"L and XL make two sets", 2, "750.00", "150.00", "600.00"},
		{"extra pin remains full price", 3, "750.00", "150.00", "650.00"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := NewPricingService().Calculate(
				[]models.PricingCartLine{
					{ProductID: 10, VariantID: &largeID, Quantity: 1, UnitPrice: pricingTestAmount(t, "300.00")},
					{ProductID: 10, VariantID: &xlargeID, Quantity: 1, UnitPrice: pricingTestAmount(t, "350.00")},
					{ProductID: 11, Quantity: test.pins, UnitPrice: pricingTestAmount(t, "50.00")},
				},
				[]models.PricingPromotion{{
					PromotionID: 1, PromotionPrice: pricingTestAmount(t, "300.00"),
					ItemGroups: []models.PricingPromotionItemGroup{
						{Options: []models.PricingPromotionItem{
							{ProductID: 10, VariantID: &largeID, Quantity: 1},
							{ProductID: 10, VariantID: &xlargeID, Quantity: 1},
						}},
						{Options: []models.PricingPromotionItem{{ProductID: 11, Quantity: 1}}},
					},
				}},
			)
			if err != nil {
				t.Fatal(err)
			}
			if result.AppliedPromotion == nil {
				t.Fatal("AppliedPromotion is nil")
			}
			assertPricingAmount(t, result.AppliedPromotion.OriginalBundlePrice, test.original)
			assertPricingAmount(t, result.Discount, test.discount)
			assertPricingAmount(t, result.NetTotal, test.net)
		})
	}
}

func TestPricingServiceRanksOROptionsByRequiredQuantityValue(t *testing.T) {
	cart := []models.PricingCartLine{
		{ProductID: 1, Quantity: 2, UnitPrice: pricingTestAmount(t, "100.00")},
		{ProductID: 2, Quantity: 5, UnitPrice: pricingTestAmount(t, "60.00")},
		{ProductID: 3, Quantity: 3, UnitPrice: pricingTestAmount(t, "10.00")},
	}
	promotions := []models.PricingPromotion{{
		PromotionID: 1, PromotionPrice: pricingTestAmount(t, "100.00"),
		ItemGroups: []models.PricingPromotionItemGroup{
			{Options: []models.PricingPromotionItem{{ProductID: 1, Quantity: 1}, {ProductID: 2, Quantity: 2}}},
			{Options: []models.PricingPromotionItem{{ProductID: 3, Quantity: 1}}},
		},
	}}
	cartBefore := append([]models.PricingCartLine(nil), cart...)
	optionsBefore := append([]models.PricingPromotionItem(nil), promotions[0].ItemGroups[0].Options...)
	result, err := NewPricingService().Calculate(cart, promotions)
	if err != nil {
		t.Fatal(err)
	}
	if result.AppliedPromotion == nil {
		t.Fatal("AppliedPromotion is nil")
	}
	// Two 2x60 options outrank 1x100; only one of the two 1x100 options fits.
	assertPricingAmount(t, result.Subtotal, "530.00")
	assertPricingAmount(t, result.AppliedPromotion.OriginalBundlePrice, "370.00")
	assertPricingAmount(t, result.AppliedPromotion.PromotionPrice, "300.00")
	assertPricingAmount(t, result.Discount, "70.00")
	assertPricingAmount(t, result.NetTotal, "460.00")
	if !reflect.DeepEqual(cart, cartBefore) || !reflect.DeepEqual(promotions[0].ItemGroups[0].Options, optionsBefore) {
		t.Fatal("pricing mutated the cart or sorted the input options")
	}
}

func TestPricingServicePoolsSelectedVariantQuantities(t *testing.T) {
	variants := []int64{21, 22, 23, 24}
	tests := []struct {
		name                    string
		quantities              [4]int32
		unitPrices              [3]string
		price                   string
		original, discount, net string
	}{
		{name: "XS and S", quantities: [4]int32{1, 1}, original: "800.00", discount: "400.00", net: "400.00"},
		{name: "two XS", quantities: [4]int32{2}, original: "800.00", discount: "400.00", net: "400.00"},
		{name: "two XS and two S", quantities: [4]int32{2, 2}, original: "1600.00", discount: "800.00", net: "800.00"},
		{name: "three XS and one S", quantities: [4]int32{3, 1}, original: "1600.00", discount: "800.00", net: "800.00"},
		{name: "three selected sizes", quantities: [4]int32{1, 1, 1}, original: "800.00", discount: "400.00", net: "800.00"},
		{name: "incomplete set", quantities: [4]int32{1}, discount: "0.00", net: "400.00"},
		{name: "unselected size cannot complete set", quantities: [4]int32{1, 0, 0, 1}, discount: "0.00", net: "800.00"},
		{name: "unselected size stays full price", quantities: [4]int32{1, 1, 0, 2}, original: "800.00", discount: "400.00", net: "1200.00"},
		{name: "different prices retain fixed set price", quantities: [4]int32{1, 1}, unitPrices: [3]string{"400.00", "450.00", "500.00"}, original: "850.00", discount: "450.00", net: "400.00"},
		{name: "prefer expensive units", quantities: [4]int32{1, 1, 1}, unitPrices: [3]string{"400.00", "450.00", "500.00"}, original: "950.00", discount: "550.00", net: "800.00"},
		{name: "zero discount", quantities: [4]int32{1, 1}, price: "800.00", original: "800.00", discount: "0.00", net: "800.00"},
		{name: "free set", quantities: [4]int32{1, 1}, price: "0.00", original: "800.00", discount: "800.00", net: "0.00"},
		{name: "more than int32 selected units", quantities: [4]int32{math.MaxInt32, math.MaxInt32}, unitPrices: [3]string{"1.00", "1.00", "1.00"}, price: "1.00", original: "4294967294.00", discount: "2147483647.00", net: "2147483647.00"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var cart []models.PricingCartLine
			options := make([]models.PricingPromotionItem, 3)
			for index := range variants {
				unitPrice := "400.00"
				if index < 3 {
					options[index] = models.PricingPromotionItem{ProductID: 10, VariantID: &variants[index], Quantity: 2}
					if test.unitPrices[index] != "" {
						unitPrice = test.unitPrices[index]
					}
				}
				if test.quantities[index] > 0 {
					cart = append(cart, models.PricingCartLine{ProductID: 10, VariantID: &variants[index], Quantity: test.quantities[index], UnitPrice: pricingTestAmount(t, unitPrice)})
				}
			}
			price := test.price
			if price == "" {
				price = "400.00"
			}
			promotion := models.PricingPromotion{PromotionID: 1, PromotionPrice: pricingTestAmount(t, price), ItemGroups: []models.PricingPromotionItemGroup{{Options: options}}}
			cartBefore := append([]models.PricingCartLine(nil), cart...)
			optionsBefore := append([]models.PricingPromotionItem(nil), options...)
			result, err := NewPricingService().Calculate(cart, []models.PricingPromotion{promotion})
			if err != nil {
				t.Fatal(err)
			}
			assertPricingAmount(t, result.Discount, test.discount)
			assertPricingAmount(t, result.NetTotal, test.net)
			if test.original == "" {
				if result.AppliedPromotion != nil {
					t.Fatalf("promotion=%+v, want no complete set", result.AppliedPromotion)
				}
			} else {
				if result.AppliedPromotion == nil {
					t.Fatal("AppliedPromotion is nil")
				}
				assertPricingAmount(t, result.AppliedPromotion.OriginalBundlePrice, test.original)
				selected := int64(test.quantities[0]) + int64(test.quantities[1]) + int64(test.quantities[2])
				assertPricingApplicationCount(t, result.AppliedPromotion, selected/2)
			}
			if !reflect.DeepEqual(cart, cartBefore) || !reflect.DeepEqual(options, optionsBefore) {
				t.Fatal("pricing mutated the cart or variant options")
			}
		})
	}
}

func TestPricingServiceComparesRepeatedDiscounts(t *testing.T) {
	for _, price := range []string{"70.00", "75.00"} {
		t.Run(price, func(t *testing.T) {
			promotions := []models.PricingPromotion{
				{PromotionID: 9, PromotionPrice: pricingTestAmount(t, price), Items: []models.PricingPromotionItem{{ProductID: 1, Quantity: 1}}},
				{PromotionID: 4, PromotionPrice: pricingTestAmount(t, "150.00"), Items: []models.PricingPromotionItem{{ProductID: 1, Quantity: 2}}},
			}
			for _, ordered := range [][]models.PricingPromotion{promotions, {promotions[1], promotions[0]}} {
				result, err := NewPricingService().Calculate(
					[]models.PricingCartLine{{ProductID: 1, Quantity: 4, UnitPrice: pricingTestAmount(t, "100.00")}}, ordered,
				)
				if err != nil {
					t.Fatal(err)
				}
				wantID, discount := int64(9), "120.00"
				if price == "75.00" {
					wantID, discount = 4, "100.00"
				}
				if result.AppliedPromotion == nil || result.AppliedPromotion.PromotionID != wantID {
					t.Fatalf("promotion = %+v, want %d", result.AppliedPromotion, wantID)
				}
				assertPricingAmount(t, result.Discount, discount)
				wantApplications := int64(4)
				if wantID == 4 {
					wantApplications = 2
				}
				assertPricingApplicationCount(t, result.AppliedPromotion, wantApplications)
			}
		})
	}
}

func TestPricingServiceKeepsNonUniformGroupsAsAlternatives(t *testing.T) {
	variants := []int64{21, 22, 23}
	for _, test := range []struct {
		name      string
		productID int64
		required  int32
		variantID *int64
	}{
		{"absent option has different quantity", 10, 3, &variants[2]},
		{"absent option has different product", 11, 2, &variants[2]},
		{"absent option is variantless", 10, 2, nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			result, err := NewPricingService().Calculate(
				[]models.PricingCartLine{
					{ProductID: 10, VariantID: &variants[0], Quantity: 1, UnitPrice: pricingTestAmount(t, "400.00")},
					{ProductID: 10, VariantID: &variants[1], Quantity: 1, UnitPrice: pricingTestAmount(t, "400.00")},
				},
				[]models.PricingPromotion{{
					PromotionID: 1, PromotionPrice: pricingTestAmount(t, "400.00"),
					ItemGroups: []models.PricingPromotionItemGroup{{Options: []models.PricingPromotionItem{
						{ProductID: 10, VariantID: &variants[0], Quantity: 2},
						{ProductID: 10, VariantID: &variants[1], Quantity: 2},
						{ProductID: test.productID, VariantID: test.variantID, Quantity: test.required},
					}}},
				}},
			)
			if err != nil {
				t.Fatal(err)
			}
			if result.AppliedPromotion != nil {
				t.Fatalf("nonuniform group unexpectedly pooled: %+v", result.AppliedPromotion)
			}
			assertPricingAmount(t, result.Discount, "0.00")
			assertPricingAmount(t, result.NetTotal, "800.00")
		})
	}
}

func TestPricingServiceLimitsPooledVariantsByOtherGroups(t *testing.T) {
	variants := []int64{21, 22}
	for _, test := range []struct {
		name          string
		shirts, pins  int32
		discount, net string
	}{
		{"one pooled set and pin", 1, 1, "450.00", "400.00"},
		{"pin limits repeated sets", 3, 2, "900.00", "1600.00"},
		{"missing required pin", 1, 0, "0.00", "800.00"},
	} {
		t.Run(test.name, func(t *testing.T) {
			cart := []models.PricingCartLine{
				{ProductID: 10, VariantID: &variants[0], Quantity: test.shirts, UnitPrice: pricingTestAmount(t, "400.00")},
				{ProductID: 10, VariantID: &variants[1], Quantity: test.shirts, UnitPrice: pricingTestAmount(t, "400.00")},
			}
			if test.pins > 0 {
				cart = append(cart, models.PricingCartLine{ProductID: 11, Quantity: test.pins, UnitPrice: pricingTestAmount(t, "50.00")})
			}
			result, err := NewPricingService().Calculate(cart, []models.PricingPromotion{{
				PromotionID: 1, PromotionPrice: pricingTestAmount(t, "400.00"),
				ItemGroups: []models.PricingPromotionItemGroup{
					{Options: []models.PricingPromotionItem{{ProductID: 10, VariantID: &variants[0], Quantity: 2}, {ProductID: 10, VariantID: &variants[1], Quantity: 2}}},
					{Options: []models.PricingPromotionItem{{ProductID: 11, Quantity: 1}}},
				},
			}})
			if err != nil {
				t.Fatal(err)
			}
			assertPricingAmount(t, result.Discount, test.discount)
			assertPricingAmount(t, result.NetTotal, test.net)
		})
	}
}

func TestPricingServiceComparesPooledVariantDiscounts(t *testing.T) {
	small, medium := int64(21), int64(22)
	for _, test := range []struct {
		name, alternativePrice string
		wantID                 int64
	}{
		{"pooled discount beats single variant", "100.00", 9},
		{"equal discounts prefer lower ID", "0.00", 4},
	} {
		t.Run(test.name, func(t *testing.T) {
			promotions := []models.PricingPromotion{
				{PromotionID: 9, PromotionPrice: pricingTestAmount(t, "400.00"), ItemGroups: []models.PricingPromotionItemGroup{{Options: []models.PricingPromotionItem{
					{ProductID: 10, VariantID: &small, Quantity: 2}, {ProductID: 10, VariantID: &medium, Quantity: 2},
				}}}},
				{PromotionID: 4, PromotionPrice: pricingTestAmount(t, test.alternativePrice), Items: []models.PricingPromotionItem{{ProductID: 10, VariantID: &small, Quantity: 1}}},
			}
			for _, ordered := range [][]models.PricingPromotion{promotions, {promotions[1], promotions[0]}} {
				result, err := NewPricingService().Calculate([]models.PricingCartLine{
					{ProductID: 10, VariantID: &small, Quantity: 1, UnitPrice: pricingTestAmount(t, "400.00")},
					{ProductID: 10, VariantID: &medium, Quantity: 1, UnitPrice: pricingTestAmount(t, "400.00")},
				}, ordered)
				if err != nil {
					t.Fatal(err)
				}
				if result.AppliedPromotion == nil || result.AppliedPromotion.PromotionID != test.wantID {
					t.Fatalf("promotion=%+v, want %d", result.AppliedPromotion, test.wantID)
				}
				assertPricingAmount(t, result.Discount, "400.00")
			}
		})
	}
}

func TestPricingServiceHandlesMoreThanInt32Applications(t *testing.T) {
	result, err := NewPricingService().Calculate(
		[]models.PricingCartLine{
			{ProductID: 1, Quantity: math.MaxInt32, UnitPrice: pricingTestAmount(t, "1.00")},
			{ProductID: 2, Quantity: math.MaxInt32, UnitPrice: pricingTestAmount(t, "1.00")},
		},
		[]models.PricingPromotion{{
			PromotionID: 1, PromotionPrice: pricingTestAmount(t, "0.50"),
			ItemGroups: []models.PricingPromotionItemGroup{{Options: []models.PricingPromotionItem{
				{ProductID: 1, Quantity: 1}, {ProductID: 2, Quantity: 1},
			}}},
		}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.AppliedPromotion == nil {
		t.Fatal("AppliedPromotion is nil")
	}
	assertPricingAmount(t, result.Subtotal, "4294967294.00")
	assertPricingAmount(t, result.AppliedPromotion.OriginalBundlePrice, "4294967294.00")
	assertPricingAmount(t, result.AppliedPromotion.PromotionPrice, "2147483647.00")
	assertPricingAmount(t, result.Discount, "2147483647.00")
	assertPricingAmount(t, result.NetTotal, "2147483647.00")
}

func TestPricingServiceRejectsRepeatedPromotionPriceOverflow(t *testing.T) {
	maximum, err := models.NewTHBAmountFromSatang(math.MaxInt64)
	if err != nil {
		t.Fatal(err)
	}
	result, err := NewPricingService().Calculate(
		[]models.PricingCartLine{{ProductID: 1, Quantity: 2, UnitPrice: pricingTestAmount(t, "1.00")}},
		[]models.PricingPromotion{{PromotionID: 1, PromotionPrice: maximum, Items: []models.PricingPromotionItem{{ProductID: 1, Quantity: 1}}}},
	)
	if !errors.Is(err, ErrPricingInvalidPromotion) || !errors.Is(err, models.ErrTHBAmountOverflow) {
		t.Fatalf("error = %v, want invalid promotion with THB overflow", err)
	}
	if result != (models.PricingResult{}) {
		t.Fatalf("result on error = %+v, want zero result", result)
	}
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
