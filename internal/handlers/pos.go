package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/esc-chula/intania-shop-api/internal/middlewares"
	"github.com/esc-chula/intania-shop-api/internal/models"
	"github.com/go-chi/chi/v5"
)

// POSService is the application surface needed by the POS HTTP adapter.
// The concrete usecase owns Cart validation, current-price resolution, stock
// checks, and Promotion calculation.
type POSService interface {
	Catalog(context.Context, int64) (models.POSCatalog, error)
	Quote(context.Context, int64, models.POSCartRequest) (models.POSQuote, error)
	Checkout(context.Context, int64, int64, string, models.POSCheckoutRequest) (models.POSOrder, bool, error)
}

// POSHandler serves the project-scoped POS catalogue, quote, and checkout
// endpoints.
type POSHandler struct {
	pos POSService
}

// NewPOSHandler constructs a POS HTTP adapter.
func NewPOSHandler(pos POSService) *POSHandler {
	return &POSHandler{pos: pos}
}

// Register mounts POS routes on an already-authenticated router.
func (handler *POSHandler) Register(router chi.Router) {
	router.Get("/projects/{project_id}/pos", handler.catalog)
	router.Post("/projects/{project_id}/checkout/quote", handler.quote)
	router.Post("/projects/{project_id}/orders", handler.createOrder)
}

func (handler *POSHandler) catalog(writer http.ResponseWriter, request *http.Request) {
	projectID, ok := projectPathID(writer, request)
	if !ok {
		return
	}

	catalog, err := handler.pos.Catalog(request.Context(), projectID)
	if err != nil {
		writeProjectErrorResponse(writer, request, err, "Unable to get POS catalogue")
		return
	}

	writeSuccess(writer, http.StatusOK, catalog)
}

func (handler *POSHandler) quote(writer http.ResponseWriter, request *http.Request) {
	projectID, ok := projectPathID(writer, request)
	if !ok {
		return
	}

	input, ok := decodePOSCartRequest(writer, request)
	if !ok {
		return
	}

	quote, err := handler.pos.Quote(request.Context(), projectID, input)
	if err != nil {
		writeProjectErrorResponse(writer, request, err, "Unable to quote POS Cart")
		return
	}

	writeSuccess(writer, http.StatusOK, quote)
}

// createOrder creates one paid POS order. The staff identity comes from the
// bearer token and the Idempotency-Key header identifies the checkout attempt;
// neither is read from the request body.
func (handler *POSHandler) createOrder(writer http.ResponseWriter, request *http.Request) {
	projectID, ok := projectPathID(writer, request)
	if !ok {
		return
	}

	identity, ok := middlewares.IdentityFromContext(request.Context())
	if !ok {
		writeProjectError(writer, request, http.StatusUnauthorized,
			models.ProjectErrorAuthenticationRequired, "Authentication required")
		return
	}

	var input models.POSCheckoutRequest
	if !decodePOSBody(writer, request, &input) {
		return
	}

	order, replayed, err := handler.pos.Checkout(request.Context(), projectID, identity.UserID,
		request.Header.Get("Idempotency-Key"), input)
	if err != nil {
		writeProjectErrorResponse(writer, request, err, "Unable to create POS order")
		return
	}

	if replayed {
		// The stored order is returned unchanged and no stock was reduced.
		writer.Header().Set("Idempotency-Replayed", "true")
		writeSuccess(writer, http.StatusOK, order)
		return
	}

	writeSuccess(writer, http.StatusCreated, order)
}

const posBodyLimit = 1 << 20

func decodePOSCartRequest(writer http.ResponseWriter, request *http.Request) (models.POSCartRequest, bool) {
	var input models.POSCartRequest
	if !decodePOSBody(writer, request, &input) {
		return models.POSCartRequest{}, false
	}

	return input, true
}

// decodePOSBody reads one JSON POS payload. Unknown fields are rejected so a
// client-supplied price, discount, or total is reported rather than ignored.
func decodePOSBody(writer http.ResponseWriter, request *http.Request, target any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, posBodyLimit))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(target); err != nil {
		writeProjectError(writer, request, http.StatusBadRequest, models.ProjectErrorValidation,
			fmt.Sprintf("Invalid request body: %s", err))
		return false
	}

	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			err = errors.New("multiple JSON values")
		}

		writeProjectError(writer, request, http.StatusBadRequest, models.ProjectErrorValidation,
			fmt.Sprintf("Invalid request body: %s", err))
		return false
	}

	return true
}
