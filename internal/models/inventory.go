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
	Gender            *string `json:"gender"`
	ConfirmationImage *string `json:"confirmation_image"`
	PaymentType       *string `json:"-"`
	TotalMoneyReceive *string `json:"-"`
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
	ReferenceType     *string   `json:"reference_type"`
	ReferenceID       *int64    `json:"reference_id"`
	CreatedBy         *string   `json:"created_by"`
	Gender            *string   `json:"gender"`
	ConfirmationImage *string   `json:"confirmation_image"`
	PaymentType       *string   `json:"payment_type"`
	TotalMoneyReceive *string   `json:"total_money_receive"`
	ProductName       *string   `json:"product_name,omitempty"`
	ProductPrice      *string   `json:"product_price,omitempty"`
	Earned            *float64  `json:"earned,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
}

// StockTransactionListResponse is a paginated inventory history response.
type StockTransactionListResponse struct {
	Transactions []StockTransaction `json:"transactions"`
	Total        int64              `json:"total"`
	Page         int32              `json:"page"`
	PageSize     int32              `json:"page_size"`
	TotalPages   int32              `json:"total_pages"`
	TotalEarned  float64            `json:"total_earned"`
}

// BulkStockReductionItem identifies one product or variant reduction.
type BulkStockReductionItem struct {
	ProductID int64  `json:"product_id"`
	VariantID *int64 `json:"variant_id"`
	Quantity  int32  `json:"quantity"`
}

// BulkStockReductionRequest is the completed legacy stock-sale command.
type BulkStockReductionRequest struct {
	Reason            string                   `json:"reason"`
	Operator          string                   `json:"operator"`
	Remark            *string                  `json:"remark"`
	Gender            string                   `json:"gender"`
	PaymentType       string                   `json:"payment_type"`
	TotalMoneyReceive string                   `json:"total_money_receive"`
	ConfirmationImage *string                  `json:"confirmation_image"`
	Items             []BulkStockReductionItem `json:"items"`
}

// BulkStockReductionResult records one successfully reduced item.
type BulkStockReductionResult struct {
	ProductID      int64  `json:"product_id"`
	VariantID      *int64 `json:"variant_id"`
	TransactionID  int64  `json:"transaction_id"`
	QuantityBefore int32  `json:"quantity_before"`
	QuantityAfter  int32  `json:"quantity_after"`
}

// BulkStockReductionError records a rejected bulk item without discarding other items.
type BulkStockReductionError struct {
	ProductID int64  `json:"product_id"`
	VariantID *int64 `json:"variant_id"`
	Error     string `json:"error"`
}

// BulkStockReductionResponse preserves the legacy partial-success result shape.
type BulkStockReductionResponse struct {
	Successful      []BulkStockReductionResult `json:"successful"`
	Failed          []BulkStockReductionError  `json:"failed"`
	TotalItems      int                        `json:"total_items"`
	SuccessfulCount int                        `json:"successful_count"`
	FailedCount     int                        `json:"failed_count"`
}
