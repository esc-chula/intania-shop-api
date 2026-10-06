package usecases

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/esc-chula/intania-shop-api/internal/models"
)

func TestValidatePromotionMutation(t *testing.T) {
	t.Parallel()

	price := testPromotionAmount(t, "10.00")
	variantID := int64(2)
	validItems := []models.ProjectPromotionItemInput{{ProductID: 1, Quantity: 1}}

	tests := []struct {
		name        string
		input       models.ProjectPromotionMutationRequest
		wantMessage string
	}{
		{
			name:        "empty name",
			input:       models.ProjectPromotionMutationRequest{Name: " \t", PromotionPrice: &price, Items: validItems},
			wantMessage: "Promotion name must not be empty",
		},
		{
			name:        "name too long",
			input:       models.ProjectPromotionMutationRequest{Name: strings.Repeat("x", models.PromotionNameMaxLength+1), PromotionPrice: &price, Items: validItems},
			wantMessage: "Promotion name must be at most 150 characters",
		},
		{
			name:        "missing price",
			input:       models.ProjectPromotionMutationRequest{Name: "Bundle", Items: validItems},
			wantMessage: "Promotion price is required",
		},
		{
			name:        "missing items",
			input:       models.ProjectPromotionMutationRequest{Name: "Bundle", PromotionPrice: &price},
			wantMessage: "Promotion must contain at least one item group",
		},
		{
			name: "non-positive product ID",
			input: models.ProjectPromotionMutationRequest{
				Name: "Bundle", PromotionPrice: &price,
				Items: []models.ProjectPromotionItemInput{{ProductID: 0, Quantity: 1}},
			},
			wantMessage: "Promotion option 1 product ID must be positive",
		},
		{
			name: "non-positive variant ID",
			input: models.ProjectPromotionMutationRequest{
				Name: "Bundle", PromotionPrice: &price,
				Items: []models.ProjectPromotionItemInput{{ProductID: 1, VariantID: testInt64Pointer(0), Quantity: 1}},
			},
			wantMessage: "Promotion option 1 variant ID must be positive",
		},
		{
			name: "non-positive quantity",
			input: models.ProjectPromotionMutationRequest{
				Name: "Bundle", PromotionPrice: &price,
				Items: []models.ProjectPromotionItemInput{{ProductID: 1, Quantity: 0}},
			},
			wantMessage: "Promotion option 1 quantity must be positive",
		},
		{
			name: "duplicate variantless item",
			input: models.ProjectPromotionMutationRequest{
				Name: "Bundle", PromotionPrice: &price,
				Items: []models.ProjectPromotionItemInput{{ProductID: 1, Quantity: 1}, {ProductID: 1, Quantity: 2}},
			},
			wantMessage: "Promotion options must not contain duplicate product/variant reference at option 2",
		},
		{
			name: "duplicate variant item",
			input: models.ProjectPromotionMutationRequest{
				Name: "Bundle", PromotionPrice: &price,
				Items: []models.ProjectPromotionItemInput{
					{ProductID: 1, VariantID: &variantID, Quantity: 1},
					{ProductID: 1, VariantID: &variantID, Quantity: 2},
				},
			},
			wantMessage: "Promotion options must not contain duplicate product/variant reference at option 2",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := validatePromotionMutation(test.input)
			if err == nil || err.Error() != test.wantMessage {
				t.Fatalf("error = %v, want %q", err, test.wantMessage)
			}

			var validation PromotionValidationError
			if !errors.As(err, &validation) {
				t.Fatalf("error %T is not a PromotionValidationError", err)
			}
		})
	}
}

