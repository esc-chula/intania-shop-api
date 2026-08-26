package repositories

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/esc-chula/intania-shop-api/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrProductNotFound = errors.New("product not found")

type ProductRepository struct{ pool *pgxpool.Pool }

func NewProductRepository(pool *pgxpool.Pool) *ProductRepository {
	return &ProductRepository{pool: pool}
}

func (repository *ProductRepository) List(ctx context.Context, offset, limit int32) ([]models.ProductListItem, int64, error) {
	rows, err := repository.pool.Query(ctx, `SELECT id, name, price::text, status::text, category, images FROM products ORDER BY id DESC OFFSET $1 LIMIT $2`, offset, limit)
	if err != nil {
		return nil, 0, fmt.Errorf("list products: %w", err)
	}
	defer rows.Close()
	items := make([]models.ProductListItem, 0)
	for rows.Next() {
		var item models.ProductListItem
		var status string
		if err := rows.Scan(&item.ProductID, &item.Name, &item.BasePrice, &status, &item.Category, &item.Images); err != nil {
			return nil, 0, fmt.Errorf("scan product list item: %w", err)
		}
		item.Status = publicStatus(status)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate products: %w", err)
	}
	var total int64
	if err := repository.pool.QueryRow(ctx, `SELECT COUNT(*) FROM products`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count products: %w", err)
	}
	return items, total, nil
}

func (repository *ProductRepository) Search(ctx context.Context, query string, offset, limit int32) ([]models.ProductListItem, error) {
	rows, err := repository.pool.Query(ctx, `SELECT id, name, price::text, status::text, category, images FROM products WHERE name ILIKE '%' || $1 || '%' ORDER BY id DESC OFFSET $2 LIMIT $3`, query, offset, limit)
	if err != nil {
		return nil, fmt.Errorf("search products: %w", err)
	}
	defer rows.Close()
	items := make([]models.ProductListItem, 0)
	for rows.Next() {
		var item models.ProductListItem
		var status string
		if err := rows.Scan(&item.ProductID, &item.Name, &item.BasePrice, &status, &item.Category, &item.Images); err != nil {
			return nil, fmt.Errorf("scan searched product: %w", err)
		}
		item.Status = publicStatus(status)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate searched products: %w", err)
	}
	return items, nil
}

func (repository *ProductRepository) Detail(ctx context.Context, productID int64) (models.ProductDetail, error) {
	var product models.ProductDetail
	var status string
	var productType *string
	err := repository.pool.QueryRow(ctx, `SELECT id, name, description, price::text, status::text, category, stock_quantity, images, product_type::text, sku, product_code, created_at, updated_at FROM products WHERE id = $1`, productID).Scan(&product.ProductID, &product.Name, &product.Description, &product.BasePrice, &status, &product.Category, &product.StockQuantity, &product.Images, &productType, &product.SKU, &product.ProductCode, &product.CreatedAt, &product.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.ProductDetail{}, ErrProductNotFound
	}
	if err != nil {
		return models.ProductDetail{}, fmt.Errorf("get product: %w", err)
	}
	product.Status = publicStatus(status)
	if productType != nil {
		value := publicType(*productType)
		product.ProductType = &value
	}
	rows, err := repository.pool.Query(ctx, `SELECT variant_id, product_id, size, color, stock_quantity, price::text FROM variants WHERE product_id = $1 ORDER BY variant_id`, productID)
	if err != nil {
		return models.ProductDetail{}, fmt.Errorf("list variants: %w", err)
	}
	defer rows.Close()
	product.Variants = make([]models.Variant, 0)
	for rows.Next() {
		var variant models.Variant
		if err := rows.Scan(&variant.VariantID, &variant.ProductID, &variant.Size, &variant.Color, &variant.StockQuantity, &variant.Price); err != nil {
			return models.ProductDetail{}, fmt.Errorf("scan variant: %w", err)
		}
		product.Variants = append(product.Variants, variant)
	}
	if err := rows.Err(); err != nil {
		return models.ProductDetail{}, fmt.Errorf("iterate variants: %w", err)
	}
	return product, nil
}

func publicStatus(value string) models.ProductStatus {
	switch strings.ToUpper(value) {
	case "PREORDER":
		return models.ProductStatusPreorder
	case "OUT_OF_STOCK":
		return models.ProductStatusOutOfStock
	default:
		return models.ProductStatusInStock
	}
}
func publicType(value string) models.ProductType {
	if strings.ToUpper(value) == "MULTIPLE" {
		return models.ProductTypeMultiple
	}
	return models.ProductTypeSingle
}
