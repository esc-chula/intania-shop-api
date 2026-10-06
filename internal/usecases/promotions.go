package usecases

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/esc-chula/intania-shop-api/internal/models"
)

// ErrInvalidPromotionID reports a promotion ID outside the valid range.
var ErrInvalidPromotionID = errors.New("promotion ID must be positive")

// PromotionValidationError reports a rejected promotion mutation payload.
// The transport layer can expose its message as a validation response without
// knowing the individual promotion rules.
type PromotionValidationError struct{ Message string }

// Error returns the client-facing validation message.
func (err PromotionValidationError) Error() string { return err.Message }

// PromotionReader is the read-only persistence surface for promotion
// consumers such as POS pricing.
type PromotionReader interface {
	List(context.Context, int64) ([]models.ProjectPromotion, error)
	Detail(context.Context, int64, int64) (models.ProjectPromotion, error)
}

// PromotionService applies request-level Promotion rules before delegating to
// the repository, which owns transactional project and pricing checks.
type PromotionService struct {
	reader PromotionReader
}

// NewPromotionService constructs a Promotion service over its persistence
// surface.
func NewPromotionService(reader PromotionReader) *PromotionService {
	return &PromotionService{reader: reader}
}

// List returns every promotion belonging to a project in the unpaginated API
// list-data shape.
func (service *PromotionService) List(ctx context.Context, projectID int64) (models.ProjectPromotionListData, error) {
	if projectID <= 0 {
		return models.ProjectPromotionListData{}, ErrInvalidProjectID
	}

	promotions, err := service.reader.List(ctx, projectID)
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

	promotion, err := service.reader.Detail(ctx, projectID, promotionID)
	if err != nil {
		return models.ProjectPromotion{}, fmt.Errorf("get promotion detail: %w", err)
	}

	return promotion, nil
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

	groupsInput := input.ItemGroups
	// Items is never populated by JSON decoding. Retain this small internal
	// compatibility path for callers that construct the Go request directly.
	if len(groupsInput) == 0 && len(input.Items) > 0 {
		groupsInput = make([]models.ProjectPromotionItemGroupInput, len(input.Items))
		for index, item := range input.Items {
			groupsInput[index] = models.ProjectPromotionItemGroupInput{Options: []models.ProjectPromotionItemInput{item}}
		}
	}
	if len(groupsInput) == 0 {
		return models.ProjectPromotionMutation{}, PromotionValidationError{Message: "Promotion must contain at least one item group"}
	}

	groups := make([]models.ProjectPromotionItemGroupInput, len(groupsInput))
	seen := make(map[promotionItemKey]struct{})
	itemNumber := 0
	for groupIndex, inputGroup := range groupsInput {
		if len(inputGroup.Options) == 0 {
			return models.ProjectPromotionMutation{}, PromotionValidationError{
				Message: fmt.Sprintf("Promotion item group %d must contain at least one option", groupIndex+1),
			}
		}

		groups[groupIndex].Options = make([]models.ProjectPromotionItemInput, len(inputGroup.Options))
		copy(groups[groupIndex].Options, inputGroup.Options)
		for _, item := range groups[groupIndex].Options {
			itemNumber++
			if item.ProductID <= 0 {
				return models.ProjectPromotionMutation{}, PromotionValidationError{
					Message: fmt.Sprintf("Promotion option %d product ID must be positive", itemNumber),
				}
			}
			if item.VariantID != nil && *item.VariantID <= 0 {
				return models.ProjectPromotionMutation{}, PromotionValidationError{
					Message: fmt.Sprintf("Promotion option %d variant ID must be positive", itemNumber),
				}
			}
			if item.Quantity <= 0 {
				return models.ProjectPromotionMutation{}, PromotionValidationError{
					Message: fmt.Sprintf("Promotion option %d quantity must be positive", itemNumber),
				}
			}

			key := newPromotionItemKey(item.ProductID, item.VariantID)
			if _, exists := seen[key]; exists {
				return models.ProjectPromotionMutation{}, PromotionValidationError{
					Message: fmt.Sprintf("Promotion options must not contain duplicate product/variant reference at option %d", itemNumber),
				}
			}
			seen[key] = struct{}{}
		}
	}

	return models.ProjectPromotionMutation{
		Name:           name,
		PromotionPrice: *input.PromotionPrice,
		ItemGroups:     groups,
		Items:          flattenPromotionItemGroups(groups),
	}, nil
}

func flattenPromotionItemGroups(groups []models.ProjectPromotionItemGroupInput) []models.ProjectPromotionItemInput {
	items := make([]models.ProjectPromotionItemInput, 0)
	for _, group := range groups {
		items = append(items, group.Options...)
	}
	return items
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
