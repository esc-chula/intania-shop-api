package usecases

import (
	"context"
	"fmt"
	"time"

	"github.com/esc-chula/intania-shop-api/internal/models"
)

type PromotionCreator interface {
	Create(context.Context, models.Date, int64, models.ProjectPromotionMutation) (models.ProjectPromotion, error)
	Update(context.Context, models.Date, int64, int64, models.ProjectPromotionMutation) (models.ProjectPromotion, error)
	Delete(context.Context, models.Date, int64, int64) error
}

// PromotionAdminService applies request-level Promotion rules before delegating to
// the repository, which owns transactional project and pricing checks.
type PromotionAdminService struct {
	creator PromotionCreator
	now     func() time.Time
}

// NewPromotionAdminService constructs a Promotion admin service over its persistence
// surface.
func NewPromotionAdminService(creator PromotionCreator) *PromotionAdminService {
	return &PromotionAdminService{creator: creator, now: time.Now}
}

// Create validates and persists a new project promotion.
func (service *PromotionAdminService) Create(ctx context.Context, projectID int64, input models.ProjectPromotionMutationRequest) (models.ProjectPromotion, error) {
	if projectID <= 0 {
		return models.ProjectPromotion{}, ErrInvalidProjectID
	}

	validated, err := validatePromotionMutation(input)
	if err != nil {
		return models.ProjectPromotion{}, err
	}

	promotion, err := service.creator.Create(ctx, service.today(), projectID, validated)
	if err != nil {
		return models.ProjectPromotion{}, fmt.Errorf("create promotion: %w", err)
	}

	return promotion, nil
}

// Update validates and completely replaces an existing promotion.
func (service *PromotionAdminService) Update(ctx context.Context, projectID, promotionID int64, input models.ProjectPromotionMutationRequest) (models.ProjectPromotion, error) {
	if projectID <= 0 {
		return models.ProjectPromotion{}, ErrInvalidProjectID
	}

	if promotionID <= 0 {
		return models.ProjectPromotion{}, ErrInvalidPromotionID
	}

	validated, err := validatePromotionMutation(input)
	if err != nil {
		return models.ProjectPromotion{}, err
	}

	promotion, err := service.creator.Update(ctx, service.today(), projectID, promotionID, validated)
	if err != nil {
		return models.ProjectPromotion{}, fmt.Errorf("update promotion: %w", err)
	}

	return promotion, nil
}

// Delete removes a promotion from a project.
func (service *PromotionAdminService) Delete(ctx context.Context, projectID, promotionID int64) error {
	if projectID <= 0 {
		return ErrInvalidProjectID
	}

	if promotionID <= 0 {
		return ErrInvalidPromotionID
	}

	if err := service.creator.Delete(ctx, service.today(), projectID, promotionID); err != nil {
		return fmt.Errorf("delete promotion: %w", err)
	}

	return nil
}

func (service *PromotionAdminService) today() models.Date {
	return models.TodayInBangkok(service.now())
}
