package usecases

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/esc-chula/intania-shop-api/internal/models"
)

type ProductReader interface {
	List(context.Context, int32, int32) ([]models.ProductListItem, int64, error)
	Search(context.Context, string, int32, int32) ([]models.ProductListItem, error)
	Detail(context.Context, int64) (models.ProductDetail, error)
}

type ProductService struct{ products ProductReader }

func NewProductService(products ProductReader) *ProductService {
	return &ProductService{products: products}
}

func (service *ProductService) List(ctx context.Context, page, pageSize int32, includeVariants bool) (any, error) {
	page, pageSize = normalizePage(page, pageSize)
	offset := (page - 1) * pageSize
	items, total, err := service.products.List(ctx, offset, pageSize)
	if err != nil {
		return nil, fmt.Errorf("list catalog: %w", err)
	}
	if !includeVariants {
		return models.ProductListResponse{Products: items, Total: total, Page: page, PageSize: pageSize, TotalPages: totalPages(total, pageSize)}, nil
	}
	details := make([]models.ProductDetail, 0, len(items))
	for _, item := range items {
		detail, err := service.products.Detail(ctx, item.ProductID)
		if err != nil {
			return nil, fmt.Errorf("get listed product detail: %w", err)
		}
		details = append(details, detail)
	}
	return models.ProductDetailListResponse{Products: details, Total: total, Page: page, PageSize: pageSize, TotalPages: totalPages(total, pageSize)}, nil
}

func (service *ProductService) Search(ctx context.Context, query string, page, pageSize int32) ([]models.ProductListItem, error) {
	if strings.TrimSpace(query) == "" {
		return nil, fmt.Errorf("search query is required")
	}
	page, pageSize = normalizePage(page, pageSize)
	items, err := service.products.Search(ctx, query, (page-1)*pageSize, pageSize)
	if err != nil {
		return nil, fmt.Errorf("search catalog: %w", err)
	}
	return items, nil
}

func (service *ProductService) Detail(ctx context.Context, productID int64) (models.ProductDetail, error) {
	if productID <= 0 {
		return models.ProductDetail{}, fmt.Errorf("product ID must be positive")
	}
	product, err := service.products.Detail(ctx, productID)
	if err != nil {
		return models.ProductDetail{}, fmt.Errorf("get product detail: %w", err)
	}
	return product, nil
}

func normalizePage(page, pageSize int32) (int32, int32) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}
	if maxPage := int32(math.MaxInt32) / pageSize; page-1 > maxPage {
		page = maxPage + 1
	}
	return page, pageSize
}
func totalPages(total int64, pageSize int32) int32 {
	return int32((total + int64(pageSize) - 1) / int64(pageSize))
}
