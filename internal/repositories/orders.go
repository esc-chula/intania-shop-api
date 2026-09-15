package repositories

import (
	"context"
	"errors"
	"fmt"

	"github.com/esc-chula/intania-shop-api/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrOrderNotFound reports an order ID that does not exist in the project.
var ErrOrderNotFound = errors.New("order not found")

// OrderRepository reads paid POS orders written by BE-008 checkout.
type OrderRepository struct{ pool *pgxpool.Pool }

// NewOrderRepository constructs an order-history repository.
func NewOrderRepository(pool *pgxpool.Pool) *OrderRepository {
	return &OrderRepository{pool: pool}
}

const orderHistoryColumns = `o.order_id, o.order_number, o.project_id, o.status::text,
	o.staff_user_id, o.staff_full_name, o.staff_email,
	o.buyer_gender::text, o.buyer_age, o.buyer_student_alumni_year,
	o.subtotal::text, o.discount::text, o.net_total::text, o.created_at,
	pay.method::text, pay.slip_url, pay.received_amount::text,
	pay.change_amount::text, pay.retained_amount::text, pay.no_change, pay.note,
	promo.promotion_id, promo.name, promo.original_bundle_price::text,
	promo.promotion_price::text, promo.discount::text`

const orderHistoryFrom = ` FROM orders o
	JOIN order_payments pay ON pay.order_id = o.order_id
	LEFT JOIN order_promotions promo ON promo.order_id = o.order_id`

const orderHistoryFilter = ` WHERE o.project_id = $1
	AND ($2::text = '' OR o.order_number ILIKE '%' || $2 || '%' ESCAPE '\')
	AND ($3::text = '' OR pay.method::text = $3)
	AND ($4::bigint = 0 OR o.staff_user_id = $4)
	AND o.created_at >= $5 AND o.created_at <= $6`

// List returns one filtered, sorted page of paid project orders.
func (repository *OrderRepository) List(ctx context.Context, projectID int64, filter models.OrderFilter, offset, limit int32) ([]models.POSOrder, int64, error) {
	if err := repository.requireProject(ctx, projectID); err != nil {
		return nil, 0, err
	}

	query := fmt.Sprintf(`SELECT %s, COUNT(*) OVER () AS total%s%s
ORDER BY %s
OFFSET $7 LIMIT $8`, orderHistoryColumns, orderHistoryFrom, orderHistoryFilter, orderByClause(filter.SortBy, filter.SortOrder))
	rows, err := repository.pool.Query(ctx, query,
		projectID, escapeLikePattern(filter.OrderNumber), string(filter.PaymentMethod), filter.StaffID,
		filter.CreatedFrom, filter.CreatedTo, offset, limit)
	if err != nil {
		return nil, 0, fmt.Errorf("list orders: %w", err)
	}
	defer rows.Close()

	orders := make([]models.POSOrder, 0)
	orderIDs := make([]int64, 0)
	var total int64
	for rows.Next() {
		order, err := scanOrderHistory(rows, &total)
		if err != nil {
			return nil, 0, err
		}
		orders = append(orders, order)
		orderIDs = append(orderIDs, order.OrderID)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate orders: %w", err)
	}

	if len(orders) == 0 {
		total, err = repository.count(ctx, projectID, filter)
		if err != nil {
			return nil, 0, err
		}
		return orders, total, nil
	}
	if err := repository.attachItems(ctx, orders, orderIDs); err != nil {
		return nil, 0, err
	}
	return orders, total, nil
}

// Export returns all paid project orders matching the list filters and sort.
func (repository *OrderRepository) Export(ctx context.Context, projectID int64, filter models.OrderFilter) ([]models.POSOrder, error) {
	if err := repository.requireProject(ctx, projectID); err != nil {
		return nil, err
	}

	query := fmt.Sprintf(`SELECT %s%s%s
ORDER BY %s`, orderHistoryColumns, orderHistoryFrom, orderHistoryFilter, orderByClause(filter.SortBy, filter.SortOrder))
	rows, err := repository.pool.Query(ctx, query,
		projectID, escapeLikePattern(filter.OrderNumber), string(filter.PaymentMethod), filter.StaffID,
		filter.CreatedFrom, filter.CreatedTo)
	if err != nil {
		return nil, fmt.Errorf("export orders: %w", err)
	}
	defer rows.Close()

	orders := make([]models.POSOrder, 0)
	orderIDs := make([]int64, 0)
	for rows.Next() {
		order, err := scanOrderHistory(rows)
		if err != nil {
			return nil, err
		}
		orders = append(orders, order)
		orderIDs = append(orderIDs, order.OrderID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate orders: %w", err)
	}
	if len(orders) == 0 {
		return orders, nil
	}
	if err := repository.attachItems(ctx, orders, orderIDs); err != nil {
		return nil, err
	}
	return orders, nil
}

// Detail returns one paid order scoped to its project.
func (repository *OrderRepository) Detail(ctx context.Context, projectID, orderID int64) (models.POSOrder, error) {
	var storedOrderID int64
	err := repository.pool.QueryRow(ctx, `
		SELECT o.order_id
		FROM orders o
		JOIN order_payments pay ON pay.order_id = o.order_id
		WHERE o.project_id = $1 AND o.order_id = $2`, projectID, orderID).Scan(&storedOrderID)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.POSOrder{}, ErrOrderNotFound
	}
	if err != nil {
		return models.POSOrder{}, fmt.Errorf("find project order: %w", err)
	}

	order, err := readPOSOrder(ctx, repository.pool, storedOrderID)
	if err != nil {
		return models.POSOrder{}, fmt.Errorf("get order: %w", err)
	}
	return order, nil
}

func (repository *OrderRepository) count(ctx context.Context, projectID int64, filter models.OrderFilter) (int64, error) {
	var total int64
	query := `SELECT COUNT(*)` + orderHistoryFrom + orderHistoryFilter
	err := repository.pool.QueryRow(ctx, query,
		projectID, escapeLikePattern(filter.OrderNumber), string(filter.PaymentMethod), filter.StaffID,
		filter.CreatedFrom, filter.CreatedTo).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("count orders: %w", err)
	}
	return total, nil
}

