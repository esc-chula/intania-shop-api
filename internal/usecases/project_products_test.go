package usecases

import (
	"context"
	"errors"
	"testing"

	"github.com/esc-chula/intania-shop-api/internal/models"
)

type projectProductStoreStub struct {
	replaceItems []models.ProjectProductAssignmentInput
}

func (stub *projectProductStoreStub) List(context.Context, models.Date, models.ProjectFilter, int32, int32) ([]models.Project, int64, error) {
	return nil, 0, nil
}

func (stub *projectProductStoreStub) Detail(context.Context, models.Date, int64) (models.Project, error) {
	return models.Project{}, nil
}

func (stub *projectProductStoreStub) Create(context.Context, models.Date, models.ProjectInput) (models.Project, error) {
	return models.Project{}, nil
}

func (stub *projectProductStoreStub) Update(context.Context, models.Date, int64, models.ProjectInput) (models.Project, error) {
	return models.Project{}, nil
}

func (stub *projectProductStoreStub) Delete(context.Context, int64) error { return nil }

func (stub *projectProductStoreStub) ReplaceProducts(_ context.Context, _ models.Date, _ int64, items []models.ProjectProductAssignmentInput) ([]models.ProjectProductAssignment, error) {
	stub.replaceItems = items
	return []models.ProjectProductAssignment{}, nil
}

func TestReplaceProductsValidatesTHBAndIdentity(t *testing.T) {
	variantID := int64(2)
	tests := []struct {
		name  string
		items []models.ProjectProductAssignmentInput
		want  error
	}{
		{name: "valid variantless item", items: []models.ProjectProductAssignmentInput{{ProductID: 1, ProjectPrice: "0.00"}}},
		{name: "valid variant item", items: []models.ProjectProductAssignmentInput{{ProductID: 1, VariantID: &variantID, ProjectPrice: "99999999.99"}}},
		{name: "negative price", items: []models.ProjectProductAssignmentInput{{ProductID: 1, ProjectPrice: "-1.00"}}, want: ErrInvalidProjectProducts},
		{name: "wrong precision", items: []models.ProjectProductAssignmentInput{{ProductID: 1, ProjectPrice: "1.0"}}, want: ErrInvalidProjectProducts},
		{name: "numeric overflow", items: []models.ProjectProductAssignmentInput{{ProductID: 1, ProjectPrice: "100000000.00"}}, want: ErrInvalidProjectProducts},
		{name: "duplicate variantless identity", items: []models.ProjectProductAssignmentInput{{ProductID: 1, ProjectPrice: "1.00"}, {ProductID: 1, ProjectPrice: "2.00"}}, want: ErrInvalidProjectProducts},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stub := &projectProductStoreStub{}
			_, err := NewProjectService(stub).ReplaceProducts(context.Background(), 1, test.items)
			if !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
			if test.want == nil && len(stub.replaceItems) != len(test.items) {
				t.Fatal("valid selection did not reach the repository")
			}
		})
	}
}
