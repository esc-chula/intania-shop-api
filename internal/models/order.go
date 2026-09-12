package models

import (
	"fmt"
	"time"
)

// OrderStatus is the client-facing order lifecycle state. Only COMPLETED
// orders are ever persisted today, so this is always the constant value.
type OrderStatus string

// OrderStatusCompleted is the only status a persisted order can have.
const OrderStatusCompleted OrderStatus = "COMPLETED"

// OrderPaymentMethod is the payment method captured at checkout.
type OrderPaymentMethod string

const (
	// OrderPaymentMethodQR represents a QR-code payment backed by an
	// uploaded, trusted payment slip.
	OrderPaymentMethodQR OrderPaymentMethod = "QR_CODE"
	// OrderPaymentMethodCash represents a cash payment.
	OrderPaymentMethodCash OrderPaymentMethod = "REAL_MONEY"
)

// ParseOrderPaymentMethod converts a client-supplied payment method filter.
func ParseOrderPaymentMethod(value string) (OrderPaymentMethod, error) {
	switch OrderPaymentMethod(value) {
	case OrderPaymentMethodQR, OrderPaymentMethodCash:
		return OrderPaymentMethod(value), nil
	default:
		return "", fmt.Errorf("invalid payment method %q", value)
	}
}

// OrderBuyerGender is the buyer demographic captured at checkout.
type OrderBuyerGender string

const (
	OrderBuyerGenderMale           OrderBuyerGender = "MALE"
	OrderBuyerGenderFemale         OrderBuyerGender = "FEMALE"
	OrderBuyerGenderPreferNotToSay OrderBuyerGender = "PREFER_NOT_TO_SAY"
)

// OrderBuyerSnapshot is the buyer information captured at checkout. It is
// never linked to a user account and is immutable once written.
type OrderBuyerSnapshot struct {
	Gender            OrderBuyerGender `json:"gender"`
	Age               *int32           `json:"age"`
	StudentAlumniYear *string          `json:"student_alumni_year"`
}

// OrderStaffSnapshot is the identity of the staff member who processed the
// order, captured at checkout so a later name/email change never rewrites
// history.
type OrderStaffSnapshot struct {
	StaffID  int64  `json:"staff_id"`
	FullName string `json:"full_name"`
	Email    string `json:"email"`
}

// OrderPaymentSnapshot is the payment method and amounts captured at
// checkout. SlipURL is present only for a QR_CODE payment.
type OrderPaymentSnapshot struct {
	Method         OrderPaymentMethod `json:"method"`
	SlipURL        *string            `json:"slip_url"`
	ReceivedAmount *THBAmount         `json:"received_amount"`
	ChangeAmount   *THBAmount         `json:"change_amount"`
	RetainedAmount *THBAmount         `json:"retained_amount"`
	NoChange       *bool              `json:"no_change"`
	Note           *string            `json:"note"`
}

// OrderItemSnapshot is one purchased line item, captured at checkout from the
// then-current Product/Variant so it never changes when the catalogue does.
type OrderItemSnapshot struct {
	OrderItemID            int64     `json:"order_item_id"`
	ProductID              int64     `json:"product_id"`
	VariantID              *int64    `json:"variant_id"`
	ProductName            string    `json:"product_name"`
	Size                   *string   `json:"size"`
	Color                  *string   `json:"color"`
	ImageURL               *string   `json:"image_url"`
	Quantity               int32     `json:"quantity"`
	UnitPrice              THBAmount `json:"unit_price"`
	LineTotal              THBAmount `json:"line_total"`
	InventoryTransactionID int64     `json:"inventory_transaction_id"`
}

// Order is the immutable, fully snapshotted representation of a completed
// POS order, returned by the order detail endpoint.
type Order struct {
	OrderID          int64                    `json:"order_id"`
	OrderNumber      string                   `json:"order_number"`
	ProjectID        int64                    `json:"project_id"`
	Status           OrderStatus              `json:"status"`
	Staff            OrderStaffSnapshot       `json:"staff"`
	Buyer            OrderBuyerSnapshot       `json:"buyer"`
	Items            []OrderItemSnapshot      `json:"items"`
	AppliedPromotion *AppliedProjectPromotion `json:"applied_promotion"`
	Subtotal         THBAmount                `json:"subtotal"`
	Discount         THBAmount                `json:"discount"`
	NetTotal         THBAmount                `json:"net_total"`
	Payment          OrderPaymentSnapshot     `json:"payment"`
	CreatedAt        time.Time                `json:"created_at"`
}

// OrderListResponse is the paginated project order history response.
type OrderListResponse struct {
	Orders     []Order `json:"orders"`
	Total      int64   `json:"total"`
	Page       int32   `json:"page"`
	PageSize   int32   `json:"page_size"`
	TotalPages int32   `json:"total_pages"`
}

// OrderSortField is the whitelisted set of columns the order history can be
// sorted by. Every sort carries a fixed order_id tie-break so paging never
// reorders across pages.
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

// OrderSortOrder is the direction of an order history sort.
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

// OrderFilter narrows the order history list and export. Every field is
// optional and combines with the others.
type OrderFilter struct {
	OrderNumber   string
	PaymentMethod OrderPaymentMethod
	StaffID       int64
	CreatedFrom   time.Time
	CreatedTo     time.Time
	SortBy        OrderSortField
	SortOrder     OrderSortOrder
}

// OrderNumberFilterMaxLength matches the order_number column.
const OrderNumberFilterMaxLength = 100

// ProjectErrorOrderNotFound reports an order that does not exist or belongs
// to a different project.
const ProjectErrorOrderNotFound ProjectAPIErrorCode = "ORDER_NOT_FOUND"
