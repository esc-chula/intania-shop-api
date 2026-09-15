package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/esc-chula/intania-shop-api/internal/models"
)

type orderStoreStub struct {
	filter    models.OrderFilter
	offset    int32
	limit     int32
	total     int64
	listErr   error
	exportErr error
	detailErr error
}

func (stub *orderStoreStub) List(_ context.Context, _ int64, filter models.OrderFilter, offset, limit int32) ([]models.POSOrder, int64, error) {
	stub.filter = filter
	stub.offset = offset
	stub.limit = limit
	return []models.POSOrder{}, stub.total, stub.listErr
}

func (stub *orderStoreStub) Export(_ context.Context, _ int64, filter models.OrderFilter) ([]models.POSOrder, error) {
	stub.filter = filter
	return []models.POSOrder{}, stub.exportErr
}

func (stub *orderStoreStub) Detail(context.Context, int64, int64) (models.POSOrder, error) {
	return models.POSOrder{}, stub.detailErr
}

type projectExistenceStoreStub struct {
	project models.Project
	err     error
}

func (stub *projectExistenceStoreStub) Detail(context.Context, models.Date, int64) (models.Project, error) {
	return stub.project, stub.err
}

func TestOrderServiceListDefaultsCreatedRangeToProjectStartInBangkok(t *testing.T) {
	startDate, err := models.ParseDate("2026-09-05")
	if err != nil {
		t.Fatal(err)
	}
	store := &orderStoreStub{}
	projects := &projectExistenceStoreStub{project: models.Project{StartDate: startDate}}
	service := NewOrderService(store, projects)
	fixedNow := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return fixedNow }

	if _, err := service.List(context.Background(), 1, OrderListQuery{}, 1, 10); err != nil {
		t.Fatal(err)
	}

	wantFrom := models.StartOfBangkokDay(startDate)
	if !store.filter.CreatedFrom.Equal(wantFrom) {
		t.Fatalf("created_from = %v, want %v", store.filter.CreatedFrom, wantFrom)
	}
	if !store.filter.CreatedTo.Equal(fixedNow) {
		t.Fatalf("created_to = %v, want %v", store.filter.CreatedTo, fixedNow)
	}
	if store.filter.SortBy != models.OrderSortByOrderNumber || store.filter.SortOrder != models.OrderSortDesc {
		t.Fatalf("default sort = %s/%s", store.filter.SortBy, store.filter.SortOrder)
	}
}

func TestOrderServiceListUsesExplicitCreatedRange(t *testing.T) {
	store := &orderStoreStub{}
	projects := &projectExistenceStoreStub{}
	service := NewOrderService(store, projects)

	query := OrderListQuery{
		CreatedFrom: "2026-09-05T00:00:00+07:00",
		CreatedTo:   "2026-09-06T00:00:00+07:00",
	}
	if _, err := service.List(context.Background(), 1, query, 1, 10); err != nil {
		t.Fatal(err)
	}

	wantFrom, _ := time.Parse(time.RFC3339, query.CreatedFrom)
	wantTo, _ := time.Parse(time.RFC3339, query.CreatedTo)
	if !store.filter.CreatedFrom.Equal(wantFrom) || !store.filter.CreatedTo.Equal(wantTo) {
		t.Fatalf("explicit range not applied: %+v", store.filter)
	}
}

func TestOrderServiceListRejectsCreatedToBeforeCreatedFrom(t *testing.T) {
	service := NewOrderService(&orderStoreStub{}, &projectExistenceStoreStub{})
	query := OrderListQuery{
		CreatedFrom: "2026-09-06T00:00:00+07:00",
		CreatedTo:   "2026-09-05T00:00:00+07:00",
	}
	_, err := service.List(context.Background(), 1, query, 1, 10)
	var validation ProjectValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("err = %v, want ProjectValidationError", err)
	}
}

func TestOrderServiceListValidatesFilters(t *testing.T) {
	tests := []struct {
		name  string
		query OrderListQuery
	}{
		{name: "order number too long", query: OrderListQuery{OrderNumber: string(make([]byte, models.OrderNumberFilterMaxLength+1))}},
		{name: "invalid payment method", query: OrderListQuery{PaymentMethod: "BITCOIN"}},
		{name: "invalid sort_by", query: OrderListQuery{SortBy: "created_at"}},
		{name: "invalid sort_order", query: OrderListQuery{SortOrder: "sideways"}},
		{name: "invalid created_from", query: OrderListQuery{CreatedFrom: "not-a-date"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := NewOrderService(&orderStoreStub{}, &projectExistenceStoreStub{})
			_, err := service.List(context.Background(), 1, test.query, 1, 10)
			var validation ProjectValidationError
			if !errors.As(err, &validation) {
				t.Fatalf("err = %v, want ProjectValidationError", err)
			}
		})
	}
}

func TestOrderServiceListRejectsInvalidProjectID(t *testing.T) {
	service := NewOrderService(&orderStoreStub{}, &projectExistenceStoreStub{})
	if _, err := service.List(context.Background(), 0, OrderListQuery{}, 1, 10); !errors.Is(err, ErrInvalidProjectID) {
		t.Fatalf("err = %v, want ErrInvalidProjectID", err)
	}
}

func TestOrderServiceDetailRejectsInvalidIDs(t *testing.T) {
	service := NewOrderService(&orderStoreStub{}, &projectExistenceStoreStub{})
	if _, err := service.Detail(context.Background(), 0, 1); !errors.Is(err, ErrInvalidProjectID) {
		t.Fatalf("err = %v, want ErrInvalidProjectID", err)
	}
	if _, err := service.Detail(context.Background(), 1, 0); !errors.Is(err, ErrInvalidOrderID) {
		t.Fatalf("err = %v, want ErrInvalidOrderID", err)
	}
}

func TestOrderServiceExportUsesSameFilterAsList(t *testing.T) {
	startDate, err := models.ParseDate("2026-09-05")
	if err != nil {
		t.Fatal(err)
	}
	store := &orderStoreStub{}
	projects := &projectExistenceStoreStub{project: models.Project{StartDate: startDate}}
	service := NewOrderService(store, projects)
	fixedNow := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return fixedNow }

	query := OrderListQuery{PaymentMethod: "QR_CODE", SortBy: "net_total", SortOrder: "asc"}
	if _, err := service.Export(context.Background(), 1, query); err != nil {
		t.Fatal(err)
	}
	if store.filter.PaymentMethod != models.POSPaymentQRCode {
		t.Fatalf("payment method filter = %s", store.filter.PaymentMethod)
	}
	if store.filter.SortBy != models.OrderSortByNetTotal || store.filter.SortOrder != models.OrderSortAsc {
		t.Fatalf("sort = %s/%s", store.filter.SortBy, store.filter.SortOrder)
	}
}
