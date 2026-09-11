package usecases

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/esc-chula/intania-shop-api/internal/models"
)

func TestPOSServiceCatalogGroupsAndSortsCategories(t *testing.T) {
	accessories := "Accessories"
	apparel := "Apparel"
	zebra := "zebra"
	project := models.Project{ProjectID: 7, Status: models.ProjectStatusCompleted}
	reader := &posSnapshotReaderStub{
		catalog: models.POSCatalogSnapshot{
			Project: project,
			Items: []models.POSResolvedItem{
				{ProductID: 1, ProductName: "Zipper", Category: &zebra, ProjectPrice: posTestAmount(t, "30.00")},
				{ProductID: 2, ProductName: "Cap", Category: &accessories, ProjectPrice: posTestAmount(t, "50.00")},
				{ProductID: 3, ProductName: "Shirt", Category: &apparel, ProjectPrice: posTestAmount(t, "100.00")},
				{ProductID: 4, ProductName: "Donation", Category: nil, ProjectPrice: posTestAmount(t, "10.00")},
			},
		},
	}
	service := NewPOSService(reader)

	catalog, err := service.Catalog(context.Background(), project.ProjectID)
	if err != nil {
		t.Fatal(err)
	}

	if catalog.Project.ProjectID != project.ProjectID || catalog.CanCheckout {
		t.Fatalf("catalog project/checkout = %+v/%v", catalog.Project, catalog.CanCheckout)
	}
	if len(catalog.Categories) != 4 {
		t.Fatalf("category count = %d, want 4", len(catalog.Categories))
	}
	for index, expected := range []*string{&accessories, &apparel, &zebra, nil} {
		category := catalog.Categories[index]
		if (category.Name == nil) != (expected == nil) {
			t.Fatalf("category %d name = %v, want %v", index, category.Name, expected)
		}
		if expected != nil && *category.Name != *expected {
			t.Fatalf("category %d name = %q, want %q", index, *category.Name, *expected)
		}
	}
	if catalog.Categories[0].Products[0].ProjectPrice != "50.00" || catalog.Categories[3].Products[0].ProjectPrice != "10.00" {
		t.Fatalf("catalogue products = %+v", catalog.Categories)
	}

	reader.catalog.Project.Status = models.ProjectStatusActive
	catalog, err = service.Catalog(context.Background(), project.ProjectID)
	if err != nil {
		t.Fatal(err)
	}
	if !catalog.CanCheckout {
		t.Fatal("ACTIVE catalogue cannot check out")
	}
}

func TestPOSServiceCatalogTreatsBlankCategoriesAsUncategorized(t *testing.T) {
	blank := "   "
	reader := &posSnapshotReaderStub{
		catalog: models.POSCatalogSnapshot{
			Project: models.Project{ProjectID: 7, Status: models.ProjectStatusActive},
			Items: []models.POSResolvedItem{
				{ProductID: 1, ProductName: "Blank", Category: &blank, ProjectPrice: posTestAmount(t, "10.00")},
			},
		},
	}

	catalog, err := NewPOSService(reader).Catalog(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}

	if len(catalog.Categories) != 1 || catalog.Categories[0].Name != nil {
		t.Fatalf("categories = %+v, want one uncategorized group", catalog.Categories)
	}
	if len(catalog.Categories[0].Products) != 1 || catalog.Categories[0].Products[0].Category != nil {
		t.Fatalf("uncategorized product = %+v", catalog.Categories[0].Products)
	}
}

func TestPOSServiceCatalogValidatesProjectIDAndPropagatesReaderError(t *testing.T) {
	readerError := errors.New("catalogue unavailable")
	reader := &posSnapshotReaderStub{catalogErr: readerError}
	service := NewPOSService(reader)

	if _, err := service.Catalog(context.Background(), 0); !errors.Is(err, ErrInvalidProjectID) {
		t.Fatalf("invalid project ID error = %v, want ErrInvalidProjectID", err)
	}
	if reader.catalogCalls != 0 {
		t.Fatal("invalid project ID reached the reader")
	}

	if _, err := service.Catalog(context.Background(), 7); !errors.Is(err, readerError) {
		t.Fatalf("reader error = %v, want %v", err, readerError)
	}
}

