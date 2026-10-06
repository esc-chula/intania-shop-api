package models

import (
	"time"
)

// PromotionNameMaxLength is the maximum number of Unicode characters in a
// trimmed promotion name.
const PromotionNameMaxLength = 150

// ProjectPromotionItemInput is the public item reference accepted when a
// promotion is created or completely replaced. The reference is deliberately
// promotion-specific; project_product_id is an internal persistence detail.
// ProductID and Quantity use value fields because zero is invalid and can be
// rejected by ordinary validation. VariantID is nullable; an omitted value is
// treated the same as an explicit JSON null. Clients should send explicit
// null when they mean a variantless assignment.
type ProjectPromotionItemInput struct {
	ProductID int64  `json:"product_id"`
	VariantID *int64 `json:"variant_id"`
	Quantity  int32  `json:"quantity"`
}

// ProjectPromotionItemGroupInput contains alternatives for one required part
// of a Promotion. Exactly one option from every group must be present in a
// Cart for the Promotion to apply.
type ProjectPromotionItemGroupInput struct {
	Options []ProjectPromotionItemInput `json:"options"`
}

// ProjectPromotionMutationRequest is the public create and full-replacement
// payload for a project promotion. PromotionPrice uses a pointer because zero
// is valid and service validation must distinguish an omitted field from
// "0.00".
type ProjectPromotionMutationRequest struct {
	Name           string                           `json:"name"`
	PromotionPrice *THBAmount                       `json:"promotion_price"`
	ItemGroups     []ProjectPromotionItemGroupInput `json:"item_groups"`
	// Items is retained only for in-process callers during the transition. It
	// is deliberately excluded from JSON, so the public API rejects it.
	Items []ProjectPromotionItemInput `json:"-"`
}

// ProjectPromotionMutation is the validated, non-JSON application command
// produced by the Promotion use case from ProjectPromotionMutationRequest.
// PromotionPrice is a value because it is no longer an optional JSON field.
type ProjectPromotionMutation struct {
	Name           string
	PromotionPrice THBAmount
	ItemGroups     []ProjectPromotionItemGroupInput
	// Items is a non-JSON compatibility projection for internal callers.
	Items []ProjectPromotionItemInput
}

// ProjectPromotionItem is an enriched promotion item returned by the API.
// Product and variant details are hydrated from current catalogue and project
// product data; they are not persisted in promotion_items.
type ProjectPromotionItem struct {
	ProductID   int64     `json:"product_id"`
	VariantID   *int64    `json:"variant_id"`
	Quantity    int32     `json:"quantity"`
	ProductName string    `json:"product_name"`
	Size        *string   `json:"size"`
	Color       *string   `json:"color"`
	UnitPrice   THBAmount `json:"unit_price"`
}

// ProjectPromotionItemGroup is an enriched required group. Options are ORed
// within the group while groups themselves are ANDed together.
type ProjectPromotionItemGroup struct {
	Options []ProjectPromotionItem `json:"options"`
}

// ProjectPromotion is the hydrated project-scoped promotion response.
type ProjectPromotion struct {
	PromotionID int64                       `json:"promotion_id"`
	ProjectID   int64                       `json:"project_id"`
	Name        string                      `json:"name"`
	ItemGroups  []ProjectPromotionItemGroup `json:"item_groups"`
	// Deprecated non-JSON flattened projection for internal callers.
	Items                  []ProjectPromotionItem `json:"-"`
	OriginalBundlePriceMin THBAmount              `json:"original_bundle_price_min"`
	OriginalBundlePriceMax THBAmount              `json:"original_bundle_price_max"`
	PromotionPrice         THBAmount              `json:"promotion_price"`
	DiscountMin            THBAmount              `json:"discount_min"`
	DiscountMax            THBAmount              `json:"discount_max"`
	OriginalBundlePrice    THBAmount              `json:"-"`
	Discount               THBAmount              `json:"-"`
	CreatedAt              time.Time              `json:"created_at"`
	UpdatedAt              time.Time              `json:"updated_at"`
}

// ProjectPromotionListData is the unpaginated project-scoped list payload.
type ProjectPromotionListData struct {
	Promotions []ProjectPromotion `json:"promotions"`
}

// ProjectPromotionEnvelope is the single-promotion success envelope described
// by the OpenAPI contract.
type ProjectPromotionEnvelope struct {
	Success bool             `json:"success"`
	Data    ProjectPromotion `json:"data"`
}

// ProjectPromotionListEnvelope is the promotion-list success envelope
// described by the OpenAPI contract.
type ProjectPromotionListEnvelope struct {
	Success bool                     `json:"success"`
	Data    ProjectPromotionListData `json:"data"`
}
