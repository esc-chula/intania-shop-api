package repositories

import (
	"context"
	"fmt"
	"github.com/esc-chula/intania-shop-api/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

type FavoriteRepository struct{ pool *pgxpool.Pool }

func NewFavoriteRepository(pool *pgxpool.Pool) *FavoriteRepository {
	return &FavoriteRepository{pool: pool}
}

func (r *FavoriteRepository) Add(ctx context.Context, userID, productID int64) (models.Favorite, error) {
	var v models.Favorite
	err := r.pool.QueryRow(ctx, `INSERT INTO favorites(user_id,product_id) VALUES($1,$2) ON CONFLICT(user_id,product_id) DO UPDATE SET user_id=EXCLUDED.user_id RETURNING user_id,product_id,created_at`, userID, productID).Scan(&v.UserID, &v.ProductID, &v.CreatedAt)
	if err != nil {
		return models.Favorite{}, fmt.Errorf("add favorite: %w", err)
	}
	return v, nil
}
