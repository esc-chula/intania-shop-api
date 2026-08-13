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

type contentStub struct{}

func (contentStub) Promos(context.Context, bool) ([]models.Promo, error) {
	return []models.Promo{}, nil
}
func (contentStub) Banners(context.Context, bool) ([]models.Banner, error) {
	return []models.Banner{}, nil
}
func (contentStub) Promo(context.Context, int64) (models.Promo, error)   { return models.Promo{}, nil }
func (contentStub) Banner(context.Context, int64) (models.Banner, error) { return models.Banner{}, nil }

func TestPublicCatalogAndContentContracts(t *testing.T) {
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
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := chi.NewRouter()
			NewProductHandler(tt.catalog).Register(mux)
			NewContentHandler(contentStub{}).Register(mux)
			request := httptest.NewRequest(http.MethodGet, tt.path, nil)
			response := httptest.NewRecorder()
			mux.ServeHTTP(response, request)
			if response.Code != tt.want {
				t.Fatalf("status=%d want %d: %s", response.Code, tt.want, response.Body.String())
			}
		})
	}
	for _, path := range []string{"/promos", "/promos/active", "/promos/1", "/banners", "/banners/active", "/banners/1"} {
		t.Run(path, func(t *testing.T) {
			mux := chi.NewRouter()
			NewContentHandler(contentStub{}).Register(mux)
			response := httptest.NewRecorder()
			mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
			if response.Code != http.StatusOK {
				t.Fatalf("status=%d: %s", response.Code, response.Body.String())
			}
		})
	}
}
