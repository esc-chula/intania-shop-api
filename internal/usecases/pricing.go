package usecases

import (
	"errors"
	"fmt"

	"github.com/esc-chula/intania-shop-api/internal/models"
)

// PricingService calculates a server-side Cart quotation from resolved
// catalogue lines and project Promotions. It is stateless and deliberately
// independent of HTTP, persistence, Project status, and stock reservation.
type PricingService struct{}

// NewPricingService constructs the stateless Pricing service.
func NewPricingService() *PricingService { return &PricingService{} }

var (
	// ErrPricingInvalidCart reports malformed resolved Cart input.
	ErrPricingInvalidCart = errors.New("invalid pricing Cart")
	// ErrPricingDuplicateCartItem reports repeated Cart identities.
	ErrPricingDuplicateCartItem = errors.New("duplicate pricing Cart item")
	// ErrPricingInvalidPromotion reports malformed Promotion input or a
	// Promotion whose price cannot produce a non-negative bundle discount.
	ErrPricingInvalidPromotion = errors.New("invalid pricing Promotion")
	// ErrPricingDuplicatePromotion reports repeated Promotion IDs.
	ErrPricingDuplicatePromotion = errors.New("duplicate pricing Promotion")
	// ErrPricingDuplicatePromotionItem reports repeated identities inside one
	// Promotion.
	ErrPricingDuplicatePromotionItem = errors.New("duplicate pricing Promotion item")
)

// Calculate returns the Cart subtotal, the best eligible one-time Promotion,
// its discount, and the resulting net total. All arithmetic uses checked
// fixed-point THBAmount operations.
func (service *PricingService) Calculate(cart []models.PricingCartLine, promotions []models.PricingPromotion) (models.PricingResult, error) {
	cartByIdentity, err := validatePricingCart(cart)
	if err != nil {
		return models.PricingResult{}, err
	}
	if err := validatePricingPromotions(promotions); err != nil {
		return models.PricingResult{}, err
	}

	subtotal, err := calculatePricingSubtotal(cart)
	if err != nil {
		return models.PricingResult{}, err
	}

	var best *pricingCandidate
	for _, promotion := range promotions {
		candidate, eligible, err := evaluatePricingPromotion(cartByIdentity, promotion)
		if err != nil {
			return models.PricingResult{}, err
		}
		if !eligible {
			continue
		}

		if best == nil || candidate.discount.GreaterThan(best.discount) ||
			(candidate.discount.Satang() == best.discount.Satang() && candidate.promotionID < best.promotionID) {
			candidateCopy := candidate
			best = &candidateCopy
		}
	}

	result := models.PricingResult{
		Subtotal: subtotal,
		Discount: models.THBAmount{},
		NetTotal: subtotal,
	}
	if best == nil {
		return result, nil
	}

	netTotal, err := subtotal.Sub(best.discount)
	if err != nil {
		return models.PricingResult{}, fmt.Errorf("%w: calculate net total: %w", ErrPricingInvalidPromotion, err)
	}

	result.AppliedPromotion = &models.AppliedProjectPromotion{
		PromotionID:         best.promotionID,
		Name:                best.name,
		OriginalBundlePrice: best.bundlePrice,
		PromotionPrice:      best.promotionPrice,
		Discount:            best.discount,
	}
	result.Discount = best.discount
	result.NetTotal = netTotal
	return result, nil
}

type pricingItemKey struct {
	productID       int64
	variantID       int64
	variantIDIsNull bool
}

func newPricingItemKey(productID int64, variantID *int64) pricingItemKey {
	if variantID == nil {
		return pricingItemKey{productID: productID, variantIDIsNull: true}
	}
	return pricingItemKey{productID: productID, variantID: *variantID}
}

func validatePricingCart(cart []models.PricingCartLine) (map[pricingItemKey]models.PricingCartLine, error) {
	if len(cart) == 0 {
		return nil, fmt.Errorf("%w: Cart must contain at least one item", ErrPricingInvalidCart)
	}

	byIdentity := make(map[pricingItemKey]models.PricingCartLine, len(cart))
	for index, line := range cart {
		if line.ProductID <= 0 || (line.VariantID != nil && *line.VariantID <= 0) || line.Quantity <= 0 {
			return nil, fmt.Errorf("%w: Cart item %d has an invalid identity or quantity", ErrPricingInvalidCart, index+1)
		}

		key := newPricingItemKey(line.ProductID, line.VariantID)
		if _, exists := byIdentity[key]; exists {
			return nil, fmt.Errorf("%w: Cart item %d repeats product %d", ErrPricingDuplicateCartItem, index+1, line.ProductID)
		}
		byIdentity[key] = line
	}

	return byIdentity, nil
}

