package models

// CartItem is a mutable line in a user's cart.
type CartItem struct {
	ItemID    int64 `json:"item_id"`
	CartID    int64 `json:"cart_id"`
	VariantID int64 `json:"variant_id"`
	Quantity  int32 `json:"quantity"`
}

// AddCartItemRequest is the approved cart mutation request.
type AddCartItemRequest struct {
	VariantID int64 `json:"variant_id"`
	Quantity  int32 `json:"quantity"`
}

// AddCartItemResponse confirms an item was added or incremented.
type AddCartItemResponse struct {
	Item    CartItem `json:"item"`
	Message string   `json:"message"`
}

// CartVariantDetail contains the catalog snapshot needed to render a cart line.
type CartVariantDetail struct {
	Size          *string   `json:"size"`
	Color         *string   `json:"color"`
	Price         *string   `json:"price"`
	ProductName   string    `json:"product_name"`
	ProductImages []*string `json:"product_images"`
}

// CartItemDetail includes a line, current availability, and catalog display data.
type CartItemDetail struct {
	ItemID         int64             `json:"item_id"`
	CartID         int64             `json:"cart_id"`
	VariantID      int64             `json:"variant_id"`
	Quantity       int32             `json:"quantity"`
	AvailableStock *int32            `json:"available_stock"`
	Variant        CartVariantDetail `json:"variant"`
}

// Cart is the authenticated user's current cart.
type Cart struct {
	CartID int64            `json:"cart_id"`
	UserID int64            `json:"user_id"`
	Items  []CartItemDetail `json:"items"`
}
