package usecases

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/esc-chula/intania-shop-api/internal/models"
)

// ErrInvalidPromotionID reports a promotion ID outside the valid range.
var ErrInvalidPromotionID = errors.New("promotion ID must be positive")

// PromotionStore is the persistence surface needed by the Promotion service.
// The repository keeps transaction and project-product resolution details
// behind this interface.
type PromotionStore interface {
	List(context.Context, int64) ([]models.ProjectPromotion, error)
	Detail(context.Context, int64, int64) (models.ProjectPromotion, error)
	Create(context.Context, models.Date, int64, models.ProjectPromotionMutation) (models.ProjectPromotion, error)
	Update(context.Context, models.Date, int64, int64, models.ProjectPromotionMutation) (models.ProjectPromotion, error)
	Delete(context.Context, models.Date, int64, int64) error
}

// PromotionValidationError reports a rejected promotion mutation payload.
// The transport layer can expose its message as a validation response without
// knowing the individual promotion rules.
type PromotionValidationError struct{ Message string }

// Error returns the client-facing validation message.
func (err PromotionValidationError) Error() string { return err.Message }

// PromotionService applies request-level Promotion rules before delegating to
// the repository, which owns transactional project and pricing checks.
type PromotionService struct {
	store PromotionStore
	now   func() time.Time
}

// NewPromotionService constructs a Promotion service over its persistence
// surface.
func NewPromotionService(store PromotionStore) *PromotionService {
	return &PromotionService{store: store, now: time.Now}
}

// List returns every promotion belonging to a project in the unpaginated API
// list-data shape.
func (service *PromotionService) List(ctx context.Context, projectID int64) (models.ProjectPromotionListData, error) {
	if projectID <= 0 {
		return models.ProjectPromotionListData{}, ErrInvalidProjectID
	}

	promotions, err := service.store.List(ctx, projectID)
	if err != nil {
		return models.ProjectPromotionListData{}, fmt.Errorf("list promotions: %w", err)
	}

	return models.ProjectPromotionListData{Promotions: promotions}, nil
}

// Detail returns one promotion belonging to a project.
func (service *PromotionService) Detail(ctx context.Context, projectID, promotionID int64) (models.ProjectPromotion, error) {
	if projectID <= 0 {
		return models.ProjectPromotion{}, ErrInvalidProjectID
	}

	if promotionID <= 0 {
		return models.ProjectPromotion{}, ErrInvalidPromotionID
	}

	promotion, err := service.store.Detail(ctx, projectID, promotionID)
	if err != nil {
		return models.ProjectPromotion{}, fmt.Errorf("get promotion detail: %w", err)
	}

	return promotion, nil
}

// Create validates and persists a new project promotion.
func (service *PromotionService) Create(ctx context.Context, projectID int64, input models.ProjectPromotionMutationRequest) (models.ProjectPromotion, error) {
	if projectID <= 0 {
		return models.ProjectPromotion{}, ErrInvalidProjectID
	}

	validated, err := validatePromotionMutation(input)
	if err != nil {
		return models.ProjectPromotion{}, err
	}

	promotion, err := service.store.Create(ctx, service.today(), projectID, validated)
	if err != nil {
		return models.ProjectPromotion{}, fmt.Errorf("create promotion: %w", err)
	}

	return promotion, nil
}

// Update validates and completely replaces an existing promotion.
func (service *PromotionService) Update(ctx context.Context, projectID, promotionID int64, input models.ProjectPromotionMutationRequest) (models.ProjectPromotion, error) {
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

	promotion, err := service.store.Update(ctx, service.today(), projectID, promotionID, validated)
	if err != nil {
		return models.ProjectPromotion{}, fmt.Errorf("update promotion: %w", err)
	}

	return promotion, nil
}

// Delete removes a promotion from a project.
func (service *PromotionService) Delete(ctx context.Context, projectID, promotionID int64) error {
	if projectID <= 0 {
		return ErrInvalidProjectID
	}

	if promotionID <= 0 {
		return ErrInvalidPromotionID
	}

	if err := service.store.Delete(ctx, service.today(), projectID, promotionID); err != nil {
		return fmt.Errorf("delete promotion: %w", err)
	}

	return nil
}

func (service *PromotionService) today() models.Date {
	return models.TodayInBangkok(service.now())
}

func validatePromotionMutation(input models.ProjectPromotionMutationRequest) (models.ProjectPromotionMutation, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return models.ProjectPromotionMutation{}, PromotionValidationError{Message: "Promotion name must not be empty"}
	}
	if utf8.RuneCountInString(name) > models.PromotionNameMaxLength {
		return models.ProjectPromotionMutation{}, PromotionValidationError{
			Message: fmt.Sprintf("Promotion name must be at most %d characters", models.PromotionNameMaxLength),
		}
	}

	if input.PromotionPrice == nil {
		return models.ProjectPromotionMutation{}, PromotionValidationError{Message: "Promotion price is required"}
	}

	if len(input.Items) == 0 {
		return models.ProjectPromotionMutation{}, PromotionValidationError{Message: "Promotion must contain at least one item"}
	}

	items := make([]models.ProjectPromotionItemInput, len(input.Items))
	copy(items, input.Items)

	seen := make(map[promotionItemKey]struct{}, len(items))

	for index, item := range items {
		if item.ProductID <= 0 {
			return models.ProjectPromotionMutation{}, PromotionValidationError{
				Message: fmt.Sprintf("Promotion item %d product ID must be positive", index+1),
			}
		}

		if item.VariantID != nil && *item.VariantID <= 0 {
			return models.ProjectPromotionMutation{}, PromotionValidationError{
				Message: fmt.Sprintf("Promotion item %d variant ID must be positive", index+1),
			}
		}

		if item.Quantity <= 0 {
			return models.ProjectPromotionMutation{}, PromotionValidationError{
				Message: fmt.Sprintf("Promotion item %d quantity must be positive", index+1),
			}
		}

		key := newPromotionItemKey(item.ProductID, item.VariantID)
		if _, exists := seen[key]; exists {
			return models.ProjectPromotionMutation{}, PromotionValidationError{
				Message: fmt.Sprintf("Promotion items must not contain duplicate product/variant reference at item %d", index+1),
			}
		}

		seen[key] = struct{}{}
	}

	return models.ProjectPromotionMutation{
		Name:           name,
		PromotionPrice: *input.PromotionPrice,
		Items:          items,
	}, nil
}

type promotionItemKey struct {
	productID       int64
	variantID       int64
	variantIDIsNull bool
}

func newPromotionItemKey(productID int64, variantID *int64) promotionItemKey {
	if variantID == nil {
		return promotionItemKey{productID: productID, variantIDIsNull: true}
	}

	return promotionItemKey{productID: productID, variantID: *variantID}
}
