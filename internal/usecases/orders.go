package usecases

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/esc-chula/intania-shop-api/internal/models"
)

// OrderStore is the persistence port required by OrderService.
type OrderStore interface {
	List(context.Context, int64, models.OrderFilter, int32, int32) ([]models.Order, int64, error)
	Export(context.Context, int64, models.OrderFilter) ([]models.Order, error)
	Detail(context.Context, int64, int64) (models.Order, error)
}

// ProjectExistenceStore is the minimal project read needed to derive an
// order history's default date range from the project's own start date.
type ProjectExistenceStore interface {
	Detail(context.Context, models.Date, int64) (models.Project, error)
}

// OrderService validates and applies order history filters, sorting, and
// pagination on top of the immutable checkout snapshot the repository reads.
type OrderService struct {
	orders   OrderStore
	projects ProjectExistenceStore
	now      func() time.Time
}

// NewOrderService constructs an order service over its stores.
func NewOrderService(orders OrderStore, projects ProjectExistenceStore) *OrderService {
	return &OrderService{orders: orders, projects: projects, now: time.Now}
}

// OrderListQuery is the validated set of client-supplied list/export filters.
type OrderListQuery struct {
	OrderNumber   string
	PaymentMethod string
	StaffID       int64
	CreatedFrom   string
	CreatedTo     string
	SortBy        string
	SortOrder     string
}

// List returns a filtered, sorted, paginated page of project orders. Filters
// are applied before pagination, and every sort carries a fixed order_id
// tie-break, so a stable ordering never shifts an order across pages.
func (service *OrderService) List(ctx context.Context, projectID int64, query OrderListQuery, page, pageSize int32) (models.OrderListResponse, error) {
	if projectID <= 0 {
		return models.OrderListResponse{}, ErrInvalidProjectID
	}

	filter, err := service.buildFilter(ctx, projectID, query)
	if err != nil {
		return models.OrderListResponse{}, err
	}

	page, pageSize = normalizePage(page, pageSize)
	orders, total, err := service.orders.List(ctx, projectID, filter, (page-1)*pageSize, pageSize)
	if err != nil {
		return models.OrderListResponse{}, fmt.Errorf("list orders: %w", err)
	}
	return models.OrderListResponse{
		Orders: orders, Total: total, Page: page, PageSize: pageSize, TotalPages: totalPages(total, pageSize),
	}, nil
}

// Export returns every order matching the filter, in the requested sort
// order, unpaginated, for the Excel export. It uses the same filter and sort
// rules as List.
func (service *OrderService) Export(ctx context.Context, projectID int64, query OrderListQuery) ([]models.Order, error) {
	if projectID <= 0 {
		return nil, ErrInvalidProjectID
	}

	filter, err := service.buildFilter(ctx, projectID, query)
	if err != nil {
		return nil, err
	}

	orders, err := service.orders.Export(ctx, projectID, filter)
	if err != nil {
		return nil, fmt.Errorf("export orders: %w", err)
	}
	return orders, nil
}

// Detail returns one immutable order, scoped to its project.
func (service *OrderService) Detail(ctx context.Context, projectID, orderID int64) (models.Order, error) {
	if projectID <= 0 {
		return models.Order{}, ErrInvalidProjectID
	}
	if orderID <= 0 {
		return models.Order{}, ErrInvalidOrderID
	}
	order, err := service.orders.Detail(ctx, projectID, orderID)
	if err != nil {
		return models.Order{}, fmt.Errorf("get order: %w", err)
	}
	return order, nil
}

// ErrInvalidOrderID reports an order ID outside the valid range.
var ErrInvalidOrderID = errors.New("order ID must be positive")

// buildFilter validates the query and resolves the default created_from/
// created_to range: 00:00 Asia/Bangkok on the project's start date through
// the current time.
func (service *OrderService) buildFilter(ctx context.Context, projectID int64, query OrderListQuery) (models.OrderFilter, error) {
	filter := models.OrderFilter{
		SortBy:    models.OrderSortByOrderNumber,
		SortOrder: models.OrderSortDesc,
	}

	if trimmed := strings.TrimSpace(query.OrderNumber); trimmed != "" {
		if utf8.RuneCountInString(trimmed) > models.OrderNumberFilterMaxLength {
			return models.OrderFilter{}, ProjectValidationError{
				Message: fmt.Sprintf("Order number filter must be at most %d characters", models.OrderNumberFilterMaxLength),
			}
		}
		filter.OrderNumber = trimmed
	}

	if trimmed := strings.TrimSpace(query.PaymentMethod); trimmed != "" {
		method, err := models.ParseOrderPaymentMethod(trimmed)
		if err != nil {
			return models.OrderFilter{}, ProjectValidationError{Message: "Invalid payment method filter"}
		}
		filter.PaymentMethod = method
	}

	if query.StaffID < 0 {
		return models.OrderFilter{}, ProjectValidationError{Message: "Staff ID filter must be positive"}
	}
	filter.StaffID = query.StaffID

	if trimmed := strings.TrimSpace(query.SortBy); trimmed != "" {
		sortBy, err := models.ParseOrderSortField(trimmed)
		if err != nil {
			return models.OrderFilter{}, ProjectValidationError{Message: "Invalid sort_by"}
		}
		filter.SortBy = sortBy
	}

	if trimmed := strings.TrimSpace(query.SortOrder); trimmed != "" {
		sortOrder, err := models.ParseOrderSortOrder(trimmed)
		if err != nil {
			return models.OrderFilter{}, ProjectValidationError{Message: "Invalid sort_order"}
		}
		filter.SortOrder = sortOrder
	}

	now := service.now()
	createdFrom, createdTo, err := service.resolveDateRange(ctx, projectID, query.CreatedFrom, query.CreatedTo, now)
	if err != nil {
		return models.OrderFilter{}, err
	}
	filter.CreatedFrom = createdFrom
	filter.CreatedTo = createdTo

	return filter, nil
}

// resolveDateRange defaults created_from to 00:00 Asia/Bangkok on the
// project's start date and created_to to the current time, and otherwise
// parses the explicit RFC3339 values the client supplied.
func (service *OrderService) resolveDateRange(ctx context.Context, projectID int64, rawFrom, rawTo string, now time.Time) (time.Time, time.Time, error) {
	createdFrom := strings.TrimSpace(rawFrom)
	createdTo := strings.TrimSpace(rawTo)

	var from, to time.Time
	var err error

	if createdFrom == "" {
		project, err := service.projects.Detail(ctx, models.TodayInBangkok(now), projectID)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("get project for default order range: %w", err)
		}
		from = models.StartOfBangkokDay(project.StartDate)
	} else if from, err = time.Parse(time.RFC3339, createdFrom); err != nil {
		return time.Time{}, time.Time{}, ProjectValidationError{Message: "Invalid created_from"}
	}

	if createdTo == "" {
		to = now
	} else if to, err = time.Parse(time.RFC3339, createdTo); err != nil {
		return time.Time{}, time.Time{}, ProjectValidationError{Message: "Invalid created_to"}
	}

	if to.Before(from) {
		return time.Time{}, time.Time{}, ProjectValidationError{Message: "created_to must not be earlier than created_from"}
	}

	return from, to, nil
}
