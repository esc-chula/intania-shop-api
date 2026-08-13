package usecases

import (
	"context"
	"github.com/esc-chula/intania-shop-api/internal/models"
	"testing"
)

type inventoryUsecaseStub struct{ bulkCalls int }

func (s *inventoryUsecaseStub) Adjust(context.Context, int64, models.AdjustStockRequest, string) (models.StockTransaction, error) {
	return models.StockTransaction{}, nil
}
func (s *inventoryUsecaseStub) ListProductTransactions(context.Context, int64, int32, int32) (models.StockTransactionListResponse, error) {
	return models.StockTransactionListResponse{}, nil
}
func (s *inventoryUsecaseStub) ListVariantTransactions(context.Context, int64, int32, int32) (models.StockTransactionListResponse, error) {
	return models.StockTransactionListResponse{}, nil
}
func (s *inventoryUsecaseStub) ListTransactions(context.Context, int32, int32) (models.StockTransactionListResponse, error) {
	return models.StockTransactionListResponse{}, nil
}
func (s *inventoryUsecaseStub) BulkReduction(context.Context, models.BulkStockReductionRequest) (models.BulkStockReductionResponse, error) {
	s.bulkCalls++
	return models.BulkStockReductionResponse{}, nil
}

func TestBulkReductionValidation(t *testing.T) {
	valid := models.BulkStockReductionRequest{Reason: "sale", Operator: "admin", Gender: "X", PaymentType: "REAL_MONEY", TotalMoneyReceive: "1.50", Items: []models.BulkStockReductionItem{{ProductID: 1, Quantity: 1}}}
	tests := []struct {
		name    string
		request models.BulkStockReductionRequest
		wantErr bool
	}{
		{"valid", valid, false},
		{"blank reason", models.BulkStockReductionRequest{}, true},
		{"invalid payment", func() models.BulkStockReductionRequest { r := valid; r.PaymentType = "CASH"; return r }(), true},
		{"zero amount", func() models.BulkStockReductionRequest { r := valid; r.TotalMoneyReceive = "0"; return r }(), true},
		{"invalid item", func() models.BulkStockReductionRequest {
			r := valid
			r.Items = []models.BulkStockReductionItem{{ProductID: 0, Quantity: 1}}
			return r
		}(), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stub := &inventoryUsecaseStub{}
			_, err := NewInventoryService(stub).BulkReduction(context.Background(), tt.request)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tt.wantErr)
			}
			if stub.bulkCalls == 0 && !tt.wantErr {
				t.Fatal("valid request did not reach store")
			}
			if stub.bulkCalls != 0 && tt.wantErr {
				t.Fatal("invalid request reached store")
			}
		})
	}
}
