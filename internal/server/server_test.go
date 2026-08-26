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
)

type fakePinger struct {
	err error
}

func (pinger fakePinger) Ping(context.Context) error {
	return pinger.err
}

func TestHandler(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	tests := map[string]struct {
		path       string
		pinger     fakePinger
		wantStatus int
		wantBody   string
	}{
		"root":      {path: "/", wantStatus: http.StatusOK, wantBody: "intania-shop-api"},
		"healthy":   {path: "/health", wantStatus: http.StatusOK, wantBody: "ok"},
		"unhealthy": {path: "/health", pinger: fakePinger{err: errors.New("unavailable")}, wantStatus: http.StatusServiceUnavailable, wantBody: "unavailable"},
		"not found": {path: "/missing", wantStatus: http.StatusNotFound},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			handler := NewHandler(Dependencies{
				Logger:             logger,
				Database:           test.pinger,
				CORSAllowedOrigins: []string{"http://localhost:3000"},
			})
			request := httptest.NewRequest(http.MethodGet, test.path, nil)
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.wantStatus)
			}
			if test.wantBody != "" && response.Body.String() != test.wantBody {
				t.Fatalf("body = %q, want %q", response.Body.String(), test.wantBody)
			}
			if response.Header().Get("X-Request-ID") == "" {
				t.Fatal("response is missing X-Request-ID")
			}
		})
	}
}

type productTokenVerifier struct{}

func (productTokenVerifier) Verify(token string) (models.Identity, error) {
	if token == "user" {
		return models.Identity{UserID: 1, Role: models.RoleUser}, nil
	}
	return models.Identity{UserID: 2, Role: models.RoleAdmin}, nil
}

type productCatalogStub struct{}

func (productCatalogStub) List(context.Context, int32, int32, bool) (any, error) {
	return models.ProductListResponse{Products: []models.ProductListItem{}}, nil
}

func (productCatalogStub) Search(context.Context, string, int32, int32) ([]models.ProductListItem, error) {
	return []models.ProductListItem{}, nil
}

func (productCatalogStub) Detail(context.Context, int64) (models.ProductDetail, error) {
	return models.ProductDetail{}, nil
}

type productMutationStub struct{}

func (productMutationStub) Create(context.Context, models.ProductInput) (models.ProductDetail, error) {
	return models.ProductDetail{}, nil
}

func (productMutationStub) Update(context.Context, int64, models.ProductInput) (models.ProductDetail, error) {
	return models.ProductDetail{}, nil
}

func (productMutationStub) Delete(context.Context, int64) error { return nil }

func (productMutationStub) CreateVariant(context.Context, int64, models.VariantInput) (models.Variant, error) {
	return models.Variant{}, nil
}

func (productMutationStub) UpdateVariant(context.Context, int64, models.VariantInput) (models.Variant, error) {
	return models.Variant{}, nil
}

func (productMutationStub) DeleteVariant(context.Context, int64) error { return nil }

func TestAllProductRoutesRequireAdmin(t *testing.T) {
	t.Parallel()
	handler := NewHandler(Dependencies{
		Logger:              slog.New(slog.NewTextHandler(io.Discard, nil)),
		ProductHandler:      handlers.NewProductHandler(productCatalogStub{}),
		ProductAdminHandler: handlers.NewProductAdminHandler(productMutationStub{}),
		TokenVerifier:       productTokenVerifier{},
	})
	tests := []struct {
		method, path, body string
		adminStatus        int
	}{
		{http.MethodGet, "/products", "", http.StatusOK},
		{http.MethodGet, "/products/search?q=shirt", "", http.StatusOK},
		{http.MethodGet, "/products/1", "", http.StatusOK},
		{http.MethodPost, "/products", `{}`, http.StatusCreated},
		{http.MethodPut, "/products/1", `{}`, http.StatusOK},
		{http.MethodDelete, "/products/1", "", http.StatusNoContent},
		{http.MethodPost, "/products/1/variants", `{}`, http.StatusCreated},
		{http.MethodPut, "/variants/1", `{}`, http.StatusOK},
		{http.MethodDelete, "/variants/1", "", http.StatusNoContent},
	}
	for _, test := range tests {
		for _, identity := range []struct {
			name, authorization string
			want                int
		}{
			{"anonymous", "", http.StatusUnauthorized},
			{"user", "Bearer user", http.StatusForbidden},
			{"admin", "Bearer admin", test.adminStatus},
		} {
			t.Run(identity.name+"_"+test.method+"_"+test.path, func(t *testing.T) {
				request := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
				request.Header.Set("Authorization", identity.authorization)
				response := httptest.NewRecorder()
				handler.ServeHTTP(response, request)
				if response.Code != identity.want {
					t.Fatalf("status=%d want=%d body=%s", response.Code, identity.want, response.Body.String())
				}
			})
		}
	}
}

func TestRemovedStorefrontRoutesAreNotRegistered(t *testing.T) {
	t.Parallel()
	handler := NewHandler(Dependencies{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	tests := []struct{ method, path string }{
		{http.MethodGet, "/cart"},
		{http.MethodPut, "/cart/items"},
		{http.MethodPut, "/favorites"},
		{http.MethodGet, "/orders"},
		{http.MethodPost, "/orders"},
		{http.MethodGet, "/orders/1"},
		{http.MethodPut, "/orders/1"},
		{http.MethodDelete, "/orders/1"},
		{http.MethodGet, "/promos"},
		{http.MethodPost, "/promos"},
		{http.MethodGet, "/promos/active"},
		{http.MethodGet, "/promos/1"},
		{http.MethodPut, "/promos/1"},
		{http.MethodDelete, "/promos/1"},
		{http.MethodGet, "/banners"},
		{http.MethodPost, "/banners"},
		{http.MethodGet, "/banners/active"},
		{http.MethodGet, "/banners/1"},
		{http.MethodPut, "/banners/1"},
		{http.MethodDelete, "/banners/1"},
		{http.MethodPost, "/stock/bulk-reduction"},
		{http.MethodPost, "/upload/product-videos"},
	}
	for _, test := range tests {
		request := httptest.NewRequest(test.method, test.path, nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusNotFound {
			t.Errorf("%s %s status=%d want=404", test.method, test.path, response.Code)
		}
	}
}
