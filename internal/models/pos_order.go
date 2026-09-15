package models

import "time"

// POSGender is the buyer demographic recorded with a POS order.
type POSGender string

const (
	// POSGenderMale records a male buyer.
	POSGenderMale POSGender = "MALE"
	// POSGenderFemale records a female buyer.
	POSGenderFemale POSGender = "FEMALE"
	// POSGenderPreferNotToSay records a buyer who declined to answer.
	POSGenderPreferNotToSay POSGender = "PREFER_NOT_TO_SAY"
)

// Valid reports whether gender is a supported buyer demographic.
func (gender POSGender) Valid() bool {
	switch gender {
	case POSGenderMale, POSGenderFemale, POSGenderPreferNotToSay:
		return true
	default:
		return false
	}
}

// POSPaymentMethod is the settlement method of a POS order.
type POSPaymentMethod string

const (
	// POSPaymentQRCode settles through a bank transfer evidenced by a slip.
	POSPaymentQRCode POSPaymentMethod = "QR_CODE"
	// POSPaymentRealMoney settles with cash handed over at the booth.
	POSPaymentRealMoney POSPaymentMethod = "REAL_MONEY"
)

// Valid reports whether method is a supported payment method.
func (method POSPaymentMethod) Valid() bool {
	return method == POSPaymentQRCode || method == POSPaymentRealMoney
}

// POSOrderStatusCompleted is the only status a paid POS order can hold.
const POSOrderStatusCompleted = "COMPLETED"

// POSBuyerAgeMax bounds the recorded buyer age so an obvious typo is rejected.
const POSBuyerAgeMax int32 = 150

// Request text caps. POSStudentAlumniYearMaxLength and POSPaymentNoteMaxLength
// match the columns the checkout snapshot writes; the slip key and
// Idempotency-Key columns are TEXT, so those two bound the API contract only.
const (
	POSStudentAlumniYearMaxLength = 50
	POSPaymentNoteMaxLength       = 500
	POSSlipObjectKeyMaxLength     = 1024
	POSIdempotencyKeyMaxLength    = 100
)

// PaymentSlipMaxBytes is the largest accepted QR payment slip upload.
const PaymentSlipMaxBytes int64 = 10 << 20

// POSBuyer is the optional demographic information collected at the booth.
type POSBuyer struct {
	Gender            POSGender `json:"gender"`
	Age               *int32    `json:"age"`
	StudentAlumniYear *string   `json:"student_alumni_year"`
}

// POSPaymentRequest is the client-supplied settlement instruction. Every
// method-specific field is a pointer so a field supplied for the wrong method
// is rejected instead of silently defaulted. Amounts owed are never accepted
// from the client; only the cash tendered is.
type POSPaymentRequest struct {
	Method         POSPaymentMethod `json:"method"`
	SlipObjectKey  *string          `json:"slip_object_key"`
	ReceivedAmount *THBAmount       `json:"received_amount"`
	NoChange       *bool            `json:"no_change"`
	Note           *string          `json:"note"`
}

// POSCheckoutRequest is the client-supplied paid checkout. Prices, discounts,
// and totals are deliberately absent: the server resolves all money from the
// current catalogue and Promotions inside the checkout transaction.
type POSCheckoutRequest struct {
	Buyer   POSBuyer             `json:"buyer"`
	Items   []POSCartItemRequest `json:"items"`
	Payment POSPaymentRequest    `json:"payment"`
}

// PaymentSlip is a stored, server-trusted QR payment slip object.
type PaymentSlip struct {
	ObjectKey   string `json:"object_key"`
	URL         string `json:"url"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
}

// POSStaffSnapshot is the acting administrator recorded on an order.
type POSStaffSnapshot struct {
	StaffID  int64  `json:"staff_id"`
	FullName string `json:"full_name"`
	Email    string `json:"email"`
}

// POSPaymentSnapshot is the settlement recorded with a paid order. The cash
// fields are null for QR orders and the slip URL is null for cash orders.
type POSPaymentSnapshot struct {
	Method         POSPaymentMethod `json:"method"`
	SlipURL        *string          `json:"slip_url"`
	ReceivedAmount *THBAmount       `json:"received_amount"`
	ChangeAmount   *THBAmount       `json:"change_amount"`
	RetainedAmount *THBAmount       `json:"retained_amount"`
	NoChange       *bool            `json:"no_change"`
	Note           *string          `json:"note"`
}

// POSOrderItemSnapshot is one paid line. Every descriptive and monetary field
// is a copy taken at payment time, so later catalogue edits cannot rewrite it.
type POSOrderItemSnapshot struct {
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

// POSOrder is an immutable paid POS order.
type POSOrder struct {
	OrderID          int64                    `json:"order_id"`
	OrderNumber      string                   `json:"order_number"`
	ProjectID        int64                    `json:"project_id"`
	Status           string                   `json:"status"`
	Staff            POSStaffSnapshot         `json:"staff"`
	Buyer            POSBuyer                 `json:"buyer"`
	Items            []POSOrderItemSnapshot   `json:"items"`
	AppliedPromotion *AppliedProjectPromotion `json:"applied_promotion"`
	Subtotal         THBAmount                `json:"subtotal"`
	Discount         THBAmount                `json:"discount"`
	NetTotal         THBAmount                `json:"net_total"`
	Payment          POSPaymentSnapshot       `json:"payment"`
	CreatedAt        time.Time                `json:"created_at"`
}

// POSCheckoutCommand is one validated checkout handed to persistence. The
// fingerprint identifies the request payload for idempotent retries.
type POSCheckoutCommand struct {
	ProjectID          int64
	StaffUserID        int64
	IdempotencyKey     string
	RequestFingerprint string
	Buyer              POSBuyer
	Items              []POSCartItemRequest
	Payment            POSPaymentRequest
}

// POSCheckoutSnapshot is the project status, staff, request-ordered sellable
// items, Promotions, and referenced payment slip read and locked inside the
// checkout transaction. Slip is nil for cash orders.
type POSCheckoutSnapshot struct {
	ProjectStatus ProjectStatus
	Staff         POSStaffSnapshot
	Items         []POSResolvedCartItem
	Promotions    []PricingPromotion
	Slip          *PaymentSlip
}

// POSCheckoutPlan is what the POS use case decided to persist for one
// checkout snapshot. LineTotals is ordered like the snapshot items.
type POSCheckoutPlan struct {
	LineTotals       []THBAmount
	Subtotal         THBAmount
	AppliedPromotion *AppliedProjectPromotion
	Discount         THBAmount
	NetTotal         THBAmount
	Payment          POSPaymentSnapshot
}
