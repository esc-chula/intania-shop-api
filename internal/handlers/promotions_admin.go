package handlers

import (
	"context"
	"net/http"

	"github.com/esc-chula/intania-shop-api/internal/models"
	"github.com/go-chi/chi/v5"
)

// PromotionAdminService is the application surface needed by the Promotion HTTP
// adapter.
type PromotionAdminService interface {
	Create(context.Context, int64, models.ProjectPromotionMutationRequest) (models.ProjectPromotion, error)
	Update(context.Context, int64, int64, models.ProjectPromotionMutationRequest) (models.ProjectPromotion, error)
	Delete(context.Context, int64, int64) error
}

// PromotionAdminHandler serves project-scoped promotion endpoints.
type PromotionAdminHandler struct {
	promotions PromotionAdminService
}

// NewPromotionAdminHandler constructs an admin-only Promotion HTTP adapter.
func NewPromotionAdminHandler(promotions PromotionAdminService) *PromotionAdminHandler {
	return &PromotionAdminHandler{promotions: promotions}
}

// Register mounts the Promotion routes on an already-authenticated router.
func (handler *PromotionAdminHandler) Register(router chi.Router) {
	router.Post("/projects/{project_id}/promotions", handler.create)
	router.Put("/projects/{project_id}/promotions/{promotion_id}", handler.update)
	router.Delete("/projects/{project_id}/promotions/{promotion_id}", handler.delete)
}

func (handler *PromotionAdminHandler) create(writer http.ResponseWriter, request *http.Request) {
	projectID, ok := projectPathID(writer, request)
	if !ok {
		return
	}

	input, ok := decodePromotionMutation(writer, request)
	if !ok {
		return
	}

	promotion, err := handler.promotions.Create(request.Context(), projectID, input)
	if err != nil {
		writePromotionErrorResponse(writer, request, err, "Unable to create promotion")
		return
	}

	writeSuccess(writer, http.StatusCreated, promotion)
}

func (handler *PromotionAdminHandler) update(writer http.ResponseWriter, request *http.Request) {
	projectID, ok := projectPathID(writer, request)
	if !ok {
		return
	}

	promotionID, ok := promotionPathID(writer, request)
	if !ok {
		return
	}

	input, ok := decodePromotionMutation(writer, request)
	if !ok {
		return
	}

	promotion, err := handler.promotions.Update(request.Context(), projectID, promotionID, input)
	if err != nil {
		writePromotionErrorResponse(writer, request, err, "Unable to update promotion")
		return
	}

	writeSuccess(writer, http.StatusOK, promotion)
}

func (handler *PromotionAdminHandler) delete(writer http.ResponseWriter, request *http.Request) {
	projectID, ok := projectPathID(writer, request)
	if !ok {
		return
	}

	promotionID, ok := promotionPathID(writer, request)
	if !ok {
		return
	}

	if err := handler.promotions.Delete(request.Context(), projectID, promotionID); err != nil {
		writePromotionErrorResponse(writer, request, err, "Unable to delete promotion")
		return
	}

	writer.WriteHeader(http.StatusNoContent)
}