func validatePricingPromotions(promotions []models.PricingPromotion) error {
	seenPromotionIDs := make(map[int64]struct{}, len(promotions))
	for promotionIndex, promotion := range promotions {
		if promotion.PromotionID <= 0 {
			return fmt.Errorf("%w: Promotion %d has an invalid ID", ErrPricingInvalidPromotion, promotionIndex+1)
		}
		if _, exists := seenPromotionIDs[promotion.PromotionID]; exists {
			return fmt.Errorf("%w: Promotion ID %d appears more than once", ErrPricingDuplicatePromotion, promotion.PromotionID)
		}
		seenPromotionIDs[promotion.PromotionID] = struct{}{}

		if len(promotion.Items) == 0 {
			return fmt.Errorf("%w: Promotion %d has no items", ErrPricingInvalidPromotion, promotion.PromotionID)
		}

		seenItems := make(map[pricingItemKey]struct{}, len(promotion.Items))
		for itemIndex, item := range promotion.Items {
			if item.ProductID <= 0 || (item.VariantID != nil && *item.VariantID <= 0) || item.Quantity <= 0 {
				return fmt.Errorf("%w: Promotion %d item %d has an invalid identity or quantity", ErrPricingInvalidPromotion, promotion.PromotionID, itemIndex+1)
			}

			key := newPricingItemKey(item.ProductID, item.VariantID)
			if _, exists := seenItems[key]; exists {
				return fmt.Errorf("%w: Promotion %d repeats product %d", ErrPricingDuplicatePromotionItem, promotion.PromotionID, item.ProductID)
			}
			seenItems[key] = struct{}{}
		}
	}

	return nil
}

func calculatePricingSubtotal(cart []models.PricingCartLine) (models.THBAmount, error) {
	var subtotal models.THBAmount
	for index, line := range cart {
		lineTotal, err := line.UnitPrice.Mul(int64(line.Quantity))
		if err != nil {
			return models.THBAmount{}, fmt.Errorf("calculate Cart item %d total: %w", index+1, err)
		}
		subtotal, err = subtotal.Add(lineTotal)
		if err != nil {
			return models.THBAmount{}, fmt.Errorf("calculate Cart subtotal: %w", err)
		}
	}
	return subtotal, nil
}

type pricingCandidate struct {
	promotionID    int64
	name           string
	promotionPrice models.THBAmount
	bundlePrice    models.THBAmount
	discount       models.THBAmount
}

func evaluatePricingPromotion(cartByIdentity map[pricingItemKey]models.PricingCartLine, promotion models.PricingPromotion) (pricingCandidate, bool, error) {
	for _, item := range promotion.Items {
		line, exists := cartByIdentity[newPricingItemKey(item.ProductID, item.VariantID)]
		if !exists || line.Quantity < item.Quantity {
			return pricingCandidate{}, false, nil
		}
	}

	var bundlePrice models.THBAmount
	for itemIndex, item := range promotion.Items {
		line := cartByIdentity[newPricingItemKey(item.ProductID, item.VariantID)]
		lineTotal, err := line.UnitPrice.Mul(int64(item.Quantity))
		if err != nil {
			return pricingCandidate{}, false, fmt.Errorf("%w: Promotion %d item %d bundle total: %w", ErrPricingInvalidPromotion, promotion.PromotionID, itemIndex+1, err)
		}
		bundlePrice, err = bundlePrice.Add(lineTotal)
		if err != nil {
			return pricingCandidate{}, false, fmt.Errorf("%w: Promotion %d bundle total: %w", ErrPricingInvalidPromotion, promotion.PromotionID, err)
		}
	}

	discount, err := bundlePrice.Sub(promotion.PromotionPrice)
	if err != nil {
		return pricingCandidate{}, false, fmt.Errorf("%w: Promotion %d price exceeds its Cart bundle: %w", ErrPricingInvalidPromotion, promotion.PromotionID, err)
	}

	return pricingCandidate{
		promotionID:    promotion.PromotionID,
		name:           promotion.Name,
		promotionPrice: promotion.PromotionPrice,
		bundlePrice:    bundlePrice,
		discount:       discount,
	}, true, nil
}
