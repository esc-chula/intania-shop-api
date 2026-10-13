package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/esc-chula/intania-shop-api/internal/models"
	"github.com/esc-chula/intania-shop-api/internal/repositories"
	"github.com/go-chi/chi/v5"
)

type productAdminHTTPStub struct{ deleteErr error }

func (stub *productAdminHTTPStub) Create(context.Context, models.ProductInput) (models.ProductDetail, error) {
	return models.ProductDetail{}, nil
}
func (stub *productAdminHTTPStub) Update(context.Context, int64, models.ProductInput) (models.ProductDetail, error) {
	return models.ProductDetail{}, nil
}
func (stub *productAdminHTTPStub) Delete(context.Context, int64) error { return stub.deleteErr }
func (stub *productAdminHTTPStub) CreateVariant(context.Context, int64, models.VariantInput) (models.Variant, error) {
	return models.Variant{}, nil
}
func (stub *productAdminHTTPStub) UpdateVariant(context.Context, int64, models.VariantInput) (models.Variant, error) {
	return models.Variant{}, nil
}
func (stub *productAdminHTTPStub) DeleteVariant(context.Context, int64) error { return nil }

func TestProductAdminDeleteMapsArchiveErrors(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   models.ProductAPIErrorCode
	}{
		{name: "project assignment", err: repositories.ErrProductInUse, wantStatus: http.StatusConflict, wantCode: models.ProductErrorInUse},
		{name: "missing product", err: repositories.ErrProductNotFound, wantStatus: http.StatusNotFound, wantCode: models.ProductErrorNotFound},
		{name: "unexpected failure", err: errors.New("database unavailable"), wantStatus: http.StatusInternalServerError, wantCode: models.ProductErrorInternal},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := chi.NewRouter()
			NewProductAdminHandler(&productAdminHTTPStub{deleteErr: test.err}).Register(router)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodDelete, "/products/7", nil))
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d; body = %s", response.Code, response.Body.String())
			}
			var body struct {
				Success bool                       `json:"success"`
				Code    models.ProductAPIErrorCode `json:"code"`
			}
			if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.Success || body.Code != test.wantCode {
				t.Fatalf("error response = %+v, want code %s", body, test.wantCode)
			}
		})
	}
}

func TestProductAdminDeleteIsNoContentWhenArchiveIsAlreadyComplete(t *testing.T) {
	router := chi.NewRouter()
	NewProductAdminHandler(&productAdminHTTPStub{}).Register(router)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodDelete, "/products/7", nil))
	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d; body = %s", response.Code, response.Body.String())
	}
}
