package server

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/esc-chula/intania-shop-api/internal/handlers"
	"github.com/esc-chula/intania-shop-api/internal/models"
	"github.com/esc-chula/intania-shop-api/internal/usecases"
)

func TestNewHandlerProtectsPOSRoutesWithAdminRole(t *testing.T) {
	tests := []struct {
		name       string
		header     string
		identity   models.Identity
		verifyErr  error
		wantStatus int
		wantCalls  int
	}{
		{name: "missing token", wantStatus: http.StatusUnauthorized},
		{name: "invalid token", header: "Bearer invalid", verifyErr: errors.New("invalid token"), wantStatus: http.StatusUnauthorized},
		{name: "user forbidden", header: "Bearer valid", identity: models.Identity{UserID: 2, Role: models.RoleUser}, wantStatus: http.StatusForbidden},
		{name: "admin allowed", header: "Bearer valid", identity: models.Identity{UserID: 1, Role: models.RoleAdmin}, wantStatus: http.StatusOK, wantCalls: 1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			posService := &serverPOSServiceStub{}
			handler := NewHandler(Dependencies{
				Logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
				POSHandler:    handlers.NewPOSHandler(posService),
				TokenVerifier: serverTokenVerifier{identity: test.identity, err: test.verifyErr},
			})

			request := httptest.NewRequest(http.MethodGet, "/projects/7/pos", nil)
			request.Header.Set("Authorization", test.header)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)

			if response.Code != test.wantStatus || posService.catalogCalls != test.wantCalls {
				t.Fatalf("status/calls = %d/%d, want %d/%d", response.Code, posService.catalogCalls, test.wantStatus, test.wantCalls)
			}
		})
	}
}

func TestNewHandlerDoesNotExposePOSRoutesWithoutAuthentication(t *testing.T) {
	posService := &serverPOSServiceStub{}
	handler := NewHandler(Dependencies{
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		POSHandler: handlers.NewPOSHandler(posService),
	})

	request := httptest.NewRequest(http.MethodGet, "/projects/7/pos", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound || posService.catalogCalls != 0 {
		t.Fatalf("status/calls = %d/%d, want 404/0", response.Code, posService.catalogCalls)
	}
}

func TestNewHandlerProtectsOrderRoutesWithAdminRole(t *testing.T) {
	tests := []struct {
		name       string
		header     string
		identity   models.Identity
		verifyErr  error
		wantStatus int
		wantCalls  int
	}{
		{name: "missing token", wantStatus: http.StatusUnauthorized},
		{name: "invalid token", header: "Bearer invalid", verifyErr: errors.New("invalid token"), wantStatus: http.StatusUnauthorized},
		{name: "user forbidden", header: "Bearer valid", identity: models.Identity{UserID: 2, Role: models.RoleUser}, wantStatus: http.StatusForbidden},
		{name: "admin allowed", header: "Bearer valid", identity: models.Identity{UserID: 1, Role: models.RoleAdmin}, wantStatus: http.StatusOK, wantCalls: 1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			orderService := &serverOrderServiceStub{}
			handler := NewHandler(Dependencies{
				Logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
				OrderHandler:  handlers.NewOrderHandler(orderService),
				TokenVerifier: serverTokenVerifier{identity: test.identity, err: test.verifyErr},
			})

			request := httptest.NewRequest(http.MethodGet, "/projects/7/orders", nil)
			request.Header.Set("Authorization", test.header)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)

			if response.Code != test.wantStatus || orderService.listCalls != test.wantCalls {
				t.Fatalf("status/calls = %d/%d, want %d/%d", response.Code, orderService.listCalls, test.wantStatus, test.wantCalls)
			}
		})
	}
}

func TestNewHandlerProtectsPromotionReadRoutesWithAdminRole(t *testing.T) {
	tests := []struct {
		name            string
		path            string
		header          string
		identity        models.Identity
		verifyErr       error
		wantStatus      int
		wantListCalls   int
		wantDetailCalls int
	}{
		{name: "missing token", path: "/projects/7/promotions", wantStatus: http.StatusUnauthorized},
		{name: "user forbidden", path: "/projects/7/promotions", header: "Bearer valid", identity: models.Identity{UserID: 2, Role: models.RoleUser}, wantStatus: http.StatusForbidden},
		{name: "user forbidden from promotion detail", path: "/projects/7/promotions/3", header: "Bearer valid", identity: models.Identity{UserID: 2, Role: models.RoleUser}, wantStatus: http.StatusForbidden},
		{name: "admin lists promotions", path: "/projects/7/promotions", header: "Bearer valid", identity: models.Identity{UserID: 1, Role: models.RoleAdmin}, wantStatus: http.StatusOK, wantListCalls: 1},
		{name: "admin gets promotion", path: "/projects/7/promotions/3", header: "Bearer valid", identity: models.Identity{UserID: 1, Role: models.RoleAdmin}, wantStatus: http.StatusOK, wantDetailCalls: 1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			promotionService := &serverPromotionServiceStub{}
			handler := NewHandler(Dependencies{
				Logger:           slog.New(slog.NewTextHandler(io.Discard, nil)),
				PromotionHandler: handlers.NewPromotionHandler(promotionService),
				TokenVerifier:    serverTokenVerifier{identity: test.identity, err: test.verifyErr},
			})

			request := httptest.NewRequest(http.MethodGet, test.path, nil)
			request.Header.Set("Authorization", test.header)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)

			if response.Code != test.wantStatus || promotionService.listCalls != test.wantListCalls || promotionService.detailCalls != test.wantDetailCalls {
				t.Fatalf("status/list/detail = %d/%d/%d, want %d/%d/%d", response.Code, promotionService.listCalls, promotionService.detailCalls, test.wantStatus, test.wantListCalls, test.wantDetailCalls)
			}
		})
	}
}

