package models

import "time"

// AddFavoriteRequest identifies a product to add for the authenticated user.
type AddFavoriteRequest struct {
	ProductID int64 `json:"product_id"`
}

// Favorite is a user-owned saved product.
type Favorite struct {
	UserID    int64     `json:"user_id"`
	ProductID int64     `json:"product_id"`
	CreatedAt time.Time `json:"created_at"`
}

// AddFavoriteResponse confirms the saved product.
type AddFavoriteResponse struct {
	UserID    int64     `json:"user_id"`
	ProductID int64     `json:"product_id"`
	CreatedAt time.Time `json:"created_at"`
	Message   string    `json:"message"`
}
