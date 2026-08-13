package repositories

import (
	"context"
	"errors"
	"fmt"
	"github.com/esc-chula/intania-shop-api/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CartRepository struct{ pool *pgxpool.Pool }

func NewCartRepository(pool *pgxpool.Pool) *CartRepository { return &CartRepository{pool: pool} }

func (r *CartRepository) Add(ctx context.Context, userID, variantID int64, quantity int32) (models.CartItem, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return models.CartItem{}, fmt.Errorf("begin cart update: %w", err)
	}
	defer rollback(ctx, tx)
	var cartID int64
	err = tx.QueryRow(ctx, `SELECT cart_id FROM cart WHERE user_id=$1 ORDER BY cart_id LIMIT 1 FOR UPDATE`, userID).Scan(&cartID)
	if err != nil {
		if _, err = tx.Exec(ctx, `INSERT INTO cart(user_id) VALUES($1)`, userID); err != nil {
			return models.CartItem{}, fmt.Errorf("create cart: %w", err)
		}
		if err = tx.QueryRow(ctx, `SELECT cart_id FROM cart WHERE user_id=$1 ORDER BY cart_id LIMIT 1`, userID).Scan(&cartID); err != nil {
			return models.CartItem{}, fmt.Errorf("find cart: %w", err)
		}
	}
	var item models.CartItem
	err = tx.QueryRow(ctx, `UPDATE cart_items SET quantity=quantity+$3 WHERE cart_id=$1 AND variant_id=$2 RETURNING item_id,cart_id,variant_id,quantity`, cartID, variantID, quantity).Scan(&item.ItemID, &item.CartID, &item.VariantID, &item.Quantity)
	if errors.Is(err, pgx.ErrNoRows) {
		err = tx.QueryRow(ctx, `INSERT INTO cart_items(cart_id,variant_id,quantity) VALUES($1,$2,$3) RETURNING item_id,cart_id,variant_id,quantity`, cartID, variantID, quantity).Scan(&item.ItemID, &item.CartID, &item.VariantID, &item.Quantity)
	}
	if err != nil {
		return models.CartItem{}, fmt.Errorf("add cart item: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return models.CartItem{}, fmt.Errorf("commit cart update: %w", err)
	}
	return item, nil
}
