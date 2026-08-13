package usecases

import (
	"context"
	"fmt"
	"github.com/esc-chula/intania-shop-api/internal/models"
)

type CartAdder interface {
	Add(context.Context, int64, int64, int32) (models.CartItem, error)
}

type CartStore interface {
	CartAdder
	Get(context.Context, int64) (models.Cart, error)
}

type CartService struct{ carts CartStore }

func NewCartService(carts CartStore) *CartService { return &CartService{carts: carts} }

func (s *CartService) Add(ctx context.Context, userID int64, request models.AddCartItemRequest) (models.AddCartItemResponse, error) {
	if userID <= 0 || request.VariantID <= 0 || request.Quantity <= 0 {
		return models.AddCartItemResponse{}, fmt.Errorf("variant ID and quantity must be positive")
	}
	item, e := s.carts.Add(ctx, userID, request.VariantID, request.Quantity)
	if e != nil {
		return models.AddCartItemResponse{}, fmt.Errorf("add cart item: %w", e)
	}
	return models.AddCartItemResponse{Item: item, Message: "Item added to cart"}, nil
}

func (s *CartService) Get(ctx context.Context, userID int64) (models.Cart, error) {
	if userID <= 0 {
		return models.Cart{}, fmt.Errorf("user ID must be positive")
	}
	cart, err := s.carts.Get(ctx, userID)
	if err != nil {
		return models.Cart{}, fmt.Errorf("get cart: %w", err)
	}
	return cart, nil
}
