package repositories

import (
	"context"
	"fmt"

	"github.com/esc-chula/intania-shop-api/internal/models"
)

func (r *ProductRepository) Create(ctx context.Context, input models.ProductInput) (models.ProductDetail, error) {
	if input.Name == nil || input.BasePrice == nil {
		return models.ProductDetail{}, fmt.Errorf("name and base price are required")
	}
	tx, e := r.pool.Begin(ctx)
	if e != nil {
		return models.ProductDetail{}, fmt.Errorf("begin product create: %w", e)
	}
	defer rollback(ctx, tx)
	status := "IN_STOCK"
	if input.Status != nil {
		status = databaseStatus(*input.Status)
	}
	productType := "SINGLE"
	if input.ProductType != nil {
		productType = databaseType(*input.ProductType)
	}
	var id int64
	e = tx.QueryRow(ctx, `INSERT INTO products(name,description,price,status,category,stock_quantity,images,product_type,sku,product_code) VALUES($1,$2,$3::numeric,$4::product_status,$5,$6,COALESCE($7,'{}'::text[]),$8::product_type,$9,$10) RETURNING id`, *input.Name, input.Description, *input.BasePrice, status, input.Category, input.StockQuantity, input.Images, productType, input.SKU, input.ProductCode).Scan(&id)
	if e != nil {
		return models.ProductDetail{}, fmt.Errorf("create product: %w", e)
	}
	for _, variant := range input.Variants {
		if _, e = tx.Exec(ctx, `INSERT INTO variants(product_id,size,color,stock_quantity,price) VALUES($1,$2,$3,$4,$5::numeric)`, id, variant.Size, variant.Color, variant.StockQuantity, variant.Price); e != nil {
			return models.ProductDetail{}, fmt.Errorf("create variant: %w", e)
		}
	}
	if e = tx.Commit(ctx); e != nil {
		return models.ProductDetail{}, fmt.Errorf("commit product: %w", e)
	}
	return r.Detail(ctx, id)
}
func databaseStatus(value models.ProductStatus) string {
	switch value {
	case models.ProductStatusPreorder:
		return "PREORDER"
	case models.ProductStatusOutOfStock:
		return "OUT_OF_STOCK"
	default:
		return "IN_STOCK"
	}
}
func databaseType(value models.ProductType) string {
	if value == models.ProductTypeMultiple {
		return "MULTIPLE"
	}
	return "SINGLE"
}

func (r *ProductRepository) Delete(ctx context.Context, productID int64) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM products WHERE id=$1`, productID)
	if err != nil {
		return fmt.Errorf("delete product: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrProductNotFound
	}
	return nil
}

func (r *ProductRepository) CreateVariant(ctx context.Context, productID int64, input models.VariantInput) (models.Variant, error) {
	var out models.Variant
	err := r.pool.QueryRow(ctx, `INSERT INTO variants(product_id,size,color,stock_quantity,price) VALUES($1,$2,$3,$4,$5::numeric) RETURNING variant_id,product_id,size,color,stock_quantity,price::text`, productID, input.Size, input.Color, input.StockQuantity, input.Price).Scan(&out.VariantID, &out.ProductID, &out.Size, &out.Color, &out.StockQuantity, &out.Price)
	if err != nil {
		return models.Variant{}, fmt.Errorf("create variant: %w", err)
	}
	return out, nil
}

func (r *ProductRepository) DeleteVariant(ctx context.Context, variantID int64) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM variants WHERE variant_id=$1`, variantID)
	if err != nil {
		return fmt.Errorf("delete variant: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("variant not found")
	}
	return nil
}

func (r *ProductRepository) Update(ctx context.Context, productID int64, input models.ProductInput) (models.ProductDetail, error) {
	var err error
	status := (*string)(nil)
	if input.Status != nil {
		value := databaseStatus(*input.Status)
		status = &value
	}
	productType := (*string)(nil)
	if input.ProductType != nil {
		value := databaseType(*input.ProductType)
		productType = &value
	}
	tag, err := r.pool.Exec(ctx, `UPDATE products SET name=COALESCE($2,name),description=COALESCE($3,description),price=COALESCE($4::numeric,price),status=COALESCE($5::product_status,status),category=COALESCE($6,category),stock_quantity=COALESCE($7,stock_quantity),images=COALESCE($8,images),product_type=COALESCE($9::product_type,product_type),sku=COALESCE($10,sku),product_code=COALESCE($11,product_code),updated_at=NOW() WHERE id=$1`, productID, input.Name, input.Description, input.BasePrice, status, input.Category, input.StockQuantity, input.Images, productType, input.SKU, input.ProductCode)
	if err != nil {
		return models.ProductDetail{}, fmt.Errorf("update product: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return models.ProductDetail{}, ErrProductNotFound
	}
	return r.Detail(ctx, productID)
}

func (r *ProductRepository) UpdateVariant(ctx context.Context, variantID int64, input models.VariantInput) (models.Variant, error) {
	var out models.Variant
	err := r.pool.QueryRow(ctx, `UPDATE variants SET size=COALESCE($2,size),color=COALESCE($3,color),stock_quantity=COALESCE($4,stock_quantity),price=COALESCE($5::numeric,price) WHERE variant_id=$1 RETURNING variant_id,product_id,size,color,stock_quantity,price::text`, variantID, input.Size, input.Color, input.StockQuantity, input.Price).Scan(&out.VariantID, &out.ProductID, &out.Size, &out.Color, &out.StockQuantity, &out.Price)
	if err != nil {
		return models.Variant{}, fmt.Errorf("update variant: %w", err)
	}
	return out, nil
}
