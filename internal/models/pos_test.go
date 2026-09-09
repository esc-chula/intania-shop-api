package models

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestPOSQuoteUsesCanonicalMoneyAndNullablePromotion(t *testing.T) {
	amount, err := ParseTHBAmount("358.00")
	if err != nil {
		t.Fatal(err)
	}

	quote := POSQuote{
		ProjectID: 7,
		Items: []POSQuoteLineItem{{
			ProductID:         10,
			VariantID:         nil,
			ProductName:       "Enamel Pin",
			Quantity:          1,
			UnitPrice:         amount,
			LineTotal:         amount,
			AvailableQuantity: 20,
		}},
		Subtotal:         amount,
		AppliedPromotion: nil,
		Discount:         mustPOSAmount(t, "0.00"),
		NetTotal:         amount,
		QuotedAt:         time.Date(2026, time.September, 10, 10, 30, 0, 0, time.FixedZone("Asia/Bangkok", 7*60*60)),
	}

	data, err := json.Marshal(quote)
	if err != nil {
		t.Fatal(err)
	}
	serialized := string(data)
	for _, expected := range []string{
		`"project_id":7`,
		`"variant_id":null`,
		`"unit_price":"358.00"`,
		`"line_total":"358.00"`,
		`"subtotal":"358.00"`,
		`"applied_promotion":null`,
		`"discount":"0.00"`,
		`"net_total":"358.00"`,
	} {
		if !strings.Contains(serialized, expected) {
			t.Fatalf("JSON = %s, missing %s", serialized, expected)
		}
	}

	if strings.Contains(serialized, `"subtotal":358`) || strings.Contains(serialized, `"discount":0`) {
		t.Fatalf("money must be JSON strings: %s", serialized)
	}
}

func TestPOSCartRequestPreservesVariantlessIdentity(t *testing.T) {
	var request POSCartRequest
	if err := json.Unmarshal([]byte(`{"items":[{"product_id":11,"variant_id":null,"quantity":2}]}`), &request); err != nil {
		t.Fatal(err)
	}
	if len(request.Items) != 1 || request.Items[0].ProductID != 11 || request.Items[0].VariantID != nil || request.Items[0].Quantity != 2 {
		t.Fatalf("request = %+v", request)
	}
}

func TestPOSAPIErrorCodes(t *testing.T) {
	if ProjectErrorNotActive != ProjectAPIErrorCode("PROJECT_NOT_ACTIVE") {
		t.Fatalf("ProjectErrorNotActive = %q", ProjectErrorNotActive)
	}
	if ProjectErrorInsufficientStock != ProjectAPIErrorCode("INSUFFICIENT_STOCK") {
		t.Fatalf("ProjectErrorInsufficientStock = %q", ProjectErrorInsufficientStock)
	}
}

func mustPOSAmount(t *testing.T, value string) THBAmount {
	t.Helper()
	amount, err := ParseTHBAmount(value)
	if err != nil {
		t.Fatalf("parse amount %q: %v", value, err)
	}
	return amount
}