func TestPOSServiceQuoteUsesSnapshotPricesAndPricingResult(t *testing.T) {
	variantID := int64(22)
	project := models.Project{ProjectID: 7, Status: models.ProjectStatusActive}
	reader := &posSnapshotReaderStub{
		quote: models.POSQuoteSnapshot{
			Project: project,
			Items: []models.POSResolvedCartItem{
				{
					Item: models.POSResolvedItem{
						ProductID: 2, VariantID: &variantID, ProductName: "Shirt",
						Size: posStringPointer("M"), Color: posStringPointer("Navy"),
						ProjectPrice: posTestAmount(t, "100.00"), StockQuantity: 5,
					},
					Quantity: 2,
				},
				{
					Item: models.POSResolvedItem{
						ProductID: 1, ProductName: "Pin", ProjectPrice: posTestAmount(t, "50.00"), StockQuantity: 2,
					},
					Quantity: 1,
				},
			},
			Promotions: []models.PricingPromotion{{
				PromotionID:    4,
				Name:           "Bundle",
				PromotionPrice: posTestAmount(t, "130.00"),
				Items: []models.PricingPromotionItem{
					{ProductID: 2, VariantID: &variantID, Quantity: 1},
					{ProductID: 1, Quantity: 1},
				},
			}},
		},
	}
	service := NewPOSService(reader)
	quotedAt := time.Date(2026, time.September, 10, 10, 30, 0, 0, time.FixedZone("Asia/Bangkok", 7*60*60))
	service.now = func() time.Time { return quotedAt }

	request := models.POSCartRequest{Items: []models.POSCartItemRequest{
		{ProductID: 2, VariantID: &variantID, Quantity: 2},
		{ProductID: 1, Quantity: 1},
	}}
	quote, err := service.Quote(context.Background(), project.ProjectID, request)
	if err != nil {
		t.Fatal(err)
	}

	if quote.ProjectID != project.ProjectID || !quote.QuotedAt.Equal(quotedAt) {
		t.Fatalf("quote identity/timestamp = %d/%s", quote.ProjectID, quote.QuotedAt)
	}
	if len(quote.Items) != 2 {
		t.Fatalf("quote item count = %d, want 2", len(quote.Items))
	}
	if quote.Items[0].ProductID != 2 || quote.Items[0].VariantID == nil || *quote.Items[0].VariantID != variantID || quote.Items[0].Quantity != 2 {
		t.Fatalf("first quote item = %+v", quote.Items[0])
	}
	if quote.Items[0].UnitPrice.String() != "100.00" || quote.Items[0].LineTotal.String() != "200.00" || quote.Items[0].AvailableQuantity != 5 {
		t.Fatalf("first quote amounts = %+v", quote.Items[0])
	}
	if quote.Items[1].UnitPrice.String() != "50.00" || quote.Items[1].LineTotal.String() != "50.00" {
		t.Fatalf("second quote amounts = %+v", quote.Items[1])
	}
	assertPricingAmount(t, quote.Subtotal, "250.00")
	assertPricingAmount(t, quote.Discount, "20.00")
	assertPricingAmount(t, quote.NetTotal, "230.00")
	if quote.AppliedPromotion == nil || quote.AppliedPromotion.PromotionID != 4 {
		t.Fatalf("applied promotion = %+v", quote.AppliedPromotion)
	}
	if reader.quoteProjectID != project.ProjectID || reader.quoteToday.String() != models.TodayInBangkok(quotedAt).String() {
		t.Fatalf("reader call = project %d date %s", reader.quoteProjectID, reader.quoteToday)
	}
	if !reflect.DeepEqual(reader.quoteCart, request.Items) {
		t.Fatalf("reader cart = %+v, want %+v", reader.quoteCart, request.Items)
	}
}

