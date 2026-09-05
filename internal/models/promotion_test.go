package models

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestProjectPromotionMutationRequestJSONUsesPublicReferences(t *testing.T) {
	price, err := ParseTHBAmount("329.00")
	if err != nil {
		t.Fatal(err)
	}
	request := ProjectPromotionMutationRequest{
		Name:           "Shirt and pin set",
		PromotionPrice: &price,
		Items: []ProjectPromotionItemInput{{
			ProductID: 10,
			VariantID: nil,
			Quantity:  2,
		}},
	}

	encoded, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	value := string(encoded)
	for _, want := range []string{`"name":"Shirt and pin set"`, `"promotion_price":"329.00"`, `"product_id":10`, `"variant_id":null`, `"quantity":2`} {
		if !strings.Contains(value, want) {
			t.Errorf("encoded request %s does not contain %s", value, want)
		}
	}
	if strings.Contains(value, "project_product_id") {
		t.Fatalf("encoded request exposed internal project_product_id: %s", value)
	}

	var decoded ProjectPromotionMutationRequest
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Name != request.Name {
		t.Fatalf("decoded name=%v", decoded.Name)
	}
	if decoded.PromotionPrice == nil || decoded.PromotionPrice.String() != price.String() {
		t.Fatalf("decoded promotion price=%v", decoded.PromotionPrice)
	}
	if decoded.Items == nil || len(decoded.Items) != 1 {
		t.Fatalf("decoded items=%v", decoded.Items)
	}
	if item := decoded.Items[0]; item.ProductID != request.Items[0].ProductID || item.VariantID != nil || item.Quantity != request.Items[0].Quantity {
		t.Fatalf("decoded item=%+v", item)
	}
}

func TestProjectPromotionMutationRequestMissingItemsDecodesToNilSlice(t *testing.T) {
	var request ProjectPromotionMutationRequest
	if err := json.Unmarshal([]byte(`{"name":"Navy","promotion_price":"0.00"}`), &request); err != nil {
		t.Fatal(err)
	}
	if request.Items != nil {
		t.Fatalf("missing items should decode to a nil slice: %#v", request.Items)
	}
	if request.Name != "Navy" || request.PromotionPrice == nil || request.PromotionPrice.String() != "0.00" {
		t.Fatalf("decoded request=%+v", request)
	}
}

func TestProjectPromotionItemInputTreatsMissingVariantAsNull(t *testing.T) {
	var omitted ProjectPromotionItemInput
	if err := json.Unmarshal([]byte(`{"product_id":10,"quantity":1}`), &omitted); err != nil {
		t.Fatal(err)
	}

	var explicitNull ProjectPromotionItemInput
	if err := json.Unmarshal([]byte(`{"product_id":10,"variant_id":null,"quantity":1}`), &explicitNull); err != nil {
		t.Fatal(err)
	}

	if omitted.VariantID != nil || explicitNull.VariantID != nil {
		t.Fatalf("missing and null variant_id should both decode to nil: omitted=%+v explicitNull=%+v", omitted, explicitNull)
	}
}

func TestProjectPromotionResponseJSONMatchesOpenAPIShape(t *testing.T) {
	price, err := ParseTHBAmount("299.00")
	if err != nil {
		t.Fatal(err)
	}
	discount, err := ParseTHBAmount("29.00")
	if err != nil {
		t.Fatal(err)
	}
	name := "Navy"
	promotion := ProjectPromotion{
		PromotionID:         3,
		ProjectID:           7,
		Name:                "Shirt and pin set",
		Items:               []ProjectPromotionItem{{ProductID: 10, Quantity: 1, ProductName: "Intania Shirt", Size: &name, UnitPrice: price}},
		OriginalBundlePrice: price,
		PromotionPrice:      price,
		Discount:            discount,
		CreatedAt:           time.Date(2026, time.August, 26, 4, 0, 0, 0, time.UTC),
		UpdatedAt:           time.Date(2026, time.August, 26, 4, 0, 0, 0, time.UTC),
	}
	response := ProjectPromotionEnvelope{Success: true, Data: promotion}

	encoded, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	value := string(encoded)
	for _, want := range []string{`"success":true`, `"promotion_id":3`, `"project_id":7`, `"unit_price":"299.00"`, `"original_bundle_price":"299.00"`, `"promotion_price":"299.00"`, `"discount":"29.00"`} {
		if !strings.Contains(value, want) {
			t.Errorf("encoded response %s does not contain %s", value, want)
		}
	}
}

func TestProjectPromotionListDataIsUnpaginated(t *testing.T) {
	response := ProjectPromotionListEnvelope{
		Success: true,
		Data:    ProjectPromotionListData{Promotions: []ProjectPromotion{{PromotionID: 1}, {PromotionID: 2}}},
	}
	encoded, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	value := string(encoded)
	if strings.Contains(value, "page_size") || strings.Contains(value, "total_pages") {
		t.Fatalf("promotion list unexpectedly contains pagination fields: %s", value)
	}
	if !strings.Contains(value, `"promotions":[`) {
		t.Fatalf("promotion list missing promotions array: %s", value)
	}
}
