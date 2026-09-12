package usecases

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/esc-chula/intania-shop-api/internal/models"
)

func TestPOSServiceCheckoutPricesCashOrderFromTheLockedSnapshot(t *testing.T) {
	store := newPOSCheckoutStore(t)
	service := NewPOSService(store)
	checkoutDay := time.Date(2026, 9, 12, 3, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return checkoutDay }

	order, replayed, err := service.Checkout(context.Background(), 7, 42, "  key-1  ",
		posCashCheckoutRequest(t, "160.00", false))
	if err != nil {
		t.Fatal(err)
	}
	if replayed || order.OrderID != 99 {
		t.Fatalf("order/replayed = %+v/%v", order, replayed)
	}

	if store.command.ProjectID != 7 || store.command.StaffUserID != 42 || store.command.IdempotencyKey != "key-1" {
		t.Fatalf("command identity = %+v", store.command)
	}
	if store.today != models.TodayInBangkok(checkoutDay) {
		t.Fatalf("checkout today = %v", store.today)
	}

	// Two shirts at 40.00 and one cap at 50.00 come from the snapshot, never
	// from the request.
	assertAmount(t, store.plan.Subtotal, "130.00")
	assertAmount(t, store.plan.Discount, "0.00")
	assertAmount(t, store.plan.NetTotal, "130.00")
	if len(store.plan.LineTotals) != 2 {
		t.Fatalf("line totals = %+v", store.plan.LineTotals)
	}
	assertAmount(t, store.plan.LineTotals[0], "80.00")
	assertAmount(t, store.plan.LineTotals[1], "50.00")

	payment := store.plan.Payment
	if payment.Method != models.POSPaymentRealMoney || payment.SlipURL != nil {
		t.Fatalf("payment = %+v", payment)
	}
	assertAmount(t, *payment.ReceivedAmount, "160.00")
	assertAmount(t, *payment.ChangeAmount, "30.00")
	assertAmount(t, *payment.RetainedAmount, "0.00")
	if *payment.NoChange {
		t.Fatalf("no_change = %v", *payment.NoChange)
	}
}

func TestPOSServiceCheckoutAppliesTheBestPromotion(t *testing.T) {
	store := newPOSCheckoutStore(t)
	store.snapshot.Promotions = []models.PricingPromotion{{
		PromotionID:    3,
		Name:           "Shirt and cap set",
		PromotionPrice: posTestAmount(t, "100.00"),
		Items: []models.PricingPromotionItem{
			{ProductID: 10, VariantID: posInt64Pointer(22), Quantity: 2},
			{ProductID: 11, Quantity: 1},
		},
	}}
	service := NewPOSService(store)

	if _, _, err := service.Checkout(context.Background(), 7, 42, "key-1",
		posCashCheckoutRequest(t, "100.00", true)); err != nil {
		t.Fatal(err)
	}

	assertAmount(t, store.plan.Subtotal, "130.00")
	assertAmount(t, store.plan.Discount, "30.00")
	assertAmount(t, store.plan.NetTotal, "100.00")
	if store.plan.AppliedPromotion == nil || store.plan.AppliedPromotion.PromotionID != 3 {
		t.Fatalf("applied promotion = %+v", store.plan.AppliedPromotion)
	}
	assertAmount(t, store.plan.AppliedPromotion.OriginalBundlePrice, "130.00")
}

func TestPOSServiceCheckoutRecordsTheTrustedSlipForQRPayments(t *testing.T) {
	store := newPOSCheckoutStore(t)
	store.snapshot.Slip = &models.PaymentSlip{
		ObjectKey:   "payment-slips/trusted.png",
		URL:         "https://storage.googleapis.com/bucket/payment-slips/trusted.png",
		ContentType: "image/png",
		Size:        2048,
	}
	service := NewPOSService(store)

	if _, _, err := service.Checkout(context.Background(), 7, 42, "key-1",
		posQRCheckoutRequest("payment-slips/trusted.png")); err != nil {
		t.Fatal(err)
	}

	payment := store.plan.Payment
	if payment.Method != models.POSPaymentQRCode || payment.SlipURL == nil || *payment.SlipURL != store.snapshot.Slip.URL {
		t.Fatalf("payment = %+v", payment)
	}
	if payment.ReceivedAmount != nil || payment.ChangeAmount != nil || payment.RetainedAmount != nil || payment.NoChange != nil {
		t.Fatalf("QR payment carries cash fields = %+v", payment)
	}
	if store.command.Payment.SlipObjectKey == nil || *store.command.Payment.SlipObjectKey != "payment-slips/trusted.png" {
		t.Fatalf("slip object key = %+v", store.command.Payment.SlipObjectKey)
	}
}

