package models

// ProductInput is the admin create/update product payload. Nil fields mean unchanged on update.
type ProductInput struct {
	Name          *string        `json:"name"`
	Description   *string        `json:"description"`
	BasePrice     *string        `json:"base_price"`
	Status        *ProductStatus `json:"status"`
	Category      *string        `json:"category"`
	StockQuantity *int32         `json:"stock_quantity"`
	Images        []*string      `json:"images"`
	ProductType   *ProductType   `json:"product_type"`
	SKU           *string        `json:"sku"`
	ProductCode   *string        `json:"product_code"`
	Variants      []VariantInput `json:"variants"`
}

// VariantInput is the variant mutation payload.
type VariantInput struct {
	Size          *string `json:"size"`
	Color         *string `json:"color"`
	StockQuantity *int32  `json:"stock_quantity"`
	Price         *string `json:"price"`
}
