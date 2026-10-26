package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/esc-chula/intania-shop-api/internal/middlewares"
	"github.com/esc-chula/intania-shop-api/internal/models"
	"github.com/esc-chula/intania-shop-api/internal/usecases"
	"github.com/go-chi/chi/v5"
)

func TestInventoryHandlerAcceptsAndValidatesOperatorName(t *testing.T) {
	for _, test := range []struct {
		name    string
		field   string
		want    string
		invalid bool
	}{
		{name: "omitted", want: "Shared account"},
		{name: "null", field: `,"operator_name":null`, want: "Shared account"},
		{name: "blank", field: `,"operator_name":"   "`, want: "Shared account"},
		{name: "explicit", field: `,"operator_name":"  มายด์  "`, want: "มายด์"},
		{name: "too long", field: `,"operator_name":"` + strings.Repeat("ก", 101) + `"`, invalid: true},
		{name: "numeric", field: `,"operator_name":123`, invalid: true},
		{name: "object", field: `,"operator_name":{}`, invalid: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := &inventoryHTTPStoreStub{}
			router := chi.NewRouter()
			router.Use(middlewares.Authenticate(posCheckoutVerifierStub{}))
			NewInventoryHandler(usecases.NewInventoryService(store)).Register(router)
			request := httptest.NewRequest(http.MethodPost, "/products/1/stock/adjust",
				strings.NewReader(`{"quantity_change":-1,"reason":"Damaged"`+test.field+`}`))
			request.Header.Set("Authorization", "Bearer staff-token")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if test.invalid {
				if response.Code != http.StatusBadRequest || store.calls != 0 {
					t.Fatalf("invalid adjustment status/calls = %d/%d: %s", response.Code, store.calls, response.Body.String())
				}
				return
			}
			if response.Code != http.StatusCreated || store.calls != 1 || store.actor != "42" {
				t.Fatalf("adjustment status/actor = %d/%q: %s", response.Code, store.actor, response.Body.String())
			}
			var envelope struct {
				Data models.StockTransaction `json:"data"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
				t.Fatal(err)
			}
			if envelope.Data.CreatedBy == nil || *envelope.Data.CreatedBy != test.want {
				t.Fatalf("response creator = %v, want %q", envelope.Data.CreatedBy, test.want)
			}
		})
	}
}

type inventoryHTTPStoreStub struct {
	usecases.StockAdjuster
	calls int
	actor string
}

func (stub *inventoryHTTPStoreStub) Adjust(_ context.Context, _ int64, request models.AdjustStockRequest, actor string) (models.StockTransaction, error) {
	stub.calls++
	stub.actor = actor
	name := "Shared account"
	if request.OperatorName != nil {
		name = *request.OperatorName
	}
	return models.StockTransaction{CreatedBy: &name}, nil
}
