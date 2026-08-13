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
	"github.com/esc-chula/intania-shop-api/internal/repositories"
	"github.com/go-chi/chi/v5"
)

type orderHandlerStub struct{ getErr error }

func (s orderHandlerStub) Create(context.Context, int64, models.CreateOrderRequest) (models.Order, error) {
	return models.Order{}, nil
}
func (s orderHandlerStub) Get(context.Context, int64, int64, bool) (models.Order, error) {
	return models.Order{}, s.getErr
}
func (s orderHandlerStub) List(context.Context, int64, bool) ([]models.Order, error) { return nil, nil }
func (s orderHandlerStub) Update(context.Context, int64, models.UpdateOrderRequest) (models.Order, error) {
	return models.Order{}, nil
}
func (s orderHandlerStub) Delete(context.Context, int64) error { return nil }

func TestOrderHandlerAccessClassification(t *testing.T) {
	tests := []struct {
		name               string
		role               models.Role
		err                error
		method, path, body string
		want               int
	}{
		{"not found", models.RoleUser, repositories.ErrOrderNotFound, http.MethodGet, "/orders/1", "", http.StatusNotFound},
		{"ownership forbidden", models.RoleUser, errors.New("not owner"), http.MethodGet, "/orders/1", "", http.StatusForbidden},
		{"user cannot update", models.RoleUser, nil, http.MethodPut, "/orders/1", `{}`, http.StatusForbidden},
		{"admin can update", models.RoleAdmin, nil, http.MethodPut, "/orders/1", `{}`, http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := chi.NewRouter()
			verifier := inventoryVerifier{identity: models.Identity{UserID: 1, Role: tt.role}}
			authenticated := mux.With(middlewares.Authenticate(verifier))
			handler := NewOrderHandler(orderHandlerStub{getErr: tt.err})
			handler.Register(authenticated)
			handler.RegisterAdmin(authenticated.With(middlewares.RequireRole(models.RoleAdmin)))
			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			req.Header.Set("Authorization", "Bearer token")
			res := httptest.NewRecorder()
			mux.ServeHTTP(res, req)
			if res.Code != tt.want {
				t.Fatalf("status=%d want %d: %s", res.Code, tt.want, res.Body.String())
			}
		})
	}
}
