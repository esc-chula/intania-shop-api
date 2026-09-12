package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/esc-chula/intania-shop-api/internal/middlewares"
	"github.com/esc-chula/intania-shop-api/internal/models"
	"github.com/esc-chula/intania-shop-api/internal/repositories"
	"github.com/esc-chula/intania-shop-api/internal/usecases"
	"github.com/go-chi/chi/v5"
)

const posCheckoutBody = `{"buyer":{"gender":"FEMALE","age":21,"student_alumni_year":"Intania 105"},` +
	`"items":[{"product_id":10,"variant_id":22,"quantity":2}],` +
	`"payment":{"method":"REAL_MONEY","received_amount":"160.00","no_change":false,"note":"booth 1"}}`

func TestPOSHandlerCreateOrderReturnsTheCreatedOrder(t *testing.T) {
	service := &posCheckoutHTTPServiceStub{order: posTestOrder(t)}
	router := posCheckoutRouter(service)

	response := servePOSCheckout(router, "/projects/7/orders", posCheckoutBody, "idem-123")
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d; body = %s", response.Code, response.Body.String())
	}
	if response.Header().Get("Idempotency-Replayed") != "" {
		t.Fatal("a newly created order was marked as replayed")
	}

	if service.calls != 1 || service.projectID != 7 || service.staffUserID != 42 || service.idempotencyKey != "idem-123" {
		t.Fatalf("checkout call = %+v", service)
	}
	if service.request.Buyer.Gender != models.POSGenderFemale || len(service.request.Items) != 1 {
		t.Fatalf("checkout request = %+v", service.request)
	}
	if service.request.Payment.ReceivedAmount == nil || service.request.Payment.ReceivedAmount.String() != "160.00" {
		t.Fatalf("received amount = %+v", service.request.Payment.ReceivedAmount)
	}
	if service.request.Payment.NoChange == nil || *service.request.Payment.NoChange {
		t.Fatalf("no_change = %+v", service.request.Payment.NoChange)
	}

	var envelope struct {
		Success bool            `json:"success"`
		Data    models.POSOrder `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	if !envelope.Success || envelope.Data.OrderNumber != "ORD-00000099" || envelope.Data.NetTotal.String() != "130.00" {
		t.Fatalf("order response = %+v", envelope)
	}
	if len(envelope.Data.Items) != 1 || envelope.Data.Items[0].InventoryTransactionID != 500 {
		t.Fatalf("order items = %+v", envelope.Data.Items)
	}
}

func TestPOSHandlerCreateOrderMarksAnIdempotentReplay(t *testing.T) {
	service := &posCheckoutHTTPServiceStub{order: posTestOrder(t), replayed: true}
	router := posCheckoutRouter(service)

	response := servePOSCheckout(router, "/projects/7/orders", posCheckoutBody, "idem-123")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d; body = %s", response.Code, response.Body.String())
	}
	if response.Header().Get("Idempotency-Replayed") != "true" {
		t.Fatalf("replay header = %q", response.Header().Get("Idempotency-Replayed"))
	}
}

func TestPOSHandlerCreateOrderForwardsAMissingIdempotencyKey(t *testing.T) {
	service := &posCheckoutHTTPServiceStub{order: posTestOrder(t)}
	router := posCheckoutRouter(service)

	// The header rule belongs to the use case, so the handler passes the empty
	// value through rather than inventing one.
	if response := servePOSCheckout(router, "/projects/7/orders", posCheckoutBody, ""); response.Code != http.StatusCreated {
		t.Fatalf("status = %d", response.Code)
	}
	if service.idempotencyKey != "" {
		t.Fatalf("idempotency key = %q", service.idempotencyKey)
	}
}

func TestPOSHandlerCreateOrderRejectsClientSuppliedMoney(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "order total", body: `{"buyer":{"gender":"MALE"},"items":[{"product_id":10,"quantity":1}],` +
			`"payment":{"method":"REAL_MONEY","received_amount":"1.00","no_change":true},"net_total":"1.00"}`},
		{name: "item price", body: `{"buyer":{"gender":"MALE"},"items":[{"product_id":10,"quantity":1,"unit_price":"0.01"}],` +
			`"payment":{"method":"REAL_MONEY","received_amount":"1.00","no_change":true}}`},
		{name: "payment discount", body: `{"buyer":{"gender":"MALE"},"items":[{"product_id":10,"quantity":1}],` +
			`"payment":{"method":"REAL_MONEY","received_amount":"1.00","no_change":true,"discount":"5.00"}}`},
		{name: "numeric amount", body: `{"buyer":{"gender":"MALE"},"items":[{"product_id":10,"quantity":1}],` +
			`"payment":{"method":"REAL_MONEY","received_amount":1,"no_change":true}}`},
		{name: "trailing JSON", body: posCheckoutBody + ` {}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := &posCheckoutHTTPServiceStub{order: posTestOrder(t)}
			router := posCheckoutRouter(service)

			response := servePOSCheckout(router, "/projects/7/orders", test.body, "idem-123")
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d; body = %s", response.Code, response.Body.String())
			}
			if apiError := decodePOSAPIError(t, response); apiError.Code != models.ProjectErrorValidation {
				t.Fatalf("error code = %s", apiError.Code)
			}
			if service.calls != 0 {
				t.Fatal("a rejected body reached the service")
			}
		})
	}
}

