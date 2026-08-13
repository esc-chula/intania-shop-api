package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/esc-chula/intania-shop-api/internal/middlewares"
	"github.com/esc-chula/intania-shop-api/internal/models"
	"github.com/esc-chula/intania-shop-api/internal/repositories"
	"github.com/go-chi/chi/v5"
	"net/http"
	"strconv"
)

type OrderWriterReader interface {
	Create(context.Context, int64, models.CreateOrderRequest) (models.Order, error)
	Get(context.Context, int64, int64, bool) (models.Order, error)
	List(context.Context, int64, bool) ([]models.Order, error)
	Update(context.Context, int64, models.UpdateOrderRequest) (models.Order, error)
	Delete(context.Context, int64) error
}

type OrderHandler struct{ orders OrderWriterReader }

func NewOrderHandler(orders OrderWriterReader) *OrderHandler { return &OrderHandler{orders: orders} }

func (h *OrderHandler) Register(router chi.Router) {
	router.Post("/orders", h.create)
	router.Get("/orders/{id}", h.get)
	router.Get("/orders", h.list)
}

func (h *OrderHandler) RegisterAdmin(router chi.Router) {
	router.Put("/orders/{id}", h.update)
	router.Delete("/orders/{id}", h.delete)
}
func (h *OrderHandler) create(w http.ResponseWriter, r *http.Request) {
	id, ok := middlewares.IdentityFromContext(r.Context())
	if !ok {
		writeError(w, 401, "Authentication required")
		return
	}
	var input models.CreateOrderRequest
	if e := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input); e != nil {
		writeError(w, 400, "Invalid JSON request body")
		return
	}
	out, e := h.orders.Create(r.Context(), id.UserID, input)
	if e != nil {
		writeError(w, 400, e.Error())
		return
	}
	writeSuccess(w, 201, out)
}
func (h *OrderHandler) get(w http.ResponseWriter, r *http.Request) {
	id, ok := middlewares.IdentityFromContext(r.Context())
	if !ok {
		writeError(w, 401, "Authentication required")
		return
	}
	orderID, e := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if e != nil {
		writeError(w, 400, "Invalid order ID")
		return
	}
	out, e := h.orders.Get(r.Context(), orderID, id.UserID, id.Role == models.RoleAdmin)
	if errors.Is(e, repositories.ErrOrderNotFound) {
		writeError(w, http.StatusNotFound, "Order not found")
		return
	}
	if e != nil {
		writeError(w, http.StatusForbidden, e.Error())
		return
	}
	writeSuccess(w, 200, out)
}

func (h *OrderHandler) list(w http.ResponseWriter, r *http.Request) {
	identity, ok := middlewares.IdentityFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	orders, err := h.orders.List(r.Context(), identity.UserID, identity.Role == models.RoleAdmin)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Unable to list orders")
		return
	}
	writeSuccess(w, http.StatusOK, orders)
}

func (h *OrderHandler) update(w http.ResponseWriter, r *http.Request) {
	orderID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || orderID <= 0 {
		writeError(w, http.StatusBadRequest, "Invalid order ID")
		return
	}
	var in models.UpdateOrderRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON request body")
		return
	}
	out, err := h.orders.Update(r.Context(), orderID, in)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeSuccess(w, http.StatusOK, out)
}

func (h *OrderHandler) delete(w http.ResponseWriter, r *http.Request) {
	orderID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || orderID <= 0 {
		writeError(w, http.StatusBadRequest, "Invalid order ID")
		return
	}
	if err := h.orders.Delete(r.Context(), orderID); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
