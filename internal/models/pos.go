package models

import "time"

// POSCartItemRequest is the client-supplied identity and quantity of one
// project sellable item. Prices and discounts are deliberately absent: the
// server resolves both from the current catalogue and promotions.
type POSCartItemRequest struct {
	ProductID int64  `json:"product_id"`
	VariantID *int64 `json:"variant_id"`
	Quantity  int32  `json:"quantity"`
}

// POSCartRequest is the client-supplied Cart used to request a quotation.
// Request-level validation (including the non-empty and unique-item rules) is
// owned by the POS use case rather than this transport DTO.
type POSCartRequest struct {
	Items []POSCartItemRequest `json:"items"`
}

// POSCategory groups the selected Project Products by their product category.
// A nil Name is the uncategorized group and is ordered after named groups.
type POSCategory struct {
	Name     *string                    `json:"name"`
	Products []ProjectProductAssignment `json:"products"`
}

// POSCatalog is the project-scoped catalogue available to the POS screen.
// Preview is available for every project status; CanCheckout is true only for
// an ACTIVE project.
type POSCatalog struct {
	Project     Project       `json:"project"`
	CanCheckout bool          `json:"can_checkout"`
	Categories  []POSCategory `json:"categories"`
}

// POSQuoteLineItem is a server-resolved Cart line. UnitPrice, LineTotal, and
// AvailableQuantity always come from the current Project Product/catalogue
// snapshot, never from the request body.
type POSQuoteLineItem struct {
	ProductID         int64     `json:"product_id"`
	VariantID         *int64    `json:"variant_id"`
	ProductName       string    `json:"product_name"`
	Size              *string   `json:"size"`
	Color             *string   `json:"color"`
	ImageURL          *string   `json:"image_url"`
	Quantity          int32     `json:"quantity"`
	UnitPrice         THBAmount `json:"unit_price"`
	LineTotal         THBAmount `json:"line_total"`
	AvailableQuantity int32     `json:"available_quantity"`
}

// AppliedProjectPromotion is the one Promotion selected for a quotation.
// OriginalBundlePrice is calculated from the current Cart line prices and the
// Promotion's required quantities.
type AppliedProjectPromotion struct {
	PromotionID         int64     `json:"promotion_id"`
	Name                string    `json:"name"`
	OriginalBundlePrice THBAmount `json:"original_bundle_price"`
	PromotionPrice      THBAmount `json:"promotion_price"`
	Discount            THBAmount `json:"discount"`
}

// POSQuote contains the server-calculated quotation for one Cart.
// AppliedPromotion is nil when no Promotion matches the Cart.
type POSQuote struct {
	ProjectID        int64                    `json:"project_id"`
	Items            []POSQuoteLineItem       `json:"items"`
	Subtotal         THBAmount                `json:"subtotal"`
	AppliedPromotion *AppliedProjectPromotion `json:"applied_promotion"`
	Discount         THBAmount                `json:"discount"`
	NetTotal         THBAmount                `json:"net_total"`
	QuotedAt         time.Time                `json:"quoted_at"`
}

// PricingCartLine is the resolved, money-safe input consumed by the reusable
// Pricing service. It intentionally contains no JSON or client-supplied price
// fields.
type PricingCartLine struct {
	ProductID int64
	VariantID *int64
	Quantity  int32
	UnitPrice THBAmount
}

// PricingPromotionItem describes one required identity in a Promotion. The
// current unit price is looked up from the resolved Cart line when calculating
// a bundle, so stale client values cannot affect the result.
type PricingPromotionItem struct {
	ProductID int64
	VariantID *int64
	Quantity  int32
}

// PricingPromotion is the minimal Promotion projection needed by the pure
// Pricing service.
type PricingPromotion struct {
	PromotionID    int64
	Name           string
	PromotionPrice THBAmount
	Items          []PricingPromotionItem
}

// PricingResult is the money-safe output of the reusable Pricing service.
type PricingResult struct {
	Subtotal         THBAmount
	AppliedPromotion *AppliedProjectPromotion
	Discount         THBAmount
	NetTotal         THBAmount
}
