package server

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
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
		{name: "user forbidden", header: "Bearer user", identity: models.Identity{UserID: 2, Role: models.RoleUser}, wantStatus: http.StatusForbidden},
		{name: "admin allowed", header: "Bearer admin", identity: models.Identity{UserID: 1, Role: models.RoleAdmin}, wantStatus: http.StatusOK, wantCalls: 1},
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
		{name: "user forbidden", header: "Bearer user", identity: models.Identity{UserID: 2, Role: models.RoleUser}, wantStatus: http.StatusForbidden},
		{name: "admin allowed", header: "Bearer admin", identity: models.Identity{UserID: 1, Role: models.RoleAdmin}, wantStatus: http.StatusOK, wantCalls: 1},
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

type serverTokenVerifier struct {
	identity models.Identity
	err      error
}

func (verifier serverTokenVerifier) Verify(string) (models.Identity, error) {
	return verifier.identity, verifier.err
}

type serverPOSServiceStub struct {
	catalogCalls int
}

func (stub *serverPOSServiceStub) Catalog(context.Context, int64) (models.POSCatalog, error) {
	stub.catalogCalls++
	return models.POSCatalog{Project: models.Project{ProjectID: 7, Status: models.ProjectStatusActive}}, nil
}

func (*serverPOSServiceStub) Quote(context.Context, int64, models.POSCartRequest) (models.POSQuote, error) {
	return models.POSQuote{}, nil
}

var _ handlers.POSService = (*serverPOSServiceStub)(nil)

type serverOrderServiceStub struct {
	listCalls int
}

func (stub *serverOrderServiceStub) List(context.Context, int64, usecases.OrderListQuery, int32, int32) (models.OrderListResponse, error) {
	stub.listCalls++
	return models.OrderListResponse{}, nil
}

func (*serverOrderServiceStub) Export(context.Context, int64, usecases.OrderListQuery) ([]models.Order, error) {
	return nil, nil
}

func (*serverOrderServiceStub) Detail(context.Context, int64, int64) (models.Order, error) {
	return models.Order{}, nil
}

var _ handlers.OrderService = (*serverOrderServiceStub)(nil)