func TestNewHandlerProtectsCheckoutAndPaymentSlipRoutesWithAdminRole(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "create order", method: http.MethodPost, path: "/projects/7/orders", body: `{}`},
		{name: "upload payment slip", method: http.MethodPost, path: "/upload/payment-slips", body: ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, identity := range []struct {
				name       string
				header     string
				role       models.Role
				verifyErr  error
				wantStatus int
			}{
				{name: "missing token", wantStatus: http.StatusUnauthorized},
				{name: "invalid token", header: "Bearer invalid", verifyErr: errors.New("invalid token"), wantStatus: http.StatusUnauthorized},
				{name: "user forbidden", header: "Bearer valid", role: models.RoleUser, wantStatus: http.StatusForbidden},
			} {
				t.Run(identity.name, func(t *testing.T) {
					posService := &serverPOSServiceStub{}
					slipService := &serverPaymentSlipServiceStub{}
					handler := NewHandler(Dependencies{
						Logger:             slog.New(slog.NewTextHandler(io.Discard, nil)),
						POSHandler:         handlers.NewPOSHandler(posService),
						PaymentSlipHandler: handlers.NewPaymentSlipHandler(slipService),
						TokenVerifier:      serverTokenVerifier{identity: models.Identity{UserID: 2, Role: identity.role}, err: identity.verifyErr},
					})

					request := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
					request.Header.Set("Authorization", identity.header)
					response := httptest.NewRecorder()
					handler.ServeHTTP(response, request)

					if response.Code != identity.wantStatus {
						t.Fatalf("status = %d, want %d", response.Code, identity.wantStatus)
					}
					if posService.checkoutCalls != 0 || slipService.calls != 0 {
						t.Fatal("an unauthorized request reached a service")
					}
				})
			}
		})
	}
}

type serverTokenVerifier struct {
	identity models.Identity
	err      error
}

func (verifier serverTokenVerifier) Verify(string) (models.Identity, error) {
	return verifier.identity, verifier.err
}

type serverPOSServiceStub struct {
	catalogCalls  int
	checkoutCalls int
}

func (stub *serverPOSServiceStub) Catalog(context.Context, int64) (models.POSCatalog, error) {
	stub.catalogCalls++
	return models.POSCatalog{Project: models.Project{ProjectID: 7, Status: models.ProjectStatusActive}}, nil
}

func (*serverPOSServiceStub) Quote(context.Context, int64, models.POSCartRequest) (models.POSQuote, error) {
	return models.POSQuote{}, nil
}

func (stub *serverPOSServiceStub) Checkout(context.Context, int64, int64, string, models.POSCheckoutRequest) (models.POSOrder, bool, error) {
	stub.checkoutCalls++
	return models.POSOrder{OrderID: 99}, false, nil
}

type serverOrderServiceStub struct {
	listCalls int
}

type serverPromotionServiceStub struct {
	listCalls   int
	detailCalls int
}

func (stub *serverPromotionServiceStub) List(context.Context, int64) (models.ProjectPromotionListData, error) {
	stub.listCalls++
	return models.ProjectPromotionListData{}, nil
}

func (stub *serverPromotionServiceStub) Detail(context.Context, int64, int64) (models.ProjectPromotion, error) {
	stub.detailCalls++
	return models.ProjectPromotion{}, nil
}

func (stub *serverOrderServiceStub) List(context.Context, int64, usecases.OrderListQuery, int32, int32) (models.OrderListResponse, error) {
	stub.listCalls++
	return models.OrderListResponse{}, nil
}

func (*serverOrderServiceStub) Export(context.Context, int64, usecases.OrderListQuery) ([]models.POSOrder, error) {
	return nil, nil
}

func (*serverOrderServiceStub) Detail(context.Context, int64, int64) (models.POSOrder, error) {
	return models.POSOrder{}, nil
}

type serverPaymentSlipServiceStub struct{ calls int }

func (stub *serverPaymentSlipServiceStub) Upload(context.Context, usecases.PaymentSlipUpload) (models.PaymentSlip, error) {
	stub.calls++
	return models.PaymentSlip{}, nil
}

var (
	_ handlers.POSService         = (*serverPOSServiceStub)(nil)
	_ handlers.OrderService       = (*serverOrderServiceStub)(nil)
	_ handlers.PromotionService   = (*serverPromotionServiceStub)(nil)
	_ handlers.PaymentSlipService = (*serverPaymentSlipServiceStub)(nil)
)
