package handlers

import (
	"context"
	"encoding/json"
	"github.com/esc-chula/intania-shop-api/internal/models"
	"github.com/go-chi/chi/v5"
	"net/http"
	"strconv"
)

type ContentAdministrator interface {
	CreatePromo(context.Context, models.ContentInput) (models.Promo, error)
	UpdatePromo(context.Context, int64, models.ContentInput) (models.Promo, error)
	DeletePromo(context.Context, int64) error
	CreateBanner(context.Context, models.ContentInput) (models.Banner, error)
	UpdateBanner(context.Context, int64, models.ContentInput) (models.Banner, error)
	DeleteBanner(context.Context, int64) error
}

type ContentAdminHandler struct{ content ContentAdministrator }

func NewContentAdminHandler(content ContentAdministrator) *ContentAdminHandler {
	return &ContentAdminHandler{content: content}
}

func (h *ContentAdminHandler) Register(router chi.Router) {
	router.Post("/promos", h.createPromo)
	router.Delete("/promos/{id}", h.deletePromo)
	router.Put("/promos/{id}", h.updatePromo)
	router.Post("/banners", h.createBanner)
	router.Delete("/banners/{id}", h.deleteBanner)
	router.Put("/banners/{id}", h.updateBanner)
}
func (h *ContentAdminHandler) input(w http.ResponseWriter, r *http.Request) (models.ContentInput, bool) {
	var in models.ContentInput
	if e := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&in); e != nil {
		writeError(w, 400, "Invalid JSON request body")
		return in, false
	}
	return in, true
}
func (h *ContentAdminHandler) createPromo(w http.ResponseWriter, r *http.Request) {
	in, ok := h.input(w, r)
	if !ok {
		return
	}
	out, e := h.content.CreatePromo(r.Context(), in)
	if e != nil {
		writeError(w, 400, e.Error())
		return
	}
	writeSuccess(w, 201, out)
}
func (h *ContentAdminHandler) deletePromo(w http.ResponseWriter, r *http.Request) {
	id, e := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if e == nil {
		e = h.content.DeletePromo(r.Context(), id)
	}
	if e != nil {
		writeError(w, 404, e.Error())
		return
	}
	w.WriteHeader(204)
}
func (h *ContentAdminHandler) createBanner(w http.ResponseWriter, r *http.Request) {
	in, ok := h.input(w, r)
	if !ok {
		return
	}
	out, e := h.content.CreateBanner(r.Context(), in)
	if e != nil {
		writeError(w, 400, e.Error())
		return
	}
	writeSuccess(w, 201, out)
}
func (h *ContentAdminHandler) deleteBanner(w http.ResponseWriter, r *http.Request) {
	id, e := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if e == nil {
		e = h.content.DeleteBanner(r.Context(), id)
	}
	if e != nil {
		writeError(w, 404, e.Error())
		return
	}
	w.WriteHeader(204)
}

func (h *ContentAdminHandler) updatePromo(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "Invalid promo ID")
		return
	}
	in, ok := h.input(w, r)
	if !ok {
		return
	}
	out, err := h.content.UpdatePromo(r.Context(), id, in)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeSuccess(w, http.StatusOK, out)
}

func (h *ContentAdminHandler) updateBanner(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "Invalid banner ID")
		return
	}
	in, ok := h.input(w, r)
	if !ok {
		return
	}
	out, err := h.content.UpdateBanner(r.Context(), id, in)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeSuccess(w, http.StatusOK, out)
}
