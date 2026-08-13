package models

import (
	"encoding/json"
	"time"
)

// ProductStatus is the client-facing status enum retained from the legacy API.
type ProductStatus string

const (
	// ProductStatusPreorder represents products available only by pre-order.
	ProductStatusPreorder ProductStatus = "Preorder"
	// ProductStatusInStock represents products that can be ordered now.
	ProductStatusInStock ProductStatus = "InStock"
	// ProductStatusOutOfStock represents unavailable products.
	ProductStatusOutOfStock ProductStatus = "OutOfStock"
)

// ProductType is the client-facing product type enum.
type ProductType string

const (
	// ProductTypeSingle represents a product without selectable variants.
	ProductTypeSingle ProductType = "Single"
	// ProductTypeMultiple represents a product with selectable variants.
	ProductTypeMultiple ProductType = "Multiple"
)

// PickupMethods contains fulfillment options for a product.
type PickupMethods struct {
	SelfPickup   bool `json:"self_pickup"`
	HomeDelivery bool `json:"home_delivery"`
}

// ProductListItem is the compact public catalog representation.
type ProductListItem struct {
	ProductID int64         `json:"product_id"`
	Name      string        `json:"name"`
	BasePrice string        `json:"base_price"`
	Status    ProductStatus `json:"status"`
	Category  *string       `json:"category"`
	Images    []*string     `json:"images"`
	MinOrder  *int32        `json:"min_order"`
	MaxOrder  *int32        `json:"max_order"`
}

// Variant is a purchasable product option.
type Variant struct {
	VariantID     int64   `json:"variant_id"`
	ProductID     int64   `json:"product_id"`
	Size          *string `json:"size"`
	Color         *string `json:"color"`
	StockQuantity *int32  `json:"stock_quantity"`
	Price         *string `json:"price"`
}

// ProductDetail is the complete public product representation.
type ProductDetail struct {
	ProductID      int64          `json:"product_id"`
	Name           string         `json:"name"`
	Description    *string        `json:"description"`
	BasePrice      string         `json:"base_price"`
	Status         ProductStatus  `json:"status"`
	Category       *string        `json:"category"`
	StockQuantity  *int32         `json:"stock_quantity"`
	Images         []*string      `json:"images"`
	PreviewVideo   *string        `json:"preview_video"`
	Shipping       []*string      `json:"shipping"`
	ProductType    *ProductType   `json:"product_type"`
	MinOrder       *int32         `json:"min_order"`
	MaxOrder       *int32         `json:"max_order"`
	SizeChart      *string        `json:"size_chart"`
	PickupMethods  *PickupMethods `json:"pickup_methods"`
	PickupLocation *string        `json:"pickup_location"`
	ShippingFee    *string        `json:"shipping_fee"`
	SKU            *string        `json:"sku"`
	ProductCode    *string        `json:"product_code"`
	CreatedAt      *time.Time     `json:"created_at"`
	UpdatedAt      *time.Time     `json:"updated_at"`
	Variants       []Variant      `json:"variants"`
}

// ProductListResponse is the paginated legacy catalog response.
type ProductListResponse struct {
	Products   []ProductListItem `json:"products"`
	Total      int64             `json:"total"`
	Page       int32             `json:"page"`
	PageSize   int32             `json:"page_size"`
	TotalPages int32             `json:"total_pages"`
}

// ProductDetailListResponse is the paginated catalog response with variants.
type ProductDetailListResponse struct {
	Products   []ProductDetail `json:"products"`
	Total      int64           `json:"total"`
	Page       int32           `json:"page"`
	PageSize   int32           `json:"page_size"`
	TotalPages int32           `json:"total_pages"`
}

// DecodePickupMethods translates the persisted JSON document when present.
func DecodePickupMethods(value []byte) (*PickupMethods, error) {
	if len(value) == 0 || string(value) == "null" {
		return nil, nil
	}
	var methods PickupMethods
	if err := json.Unmarshal(value, &methods); err != nil {
		return nil, err
	}
	return &methods, nil
}
