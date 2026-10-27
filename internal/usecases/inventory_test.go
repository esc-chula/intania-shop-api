package usecases

import (
	"context"
	"strings"
	"testing"

	"github.com/esc-chula/intania-shop-api/internal/models"
)

func TestInventoryAdjustNormalizesOperatorNameBeforePersistence(t *testing.T) {
	for _, test := range []struct {
		name    string
		input   *string
		want    string
		invalid bool
	}{
		{name: "omitted or null"},
		{name: "empty", input: posStringPointer("")},
		{name: "blank", input: posStringPointer(" \t\n\u2003 ")},
		{name: "Thai name", input: posStringPointer("  มายด์  "), want: "มายด์"},
		{name: "100 Unicode characters", input: posStringPointer(" " + strings.Repeat("ก", 100) + " "), want: strings.Repeat("ก", 100)},
		{name: "101 Unicode characters", input: posStringPointer(strings.Repeat("ก", 101)), invalid: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := &inventoryOperatorStoreStub{}
			service := NewInventoryService(store)
			_, err := service.Adjust(context.Background(), 7, models.AdjustStockRequest{
				QuantityChange: -1, Reason: "Damaged", OperatorName: test.input,
			}, "42")
			if test.invalid {
				if err == nil || !strings.Contains(err.Error(), "Operator name cannot be longer than 100 characters") || store.calls != 0 {
					t.Fatalf("invalid name reached persistence: calls = %d, error = %v", store.calls, err)
				}
				return
			}
			if err != nil || store.calls != 1 || store.actor != "42" {
				t.Fatalf("calls = %d, actor = %q, error = %v", store.calls, store.actor, err)
			}
			if test.want == "" {
				if store.request.OperatorName != nil {
					t.Fatalf("blank operator = %v, want nil", store.request.OperatorName)
				}
			} else if store.request.OperatorName == nil || *store.request.OperatorName != test.want {
				t.Fatalf("operator = %v, want %q", store.request.OperatorName, test.want)
			}
		})
	}
}

type inventoryOperatorStoreStub struct {
	StockAdjuster
	calls   int
	actor   string
	request models.AdjustStockRequest
}

func (stub *inventoryOperatorStoreStub) Adjust(_ context.Context, _ int64, request models.AdjustStockRequest, actor string) (models.StockTransaction, error) {
	stub.calls++
	stub.actor = actor
	stub.request = request
	return models.StockTransaction{}, nil
}