func TestPOSServiceQuoteValidatesCartBeforePersistence(t *testing.T) {
	variantID := int64(22)
	validProject := models.Project{ProjectID: 7, Status: models.ProjectStatusActive}
	tests := []struct {
		name    string
		request models.POSCartRequest
		message string
	}{
		{name: "empty", request: models.POSCartRequest{}, message: "Cart must contain at least one item"},
		{name: "invalid product", request: models.POSCartRequest{Items: []models.POSCartItemRequest{{ProductID: 0, Quantity: 1}}}, message: "Cart item 1 product ID must be positive"},
		{name: "invalid variant", request: models.POSCartRequest{Items: []models.POSCartItemRequest{{ProductID: 1, VariantID: posInt64Pointer(0), Quantity: 1}}}, message: "Cart item 1 variant ID must be positive"},
		{name: "invalid quantity", request: models.POSCartRequest{Items: []models.POSCartItemRequest{{ProductID: 1, Quantity: 0}}}, message: "Cart item 1 quantity must be positive"},
		{name: "duplicate", request: models.POSCartRequest{Items: []models.POSCartItemRequest{{ProductID: 1, Quantity: 1}, {ProductID: 1, Quantity: 2}}}, message: "Cart items must not contain duplicate product/variant reference at item 2"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reader := &posSnapshotReaderStub{quote: models.POSQuoteSnapshot{Project: validProject}}
			_, err := NewPOSService(reader).Quote(context.Background(), validProject.ProjectID, test.request)
			var validation POSValidationError
			if !errors.As(err, &validation) || validation.Message != test.message || !errors.Is(err, ErrInvalidPOSCart) {
				t.Fatalf("error = %v, want POSValidationError %q", err, test.message)
			}
			if reader.quoteCalls != 0 {
				t.Fatal("invalid Cart reached the reader")
			}
		})
	}

	reader := &posSnapshotReaderStub{quote: models.POSQuoteSnapshot{
		Project: validProject,
		Items: []models.POSResolvedCartItem{
			{Item: models.POSResolvedItem{ProductID: 1, ProjectPrice: posTestAmount(t, "10.00"), StockQuantity: 1}, Quantity: 1},
			{Item: models.POSResolvedItem{ProductID: 1, VariantID: &variantID, ProjectPrice: posTestAmount(t, "10.00"), StockQuantity: 1}, Quantity: 1},
		},
	}}
	if _, err := NewPOSService(reader).Quote(context.Background(), validProject.ProjectID, models.POSCartRequest{Items: []models.POSCartItemRequest{
		{ProductID: 1, Quantity: 1},
		{ProductID: 1, VariantID: &variantID, Quantity: 1},
	}}); err != nil {
		t.Fatalf("variantless and variant identities rejected: %v", err)
	}
}

func TestPOSServiceQuoteRejectsCartBeyondDatabaseParameterBound(t *testing.T) {
	items := make([]models.POSCartItemRequest, posCartMaxItems+1)
	for index := range items {
		items[index] = models.POSCartItemRequest{
			ProductID: int64(index + 1),
			Quantity:  1,
		}
	}

	reader := &posSnapshotReaderStub{}
	_, err := NewPOSService(reader).Quote(context.Background(), 7, models.POSCartRequest{Items: items})
	var validation POSValidationError
	if !errors.As(err, &validation) || !errors.Is(err, ErrInvalidPOSCart) {
		t.Fatalf("error = %v, want POSValidationError", err)
	}
	if validation.Message != "Cart cannot contain more than 16383 items" {
		t.Fatalf("validation message = %q", validation.Message)
	}
	if reader.quoteCalls != 0 {
		t.Fatal("oversized Cart reached the reader")
	}
}

func TestPOSServiceQuoteRejectsInactiveProjectBeforeStock(t *testing.T) {
	for _, status := range []models.ProjectStatus{models.ProjectStatusNotStarted, models.ProjectStatusCompleted} {
		reader := &posSnapshotReaderStub{quote: models.POSQuoteSnapshot{
			Project: models.Project{ProjectID: 7, Status: status},
			Items: []models.POSResolvedCartItem{{
				Item:     models.POSResolvedItem{ProductID: 1, ProjectPrice: posTestAmount(t, "10.00"), StockQuantity: 0},
				Quantity: 1,
			}},
		}}

		_, err := NewPOSService(reader).Quote(context.Background(), 7, models.POSCartRequest{Items: []models.POSCartItemRequest{{ProductID: 1, Quantity: 1}}})
		if !errors.Is(err, ErrProjectNotActive) {
			t.Fatalf("status %s error = %v, want ErrProjectNotActive", status, err)
		}
	}
}

