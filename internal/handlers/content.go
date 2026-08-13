package handlers

import (
	"context"
	"github.com/esc-chula/intania-shop-api/internal/models"
	"github.com/go-chi/chi/v5"
	"net/http"
	"strconv"
)

type ContentReader interface {
	Promos(context.Context, bool) ([]models.Promo, error)
	Banners(context.Context, bool) ([]models.Banner, error)
	Promo(context.Context, int64) (models.Promo, error)
	Banner(context.Context, int64) (models.Banner, error)
}

type ContentHandler struct{ content ContentReader }

func NewContentHandler(content ContentReader) *ContentHandler {
	return &ContentHandler{content: content}
}

func (h *ContentHandler) Register(router chi.Router) {
	router.Get("/promos", h.promos)
	router.Get("/promos/active", h.activePromos)
	router.Get("/promos/{id}", h.promo)
	router.Get("/banners", h.banners)
	router.Get("/banners/active", h.activeBanners)
	router.Get("/banners/{id}", h.banner)
}
func (h *ContentHandler) promos(w http.ResponseWriter, r *http.Request) {
	h.writePromos(w, r, false)
}
func (h *ContentHandler) activePromos(w http.ResponseWriter, r *http.Request) {
	h.writePromos(w, r, true)
}
func (h *ContentHandler) writePromos(w http.ResponseWriter, r *http.Request, a bool) {
	v, e := h.content.Promos(r.Context(), a)
	if e != nil {
		writeError(w, 500, "Unable to list promos")
		return
	}
	writeSuccess(w, 200, v)
}
func (h *ContentHandler) banners(w http.ResponseWriter, r *http.Request) {
	h.writeBanners(w, r, false)
}
func (h *ContentHandler) activeBanners(w http.ResponseWriter, r *http.Request) {
	h.writeBanners(w, r, true)
}
func (h *ContentHandler) writeBanners(w http.ResponseWriter, r *http.Request, a bool) {
	v, e := h.content.Banners(r.Context(), a)
	if e != nil {
		writeError(w, 500, "Unable to list banners")
		return
	}
	writeSuccess(w, 200, v)
}

func (h *ContentHandler) promo(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "Invalid promo ID")
		return
	}
	out, err := h.content.Promo(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "Promo not found")
		return
	}
	writeSuccess(w, http.StatusOK, out)
}

func (h *ContentHandler) banner(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "Invalid banner ID")
		return
	}
	out, err := h.content.Banner(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "Banner not found")
		return
	}
	writeSuccess(w, http.StatusOK, out)
}
