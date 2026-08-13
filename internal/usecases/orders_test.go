package usecases

import (
	"context"
	"errors"
	"testing"

	"github.com/esc-chula/intania-shop-api/internal/models"
)

type orderStoreStub struct {
	order       models.Order
	listByUser  int64
	listAllCall bool
}

func (s *orderStoreStub) Create(context.Context, int64, models.CreateOrderRequest) (models.Order, error) {
	return s.order, nil
}
func (s *orderStoreStub) FindByID(context.Context, int64) (models.Order, error) { return s.order, nil }
func (s *orderStoreStub) ListByUser(_ context.Context, userID int64) ([]models.Order, error) {
	s.listByUser = userID
	return []models.Order{s.order}, nil
}
func (s *orderStoreStub) ListAll(context.Context) ([]models.Order, error) {
	s.listAllCall = true
	return []models.Order{s.order}, nil
}
func (s *orderStoreStub) Update(context.Context, int64, models.UpdateOrderRequest) (models.Order, error) {
	return s.order, nil
}
func (s *orderStoreStub) Delete(context.Context, int64) error { return nil }

func TestOrderOwnershipAndListing(t *testing.T) {
	tests := []struct {
		name      string
		requester int64
		admin     bool
		wantErr   bool
		wantAll   bool
	}{
		{"owner reads", 7, false, false, false},
		{"non-owner is denied", 8, false, true, false},
		{"admin reads", 8, true, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &orderStoreStub{order: models.Order{OrderID: 1, UserID: 7}}
			_, err := NewOrderService(store).Get(context.Background(), 1, tt.requester, tt.admin)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tt.wantErr)
			}
		})
	}

	store := &orderStoreStub{order: models.Order{OrderID: 1, UserID: 7}}
	service := NewOrderService(store)
	if _, err := service.List(context.Background(), 7, false); err != nil || store.listByUser != 7 || store.listAllCall {
		t.Fatalf("user list err=%v user=%d all=%v", err, store.listByUser, store.listAllCall)
	}
	store.listByUser, store.listAllCall = 0, false
	if _, err := service.List(context.Background(), 7, true); err != nil || !store.listAllCall || store.listByUser != 0 {
		t.Fatalf("admin list err=%v user=%d all=%v", err, store.listByUser, store.listAllCall)
	}
}

func TestOrderGetPropagatesStoreError(t *testing.T) {
	store := &orderStoreStub{}
	service := NewOrderService(orderStoreError{store: store})
	if _, err := service.Get(context.Background(), 1, 1, false); !errors.Is(err, errOrderStore) {
		t.Fatalf("err=%v", err)
	}
}

var errOrderStore = errors.New("store failed")

type orderStoreError struct{ store *orderStoreStub }

func (s orderStoreError) Create(context.Context, int64, models.CreateOrderRequest) (models.Order, error) {
	return models.Order{}, errOrderStore
}
func (s orderStoreError) FindByID(context.Context, int64) (models.Order, error) {
	return models.Order{}, errOrderStore
}
func (s orderStoreError) ListByUser(context.Context, int64) ([]models.Order, error) {
	return nil, errOrderStore
}
func (s orderStoreError) ListAll(context.Context) ([]models.Order, error) { return nil, errOrderStore }
func (s orderStoreError) Update(context.Context, int64, models.UpdateOrderRequest) (models.Order, error) {
	return models.Order{}, errOrderStore
}
func (s orderStoreError) Delete(context.Context, int64) error { return errOrderStore }