func TestPOSServiceQuoteAggregatesStockConflicts(t *testing.T) {
	variantID := int64(22)
	reader := &posSnapshotReaderStub{quote: models.POSQuoteSnapshot{
		Project: models.Project{ProjectID: 7, Status: models.ProjectStatusActive},
		Items: []models.POSResolvedCartItem{
			{Item: models.POSResolvedItem{ProductID: 1, ProjectPrice: posTestAmount(t, "10.00"), StockQuantity: 2}, Quantity: 3},
			{Item: models.POSResolvedItem{ProductID: 2, VariantID: &variantID, ProjectPrice: posTestAmount(t, "20.00"), StockQuantity: 1}, Quantity: 4},
		},
	}}

	_, err := NewPOSService(reader).Quote(context.Background(), 7, models.POSCartRequest{Items: []models.POSCartItemRequest{
		{ProductID: 1, Quantity: 3},
		{ProductID: 2, VariantID: &variantID, Quantity: 4},
	}})
	if !errors.Is(err, ErrInsufficientStock) {
		t.Fatalf("stock error = %v, want ErrInsufficientStock", err)
	}
	var stockError POSInsufficientStockError
	if !errors.As(err, &stockError) {
		t.Fatalf("stock error type = %T, want POSInsufficientStockError", err)
	}
	want := []models.POSStockConflict{
		{ProductID: 1, RequestedQuantity: 3, AvailableQuantity: 2},
		{ProductID: 2, VariantID: &variantID, RequestedQuantity: 4, AvailableQuantity: 1},
	}
	if !reflect.DeepEqual(stockError.Items, want) {
		t.Fatalf("stock conflicts = %+v, want %+v", stockError.Items, want)
	}
}

func TestPOSServiceQuotePropagatesReaderAndPricingErrors(t *testing.T) {
	readerError := errors.New("quote storage unavailable")
	reader := &posSnapshotReaderStub{quoteErr: readerError}
	if _, err := NewPOSService(reader).Quote(context.Background(), 7, models.POSCartRequest{Items: []models.POSCartItemRequest{{ProductID: 1, Quantity: 1}}}); !errors.Is(err, readerError) {
		t.Fatalf("reader error = %v, want %v", err, readerError)
	}

	reader = &posSnapshotReaderStub{quote: models.POSQuoteSnapshot{
		Project: models.Project{ProjectID: 7, Status: models.ProjectStatusActive},
		Items: []models.POSResolvedCartItem{{
			Item:     models.POSResolvedItem{ProductID: 1, ProjectPrice: posTestAmount(t, "10.00"), StockQuantity: 1},
			Quantity: 1,
		}},
		Promotions: []models.PricingPromotion{{
			PromotionID:    1,
			PromotionPrice: posTestAmount(t, "11.00"),
			Items:          []models.PricingPromotionItem{{ProductID: 1, Quantity: 1}},
		}},
	}}
	_, err := NewPOSService(reader).Quote(context.Background(), 7, models.POSCartRequest{Items: []models.POSCartItemRequest{{ProductID: 1, Quantity: 1}}})
	if !errors.Is(err, ErrPricingInvalidPromotion) || !errors.Is(err, models.ErrTHBAmountUnderflow) {
		t.Fatalf("pricing error = %v, want pricing invalid and THB underflow", err)
	}
}

type posSnapshotReaderStub struct {
	catalog          models.POSCatalogSnapshot
	quote            models.POSQuoteSnapshot
	catalogErr       error
	quoteErr         error
	catalogCalls     int
	quoteCalls       int
	catalogProjectID int64
	quoteProjectID   int64
	catalogToday     models.Date
	quoteToday       models.Date
	quoteCart        []models.POSCartItemRequest
}

func (stub *posSnapshotReaderStub) Catalog(_ context.Context, today models.Date, projectID int64) (models.POSCatalogSnapshot, error) {
	stub.catalogCalls++
	stub.catalogToday = today
	stub.catalogProjectID = projectID
	return stub.catalog, stub.catalogErr
}

func (stub *posSnapshotReaderStub) QuoteSnapshot(_ context.Context, today models.Date, projectID int64, cart []models.POSCartItemRequest) (models.POSQuoteSnapshot, error) {
	stub.quoteCalls++
	stub.quoteToday = today
	stub.quoteProjectID = projectID
	stub.quoteCart = append([]models.POSCartItemRequest(nil), cart...)
	return stub.quote, stub.quoteErr
}

func posTestAmount(t *testing.T, value string) models.THBAmount {
	t.Helper()
	amount, err := models.ParseTHBAmount(value)
	if err != nil {
		t.Fatalf("parse amount %q: %v", value, err)
	}
	return amount
}

func posStringPointer(value string) *string { return &value }

func posInt64Pointer(value int64) *int64 { return &value }
