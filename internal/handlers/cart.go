package handlers

import (
	"context"
	"encoding/json"
	"github.com/go-chi/chi/v5"
	"net/http"

	"github.com/esc-chula/intania-shop-api/internal/middlewares"
	"github.com/esc-chula/intania-shop-api/internal/models"
)

type CartAdder interface {
	Add(context.Context, int64, models.AddCartItemRequest) (models.AddCartItemResponse, error)
	Get(context.Context, int64) (models.Cart, error)
}

type CartHandler struct{ carts CartAdder }

func NewCartHandler(carts CartAdder) *CartHandler { return &CartHandler{carts: carts} }

func (handler *CartHandler) Register(router chi.Router) {
	router.Get("/cart", handler.get)
	router.Put("/cart/items", handler.add)
}

func (handler *CartHandler) get(writer http.ResponseWriter, request *http.Request) {
	identity, ok := middlewares.IdentityFromContext(request.Context())
	if !ok {
		writeError(writer, http.StatusUnauthorized, "Authentication required")
		return
	}
	cart, err := handler.carts.Get(request.Context(), identity.UserID)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "Unable to load cart")
		return
	}
	writeSuccess(writer, http.StatusOK, cart)
}

func (handler *CartHandler) add(writer http.ResponseWriter, request *http.Request) {
	identity, ok := middlewares.IdentityFromContext(request.Context())
	if !ok {
		writeError(writer, http.StatusUnauthorized, "Authentication required")
		return
	}
	request.Body = http.MaxBytesReader(writer, request.Body, 1<<20)
	var input models.AddCartItemRequest
	if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
		writeError(writer, http.StatusBadRequest, "Invalid JSON request body")
		return
	}
	response, err := handler.carts.Add(request.Context(), identity.UserID, input)
	if err != nil {
		writeError(writer, http.StatusBadRequest, err.Error())
		return
	}
	writeSuccess(writer, http.StatusOK, response)
}
