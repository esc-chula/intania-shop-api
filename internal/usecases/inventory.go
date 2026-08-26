package usecases

import (
	"context"
	"fmt"
	"github.com/esc-chula/intania-shop-api/internal/models"
)

type StockAdjuster interface {
	Adjust(context.Context, int64, models.AdjustStockRequest, string) (models.StockTransaction, error)
	ListProductTransactions(context.Context, int64, int32, int32) (models.StockTransactionListResponse, error)
	ListVariantTransactions(context.Context, int64, int32, int32) (models.StockTransactionListResponse, error)
	ListTransactions(context.Context, int32, int32) (models.StockTransactionListResponse, error)
}

type InventoryService struct{ inventory StockAdjuster }

func NewInventoryService(i StockAdjuster) *InventoryService { return &InventoryService{inventory: i} }

func (s *InventoryService) Adjust(ctx context.Context, productID int64, r models.AdjustStockRequest, actor string) (models.StockTransaction, error) {
	if productID <= 0 || r.QuantityChange == 0 {
		return models.StockTransaction{}, fmt.Errorf("product ID and non-zero quantity change are required")
	}
	return s.inventory.Adjust(ctx, productID, r, actor)
}

func (s *InventoryService) ProductTransactions(ctx context.Context, productID, page, pageSize int32) (models.StockTransactionListResponse, error) {
	if productID <= 0 {
		return models.StockTransactionListResponse{}, fmt.Errorf("product ID must be positive")
	}
	page, pageSize = inventoryPagination(page, pageSize)
	return s.inventory.ListProductTransactions(ctx, int64(productID), (page-1)*pageSize, pageSize)
}

func (s *InventoryService) VariantTransactions(ctx context.Context, variantID, page, pageSize int32) (models.StockTransactionListResponse, error) {
	if variantID <= 0 {
		return models.StockTransactionListResponse{}, fmt.Errorf("variant ID must be positive")
	}
	page, pageSize = inventoryPagination(page, pageSize)
	return s.inventory.ListVariantTransactions(ctx, int64(variantID), (page-1)*pageSize, pageSize)
}

func (s *InventoryService) Transactions(ctx context.Context, page, pageSize int32) (models.StockTransactionListResponse, error) {
	page, pageSize = inventoryPagination(page, pageSize)
	return s.inventory.ListTransactions(ctx, (page-1)*pageSize, pageSize)
}

func inventoryPagination(page, pageSize int32) (int32, int32) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}
