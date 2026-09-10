package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/esc-chula/intania-shop-api/internal/models"
	"github.com/go-chi/chi/v5"
)

// POSService is the application surface needed by the POS HTTP adapter.
// The concrete usecase owns Cart validation, current-price resolution, stock
// checks, and Promotion calculation.
type POSService interface {
	Catalog(context.Context, int64) (models.POSCatalog, error)
	Quote(context.Context, int64, models.POSCartRequest) (models.POSQuote, error)
}

// POSHandler serves the project-scoped POS catalogue and quote endpoints.
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

const posBodyLimit = 1 << 20

func decodePOSCartRequest(writer http.ResponseWriter, request *http.Request) (models.POSCartRequest, bool) {
	decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, posBodyLimit))
	decoder.DisallowUnknownFields()

	var input models.POSCartRequest
	if err := decoder.Decode(&input); err != nil {
		writeProjectError(writer, request, http.StatusBadRequest, models.ProjectErrorValidation,
			fmt.Sprintf("Invalid request body: %s", err))
		return models.POSCartRequest{}, false
	}

	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			err = errors.New("multiple JSON values")
		}

		writeProjectError(writer, request, http.StatusBadRequest, models.ProjectErrorValidation,
			fmt.Sprintf("Invalid request body: %s", err))
		return models.POSCartRequest{}, false
	}

	return input, true
}
