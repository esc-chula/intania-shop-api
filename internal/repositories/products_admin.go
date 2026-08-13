package repositories

import (
	"context"
	"encoding/json"
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
	var methods []byte
	if input.PickupMethods != nil {
		methods, e = json.Marshal(input.PickupMethods)
		if e != nil {
			return models.ProductDetail{}, fmt.Errorf("marshal pickup methods: %w", e)
		}
	}
	var id int64
	e = tx.QueryRow(ctx, `INSERT INTO products(name,description,price,status,category,stock_quantity,images,preview_video,shipping,product_type,min_order,max_order,size_chart,pickup_methods,pickup_location,shipping_fee,sku,product_code) VALUES($1,$2,$3::numeric,$4::product_status,$5,$6,$7,$8,$9,$10::product_type,$11,$12,$13,$14,$15,$16::numeric,$17,$18) RETURNING id`, *input.Name, input.Description, *input.BasePrice, status, input.Category, input.StockQuantity, input.Images, input.PreviewVideo, input.Shipping, productType, input.MinOrder, input.MaxOrder, input.SizeChart, methods, input.PickupLocation, input.ShippingFee, input.SKU, input.ProductCode).Scan(&id)
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
	var methods []byte
	var err error
	if input.PickupMethods != nil {
		methods, err = json.Marshal(input.PickupMethods)
		if err != nil {
			return models.ProductDetail{}, fmt.Errorf("marshal pickup methods: %w", err)
		}
	}
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
	tag, err := r.pool.Exec(ctx, `UPDATE products SET name=COALESCE($2,name),description=COALESCE($3,description),price=COALESCE($4::numeric,price),status=COALESCE($5::product_status,status),category=COALESCE($6,category),stock_quantity=COALESCE($7,stock_quantity),images=COALESCE($8,images),preview_video=COALESCE($9,preview_video),shipping=COALESCE($10,shipping),product_type=COALESCE($11::product_type,product_type),min_order=COALESCE($12,min_order),max_order=COALESCE($13,max_order),size_chart=COALESCE($14,size_chart),pickup_methods=COALESCE($15::jsonb,pickup_methods),pickup_location=COALESCE($16,pickup_location),shipping_fee=COALESCE($17::numeric,shipping_fee),sku=COALESCE($18,sku),product_code=COALESCE($19,product_code),updated_at=NOW() WHERE id=$1`, productID, input.Name, input.Description, input.BasePrice, status, input.Category, input.StockQuantity, input.Images, input.PreviewVideo, input.Shipping, productType, input.MinOrder, input.MaxOrder, input.SizeChart, methods, input.PickupLocation, input.ShippingFee, input.SKU, input.ProductCode)
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
