package repositories

import (
	"context"
	"errors"
	"fmt"

	"github.com/esc-chula/intania-shop-api/internal/models"
	"github.com/jackc/pgx/v5"
)

func (repository *CartRepository) Get(ctx context.Context, userID int64) (models.Cart, error) {
	var cart models.Cart
	cart.UserID = userID
	err := repository.pool.QueryRow(ctx, `SELECT cart_id FROM cart WHERE user_id=$1 ORDER BY cart_id LIMIT 1`, userID).Scan(&cart.CartID)
	if errors.Is(err, pgx.ErrNoRows) {
		err = repository.pool.QueryRow(ctx, `INSERT INTO cart(user_id) VALUES($1) RETURNING cart_id`, userID).Scan(&cart.CartID)
	}
	if err != nil {
		return models.Cart{}, fmt.Errorf("find or create cart: %w", err)
	}
	rows, err := repository.pool.Query(ctx, `SELECT ci.item_id,ci.cart_id,ci.variant_id,ci.quantity,v.stock_quantity,v.size,v.color,v.price::text,p.name,p.images FROM cart_items ci JOIN variants v ON v.variant_id=ci.variant_id JOIN products p ON p.id=v.product_id WHERE ci.cart_id=$1 ORDER BY ci.item_id`, cart.CartID)
	if err != nil {
		return models.Cart{}, fmt.Errorf("list cart items: %w", err)
	}
	defer rows.Close()
	cart.Items = make([]models.CartItemDetail, 0)
	for rows.Next() {
		var item models.CartItemDetail
		if err := rows.Scan(&item.ItemID, &item.CartID, &item.VariantID, &item.Quantity, &item.AvailableStock, &item.Variant.Size, &item.Variant.Color, &item.Variant.Price, &item.Variant.ProductName, &item.Variant.ProductImages); err != nil {
			return models.Cart{}, fmt.Errorf("scan cart item: %w", err)
		}
		cart.Items = append(cart.Items, item)
	}
	if err := rows.Err(); err != nil {
		return models.Cart{}, fmt.Errorf("iterate cart items: %w", err)
	}
	return cart, nil
}
