package models

import (
	"fmt"
	"time"
)

// OrderListResponse is the paginated project POS order history response.
type OrderListResponse struct {
	Orders     []POSOrder `json:"orders"`
	Total      int64      `json:"total"`
	Page       int32      `json:"page"`
	PageSize   int32      `json:"page_size"`
	TotalPages int32      `json:"total_pages"`
}

// OrderSortField is the whitelisted set of columns the order history can sort by.
type OrderSortField string

const (
	OrderSortByOrderNumber OrderSortField = "order_number"
	OrderSortByNetTotal    OrderSortField = "net_total"
)

// ParseOrderSortField converts a client-supplied sort_by value.
func ParseOrderSortField(value string) (OrderSortField, error) {
	switch OrderSortField(value) {
	case OrderSortByOrderNumber, OrderSortByNetTotal:
		return OrderSortField(value), nil
	default:
		return "", fmt.Errorf("invalid sort_by %q", value)
	}
}

// OrderSortOrder is the direction of an order-history sort.
type OrderSortOrder string

const (
	OrderSortAsc  OrderSortOrder = "asc"
	OrderSortDesc OrderSortOrder = "desc"
)

// ParseOrderSortOrder converts a client-supplied sort_order value.
func ParseOrderSortOrder(value string) (OrderSortOrder, error) {
	switch OrderSortOrder(value) {
	case OrderSortAsc, OrderSortDesc:
		return OrderSortOrder(value), nil
	default:
		return "", fmt.Errorf("invalid sort_order %q", value)
	}
}

// OrderFilter narrows paid POS order history. Every field is optional.
type OrderFilter struct {
	OrderNumber   string
	PaymentMethod POSPaymentMethod
	StaffID       int64
	CreatedFrom   time.Time
	CreatedTo     time.Time
	SortBy        OrderSortField
	SortOrder     OrderSortOrder
}

// OrderNumberFilterMaxLength matches the order_number column.
const OrderNumberFilterMaxLength = 100

// ProjectErrorOrderNotFound reports an order that does not exist in the project.
const ProjectErrorOrderNotFound ProjectAPIErrorCode = "ORDER_NOT_FOUND"
