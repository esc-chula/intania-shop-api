package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/esc-chula/intania-shop-api/internal/middlewares"
	"github.com/esc-chula/intania-shop-api/internal/models"
	"github.com/go-chi/chi/v5"
)

type inventoryVerifier struct {
	identity models.Identity
	err      error
}

func (v inventoryVerifier) Verify(string) (models.Identity, error) { return v.identity, v.err }

type inventoryStub struct {
	actor string
	calls int
}

func (s *inventoryStub) Adjust(_ context.Context, _ int64, _ models.AdjustStockRequest, actor string) (models.StockTransaction, error) {
	s.actor = actor
	s.calls++
	return models.StockTransaction{TransactionID: 1}, nil
}
func (s *inventoryStub) ProductTransactions(context.Context, int32, int32, int32) (models.StockTransactionListResponse, error) {
	return models.StockTransactionListResponse{}, nil
}
func (s *inventoryStub) VariantTransactions(context.Context, int32, int32, int32) (models.StockTransactionListResponse, error) {
	return models.StockTransactionListResponse{}, nil
}
func (s *inventoryStub) Transactions(context.Context, int32, int32) (models.StockTransactionListResponse, error) {
	return models.StockTransactionListResponse{}, nil
}
func (s *inventoryStub) BulkReduction(context.Context, models.BulkStockReductionRequest) (models.BulkStockReductionResponse, error) {
	return models.BulkStockReductionResponse{}, nil
}

var _ middlewares.TokenVerifier = inventoryVerifier{}

func TestInventoryHandlerAuthorization(t *testing.T) {
	tests := []struct {
		name      string
		verifier  inventoryVerifier
		header    string
		want      int
		wantCalls int
	}{
		{"missing token", inventoryVerifier{}, "", http.StatusUnauthorized, 0},
		{"invalid token", inventoryVerifier{err: errors.New("bad")}, "Bearer token", http.StatusUnauthorized, 0},
		{"user forbidden", inventoryVerifier{identity: models.Identity{UserID: 9, Role: models.RoleUser}}, "Bearer token", http.StatusForbidden, 0},
		{"admin succeeds", inventoryVerifier{identity: models.Identity{UserID: 42, Role: models.RoleAdmin}}, "Bearer token", http.StatusCreated, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &inventoryStub{}
			mux := chi.NewRouter()
			admin := mux.With(middlewares.Authenticate(tt.verifier), middlewares.RequireRole(models.RoleAdmin))
			NewInventoryHandler(service).Register(admin)
			request := httptest.NewRequest(http.MethodPost, "/products/3/stock/adjust", strings.NewReader(`{"quantity_change":1,"reason":"STOCK_RECEIVED"}`))
			request.Header.Set("Authorization", tt.header)
			response := httptest.NewRecorder()
			mux.ServeHTTP(response, request)
			if response.Code != tt.want {
				t.Fatalf("status=%d want %d: %s", response.Code, tt.want, response.Body.String())
			}
			if service.calls != tt.wantCalls {
				t.Fatalf("calls=%d want %d", service.calls, tt.wantCalls)
			}
			if tt.wantCalls > 0 && service.actor != "42" {
				t.Fatalf("actor=%q", service.actor)
			}
		})
	}
}
