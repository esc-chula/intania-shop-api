package usecases

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/esc-chula/intania-shop-api/internal/models"
)

// Checkout creates one paid POS order. Persistence owns the transaction and
// the row locks; every rule below is applied to the snapshot read inside it,
// so prices, Promotions, stock, and payment are validated against current data
// rather than anything the client sent.
//
// The boolean reports an idempotent replay: the stored order is returned
// unchanged and no stock was reduced again.
func (service *POSService) Checkout(ctx context.Context, projectID, staffUserID int64, idempotencyKey string, request models.POSCheckoutRequest) (models.POSOrder, bool, error) {
	if projectID <= 0 {
		return models.POSOrder{}, false, ErrInvalidProjectID
	}
	if staffUserID <= 0 {
		return models.POSOrder{}, false, POSValidationError{Message: "Authenticated staff is required"}
	}

	command, err := newPOSCheckoutCommand(projectID, staffUserID, idempotencyKey, request)
	if err != nil {
		return models.POSOrder{}, false, err
	}

	today := models.TodayInBangkok(service.now())
	order, replayed, err := service.store.Checkout(ctx, today, command,
		func(snapshot models.POSCheckoutSnapshot) (models.POSCheckoutPlan, error) {
			return planPOSCheckout(service.pricing, command, snapshot)
		})
	if err != nil {
		return models.POSOrder{}, false, fmt.Errorf("create POS order: %w", err)
	}

	return order, replayed, nil
}

// planPOSCheckout turns one locked checkout snapshot into the rows to persist.
// It is called inside the checkout transaction, and any error it returns rolls
// the whole checkout back.
func planPOSCheckout(pricing *PricingService, command models.POSCheckoutCommand, snapshot models.POSCheckoutSnapshot) (models.POSCheckoutPlan, error) {
	if snapshot.ProjectStatus != models.ProjectStatusActive {
		return models.POSCheckoutPlan{}, ErrProjectNotActive
	}

	if conflicts := findPOSStockConflicts(snapshot.Items); len(conflicts) != 0 {
		return models.POSCheckoutPlan{}, POSInsufficientStockError{Items: conflicts}
	}

	result, err := pricing.Calculate(pricingCartFromSnapshot(snapshot.Items), snapshot.Promotions)
	if err != nil {
		return models.POSCheckoutPlan{}, fmt.Errorf("calculate POS order totals: %w", err)
	}

	lineTotals, err := posLineTotals(snapshot.Items)
	if err != nil {
		return models.POSCheckoutPlan{}, err
	}

	payment, err := planPOSPayment(command.Payment, snapshot.Slip, result.NetTotal)
	if err != nil {
		return models.POSCheckoutPlan{}, err
	}

	return models.POSCheckoutPlan{
		LineTotals:       lineTotals,
		Subtotal:         result.Subtotal,
		AppliedPromotion: result.AppliedPromotion,
		Discount:         result.Discount,
		NetTotal:         result.NetTotal,
		Payment:          payment,
	}, nil
}

// planPOSPayment settles the request against the server-calculated total. A QR
// order records the URL of the trusted slip that persistence resolved; a cash
// order must cover the total, and declining change requires the exact amount.
func planPOSPayment(request models.POSPaymentRequest, slip *models.PaymentSlip, netTotal models.THBAmount) (models.POSPaymentSnapshot, error) {
	snapshot := models.POSPaymentSnapshot{Method: request.Method, Note: request.Note}

	if request.Method == models.POSPaymentQRCode {
		if slip == nil {
			return models.POSPaymentSnapshot{}, errors.New("QR checkout reached payment planning without a trusted slip")
		}
		slipURL := slip.URL
		snapshot.SlipURL = &slipURL
		return snapshot, nil
	}

	if request.ReceivedAmount == nil || request.NoChange == nil {
		return models.POSPaymentSnapshot{}, errors.New("cash checkout reached payment planning without a tendered amount")
	}

	received := *request.ReceivedAmount
	excess, err := received.Sub(netTotal)
	if errors.Is(err, models.ErrTHBAmountUnderflow) {
		return models.POSPaymentSnapshot{}, POSValidationError{
			Message: fmt.Sprintf("Received amount %s is less than the order total %s", received, netTotal),
		}
	}
	if err != nil {
		return models.POSPaymentSnapshot{}, fmt.Errorf("settle cash payment: %w", err)
	}

	change := excess
	// Change is handed back rather than retained, so the retained amount stays
	// zero: declining change requires the exact amount.
	retained := models.THBAmount{}
	noChange := *request.NoChange
	if noChange && excess.Satang() != 0 {
		return models.POSPaymentSnapshot{}, POSValidationError{
			Message: fmt.Sprintf("Received amount must equal the order total %s exactly when no_change is true", netTotal),
		}
	}

	snapshot.ReceivedAmount = &received
	snapshot.ChangeAmount = &change
	snapshot.RetainedAmount = &retained
	snapshot.NoChange = &noChange
	return snapshot, nil
}