func TestValidatePromotionMutationTrimsNameAndAllowsDistinctVariantReferences(t *testing.T) {
	t.Parallel()

	price := testPromotionAmount(t, "10.00")
	variantID := int64(2)
	mutation, err := validatePromotionMutation(models.ProjectPromotionMutationRequest{
		Name:           "  Bundle  ",
		PromotionPrice: &price,
		Items: []models.ProjectPromotionItemInput{
			{ProductID: 1, Quantity: 1},
			{ProductID: 1, VariantID: &variantID, Quantity: 2},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if mutation.Name != "Bundle" {
		t.Fatalf("name = %q, want Bundle", mutation.Name)
	}
	if mutation.PromotionPrice.String() != "10.00" || len(mutation.Items) != 2 {
		t.Fatalf("validated mutation = %+v", mutation)
	}
}

func TestValidatePromotionMutationAcceptsORGroupsAndRejectsCrossGroupDuplicate(t *testing.T) {
	price := testPromotionAmount(t, "10.00")
	variantID := int64(2)
	mutation, err := validatePromotionMutation(models.ProjectPromotionMutationRequest{
		Name:           " Shirt or pin ",
		PromotionPrice: &price,
		ItemGroups: []models.ProjectPromotionItemGroupInput{
			{Options: []models.ProjectPromotionItemInput{{ProductID: 1, Quantity: 1}, {ProductID: 1, VariantID: &variantID, Quantity: 1}}},
			{Options: []models.ProjectPromotionItemInput{{ProductID: 2, Quantity: 1}}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if mutation.Name != "Shirt or pin" || len(mutation.ItemGroups) != 2 || len(mutation.ItemGroups[0].Options) != 2 {
		t.Fatalf("validated mutation = %+v", mutation)
	}

	_, err = validatePromotionMutation(models.ProjectPromotionMutationRequest{
		Name:           "Duplicate",
		PromotionPrice: &price,
		ItemGroups: []models.ProjectPromotionItemGroupInput{
			{Options: []models.ProjectPromotionItemInput{{ProductID: 1, Quantity: 1}}},
			{Options: []models.ProjectPromotionItemInput{{ProductID: 1, Quantity: 1}}},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "duplicate product/variant") {
		t.Fatalf("duplicate cross-group error = %v", err)
	}
}

func TestPromotionServiceValidatesIDsAndDelegatesReads(t *testing.T) {
	t.Parallel()

	reader := &promotionReaderStub{
		promotions: []models.ProjectPromotion{{PromotionID: 7, ProjectID: 3}},
		promotion:  models.ProjectPromotion{PromotionID: 7, ProjectID: 3},
	}
	service := NewPromotionService(reader)

	if _, err := service.List(context.Background(), 0); !errors.Is(err, ErrInvalidProjectID) {
		t.Fatalf("List invalid project error = %v", err)
	}
	if _, err := service.Detail(context.Background(), 3, 0); !errors.Is(err, ErrInvalidPromotionID) {
		t.Fatalf("Detail invalid promotion error = %v", err)
	}

	list, err := service.List(context.Background(), 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Promotions) != 1 || reader.listProjectID != 3 {
		t.Fatalf("list = %+v, project ID = %d", list, reader.listProjectID)
	}

	detail, err := service.Detail(context.Background(), 3, 7)
	if err != nil {
		t.Fatal(err)
	}
	if detail.PromotionID != 7 || reader.detailProjectID != 3 || reader.detailPromotionID != 7 {
		t.Fatalf("detail = %+v, reader IDs = %d/%d", detail, reader.detailProjectID, reader.detailPromotionID)
	}
}

func TestPromotionAdminServiceDelegatesValidatedMutations(t *testing.T) {
	t.Parallel()

	creator := &promotionCreatorStub{}
	service := NewPromotionAdminService(creator)
	service.now = func() time.Time { return time.Date(2026, time.September, 6, 8, 0, 0, 0, time.UTC) }
	price := testPromotionAmount(t, "10.00")
	input := models.ProjectPromotionMutationRequest{
		Name:           "  Bundle  ",
		PromotionPrice: &price,
		Items:          []models.ProjectPromotionItemInput{{ProductID: 1, Quantity: 1}},
	}

	created, err := service.Create(context.Background(), 3, input)
	if err != nil {
		t.Fatal(err)
	}
	if created.PromotionID != 11 || creator.createProjectID != 3 || creator.createMutation.Name != "Bundle" {
		t.Fatalf("created = %+v, creator = %+v", created, creator)
	}

	updated, err := service.Update(context.Background(), 3, 11, input)
	if err != nil {
		t.Fatal(err)
	}
	if updated.PromotionID != 12 || creator.updateProjectID != 3 || creator.updatePromotionID != 11 {
		t.Fatalf("updated = %+v, creator = %+v", updated, creator)
	}

	if err := service.Delete(context.Background(), 3, 11); err != nil {
		t.Fatal(err)
	}
	if creator.deleteProjectID != 3 || creator.deletePromotionID != 11 {
		t.Fatalf("delete IDs = %d/%d", creator.deleteProjectID, creator.deletePromotionID)
	}
}

func TestPromotionAdminServicePreservesCreatorErrors(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("storage failure")
	creator := &promotionCreatorStub{createErr: wantErr, updateErr: wantErr, deleteErr: wantErr}
	service := NewPromotionAdminService(creator)
	price := testPromotionAmount(t, "10.00")
	input := models.ProjectPromotionMutationRequest{
		Name:           "Bundle",
		PromotionPrice: &price,
		Items:          []models.ProjectPromotionItemInput{{ProductID: 1, Quantity: 1}},
	}

	if _, err := service.Create(context.Background(), 3, input); !errors.Is(err, wantErr) {
		t.Fatalf("Create error = %v", err)
	}
	if _, err := service.Update(context.Background(), 3, 11, input); !errors.Is(err, wantErr) {
		t.Fatalf("Update error = %v", err)
	}
	if err := service.Delete(context.Background(), 3, 11); !errors.Is(err, wantErr) {
		t.Fatalf("Delete error = %v", err)
	}
}

func TestPromotionAdminServiceRejectsInvalidIDsAndPayloadBeforePersistence(t *testing.T) {
	t.Parallel()

	creator := &promotionCreatorStub{}
	service := NewPromotionAdminService(creator)
	price := testPromotionAmount(t, "10.00")
	validInput := models.ProjectPromotionMutationRequest{
		Name:           "Bundle",
		PromotionPrice: &price,
		Items:          []models.ProjectPromotionItemInput{{ProductID: 1, Quantity: 1}},
	}

	if _, err := service.Create(context.Background(), 0, validInput); !errors.Is(err, ErrInvalidProjectID) {
		t.Fatalf("Create invalid project error = %v", err)
	}
	if _, err := service.Update(context.Background(), 0, 1, validInput); !errors.Is(err, ErrInvalidProjectID) {
		t.Fatalf("Update invalid project error = %v", err)
	}
	if _, err := service.Update(context.Background(), 1, 0, validInput); !errors.Is(err, ErrInvalidPromotionID) {
		t.Fatalf("Update invalid promotion error = %v", err)
	}
	if err := service.Delete(context.Background(), 0, 1); !errors.Is(err, ErrInvalidProjectID) {
		t.Fatalf("Delete invalid project error = %v", err)
	}
	if err := service.Delete(context.Background(), 1, 0); !errors.Is(err, ErrInvalidPromotionID) {
		t.Fatalf("Delete invalid promotion error = %v", err)
	}

	if _, err := service.Create(context.Background(), 1, models.ProjectPromotionMutationRequest{Name: "Bundle", Items: validInput.Items}); err == nil {
		t.Fatal("Create missing price succeeded")
	}
	if creator.createProjectID != 0 || creator.updateProjectID != 0 || creator.deleteProjectID != 0 {
		t.Fatalf("invalid requests reached persistence: %+v", creator)
	}
}

type promotionReaderStub struct {
	promotions        []models.ProjectPromotion
	promotion         models.ProjectPromotion
	listErr           error
	detailErr         error
	listProjectID     int64
	detailProjectID   int64
	detailPromotionID int64
}

func (stub *promotionReaderStub) List(_ context.Context, projectID int64) ([]models.ProjectPromotion, error) {
	stub.listProjectID = projectID
	return stub.promotions, stub.listErr
}

func (stub *promotionReaderStub) Detail(_ context.Context, projectID, promotionID int64) (models.ProjectPromotion, error) {
	stub.detailProjectID = projectID
	stub.detailPromotionID = promotionID
	return stub.promotion, stub.detailErr
}

type promotionCreatorStub struct {
	createErr         error
	updateErr         error
	deleteErr         error
	createProjectID   int64
	updateProjectID   int64
	updatePromotionID int64
	deleteProjectID   int64
	deletePromotionID int64
	createMutation    models.ProjectPromotionMutation
}

func (stub *promotionCreatorStub) Create(_ context.Context, _ models.Date, projectID int64, mutation models.ProjectPromotionMutation) (models.ProjectPromotion, error) {
	stub.createProjectID = projectID
	stub.createMutation = mutation
	return models.ProjectPromotion{PromotionID: 11}, stub.createErr
}

func (stub *promotionCreatorStub) Update(_ context.Context, _ models.Date, projectID, promotionID int64, _ models.ProjectPromotionMutation) (models.ProjectPromotion, error) {
	stub.updateProjectID = projectID
	stub.updatePromotionID = promotionID
	return models.ProjectPromotion{PromotionID: 12}, stub.updateErr
}

func (stub *promotionCreatorStub) Delete(_ context.Context, _ models.Date, projectID, promotionID int64) error {
	stub.deleteProjectID = projectID
	stub.deletePromotionID = promotionID
	return stub.deleteErr
}

func testPromotionAmount(t *testing.T, value string) models.THBAmount {
	t.Helper()
	amount, err := models.ParseTHBAmount(value)
	if err != nil {
		t.Fatal(err)
	}
	return amount
}

func testInt64Pointer(value int64) *int64 { return &value }