func TestPOSServiceCheckoutRejectsCashThatDoesNotSettleTheTotal(t *testing.T) {
	tests := []struct {
		name     string
		received string
		noChange bool
		message  string
	}{
		{name: "less than the total", received: "129.99", noChange: false, message: "is less than the order total"},
		{name: "excess while declining change", received: "200.00", noChange: true, message: "exactly when no_change is true"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := newPOSCheckoutStore(t)
			service := NewPOSService(store)

			_, _, err := service.Checkout(context.Background(), 7, 42, "key-1",
				posCashCheckoutRequest(t, test.received, test.noChange))

			var validation POSValidationError
			if !errors.As(err, &validation) || !strings.Contains(validation.Message, test.message) {
				t.Fatalf("error = %v", err)
			}
			if store.planned {
				t.Fatal("a rejected payment still produced a checkout plan")
			}
		})
	}
}

func TestPOSServiceCheckoutRejectsNonActiveProjectsAndStockConflicts(t *testing.T) {
	t.Run("project is not active", func(t *testing.T) {
		store := newPOSCheckoutStore(t)
		store.snapshot.ProjectStatus = models.ProjectStatusCompleted
		service := NewPOSService(store)

		_, _, err := service.Checkout(context.Background(), 7, 42, "key-1",
			posCashCheckoutRequest(t, "130.00", true))
		if !errors.Is(err, ErrProjectNotActive) {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("stock is insufficient", func(t *testing.T) {
		store := newPOSCheckoutStore(t)
		store.snapshot.Items[0].Item.StockQuantity = 1
		service := NewPOSService(store)

		_, _, err := service.Checkout(context.Background(), 7, 42, "key-1",
			posCashCheckoutRequest(t, "130.00", true))

		var conflict POSInsufficientStockError
		if !errors.As(err, &conflict) || len(conflict.Items) != 1 {
			t.Fatalf("error = %v", err)
		}
		if conflict.Items[0].RequestedQuantity != 2 || conflict.Items[0].AvailableQuantity != 1 {
			t.Fatalf("conflict = %+v", conflict.Items[0])
		}
	})
}

func TestPOSServiceCheckoutRejectsInvalidRequests(t *testing.T) {
	cash := func(t *testing.T) models.POSCheckoutRequest {
		return posCashCheckoutRequest(t, "130.00", true)
	}

	tests := []struct {
		name           string
		idempotencyKey string
		mutate         func(*models.POSCheckoutRequest)
		message        string
	}{
		{
			name:           "missing idempotency key",
			idempotencyKey: "   ",
			mutate:         func(*models.POSCheckoutRequest) {},
			message:        "Idempotency-Key header is required",
		},
		{
			name:           "idempotency key too long",
			idempotencyKey: strings.Repeat("k", models.POSIdempotencyKeyMaxLength+1),
			mutate:         func(*models.POSCheckoutRequest) {},
			message:        "cannot be longer than 100 characters",
		},
		{
			name:    "empty cart",
			mutate:  func(request *models.POSCheckoutRequest) { request.Items = nil },
			message: "Cart must contain at least one item",
		},
		{
			name: "duplicate cart item",
			mutate: func(request *models.POSCheckoutRequest) {
				request.Items = append(request.Items, request.Items[0])
			},
			message: "duplicate product/variant reference",
		},
		{
			name:    "unknown gender",
			mutate:  func(request *models.POSCheckoutRequest) { request.Buyer.Gender = "ROBOT" },
			message: "Buyer gender must be",
		},
		{
			name: "age out of range",
			mutate: func(request *models.POSCheckoutRequest) {
				age := models.POSBuyerAgeMax + 1
				request.Buyer.Age = &age
			},
			message: "Buyer age must be between",
		},
		{
			name: "cohort too long",
			mutate: func(request *models.POSCheckoutRequest) {
				cohort := strings.Repeat("y", models.POSStudentAlumniYearMaxLength+1)
				request.Buyer.StudentAlumniYear = &cohort
			},
			message: "student_alumni_year cannot be longer",
		},
		{
			name:    "unknown payment method",
			mutate:  func(request *models.POSCheckoutRequest) { request.Payment.Method = "CHEQUE" },
			message: "Payment method must be",
		},
		{
			name: "cash without received amount",
			mutate: func(request *models.POSCheckoutRequest) {
				request.Payment.ReceivedAmount = nil
			},
			message: "received_amount and no_change are required",
		},
		{
			name: "cash with a slip key",
			mutate: func(request *models.POSCheckoutRequest) {
				request.Payment.SlipObjectKey = posStringPointer("payment-slips/forged.png")
			},
			message: "slip_object_key is only valid",
		},
		{
			name: "QR without a slip key",
			mutate: func(request *models.POSCheckoutRequest) {
				request.Payment = models.POSPaymentRequest{Method: models.POSPaymentQRCode}
			},
			message: "slip_object_key is required",
		},
		{
			name: "QR with cash fields",
			mutate: func(request *models.POSCheckoutRequest) {
				request.Payment.Method = models.POSPaymentQRCode
				request.Payment.SlipObjectKey = posStringPointer("payment-slips/trusted.png")
			},
			message: "only valid for a REAL_MONEY payment",
		},
		{
			name: "note too long",
			mutate: func(request *models.POSCheckoutRequest) {
				note := strings.Repeat("n", models.POSPaymentNoteMaxLength+1)
				request.Payment.Note = &note
			},
			message: "Payment note cannot be longer",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := newPOSCheckoutStore(t)
			service := NewPOSService(store)
			request := cash(t)
			test.mutate(&request)

			key := test.idempotencyKey
			if key == "" {
				key = "key-1"
			}

			_, _, err := service.Checkout(context.Background(), 7, 42, key, request)

			var validation POSValidationError
			if !errors.As(err, &validation) || !strings.Contains(validation.Message, test.message) {
				t.Fatalf("error = %v, want a validation error containing %q", err, test.message)
			}
			if store.calls != 0 {
				t.Fatal("an invalid request reached persistence")
			}
		})
	}
}

func TestPOSServiceCheckoutRejectsAnInvalidProjectOrStaff(t *testing.T) {
	store := newPOSCheckoutStore(t)
	service := NewPOSService(store)

	if _, _, err := service.Checkout(context.Background(), 0, 42, "key-1",
		posCashCheckoutRequest(t, "130.00", true)); !errors.Is(err, ErrInvalidProjectID) {
		t.Fatalf("project error = %v", err)
	}
	if _, _, err := service.Checkout(context.Background(), 7, 0, "key-1",
		posCashCheckoutRequest(t, "130.00", true)); !errors.Is(err, ErrInvalidPOSCart) {
		t.Fatalf("staff error = %v", err)
	}
	if store.calls != 0 {
		t.Fatal("an invalid request reached persistence")
	}
}

// planPOSPayment takes a raw request, so it must reject an incomplete one
// rather than dereference the method-specific fields the use case validated.
func TestPlanPOSPaymentRejectsAnIncompleteRequest(t *testing.T) {
	netTotal := posTestAmount(t, "130.00")
	amount := posTestAmount(t, "130.00")
	noChange := false

	tests := []struct {
		name    string
		request models.POSPaymentRequest
		slip    *models.PaymentSlip
	}{
		{name: "QR without a resolved slip", request: models.POSPaymentRequest{Method: models.POSPaymentQRCode}},
		{name: "cash without a received amount",
			request: models.POSPaymentRequest{Method: models.POSPaymentRealMoney, NoChange: &noChange}},
		{name: "cash without no_change",
			request: models.POSPaymentRequest{Method: models.POSPaymentRealMoney, ReceivedAmount: &amount}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := planPOSPayment(test.request, test.slip, netTotal); err == nil {
				t.Fatal("an incomplete payment request planned successfully")
			}
		})
	}
}

// A fingerprint that ignored the payment would let one Idempotency-Key replay
// a cash order onto a QR retry, or the reverse.
func TestPOSCheckoutFingerprintCoversThePaymentDimension(t *testing.T) {
	fingerprint := func(t *testing.T, request models.POSCheckoutRequest) string {
		t.Helper()
		command, err := newPOSCheckoutCommand(7, 42, "key-1", request)
		if err != nil {
			t.Fatal(err)
		}
		return command.RequestFingerprint
	}

	cash := fingerprint(t, posCashCheckoutRequest(t, "130.00", false))
	more := fingerprint(t, posCashCheckoutRequest(t, "500.00", false))
	exact := fingerprint(t, posCashCheckoutRequest(t, "130.00", true))
	qr := fingerprint(t, posQRCheckoutRequest("payment-slips/one.png"))
	otherSlip := fingerprint(t, posQRCheckoutRequest("payment-slips/two.png"))

	for _, other := range []struct {
		name  string
		value string
	}{
		{name: "a different received amount", value: more},
		{name: "a different no_change", value: exact},
		{name: "a QR payment", value: qr},
	} {
		if other.value == cash {
			t.Fatalf("%s fingerprinted the same as the cash request", other.name)
		}
	}
	if qr == otherSlip {
		t.Fatal("a different slip object key fingerprinted the same")
	}
}

func TestPOSServiceCheckoutFingerprintsTheNormalizedRequest(t *testing.T) {
	fingerprint := func(t *testing.T, mutate func(*models.POSCheckoutRequest)) string {
		t.Helper()
		store := newPOSCheckoutStore(t)
		service := NewPOSService(store)
		request := posCashCheckoutRequest(t, "130.00", true)
		mutate(&request)
		if _, _, err := service.Checkout(context.Background(), 7, 42, "key-1", request); err != nil {
			t.Fatal(err)
		}
		return store.command.RequestFingerprint
	}

	identical := fingerprint(t, func(*models.POSCheckoutRequest) {})
	if identical == "" {
		t.Fatal("fingerprint is empty")
	}
	if again := fingerprint(t, func(*models.POSCheckoutRequest) {}); again != identical {
		t.Fatalf("identical requests fingerprinted differently: %s and %s", identical, again)
	}

	// Trimming happens before fingerprinting, so the same request with padded
	// text still replays.
	padded := fingerprint(t, func(request *models.POSCheckoutRequest) {
		request.Payment.Note = posStringPointer("  booth 1  ")
	})
	trimmed := fingerprint(t, func(request *models.POSCheckoutRequest) {
		request.Payment.Note = posStringPointer("booth 1")
	})
	if padded != trimmed {
		t.Fatalf("normalized note fingerprinted differently: %s and %s", padded, trimmed)
	}
	if padded == identical {
		t.Fatal("a different payload produced the same fingerprint")
	}

	quantityChanged := fingerprint(t, func(request *models.POSCheckoutRequest) {
		request.Items[0].Quantity = 3
	})
	if quantityChanged == identical {
		t.Fatal("a different cart produced the same fingerprint")
	}
}

func TestPOSServiceCheckoutReportsAReplayedOrder(t *testing.T) {
	store := newPOSCheckoutStore(t)
	store.replayed = true
	service := NewPOSService(store)

	order, replayed, err := service.Checkout(context.Background(), 7, 42, "key-1",
		posCashCheckoutRequest(t, "130.00", true))
	if err != nil {
		t.Fatal(err)
	}
	if !replayed || order.OrderID != 99 {
		t.Fatalf("order/replayed = %+v/%v", order, replayed)
	}
}

// Checkout keeps the read-only snapshot stub usable as a POSStore in the
// catalogue and quote tests, which never check out.
func (stub *posSnapshotReaderStub) Checkout(context.Context, models.Date, models.POSCheckoutCommand,
	func(models.POSCheckoutSnapshot) (models.POSCheckoutPlan, error)) (models.POSOrder, bool, error) {
	return models.POSOrder{}, false, errors.New("checkout is not configured in this stub")
}

// posCheckoutStoreStub records the command, runs the planner against a
// configurable locked snapshot, and reports the plan the use case produced.
type posCheckoutStoreStub struct {
	posSnapshotReaderStub
	snapshot models.POSCheckoutSnapshot
	order    models.POSOrder
	replayed bool
	calls    int
	planned  bool
	today    models.Date
	command  models.POSCheckoutCommand
	plan     models.POSCheckoutPlan
}

func newPOSCheckoutStore(t *testing.T) *posCheckoutStoreStub {
	t.Helper()
	return &posCheckoutStoreStub{
		snapshot: models.POSCheckoutSnapshot{
			ProjectStatus: models.ProjectStatusActive,
			Staff:         models.POSStaffSnapshot{StaffID: 42, FullName: "Ada", Email: "ada@example.com"},
			Items: []models.POSResolvedCartItem{
				{
					Item: models.POSResolvedItem{
						ProductID:     10,
						VariantID:     posInt64Pointer(22),
						ProductName:   "Shirt",
						StockQuantity: 5,
						ProjectPrice:  posTestAmount(t, "40.00"),
					},
					Quantity: 2,
				},
				{
					Item: models.POSResolvedItem{
						ProductID:     11,
						ProductName:   "Cap",
						StockQuantity: 3,
						ProjectPrice:  posTestAmount(t, "50.00"),
					},
					Quantity: 1,
				},
			},
		},
		order: models.POSOrder{OrderID: 99, OrderNumber: "ORD-00000099"},
	}
}

func (stub *posCheckoutStoreStub) Checkout(_ context.Context, today models.Date, command models.POSCheckoutCommand,
	planner func(models.POSCheckoutSnapshot) (models.POSCheckoutPlan, error)) (models.POSOrder, bool, error) {
	stub.calls++
	stub.today = today
	stub.command = command
	if stub.replayed {
		return stub.order, true, nil
	}

	plan, err := planner(stub.snapshot)
	if err != nil {
		return models.POSOrder{}, false, err
	}
	stub.planned = true
	stub.plan = plan

	return stub.order, false, nil
}

func posCashCheckoutRequest(t *testing.T, received string, noChange bool) models.POSCheckoutRequest {
	t.Helper()
	amount := posTestAmount(t, received)
	age := int32(21)
	return models.POSCheckoutRequest{
		Buyer: models.POSBuyer{
			Gender:            models.POSGenderFemale,
			Age:               &age,
			StudentAlumniYear: posStringPointer("Intania 105"),
		},
		Items: []models.POSCartItemRequest{
			{ProductID: 10, VariantID: posInt64Pointer(22), Quantity: 2},
			{ProductID: 11, Quantity: 1},
		},
		Payment: models.POSPaymentRequest{
			Method:         models.POSPaymentRealMoney,
			ReceivedAmount: &amount,
			NoChange:       &noChange,
		},
	}
}

func posQRCheckoutRequest(slipObjectKey string) models.POSCheckoutRequest {
	return models.POSCheckoutRequest{
		Buyer: models.POSBuyer{Gender: models.POSGenderPreferNotToSay},
		Items: []models.POSCartItemRequest{
			{ProductID: 10, VariantID: posInt64Pointer(22), Quantity: 2},
			{ProductID: 11, Quantity: 1},
		},
		Payment: models.POSPaymentRequest{
			Method:        models.POSPaymentQRCode,
			SlipObjectKey: &slipObjectKey,
		},
	}
}

func assertAmount(t *testing.T, amount models.THBAmount, want string) {
	t.Helper()
	if amount.String() != want {
		t.Fatalf("amount = %s, want %s", amount, want)
	}
}

func TestPOSServiceCheckoutMeasuresTextInCharactersNotBytes(t *testing.T) {
	// VARCHAR limits count characters, so a Thai cohort at the limit must be
	// accepted even though it is three bytes per character.
	cohort := strings.Repeat("ก", models.POSStudentAlumniYearMaxLength)
	store := newPOSCheckoutStore(t)
	service := NewPOSService(store)
	request := posCashCheckoutRequest(t, "130.00", true)
	request.Buyer.StudentAlumniYear = &cohort

	if _, _, err := service.Checkout(context.Background(), 7, 42, "key-1", request); err != nil {
		t.Fatalf("cohort of %d characters (%d bytes) was rejected: %v",
			models.POSStudentAlumniYearMaxLength, len(cohort), err)
	}
	if store.command.Buyer.StudentAlumniYear == nil || *store.command.Buyer.StudentAlumniYear != cohort {
		t.Fatalf("cohort = %+v", store.command.Buyer.StudentAlumniYear)
	}

	tooLong := cohort + "ก"
	request.Buyer.StudentAlumniYear = &tooLong
	var validation POSValidationError
	if _, _, err := service.Checkout(context.Background(), 7, 42, "key-2", request); !errors.As(err, &validation) {
		t.Fatalf("error = %v, want a validation error", err)
	}
}