func TestPOSHandlerCreateOrderMapsCheckoutErrors(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   models.ProjectAPIErrorCode
	}{
		{
			name:       "idempotency key reused",
			err:        repositories.ErrIdempotencyKeyReused,
			wantStatus: http.StatusConflict,
			wantCode:   models.ProjectErrorIdempotencyKeyReused,
		},
		{
			name:       "untrusted payment slip",
			err:        repositories.ErrPaymentSlipNotTrusted,
			wantStatus: http.StatusConflict,
			wantCode:   models.ProjectErrorInvalidPaymentSlip,
		},
		{
			name:       "project is not active",
			err:        usecases.ErrProjectNotActive,
			wantStatus: http.StatusConflict,
			wantCode:   models.ProjectErrorNotActive,
		},
		{
			name:       "item is not sellable",
			err:        repositories.ErrProductNotSellable,
			wantStatus: http.StatusConflict,
			wantCode:   models.ProjectErrorProductNotSellable,
		},
		{
			name: "insufficient stock",
			err: usecases.POSInsufficientStockError{Items: []models.POSStockConflict{
				{ProductID: 10, RequestedQuantity: 4, AvailableQuantity: 2},
			}},
			wantStatus: http.StatusConflict,
			wantCode:   models.ProjectErrorInsufficientStock,
		},
		{
			name:       "invalid payment",
			err:        usecases.POSValidationError{Message: "Received amount is less than the order total"},
			wantStatus: http.StatusBadRequest,
			wantCode:   models.ProjectErrorValidation,
		},
		{
			name:       "missing project",
			err:        repositories.ErrProjectNotFound,
			wantStatus: http.StatusNotFound,
			wantCode:   models.ProjectErrorNotFound,
		},
		{
			name:       "unexpected failure",
			err:        errors.New("database unavailable"),
			wantStatus: http.StatusInternalServerError,
			wantCode:   models.ProjectErrorInternal,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := posCheckoutRouter(&posCheckoutHTTPServiceStub{err: test.err})

			response := servePOSCheckout(router, "/projects/7/orders", posCheckoutBody, "idem-123")
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d; body = %s", response.Code, test.wantStatus, response.Body.String())
			}
			if apiError := decodePOSAPIError(t, response); apiError.Code != test.wantCode {
				t.Fatalf("error code = %s, want %s", apiError.Code, test.wantCode)
			}
		})
	}
}

func TestPOSHandlerCreateOrderRequiresAnAuthenticatedStaff(t *testing.T) {
	service := &posCheckoutHTTPServiceStub{order: posTestOrder(t)}
	router := chi.NewRouter()
	NewPOSHandler(service).Register(router)

	response := servePOSCheckout(router, "/projects/7/orders", posCheckoutBody, "idem-123")
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d; body = %s", response.Code, response.Body.String())
	}
	if apiError := decodePOSAPIError(t, response); apiError.Code != models.ProjectErrorAuthenticationRequired {
		t.Fatalf("error code = %s", apiError.Code)
	}
	if service.calls != 0 {
		t.Fatal("an unauthenticated request reached the service")
	}
}

