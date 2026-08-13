package usecases

import (
	"context"
	"fmt"
	"github.com/esc-chula/intania-shop-api/internal/models"
)

type ProductCreator interface {
	Create(context.Context, models.ProductInput) (models.ProductDetail, error)
	Update(context.Context, int64, models.ProductInput) (models.ProductDetail, error)
	Delete(context.Context, int64) error
	CreateVariant(context.Context, int64, models.VariantInput) (models.Variant, error)
	UpdateVariant(context.Context, int64, models.VariantInput) (models.Variant, error)
	DeleteVariant(context.Context, int64) error
}

type ProductAdminService struct{ products ProductCreator }

func NewProductAdminService(products ProductCreator) *ProductAdminService {
	return &ProductAdminService{products: products}
}

func (s *ProductAdminService) Create(ctx context.Context, input models.ProductInput) (models.ProductDetail, error) {
	if input.Name == nil || *input.Name == "" || input.BasePrice == nil {
		return models.ProductDetail{}, fmt.Errorf("name and base price are required")
	}
	return s.products.Create(ctx, input)
}

func (s *ProductAdminService) Update(ctx context.Context, productID int64, input models.ProductInput) (models.ProductDetail, error) {
	if productID <= 0 {
		return models.ProductDetail{}, fmt.Errorf("product ID must be positive")
	}
	return s.products.Update(ctx, productID, input)
}

func (s *ProductAdminService) Delete(ctx context.Context, productID int64) error {
	if productID <= 0 {
		return fmt.Errorf("product ID must be positive")
	}
	return s.products.Delete(ctx, productID)
}

func (s *ProductAdminService) CreateVariant(ctx context.Context, productID int64, input models.VariantInput) (models.Variant, error) {
	if productID <= 0 {
		return models.Variant{}, fmt.Errorf("product ID must be positive")
	}
	return s.products.CreateVariant(ctx, productID, input)
}

func (s *ProductAdminService) UpdateVariant(ctx context.Context, variantID int64, input models.VariantInput) (models.Variant, error) {
	if variantID <= 0 {
		return models.Variant{}, fmt.Errorf("variant ID must be positive")
	}
	return s.products.UpdateVariant(ctx, variantID, input)
}

func (s *ProductAdminService) DeleteVariant(ctx context.Context, variantID int64) error {
	if variantID <= 0 {
		return fmt.Errorf("variant ID must be positive")
	}
	return s.products.DeleteVariant(ctx, variantID)
}
