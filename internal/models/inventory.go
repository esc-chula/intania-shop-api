package models

import "time"

// AdjustStockRequest changes either product or variant stock. The acting user is derived from JWT claims.
type AdjustStockRequest struct {
	VariantID         *int64  `json:"variant_id"`
	QuantityChange    int32   `json:"quantity_change"`
	Reason            string  `json:"reason"`
	Notes             *string `json:"notes"`
	ReferenceType     *string `json:"reference_type"`
	ReferenceID       *int64  `json:"reference_id"`
	ConfirmationImage *string `json:"confirmation_image"`
}

// StockTransaction records an immutable inventory change.
type StockTransaction struct {
	TransactionID     int64     `json:"transaction_id"`
	ProductID         *int64    `json:"product_id"`
	VariantID         *int64    `json:"variant_id"`
	TransactionType   string    `json:"transaction_type"`
	QuantityChange    int32     `json:"quantity_change"`
	QuantityBefore    int32     `json:"quantity_before"`
	QuantityAfter     int32     `json:"quantity_after"`
	Reason            *string   `json:"reason"`
	Notes             *string   `json:"notes"`
	ReferenceType     *string   `json:"reference_type"`
	ReferenceID       *int64    `json:"reference_id"`
	CreatedBy         *string   `json:"created_by"`
	ConfirmationImage *string   `json:"confirmation_image"`
	ProductName       *string   `json:"product_name,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
}

// StockTransactionListResponse is a paginated inventory history response.
type StockTransactionListResponse struct {
	Transactions []StockTransaction `json:"transactions"`
	Total        int64              `json:"total"`
	Page         int32              `json:"page"`
	PageSize     int32              `json:"page_size"`
	TotalPages   int32              `json:"total_pages"`
}
