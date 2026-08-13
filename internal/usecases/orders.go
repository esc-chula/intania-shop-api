package usecases

import (
	"context"
	"fmt"
	"strings"

	"github.com/esc-chula/intania-shop-api/internal/models"
)

type OrderStore interface {
	Create(context.Context, int64, models.CreateOrderRequest) (models.Order, error)
	FindByID(context.Context, int64) (models.Order, error)
	ListByUser(context.Context, int64) ([]models.Order, error)
	ListAll(context.Context) ([]models.Order, error)
	Update(context.Context, int64, models.UpdateOrderRequest) (models.Order, error)
	Delete(context.Context, int64) error
}

type OrderService struct{ orders OrderStore }

func NewOrderService(orders OrderStore) *OrderService { return &OrderService{orders: orders} }

func (s *OrderService) Create(ctx context.Context, userID int64, request models.CreateOrderRequest) (models.Order, error) {
	if userID <= 0 || len(request.Items) == 0 {
		return models.Order{}, fmt.Errorf("order must contain at least one item")
	}
	for _, item := range request.Items {
		if item.VariantID <= 0 || item.Quantity <= 0 {
			return models.Order{}, fmt.Errorf("variant ID and quantity must be positive")
		}
	}
	if request.DeliveryType != nil && *request.DeliveryType == models.DeliveryShipping && (request.ShippingAddress == nil || strings.TrimSpace(*request.ShippingAddress) == "") {
		return models.Order{}, fmt.Errorf("shipping address is required for delivery")
	}
	return s.orders.Create(ctx, userID, request)
}

func (s *OrderService) Get(ctx context.Context, orderID, userID int64, admin bool) (models.Order, error) {
	if orderID <= 0 {
		return models.Order{}, fmt.Errorf("order ID must be positive")
	}
	order, e := s.orders.FindByID(ctx, orderID)
	if e != nil {
		return models.Order{}, e
	}
	if !admin && order.UserID != userID {
		return models.Order{}, fmt.Errorf("order does not belong to authenticated user")
	}
	return order, nil
}

func (s *OrderService) List(ctx context.Context, userID int64, admin bool) ([]models.Order, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("authenticated user ID must be positive")
	}
	if admin {
		return s.orders.ListAll(ctx)
	}
	return s.orders.ListByUser(ctx, userID)
}

func (s *OrderService) Update(ctx context.Context, orderID int64, request models.UpdateOrderRequest) (models.Order, error) {
	if orderID <= 0 {
		return models.Order{}, fmt.Errorf("order ID must be positive")
	}
	if request.OrderStatus != nil && !validOrderStatus(*request.OrderStatus) {
		return models.Order{}, fmt.Errorf("invalid order status")
	}
	if request.DeliveryType != nil && *request.DeliveryType != models.DeliveryPickup && *request.DeliveryType != models.DeliveryShipping {
		return models.Order{}, fmt.Errorf("invalid delivery type")
	}
	if request.DeliveryType != nil && *request.DeliveryType == models.DeliveryShipping && request.ShippingAddress != nil && strings.TrimSpace(*request.ShippingAddress) == "" {
		return models.Order{}, fmt.Errorf("shipping address is required for delivery")
	}
	return s.orders.Update(ctx, orderID, request)
}

func (s *OrderService) Delete(ctx context.Context, orderID int64) error {
	if orderID <= 0 {
		return fmt.Errorf("order ID must be positive")
	}
	return s.orders.Delete(ctx, orderID)
}

func validOrderStatus(status models.OrderStatus) bool {
	switch status {
	case models.OrderStatusCart, models.OrderStatusPendingPayment, models.OrderStatusConfirmed, models.OrderStatusShipping, models.OrderStatusCompleted:
		return true
	default:
		return false
	}
}
