package models

import "time"

// DeliveryType represents an approved legacy delivery mode.
type DeliveryType string

// OrderStatus is the persisted lifecycle status of an order.
type OrderStatus string

const (
	OrderStatusCart           OrderStatus = "CART"
	OrderStatusPendingPayment OrderStatus = "PENDING_PAYMENT"
	OrderStatusConfirmed      OrderStatus = "CONFIRMED"
	OrderStatusShipping       OrderStatus = "SHIPPING"
	OrderStatusCompleted      OrderStatus = "COMPLETED"
)

const (
	DeliveryPickup   DeliveryType = "PICKUP"
	DeliveryShipping DeliveryType = "SHIPPING"
)

// CreateOrderItem identifies a variant and requested quantity.
type CreateOrderItem struct {
	VariantID int64 `json:"variant_id"`
	Quantity  int32 `json:"quantity"`
}

// CreateOrderRequest intentionally omits legacy user_id; identity comes from the access token.
type CreateOrderRequest struct {
	Items           []CreateOrderItem `json:"items"`
	DeliveryType    *DeliveryType     `json:"delivery_type"`
	ShippingAddress *string           `json:"shipping_address"`
	DeliveryFee     *string           `json:"delivery_fee"`
}

type UpdateOrderRequest struct {
	OrderStatus     *OrderStatus  `json:"order_status"`
	DeliveryType    *DeliveryType `json:"delivery_type"`
	ShippingAddress *string       `json:"shipping_address"`
	TrackingNumber  *string       `json:"tracking_number"`
}

type OrderItem struct {
	OrderItemID  int64   `json:"order_item_id"`
	OrderID      int64   `json:"order_id"`
	VariantID    int64   `json:"variant_id"`
	Quantity     int32   `json:"quantity"`
	UnitPrice    string  `json:"unit_price"`
	ProductName  *string `json:"product_name"`
	VariantSize  *string `json:"variant_size"`
	VariantColor *string `json:"variant_color"`
	ImageURL     *string `json:"image_url"`
}

// Order is a user-owned checkout record.
type Order struct {
	OrderID         int64         `json:"order_id"`
	OrderCode       string        `json:"order_code"`
	UserID          int64         `json:"user_id"`
	TotalAmount     string        `json:"total_amount"`
	DeliveryFee     string        `json:"delivery_fee"`
	OrderStatus     OrderStatus   `json:"order_status"`
	DeliveryType    *DeliveryType `json:"delivery_type"`
	ShippingAddress *string       `json:"shipping_address"`
	TrackingNumber  *string       `json:"tracking_number"`
	CreatedAt       time.Time     `json:"created_at"`
	Items           []OrderItem   `json:"items"`
}