func TestPOSHandlerCreateOrderRejectsAnInvalidProjectID(t *testing.T) {
	service := &posCheckoutHTTPServiceStub{order: posTestOrder(t)}
	router := posCheckoutRouter(service)

	response := servePOSCheckout(router, "/projects/seven/orders", posCheckoutBody, "idem-123")
	if response.Code != http.StatusBadRequest || service.calls != 0 {
		t.Fatalf("status/calls = %d/%d", response.Code, service.calls)
	}
}

// posCheckoutRouter mounts the POS routes behind the real authentication
// middleware so the staff identity comes from the bearer token.
func posCheckoutRouter(service POSService) http.Handler {
	router := chi.NewRouter()
	router.Use(middlewares.Authenticate(posCheckoutVerifierStub{}))
	NewPOSHandler(service).Register(router)
	return router
}

func servePOSCheckout(router http.Handler, path, body, idempotencyKey string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer staff-token")
	request.Header.Set("Content-Type", "application/json")
	if idempotencyKey != "" {
		request.Header.Set("Idempotency-Key", idempotencyKey)
	}

	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func posTestOrder(t *testing.T) models.POSOrder {
	t.Helper()
	noChange := false
	received := handlerTestAmount(t, "160.00")
	change := handlerTestAmount(t, "30.00")
	retained := handlerTestAmount(t, "0.00")

	return models.POSOrder{
		OrderID:     99,
		OrderNumber: "ORD-00000099",
		ProjectID:   7,
		Status:      models.POSOrderStatusCompleted,
		Staff:       models.POSStaffSnapshot{StaffID: 42, FullName: "Ada", Email: "ada@example.com"},
		Buyer:       models.POSBuyer{Gender: models.POSGenderFemale},
		Items: []models.POSOrderItemSnapshot{{
			OrderItemID:            1,
			ProductID:              10,
			ProductName:            "Shirt",
			Quantity:               2,
			UnitPrice:              handlerTestAmount(t, "40.00"),
			LineTotal:              handlerTestAmount(t, "80.00"),
			InventoryTransactionID: 500,
		}},
		Subtotal: handlerTestAmount(t, "130.00"),
		Discount: handlerTestAmount(t, "0.00"),
		NetTotal: handlerTestAmount(t, "130.00"),
		Payment: models.POSPaymentSnapshot{
			Method:         models.POSPaymentRealMoney,
			ReceivedAmount: &received,
			ChangeAmount:   &change,
			RetainedAmount: &retained,
			NoChange:       &noChange,
		},
	}
}

// Checkout keeps the catalogue and quote stub usable as a POSService; the
// checkout tests use posCheckoutHTTPServiceStub instead.
func (stub *posHTTPServiceStub) Checkout(context.Context, int64, int64, string, models.POSCheckoutRequest) (models.POSOrder, bool, error) {
	return models.POSOrder{}, false, errors.New("checkout is not configured in this stub")
}

type posCheckoutHTTPServiceStub struct {
	posHTTPServiceStub
	order          models.POSOrder
	replayed       bool
	err            error
	calls          int
	projectID      int64
	staffUserID    int64
	idempotencyKey string
	request        models.POSCheckoutRequest
}

func (stub *posCheckoutHTTPServiceStub) Checkout(_ context.Context, projectID, staffUserID int64,
	idempotencyKey string, request models.POSCheckoutRequest) (models.POSOrder, bool, error) {
	stub.calls++
	stub.projectID = projectID
	stub.staffUserID = staffUserID
	stub.idempotencyKey = idempotencyKey
	stub.request = request
	if stub.err != nil {
		return models.POSOrder{}, false, stub.err
	}

	return stub.order, stub.replayed, nil
}

type posCheckoutVerifierStub struct{}

func (posCheckoutVerifierStub) Verify(string) (models.Identity, error) {
	return models.Identity{UserID: 42, Role: models.RoleAdmin}, nil
}

var _ POSService = (*posCheckoutHTTPServiceStub)(nil)
