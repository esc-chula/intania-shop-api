package usecases

import (
	"context"
	"fmt"
	"github.com/esc-chula/intania-shop-api/internal/models"
)

type FavoriteAdder interface {
	Add(context.Context, int64, int64) (models.Favorite, error)
}

type FavoriteService struct{ favorites FavoriteAdder }

func NewFavoriteService(f FavoriteAdder) *FavoriteService { return &FavoriteService{favorites: f} }

func (s *FavoriteService) Add(ctx context.Context, userID int64, request models.AddFavoriteRequest) (models.AddFavoriteResponse, error) {
	if userID <= 0 || request.ProductID <= 0 {
		return models.AddFavoriteResponse{}, fmt.Errorf("product ID must be positive")
	}
	v, e := s.favorites.Add(ctx, userID, request.ProductID)
	if e != nil {
		return models.AddFavoriteResponse{}, fmt.Errorf("add favorite: %w", e)
	}
	return models.AddFavoriteResponse{UserID: v.UserID, ProductID: v.ProductID, CreatedAt: v.CreatedAt, Message: "Product added to favorites"}, nil
}
