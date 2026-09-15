package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/esc-chula/intania-shop-api/internal/models"
	"github.com/esc-chula/intania-shop-api/internal/usecases"
	"github.com/go-chi/chi/v5"
)

func TestOrderHandlersRejectInvalidStaffIDBeforeCallingService(t *testing.T) {
	tests := []struct {
		name   string
		target string
	}{
		{name: "list non numeric", target: "/projects/1/orders?staff_id=abc"},
		{name: "list blank", target: "/projects/1/orders?staff_id="},
		{name: "list zero", target: "/projects/1/orders?staff_id=0"},
		{name: "list negative", target: "/projects/1/orders?staff_id=-1"},
		{name: "list overflow", target: "/projects/1/orders?staff_id=9223372036854775808"},
		{name: "list repeated", target: "/projects/1/orders?staff_id=1&staff_id=2"},
		{name: "export non numeric", target: "/projects/1/orders/export?staff_id=abc"},
		{name: "export blank", target: "/projects/1/orders/export?staff_id="},
		{name: "export zero", target: "/projects/1/orders/export?staff_id=0"},
		{name: "export negative", target: "/projects/1/orders/export?staff_id=-1"},
		{name: "export overflow", target: "/projects/1/orders/export?staff_id=9223372036854775808"},
		{name: "export repeated", target: "/projects/1/orders/export?staff_id=1&staff_id=2"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := &orderHTTPServiceStub{}
			response := serveOrderRequest(service, test.target)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body = %s", response.Code, http.StatusBadRequest, response.Body.String())
			}
			if service.listCalls != 0 || service.exportCalls != 0 {
				t.Fatalf("service calls = list %d/export %d, want 0/0", service.listCalls, service.exportCalls)
			}
			if !strings.Contains(response.Body.String(), "Invalid staff_id") {
				t.Fatalf("body = %s, want Invalid staff_id", response.Body.String())
			}
		})
	}
}

func TestOrderHandlersAcceptOmittedAndInt64StaffID(t *testing.T) {
	tests := []struct {
		name        string
		target      string
		wantStaffID int64
	}{
		{name: "list omitted", target: "/projects/1/orders", wantStaffID: 0},
		{name: "list int64", target: "/projects/1/orders?staff_id=2147483648", wantStaffID: 2147483648},
		{name: "export omitted", target: "/projects/1/orders/export", wantStaffID: 0},
		{name: "export int64", target: "/projects/1/orders/export?staff_id=2147483648", wantStaffID: 2147483648},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := &orderHTTPServiceStub{}
			response := serveOrderRequest(service, test.target)
			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d; body = %s", response.Code, http.StatusOK, response.Body.String())
			}
			query := service.listQuery
			if strings.Contains(test.target, "/export") {
				query = service.exportQuery
			}
			if query.StaffID != test.wantStaffID {
				t.Fatalf("staff ID = %d, want %d", query.StaffID, test.wantStaffID)
			}
		})
	}
}

type orderHTTPServiceStub struct {
	listCalls   int
	exportCalls int
	listQuery   usecases.OrderListQuery
	exportQuery usecases.OrderListQuery
}

func (stub *orderHTTPServiceStub) List(_ context.Context, _ int64, query usecases.OrderListQuery, _, _ int32) (models.OrderListResponse, error) {
	stub.listCalls++
	stub.listQuery = query
	return models.OrderListResponse{Orders: []models.POSOrder{}}, nil
}

func (stub *orderHTTPServiceStub) Export(_ context.Context, _ int64, query usecases.OrderListQuery) ([]models.POSOrder, error) {
	stub.exportCalls++
	stub.exportQuery = query
	return []models.POSOrder{}, nil
}

func (*orderHTTPServiceStub) Detail(context.Context, int64, int64) (models.POSOrder, error) {
	return models.POSOrder{}, nil
}

func serveOrderRequest(service OrderService, target string) *httptest.ResponseRecorder {
	router := chi.NewRouter()
	NewOrderHandler(service).Register(router)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, target, nil))
	return response
}
