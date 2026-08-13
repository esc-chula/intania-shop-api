package handlers

import (
	"context"
	"errors"
	"github.com/go-chi/chi/v5"
	"net/http"
	"strconv"

	"github.com/esc-chula/intania-shop-api/internal/models"
	"github.com/esc-chula/intania-shop-api/internal/repositories"
)

type CatalogReader interface {
	List(context.Context, int32, int32, bool) (any, error)
	Search(context.Context, string, int32, int32) ([]models.ProductListItem, error)
	Detail(context.Context, int64) (models.ProductDetail, error)
}

type ProductHandler struct{ catalog CatalogReader }

func NewProductHandler(catalog CatalogReader) *ProductHandler {
	return &ProductHandler{catalog: catalog}
}

func (handler *ProductHandler) Register(router chi.Router) {
	router.Get("/products", handler.list)
	router.Get("/products/search", handler.search)
	router.Get("/products/{id}", handler.detail)
}

func (handler *ProductHandler) list(writer http.ResponseWriter, request *http.Request) {
	data, err := handler.catalog.List(request.Context(), queryInt(request, "page", 1), queryInt(request, "page_size", 10), request.URL.Query().Get("include_variants") == "true")
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "Unable to list products")
		return
	}
	writeSuccess(writer, http.StatusOK, data)
}
func (handler *ProductHandler) search(writer http.ResponseWriter, request *http.Request) {
	data, err := handler.catalog.Search(request.Context(), request.URL.Query().Get("q"), queryInt(request, "page", 1), queryInt(request, "page_size", 10))
	if err != nil {
		writeError(writer, http.StatusBadRequest, err.Error())
		return
	}
	writeSuccess(writer, http.StatusOK, data)
}
func (handler *ProductHandler) detail(writer http.ResponseWriter, request *http.Request) {
	id, err := strconv.ParseInt(request.PathValue("id"), 10, 64)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "Invalid product ID")
		return
	}
	data, err := handler.catalog.Detail(request.Context(), id)
	if errors.Is(err, repositories.ErrProductNotFound) {
		writeError(writer, http.StatusNotFound, "Product not found")
		return
	}
	if err != nil {
		writeError(writer, http.StatusBadRequest, err.Error())
		return
	}
	writeSuccess(writer, http.StatusOK, data)
}
func queryInt(request *http.Request, name string, fallback int32) int32 {
	value, err := strconv.ParseInt(request.URL.Query().Get(name), 10, 32)
	if err != nil {
		return fallback
	}
	return int32(value)
}
