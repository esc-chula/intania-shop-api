package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/esc-chula/intania-shop-api/internal/models"
	"github.com/esc-chula/intania-shop-api/internal/repositories"
	"github.com/esc-chula/intania-shop-api/internal/usecases"
	"github.com/go-chi/chi/v5"
)

func TestPOSHandlerCatalogSuccess(t *testing.T) {
	service := &posHTTPServiceStub{
		catalogData: models.POSCatalog{
			Project:     models.Project{ProjectID: 7, Status: models.ProjectStatusActive},
			CanCheckout: true,
			Categories: []models.POSCategory{{
				Products: []models.ProjectProductAssignment{{ProductID: 10, ProjectPrice: "299.00"}},
			}},
		},
	}
	router := chi.NewRouter()
	NewPOSHandler(service).Register(router)

	response := servePOSRequest(router, http.MethodGet, "/projects/7/pos", "")
	if response.Code != http.StatusOK || service.catalogCalls != 1 || service.catalogProjectID != 7 {
		t.Fatalf("status/call/project = %d/%d/%d", response.Code, service.catalogCalls, service.catalogProjectID)
	}

	var body struct {
		Success bool              `json:"success"`
		Data    models.POSCatalog `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if !body.Success || body.Data.Project.ProjectID != 7 || !body.Data.CanCheckout {
		t.Fatalf("catalog response = %+v", body)
	}
}

func TestPOSHandlerQuoteSuccess(t *testing.T) {
	service := &posHTTPServiceStub{
		quoteData: models.POSQuote{
			ProjectID: 7,
			Subtotal:  handlerTestAmount(t, "20.00"),
			Discount:  handlerTestAmount(t, "0.00"),
			NetTotal:  handlerTestAmount(t, "20.00"),
		},
	}
	router := chi.NewRouter()
	NewPOSHandler(service).Register(router)

	body := `{"items":[{"product_id":10,"variant_id":null,"quantity":2}]}`
	response := servePOSRequest(router, http.MethodPost, "/projects/7/checkout/quote", body)
	if response.Code != http.StatusOK || service.quoteCalls != 1 || service.quoteProjectID != 7 {
		t.Fatalf("status/call/project = %d/%d/%d", response.Code, service.quoteCalls, service.quoteProjectID)
	}
	if len(service.quoteRequest.Items) != 1 || service.quoteRequest.Items[0].ProductID != 10 || service.quoteRequest.Items[0].Quantity != 2 {
		t.Fatalf("quote request = %+v", service.quoteRequest)
	}

	var envelope struct {
		Success bool            `json:"success"`
		Data    models.POSQuote `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	if !envelope.Success || envelope.Data.ProjectID != 7 || envelope.Data.NetTotal.String() != "20.00" {
		t.Fatalf("quote response = %+v", envelope)
	}
}

func TestPOSHandlerRejectsMalformedQuoteBodies(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "empty body", body: ""},
		{name: "malformed JSON", body: `{"items":[`},
		{name: "unknown money field", body: `{"items":[],"subtotal":"0.00"}`},
		{name: "unknown item price field", body: `{"items":[{"product_id":10,"quantity":1,"unit_price":"1.00"}]}`},
		{name: "trailing JSON", body: `{"items":[{"product_id":10,"quantity":1}]} {}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := &posHTTPServiceStub{}
			router := chi.NewRouter()
			NewPOSHandler(service).Register(router)

			response := servePOSRequest(router, http.MethodPost, "/projects/7/checkout/quote", test.body)
			if response.Code != http.StatusBadRequest || service.quoteCalls != 0 {
				t.Fatalf("status/calls = %d/%d; body = %s", response.Code, service.quoteCalls, response.Body.String())
			}
			if apiError := decodePOSAPIError(t, response); apiError.Code != models.ProjectErrorValidation {
				t.Fatalf("error code = %s", apiError.Code)
			}
		})
	}
}

func TestPOSHandlerRejectsOversizedQuoteBody(t *testing.T) {
	service := &posHTTPServiceStub{}
	router := chi.NewRouter()
	NewPOSHandler(service).Register(router)

	body := `{"items":[{"product_id":10,"quantity":1}],"padding":"` + strings.Repeat("x", posBodyLimit) + `"}`
	response := servePOSRequest(router, http.MethodPost, "/projects/7/checkout/quote", body)
	if response.Code != http.StatusBadRequest || service.quoteCalls != 0 {
		t.Fatalf("status/calls = %d/%d", response.Code, service.quoteCalls)
	}
}

func TestPOSHandlerRejectsMalformedProjectID(t *testing.T) {
	service := &posHTTPServiceStub{}
	router := chi.NewRouter()
	NewPOSHandler(service).Register(router)

	for _, test := range []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodGet, path: "/projects/not-a-number/pos"},
		{method: http.MethodPost, path: "/projects/not-a-number/checkout/quote", body: `{"items":[{"product_id":10,"quantity":1}]}`},
	} {
		response := servePOSRequest(router, test.method, test.path, test.body)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("%s %s status = %d", test.method, test.path, response.Code)
		}
	}
	if service.catalogCalls != 0 || service.quoteCalls != 0 {
		t.Fatalf("service calls = catalog %d quote %d", service.catalogCalls, service.quoteCalls)
	}
}

func TestPOSHandlerMapsDomainErrors(t *testing.T) {
	stockConflict := usecases.POSInsufficientStockError{Items: []models.POSStockConflict{{
		ProductID:         10,
		RequestedQuantity: 4,
		AvailableQuantity: 2,
	}}}
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   models.ProjectAPIErrorCode
	}{
		{
			name:       "validation",
			err:        usecases.POSValidationError{Message: "Cart must contain at least one item"},
			wantStatus: http.StatusBadRequest,
			wantCode:   models.ProjectErrorValidation,
		},
		{
			name:       "project not found",
			err:        repositories.ErrProjectNotFound,
			wantStatus: http.StatusNotFound,
			wantCode:   models.ProjectErrorNotFound,
		},
		{
			name:       "project not active",
			err:        usecases.ErrProjectNotActive,
			wantStatus: http.StatusConflict,
			wantCode:   models.ProjectErrorNotActive,
		},
		{
			name:       "product not sellable",
			err:        repositories.ErrProductNotSellable,
			wantStatus: http.StatusConflict,
			wantCode:   models.ProjectErrorProductNotSellable,
		},
		{
			name:       "stock conflict",
			err:        stockConflict,
			wantStatus: http.StatusConflict,
			wantCode:   models.ProjectErrorInsufficientStock,
		},
		{
			name:       "unexpected",
			err:        errors.New("database unavailable"),
			wantStatus: http.StatusInternalServerError,
			wantCode:   models.ProjectErrorInternal,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := &posHTTPServiceStub{quoteErr: test.err}
			router := chi.NewRouter()
			NewPOSHandler(service).Register(router)

			response := servePOSRequest(router, http.MethodPost, "/projects/7/checkout/quote", `{"items":[{"product_id":10,"quantity":1}]}`)
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d; body = %s", response.Code, test.wantStatus, response.Body.String())
			}
			apiError := decodePOSAPIError(t, response)
			if apiError.Code != test.wantCode {
				t.Fatalf("error code = %s, want %s", apiError.Code, test.wantCode)
			}
			if test.name == "stock conflict" {
				if len(apiError.Details.Items) != 1 || apiError.Details.Items[0].AvailableQuantity != 2 {
					t.Fatalf("stock details = %+v", apiError.Details.Items)
				}
			}
		})
	}
}

type posHTTPServiceStub struct {
	catalogData      models.POSCatalog
	quoteData        models.POSQuote
	catalogErr       error
	quoteErr         error
	catalogCalls     int
	quoteCalls       int
	catalogProjectID int64
	quoteProjectID   int64
	quoteRequest     models.POSCartRequest
}

func (stub *posHTTPServiceStub) Catalog(_ context.Context, projectID int64) (models.POSCatalog, error) {
	stub.catalogCalls++
	stub.catalogProjectID = projectID
	return stub.catalogData, stub.catalogErr
}

func (stub *posHTTPServiceStub) Quote(_ context.Context, projectID int64, request models.POSCartRequest) (models.POSQuote, error) {
	stub.quoteCalls++
	stub.quoteProjectID = projectID
	stub.quoteRequest = request
	return stub.quoteData, stub.quoteErr
}

type posAPIError struct {
	Success bool                       `json:"success"`
	Error   string                     `json:"error"`
	Code    models.ProjectAPIErrorCode `json:"code"`
	Details struct {
		Items []models.POSStockConflict `json:"items"`
	} `json:"details"`
	RequestID string `json:"request_id"`
}

func servePOSRequest(router http.Handler, method, path, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func decodePOSAPIError(t *testing.T, response *httptest.ResponseRecorder) posAPIError {
	t.Helper()
	var body posAPIError
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode POS error: %v; body = %s", err, response.Body.String())
	}
	if body.Success {
		t.Fatalf("POS error response marked success: %+v", body)
	}
	return body
}

func handlerTestAmount(t *testing.T, value string) models.THBAmount {
	t.Helper()
	amount, err := models.ParseTHBAmount(value)
	if err != nil {
		t.Fatalf("parse amount %q: %v", value, err)
	}
	return amount
}

var _ POSService = (*posHTTPServiceStub)(nil)
