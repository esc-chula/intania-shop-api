package usecases

import (
	"cmp"
	"errors"
	"fmt"
	"slices"

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
	// ErrPricingDuplicatePromotionItem reports repeated identities anywhere
	// inside one Promotion's item groups.
	ErrPricingDuplicatePromotionItem = errors.New("duplicate pricing Promotion item")
)

// Calculate returns the Cart subtotal, the best eligible repeated Promotion,
// its total discount, and the resulting net total. All arithmetic uses checked
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
		ApplicationCount:    &best.applicationCount,
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

		groups := pricingPromotionGroups(promotion)
		if len(groups) == 0 {
			return fmt.Errorf("%w: Promotion %d has no item groups", ErrPricingInvalidPromotion, promotion.PromotionID)
		}

		seenItems := make(map[pricingItemKey]struct{})
		for groupIndex, group := range groups {
			if len(group.Options) == 0 {
				return fmt.Errorf("%w: Promotion %d group %d has no options", ErrPricingInvalidPromotion, promotion.PromotionID, groupIndex+1)
			}
			for optionIndex, item := range group.Options {
				if item.ProductID <= 0 || (item.VariantID != nil && *item.VariantID <= 0) || item.Quantity <= 0 {
					return fmt.Errorf("%w: Promotion %d group %d option %d has an invalid identity or quantity", ErrPricingInvalidPromotion, promotion.PromotionID, groupIndex+1, optionIndex+1)
				}
				key := newPricingItemKey(item.ProductID, item.VariantID)
				if _, exists := seenItems[key]; exists {
					return fmt.Errorf("%w: Promotion %d repeats product %d", ErrPricingDuplicatePromotionItem, promotion.PromotionID, item.ProductID)
				}
				seenItems[key] = struct{}{}
			}
		}
	}

	return nil
}

func pricingPromotionGroups(promotion models.PricingPromotion) []models.PricingPromotionItemGroup {
	if len(promotion.ItemGroups) > 0 {
		return promotion.ItemGroups
	}
	groups := make([]models.PricingPromotionItemGroup, len(promotion.Items))
	for index, item := range promotion.Items {
		groups[index] = models.PricingPromotionItemGroup{Options: []models.PricingPromotionItem{item}}
	}
	return groups
}

// pricingGroupQuantity counts individual pieces for interchangeable variants.
// Other OR groups count complete option bundles as one unit per application.
func pricingGroupQuantity(group models.PricingPromotionItemGroup) int64 {
	first := group.Options[0]
	for _, option := range group.Options {
		if option.VariantID == nil || option.ProductID != first.ProductID || option.Quantity != first.Quantity {
			return 1
		}
	}
	return int64(first.Quantity)
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
	applicationCount int64
	promotionID      int64
	name             string
	promotionPrice   models.THBAmount
	bundlePrice      models.THBAmount
	discount         models.THBAmount
}

func evaluatePricingPromotion(cartByIdentity map[pricingItemKey]models.PricingCartLine, promotion models.PricingPromotion) (pricingCandidate, bool, error) {
	type availableOption struct {
		price models.THBAmount
		count int64
	}
	groups := pricingPromotionGroups(promotion)
	available := make([][]availableOption, len(groups))
	groupQuantities := make([]int64, len(groups))
	var applications int64
	for groupIndex, group := range groups {
		// Inspect the full definition, including options absent from the Cart.
		groupQuantities[groupIndex] = pricingGroupQuantity(group)
		var capacity int64
		for optionIndex, option := range group.Options {
			quantity := int64(option.Quantity)
			if groupQuantities[groupIndex] > 1 {
				quantity = 1
			}
			line, exists := cartByIdentity[newPricingItemKey(option.ProductID, option.VariantID)]
			if !exists || int64(line.Quantity) < quantity {
				continue
			}
			lineTotal, err := line.UnitPrice.Mul(quantity)
			if err != nil {
				return pricingCandidate{}, false, fmt.Errorf("%w: Promotion %d group %d option %d bundle total: %w", ErrPricingInvalidPromotion, promotion.PromotionID, groupIndex+1, optionIndex+1, err)
			}
			count := int64(line.Quantity) / quantity
			capacity += count
			available[groupIndex] = append(available[groupIndex], availableOption{price: lineTotal, count: count})
		}
		capacity /= groupQuantities[groupIndex]
		if capacity == 0 {
			return pricingCandidate{}, false, nil
		}
		if groupIndex == 0 || capacity < applications {
			applications = capacity
		}
	}

	var bundlePrice models.THBAmount
	for groupIndex, options := range available {
		// ponytail: identities cannot overlap across groups, so choosing the
		// highest-value options independently maximizes the total discount.
		slices.SortStableFunc(options, func(a, b availableOption) int { return cmp.Compare(b.price.Satang(), a.price.Satang()) })
		remaining := applications * groupQuantities[groupIndex]
		for _, option := range options {
			count := min(remaining, option.count)
			total, err := option.price.Mul(count)
			if err != nil {
				return pricingCandidate{}, false, fmt.Errorf("%w: Promotion %d group %d repeated total: %w", ErrPricingInvalidPromotion, promotion.PromotionID, groupIndex+1, err)
			}
			bundlePrice, err = bundlePrice.Add(total)
			if err != nil {
				return pricingCandidate{}, false, fmt.Errorf("%w: Promotion %d bundle total: %w", ErrPricingInvalidPromotion, promotion.PromotionID, err)
			}
			remaining -= count
			if remaining == 0 {
				break
			}
		}
	}
	promotionPrice, err := promotion.PromotionPrice.Mul(applications)
	if err != nil {
		return pricingCandidate{}, false, fmt.Errorf("%w: Promotion %d repeated price: %w", ErrPricingInvalidPromotion, promotion.PromotionID, err)
	}
	discount, err := bundlePrice.Sub(promotionPrice)
	if err != nil {
		return pricingCandidate{}, false, fmt.Errorf("%w: Promotion %d price exceeds its Cart bundle: %w", ErrPricingInvalidPromotion, promotion.PromotionID, err)
	}

	return pricingCandidate{
		applicationCount: applications,
		promotionID:      promotion.PromotionID,
		name:             promotion.Name,
		promotionPrice:   promotionPrice,
		bundlePrice:      bundlePrice,
		discount:         discount,
	}, true, nil
}
