package models

import (
	"time"
)

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

// ProjectPromotionMutationRequest is the public create and full-replacement
// payload for a project promotion. PromotionPrice uses a pointer because zero
// is valid and service validation must distinguish an omitted field from
// "0.00". Items uses a plain slice, with nil or empty values rejected by
// validation.
type ProjectPromotionMutationRequest struct {
	Name           string                      `json:"name"`
	PromotionPrice *THBAmount                  `json:"promotion_price"`
	Items          []ProjectPromotionItemInput `json:"items"`
}

// ProjectPromotionMutation is the non-JSON application command produced from
// ProjectPromotionMutationRequest after required-field presence and money
// decoding have been checked. PromotionPrice is a value because it is no
// longer an optional JSON field.
type ProjectPromotionMutation struct {
	Name           string
	PromotionPrice THBAmount
	Items          []ProjectPromotionItemInput
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

// ProjectPromotion is the hydrated project-scoped promotion response.
type ProjectPromotion struct {
	PromotionID         int64                  `json:"promotion_id"`
	ProjectID           int64                  `json:"project_id"`
	Name                string                 `json:"name"`
	Items               []ProjectPromotionItem `json:"items"`
	OriginalBundlePrice THBAmount              `json:"original_bundle_price"`
	PromotionPrice      THBAmount              `json:"promotion_price"`
	Discount            THBAmount              `json:"discount"`
	CreatedAt           time.Time              `json:"created_at"`
	UpdatedAt           time.Time              `json:"updated_at"`
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