func (repository *OrderRepository) requireProject(ctx context.Context, projectID int64) error {
	var exists int
	err := repository.pool.QueryRow(ctx, `SELECT 1 FROM projects WHERE project_id = $1`, projectID).Scan(&exists)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrProjectNotFound
	}
	if err != nil {
		return fmt.Errorf("check project: %w", err)
	}
	return nil
}

func (repository *OrderRepository) attachItems(ctx context.Context, orders []models.POSOrder, orderIDs []int64) error {
	itemsByOrder, err := repository.itemsFor(ctx, orderIDs)
	if err != nil {
		return err
	}
	for index := range orders {
		orders[index].Items = itemsByOrder[orders[index].OrderID]
	}
	return nil
}

func (repository *OrderRepository) itemsFor(ctx context.Context, orderIDs []int64) (map[int64][]models.POSOrderItemSnapshot, error) {
	rows, err := repository.pool.Query(ctx, `
		SELECT order_item_id, order_id, product_id, variant_id, product_name,
		       size, color, image_url, quantity, unit_price::text, line_total::text,
		       inventory_transaction_id
		FROM order_items
		WHERE order_id = ANY($1)
		ORDER BY order_id, order_item_id`, orderIDs)
	if err != nil {
		return nil, fmt.Errorf("list order items: %w", err)
	}
	defer rows.Close()

	itemsByOrder := make(map[int64][]models.POSOrderItemSnapshot)
	for rows.Next() {
		var (
			item      models.POSOrderItemSnapshot
			orderID   int64
			unitPrice string
			lineTotal string
		)
		if err := rows.Scan(&item.OrderItemID, &orderID, &item.ProductID, &item.VariantID,
			&item.ProductName, &item.Size, &item.Color, &item.ImageURL, &item.Quantity,
			&unitPrice, &lineTotal, &item.InventoryTransactionID); err != nil {
			return nil, fmt.Errorf("scan order item: %w", err)
		}
		var err error
		if item.UnitPrice, err = parsePOSOrderAmount(orderID, unitPrice); err != nil {
			return nil, err
		}
		if item.LineTotal, err = parsePOSOrderAmount(orderID, lineTotal); err != nil {
			return nil, err
		}
		itemsByOrder[orderID] = append(itemsByOrder[orderID], item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate order items: %w", err)
	}
	return itemsByOrder, nil
}

type orderHistoryScanner interface{ Scan(dest ...any) error }

func scanOrderHistory(row orderHistoryScanner, extra ...any) (models.POSOrder, error) {
	var (
		order          models.POSOrder
		gender         string
		subtotal       string
		discount       string
		netTotal       string
		method         string
		receivedAmount *string
		changeAmount   *string
		retainedAmount *string
		promotionID    *int64
		promotionName  *string
		bundlePrice    *string
		promotionPrice *string
		promotionSaved *string
	)

	destinations := append([]any{
		&order.OrderID, &order.OrderNumber, &order.ProjectID, &order.Status,
		&order.Staff.StaffID, &order.Staff.FullName, &order.Staff.Email,
		&gender, &order.Buyer.Age, &order.Buyer.StudentAlumniYear,
		&subtotal, &discount, &netTotal, &order.CreatedAt,
		&method, &order.Payment.SlipURL, &receivedAmount,
		&changeAmount, &retainedAmount, &order.Payment.NoChange, &order.Payment.Note,
		&promotionID, &promotionName, &bundlePrice, &promotionPrice, &promotionSaved,
	}, extra...)
	if err := row.Scan(destinations...); err != nil {
		return models.POSOrder{}, fmt.Errorf("scan order: %w", err)
	}

	order.Buyer.Gender = models.POSGender(gender)
	order.Payment.Method = models.POSPaymentMethod(method)
	var err error
	if order.Subtotal, err = parsePOSOrderAmount(order.OrderID, subtotal); err != nil {
		return models.POSOrder{}, err
	}
	if order.Discount, err = parsePOSOrderAmount(order.OrderID, discount); err != nil {
		return models.POSOrder{}, err
	}
	if order.NetTotal, err = parsePOSOrderAmount(order.OrderID, netTotal); err != nil {
		return models.POSOrder{}, err
	}
	if order.Payment.ReceivedAmount, err = parseOptionalPOSOrderAmount(order.OrderID, receivedAmount); err != nil {
		return models.POSOrder{}, err
	}
	if order.Payment.ChangeAmount, err = parseOptionalPOSOrderAmount(order.OrderID, changeAmount); err != nil {
		return models.POSOrder{}, err
	}
	if order.Payment.RetainedAmount, err = parseOptionalPOSOrderAmount(order.OrderID, retainedAmount); err != nil {
		return models.POSOrder{}, err
	}

	if promotionID != nil {
		if promotionName == nil || bundlePrice == nil || promotionPrice == nil || promotionSaved == nil {
			return models.POSOrder{}, fmt.Errorf("%w: order %d has an incomplete promotion snapshot", ErrPOSDataCorrupt, order.OrderID)
		}
		promotion := models.AppliedProjectPromotion{PromotionID: *promotionID, Name: *promotionName}
		if promotion.OriginalBundlePrice, err = parsePOSOrderAmount(order.OrderID, *bundlePrice); err != nil {
			return models.POSOrder{}, err
		}
		if promotion.PromotionPrice, err = parsePOSOrderAmount(order.OrderID, *promotionPrice); err != nil {
			return models.POSOrder{}, err
		}
		if promotion.Discount, err = parsePOSOrderAmount(order.OrderID, *promotionSaved); err != nil {
			return models.POSOrder{}, err
		}
		order.AppliedPromotion = &promotion
	}
	return order, nil
}

func orderByClause(sortBy models.OrderSortField, sortOrder models.OrderSortOrder) string {
	direction := "asc"
	if sortOrder == models.OrderSortDesc {
		direction = "desc"
	}
	column := "o.order_number"
	if sortBy == models.OrderSortByNetTotal {
		column = "o.net_total"
	}
	return fmt.Sprintf("%s %s, o.order_id %s", column, direction, direction)
}
