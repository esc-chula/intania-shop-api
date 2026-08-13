package handlers

import (
	"context"
	"encoding/json"
	"github.com/esc-chula/intania-shop-api/internal/models"
	"github.com/go-chi/chi/v5"
	"net/http"
	"strconv"
)

type ProductCreator interface {
	Create(context.Context, models.ProductInput) (models.ProductDetail, error)
	Update(context.Context, int64, models.ProductInput) (models.ProductDetail, error)
	Delete(context.Context, int64) error
	CreateVariant(context.Context, int64, models.VariantInput) (models.Variant, error)
	UpdateVariant(context.Context, int64, models.VariantInput) (models.Variant, error)
	DeleteVariant(context.Context, int64) error
}

type ProductAdminHandler struct{ products ProductCreator }

func NewProductAdminHandler(products ProductCreator) *ProductAdminHandler {
	return &ProductAdminHandler{products: products}
}

func (h *ProductAdminHandler) Register(router chi.Router) {
	router.Post("/products", h.create)
	router.Put("/products/{id}", h.update)
	router.Delete("/products/{id}", h.delete)
	router.Post("/products/{id}/variants", h.createVariant)
	router.Put("/variants/{id}", h.updateVariant)
	router.Delete("/variants/{id}", h.deleteVariant)
}
func (h *ProductAdminHandler) create(w http.ResponseWriter, r *http.Request) {
	var in models.ProductInput
	if e := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&in); e != nil {
		writeError(w, 400, "Invalid JSON request body")
		return
	}
	out, e := h.products.Create(r.Context(), in)
	if e != nil {
		writeError(w, 400, e.Error())
		return
	}
	writeSuccess(w, 201, out)
}

func productPathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "Invalid product or variant ID")
		return 0, false
	}
	return id, true
}

func (h *ProductAdminHandler) update(w http.ResponseWriter, r *http.Request) {
	id, ok := productPathID(w, r)
	if !ok {
		return
	}
	var in models.ProductInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON request body")
		return
	}
	out, err := h.products.Update(r.Context(), id, in)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeSuccess(w, http.StatusOK, out)
}

func (h *ProductAdminHandler) delete(w http.ResponseWriter, r *http.Request) {
	id, ok := productPathID(w, r)
	if !ok {
		return
	}
	if err := h.products.Delete(r.Context(), id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ProductAdminHandler) createVariant(w http.ResponseWriter, r *http.Request) {
	productID, ok := productPathID(w, r)
	if !ok {
		return
	}
	var in models.VariantInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON request body")
		return
	}
	out, err := h.products.CreateVariant(r.Context(), productID, in)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeSuccess(w, http.StatusCreated, out)
}

func (h *ProductAdminHandler) updateVariant(w http.ResponseWriter, r *http.Request) {
	variantID, ok := productPathID(w, r)
	if !ok {
		return
	}
	var in models.VariantInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON request body")
		return
	}
	out, err := h.products.UpdateVariant(r.Context(), variantID, in)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeSuccess(w, http.StatusOK, out)
}

func (h *ProductAdminHandler) deleteVariant(w http.ResponseWriter, r *http.Request) {
	variantID, ok := productPathID(w, r)
	if !ok {
		return
	}
	if err := h.products.DeleteVariant(r.Context(), variantID); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
