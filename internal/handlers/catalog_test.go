package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/esc-chula/intania-shop-api/internal/models"
	"github.com/esc-chula/intania-shop-api/internal/repositories"
	"github.com/go-chi/chi/v5"
)

type catalogStub struct {
	detailErr error
	searchErr error
}

func (s catalogStub) List(context.Context, int32, int32, bool) (any, error) {
	return models.ProductListResponse{Products: []models.ProductListItem{}}, nil
}

func (s catalogStub) Search(context.Context, string, int32, int32) ([]models.ProductListItem, error) {
	return []models.ProductListItem{}, s.searchErr
}

func (s catalogStub) Detail(context.Context, int64) (models.ProductDetail, error) {
	return models.ProductDetail{}, s.detailErr
}

func TestCatalogContracts(t *testing.T) {
	tests := []struct {
		name, path string
		catalog    catalogStub
		want       int
	}{
		{"products list", "/products", catalogStub{}, http.StatusOK},
		{"search service validation", "/products/search", catalogStub{searchErr: errors.New("search query is required")}, http.StatusBadRequest},
		{"missing product", "/products/9", catalogStub{detailErr: repositories.ErrProductNotFound}, http.StatusNotFound},
		{"invalid product ID", "/products/nope", catalogStub{}, http.StatusBadRequest},
		{"catalog error", "/products/9", catalogStub{detailErr: errors.New("db")}, http.StatusBadRequest},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mux := chi.NewRouter()
			NewProductHandler(test.catalog).Register(mux)
			request := httptest.NewRequest(http.MethodGet, test.path, nil)
			response := httptest.NewRecorder()
			mux.ServeHTTP(response, request)
			if response.Code != test.want {
				t.Fatalf("status=%d want %d: %s", response.Code, test.want, response.Body.String())
			}
		})
	}
}
