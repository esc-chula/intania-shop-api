package repositories

import (
	"context"
	"errors"
	"fmt"

	"github.com/esc-chula/intania-shop-api/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type OrderRepository struct{ pool *pgxpool.Pool }

var ErrOrderNotFound = errors.New("order not found")

func NewOrderRepository(pool *pgxpool.Pool) *OrderRepository { return &OrderRepository{pool: pool} }

func (repository *OrderRepository) Create(ctx context.Context, userID int64, request models.CreateOrderRequest) (models.Order, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return models.Order{}, fmt.Errorf("begin order: %w", err)
	}
	defer rollback(ctx, tx)
	var orderID int64
	var fee = "0"
	if request.DeliveryFee != nil {
		fee = *request.DeliveryFee
	}
	var deliveryType *string
	if request.DeliveryType != nil {
		value := string(*request.DeliveryType)
		deliveryType = &value
	}
	err = tx.QueryRow(ctx, `INSERT INTO orders(user_id,order_code,total_amount,delivery_fee,order_status,delivery_type,shipping_address) VALUES($1,'PENDING',0,$2::numeric,'PENDING_PAYMENT',$3::delivery_type,$4) RETURNING order_id`, userID, fee, deliveryType, request.ShippingAddress).Scan(&orderID)
	if err != nil {
		return models.Order{}, fmt.Errorf("create order: %w", err)
	}
	for _, input := range request.Items {
		var productName string
		var available, minOrder, maxOrder *int32
		var unitPrice string
		var imageURL *string
		var size, color *string
		err = tx.QueryRow(ctx, `SELECT p.name,p.price::text,(p.images)[1],v.size,v.color,v.stock_quantity,p.min_order,p.max_order FROM variants v JOIN products p ON p.id=v.product_id WHERE v.variant_id=$1 FOR KEY SHARE`, input.VariantID).Scan(&productName, &unitPrice, &imageURL, &size, &color, &available, &minOrder, &maxOrder)
		if err != nil {
			return models.Order{}, fmt.Errorf("load variant %d: %w", input.VariantID, err)
		}
		if minOrder != nil && input.Quantity < *minOrder {
			return models.Order{}, fmt.Errorf("quantity must be at least %d for this product", *minOrder)
		}
		if maxOrder != nil && input.Quantity > *maxOrder {
			return models.Order{}, fmt.Errorf("maximum quantity is %d for this product", *maxOrder)
		}
		if available != nil && input.Quantity > *available {
			return models.Order{}, fmt.Errorf("insufficient stock for variant")
		}
		if _, err = tx.Exec(ctx, `INSERT INTO order_items(order_id,variant_id,quantity,unit_price,product_name,variant_size,variant_color,image_url) VALUES($1,$2,$3,$4::numeric,$5,$6,$7,$8)`, orderID, input.VariantID, input.Quantity, unitPrice, productName, size, color, imageURL); err != nil {
			return models.Order{}, fmt.Errorf("create order item: %w", err)
		}
	}
	if _, err = tx.Exec(ctx, `UPDATE orders SET order_code='ORD-' || LPAD(order_id::text,3,'0'), total_amount=(SELECT COALESCE(SUM(quantity*unit_price),0) FROM order_items WHERE order_id=$1)+delivery_fee WHERE order_id=$1`, orderID); err != nil {
		return models.Order{}, fmt.Errorf("finalize order: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return models.Order{}, fmt.Errorf("commit order: %w", err)
	}
	return repository.FindByID(ctx, orderID)
}

// FindByID loads an order header. Detailed item loading is added with the order read handler.
func (repository *OrderRepository) FindByID(ctx context.Context, orderID int64) (models.Order, error) {
	var order models.Order
	var deliveryType *string
	err := repository.pool.QueryRow(ctx, `SELECT order_id,order_code,user_id,total_amount::text,delivery_fee::text,order_status::text,delivery_type::text,shipping_address,tracking_number,created_at FROM orders WHERE order_id=$1`, orderID).Scan(&order.OrderID, &order.OrderCode, &order.UserID, &order.TotalAmount, &order.DeliveryFee, &order.OrderStatus, &deliveryType, &order.ShippingAddress, &order.TrackingNumber, &order.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.Order{}, ErrOrderNotFound
		}
		return models.Order{}, fmt.Errorf("find order: %w", err)
	}
	if deliveryType != nil {
		value := models.DeliveryType(*deliveryType)
		order.DeliveryType = &value
	}
	rows, err := repository.pool.Query(ctx, `SELECT order_item_id,order_id,variant_id,quantity,unit_price::text,product_name,variant_size,variant_color,image_url FROM order_items WHERE order_id=$1 ORDER BY order_item_id`, orderID)
	if err != nil {
		return models.Order{}, fmt.Errorf("list order items: %w", err)
	}
	defer rows.Close()
	order.Items = make([]models.OrderItem, 0)
	for rows.Next() {
		var item models.OrderItem
		if err := rows.Scan(&item.OrderItemID, &item.OrderID, &item.VariantID, &item.Quantity, &item.UnitPrice, &item.ProductName, &item.VariantSize, &item.VariantColor, &item.ImageURL); err != nil {
			return models.Order{}, fmt.Errorf("scan order item: %w", err)
		}
		order.Items = append(order.Items, item)
	}
	if err := rows.Err(); err != nil {
		return models.Order{}, fmt.Errorf("iterate order items: %w", err)
	}
	return order, nil
}

func (repository *OrderRepository) ListByUser(ctx context.Context, userID int64) ([]models.Order, error) {
	return repository.list(ctx, `SELECT order_id FROM orders WHERE user_id=$1 ORDER BY order_id DESC`, userID)
}

func (repository *OrderRepository) ListAll(ctx context.Context) ([]models.Order, error) {
	return repository.list(ctx, `SELECT order_id FROM orders ORDER BY order_id DESC`)
}

func (repository *OrderRepository) list(ctx context.Context, query string, args ...any) ([]models.Order, error) {
	rows, err := repository.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list orders: %w", err)
	}
	defer rows.Close()
	orders := make([]models.Order, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan order ID: %w", err)
		}
		order, err := repository.FindByID(ctx, id)
		if err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate orders: %w", err)
	}
	return orders, nil
}

func (repository *OrderRepository) Update(ctx context.Context, orderID int64, request models.UpdateOrderRequest) (models.Order, error) {
	var deliveryType *string
	if request.DeliveryType != nil {
		value := string(*request.DeliveryType)
		deliveryType = &value
	}
	var id int64
	err := repository.pool.QueryRow(ctx, `UPDATE orders SET order_status=COALESCE($2::order_status,order_status),delivery_type=COALESCE($3::delivery_type,delivery_type),shipping_address=COALESCE($4,shipping_address),tracking_number=COALESCE($5,tracking_number) WHERE order_id=$1 RETURNING order_id`, orderID, request.OrderStatus, deliveryType, request.ShippingAddress, request.TrackingNumber).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.Order{}, ErrOrderNotFound
	}
	if err != nil {
		return models.Order{}, fmt.Errorf("update order: %w", err)
	}
	return repository.FindByID(ctx, id)
}

func (repository *OrderRepository) Delete(ctx context.Context, orderID int64) error {
	tag, err := repository.pool.Exec(ctx, `DELETE FROM orders WHERE order_id=$1`, orderID)
	if err != nil {
		return fmt.Errorf("delete order: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrOrderNotFound
	}
	return nil
}
