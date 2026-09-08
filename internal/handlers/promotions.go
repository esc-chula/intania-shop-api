package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/esc-chula/intania-shop-api/internal/models"
	"github.com/esc-chula/intania-shop-api/internal/repositories"
	"github.com/esc-chula/intania-shop-api/internal/usecases"
	"github.com/go-chi/chi/v5"
)

// PromotionService is the application surface needed by the Promotion HTTP
// adapter.
type PromotionService interface {
	List(context.Context, int64) (models.ProjectPromotionListData, error)
	Detail(context.Context, int64, int64) (models.ProjectPromotion, error)
}

// PromotionHandler serves project-scoped promotion endpoints.
type PromotionHandler struct {
	promotions PromotionService
}

// NewPromotionHandler constructs a Promotion HTTP adapter.
func NewPromotionHandler(promotions PromotionService) *PromotionHandler {
	return &PromotionHandler{promotions: promotions}
}

// Register mounts the Promotion routes on an already-authenticated router.
func (handler *PromotionHandler) Register(router chi.Router) {
	router.Get("/projects/{project_id}/promotions", handler.list)
	router.Get("/projects/{project_id}/promotions/{promotion_id}", handler.detail)
}

func (handler *PromotionHandler) list(writer http.ResponseWriter, request *http.Request) {
	projectID, ok := projectPathID(writer, request)
	if !ok {
		return
	}

	data, err := handler.promotions.List(request.Context(), projectID)
	if err != nil {
		writePromotionErrorResponse(writer, request, err, "Unable to list promotions")
		return
	}

	writeSuccess(writer, http.StatusOK, data)
}

func (handler *PromotionHandler) detail(writer http.ResponseWriter, request *http.Request) {
	projectID, ok := projectPathID(writer, request)
	if !ok {
		return
	}

	promotionID, ok := promotionPathID(writer, request)
	if !ok {
		return
	}

	promotion, err := handler.promotions.Detail(request.Context(), projectID, promotionID)
	if err != nil {
		writePromotionErrorResponse(writer, request, err, "Unable to get promotion")
		return
	}

	writeSuccess(writer, http.StatusOK, promotion)
}

func promotionPathID(writer http.ResponseWriter, request *http.Request) (int64, bool) {
	promotionID, err := strconv.ParseInt(request.PathValue("promotion_id"), 10, 64)
	if err != nil {
		writeProjectError(writer, request, http.StatusBadRequest, models.ProjectErrorValidation, "Invalid promotion ID")
		return 0, false
	}

	return promotionID, true
}

func decodePromotionMutation(writer http.ResponseWriter, request *http.Request) (models.ProjectPromotionMutationRequest, bool) {
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()

	var input models.ProjectPromotionMutationRequest
	if err := decoder.Decode(&input); err != nil {
		writeProjectError(writer, request, http.StatusBadRequest, models.ProjectErrorValidation,
			fmt.Sprintf("Invalid request body: %s", err))
		return models.ProjectPromotionMutationRequest{}, false
	}

	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			err = errors.New("multiple JSON values")
		}

		writeProjectError(writer, request, http.StatusBadRequest, models.ProjectErrorValidation,
			fmt.Sprintf("Invalid request body: %s", err))
		return models.ProjectPromotionMutationRequest{}, false
	}

	return input, true
}

func writePromotionErrorResponse(writer http.ResponseWriter, request *http.Request, err error, fallbackMessage string) {
	var validation usecases.PromotionValidationError
	switch {
	case errors.As(err, &validation):
		writeProjectError(writer, request, http.StatusBadRequest, models.ProjectErrorValidation, validation.Message)
	case errors.Is(err, usecases.ErrInvalidProjectID):
		writeProjectError(writer, request, http.StatusBadRequest, models.ProjectErrorValidation, "Invalid project ID")
	case errors.Is(err, usecases.ErrInvalidPromotionID):
		writeProjectError(writer, request, http.StatusBadRequest, models.ProjectErrorValidation, "Invalid promotion ID")
	case errors.Is(err, repositories.ErrProjectNotFound):
		writeProjectError(writer, request, http.StatusNotFound, models.ProjectErrorNotFound, "Project not found")
	case errors.Is(err, repositories.ErrPromotionNotFound):
		writeProjectError(writer, request, http.StatusNotFound, models.ProjectErrorPromotionNotFound, "Promotion not found")
	case errors.Is(err, repositories.ErrProductNotSellable):
		writeProjectError(writer, request, http.StatusConflict, models.ProjectErrorProductNotSellable, "Product is not sellable in project")
	case errors.Is(err, repositories.ErrProjectCompleted):
		writeProjectError(writer, request, http.StatusConflict, models.ProjectErrorCompleted, "Project is completed")
	case errors.Is(err, repositories.ErrPromotionPriceExceedsBundle):
		writeProjectError(writer, request, http.StatusBadRequest, models.ProjectErrorValidation, "Promotion price cannot exceed the current bundle price")
	case errors.Is(err, models.ErrTHBAmountOverflow):
		writeProjectError(writer, request, http.StatusBadRequest, models.ProjectErrorValidation, "Promotion bundle price exceeds the supported range")
	default:
		writeProjectError(writer, request, http.StatusInternalServerError, models.ProjectErrorInternal, fallbackMessage)
	}
}