// newPOSCheckoutCommand validates and normalizes one checkout request, then
// fingerprints it so an Idempotency-Key replayed with a different payload can
// be told apart from an identical retry.
func newPOSCheckoutCommand(projectID, staffUserID int64, idempotencyKey string, request models.POSCheckoutRequest) (models.POSCheckoutCommand, error) {
	key := strings.TrimSpace(idempotencyKey)
	switch {
	case key == "":
		return models.POSCheckoutCommand{}, POSValidationError{Message: "Idempotency-Key header is required"}
	case utf8.RuneCountInString(key) > models.POSIdempotencyKeyMaxLength:
		return models.POSCheckoutCommand{}, POSValidationError{
			Message: fmt.Sprintf("Idempotency-Key header cannot be longer than %d characters", models.POSIdempotencyKeyMaxLength),
		}
	}

	if err := validatePOSCartRequest(models.POSCartRequest{Items: request.Items}); err != nil {
		return models.POSCheckoutCommand{}, err
	}

	buyer, err := validatePOSBuyer(request.Buyer)
	if err != nil {
		return models.POSCheckoutCommand{}, err
	}

	payment, err := validatePOSPaymentRequest(request.Payment)
	if err != nil {
		return models.POSCheckoutCommand{}, err
	}

	command := models.POSCheckoutCommand{
		ProjectID:      projectID,
		StaffUserID:    staffUserID,
		IdempotencyKey: key,
		Buyer:          buyer,
		Items:          request.Items,
		Payment:        payment,
	}

	fingerprint, err := posCheckoutFingerprint(command)
	if err != nil {
		return models.POSCheckoutCommand{}, err
	}
	command.RequestFingerprint = fingerprint

	return command, nil
}

func validatePOSBuyer(buyer models.POSBuyer) (models.POSBuyer, error) {
	if !buyer.Gender.Valid() {
		return models.POSBuyer{}, POSValidationError{
			Message: "Buyer gender must be MALE, FEMALE, or PREFER_NOT_TO_SAY",
		}
	}

	validated := models.POSBuyer{Gender: buyer.Gender}

	if buyer.Age != nil {
		if *buyer.Age < 0 || *buyer.Age > models.POSBuyerAgeMax {
			return models.POSBuyer{}, POSValidationError{
				Message: fmt.Sprintf("Buyer age must be between 0 and %d", models.POSBuyerAgeMax),
			}
		}
		age := *buyer.Age
		validated.Age = &age
	}

	cohort, err := optionalPOSText(buyer.StudentAlumniYear, models.POSStudentAlumniYearMaxLength, "Buyer student_alumni_year")
	if err != nil {
		return models.POSBuyer{}, err
	}
	validated.StudentAlumniYear = cohort

	return validated, nil
}

func validatePOSPaymentRequest(payment models.POSPaymentRequest) (models.POSPaymentRequest, error) {
	if !payment.Method.Valid() {
		return models.POSPaymentRequest{}, POSValidationError{
			Message: "Payment method must be QR_CODE or REAL_MONEY",
		}
	}

	note, err := optionalPOSText(payment.Note, models.POSPaymentNoteMaxLength, "Payment note")
	if err != nil {
		return models.POSPaymentRequest{}, err
	}
	validated := models.POSPaymentRequest{Method: payment.Method, Note: note}

	if payment.Method == models.POSPaymentQRCode {
		if payment.ReceivedAmount != nil || payment.NoChange != nil {
			return models.POSPaymentRequest{}, POSValidationError{
				Message: "received_amount and no_change are only valid for a REAL_MONEY payment",
			}
		}

		slipObjectKey, err := optionalPOSText(payment.SlipObjectKey, models.POSSlipObjectKeyMaxLength, "Payment slip_object_key")
		if err != nil {
			return models.POSPaymentRequest{}, err
		}
		if slipObjectKey == nil {
			return models.POSPaymentRequest{}, POSValidationError{
				Message: "slip_object_key is required for a QR_CODE payment",
			}
		}
		validated.SlipObjectKey = slipObjectKey
		return validated, nil
	}

	if payment.SlipObjectKey != nil {
		return models.POSPaymentRequest{}, POSValidationError{
			Message: "slip_object_key is only valid for a QR_CODE payment",
		}
	}
	if payment.ReceivedAmount == nil || payment.NoChange == nil {
		return models.POSPaymentRequest{}, POSValidationError{
			Message: "received_amount and no_change are required for a REAL_MONEY payment",
		}
	}

	receivedAmount := *payment.ReceivedAmount
	noChange := *payment.NoChange
	validated.ReceivedAmount = &receivedAmount
	validated.NoChange = &noChange
	return validated, nil
}

// optionalPOSText trims an optional text field and treats a blank value as
// absent, so that "" and "   " never reach the order snapshot.
func optionalPOSText(value *string, maxLength int, label string) (*string, error) {
	if value == nil {
		return nil, nil
	}

	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil, nil
	}
	if utf8.RuneCountInString(trimmed) > maxLength {
		return nil, POSValidationError{
			Message: fmt.Sprintf("%s cannot be longer than %d characters", label, maxLength),
		}
	}

	return &trimmed, nil
}

// posCheckoutFingerprint hashes the normalized request so the same
// Idempotency-Key can only replay the request that created the order.
func posCheckoutFingerprint(command models.POSCheckoutCommand) (string, error) {
	encoded, err := json.Marshal(struct {
		ProjectID   int64                       `json:"project_id"`
		StaffUserID int64                       `json:"staff_user_id"`
		Buyer       models.POSBuyer             `json:"buyer"`
		Items       []models.POSCartItemRequest `json:"items"`
		Payment     models.POSPaymentRequest    `json:"payment"`
	}{
		ProjectID:   command.ProjectID,
		StaffUserID: command.StaffUserID,
		Buyer:       command.Buyer,
		Items:       command.Items,
		Payment:     command.Payment,
	})
	if err != nil {
		return "", fmt.Errorf("fingerprint POS checkout request: %w", err)
	}

	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}
