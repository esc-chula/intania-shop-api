package handlers

import (
	"context"
	"encoding/json"
	"github.com/esc-chula/intania-shop-api/internal/middlewares"
	"github.com/esc-chula/intania-shop-api/internal/models"
	"github.com/go-chi/chi/v5"
	"net/http"
)

type FavoriteAdder interface {
	Add(context.Context, int64, models.AddFavoriteRequest) (models.AddFavoriteResponse, error)
}

type FavoriteHandler struct{ favorites FavoriteAdder }

func NewFavoriteHandler(f FavoriteAdder) *FavoriteHandler { return &FavoriteHandler{favorites: f} }

func (h *FavoriteHandler) Register(router chi.Router) {
	router.Put("/favorites", h.add)
}
func (h *FavoriteHandler) add(w http.ResponseWriter, r *http.Request) {
	id, ok := middlewares.IdentityFromContext(r.Context())
	if !ok {
		writeError(w, 401, "Authentication required")
		return
	}
	var input models.AddFavoriteRequest
	if e := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input); e != nil {
		writeError(w, 400, "Invalid JSON request body")
		return
	}
	out, e := h.favorites.Add(r.Context(), id.UserID, input)
	if e != nil {
		writeError(w, 400, e.Error())
		return
	}
	writeSuccess(w, 200, out)
}
