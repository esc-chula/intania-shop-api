package models

// ProductInput is the legacy-compatible create/update product payload. Nil fields mean unchanged on update.
type ProductInput struct {
	Name           *string        `json:"name"`
	Description    *string        `json:"description"`
	BasePrice      *string        `json:"base_price"`
	Status         *ProductStatus `json:"status"`
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
	Variants       []VariantInput `json:"variants"`
}

// VariantInput is the legacy-compatible variant mutation payload.
type VariantInput struct {
	Size          *string `json:"size"`
	Color         *string `json:"color"`
	StockQuantity *int32  `json:"stock_quantity"`
	Price         *string `json:"price"`
}
