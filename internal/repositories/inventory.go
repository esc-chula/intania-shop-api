package repositories

import (
	"context"
	"errors"
	"fmt"

	"github.com/esc-chula/intania-shop-api/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type InventoryRepository struct{ pool *pgxpool.Pool }

func NewInventoryRepository(pool *pgxpool.Pool) *InventoryRepository {
	return &InventoryRepository{pool: pool}
}

func (r *InventoryRepository) Adjust(ctx context.Context, productID int64, request models.AdjustStockRequest, actor string) (models.StockTransaction, error) {
	tx, e := r.pool.Begin(ctx)
	if e != nil {
		return models.StockTransaction{}, fmt.Errorf("begin stock adjustment: %w", e)
	}
	defer rollback(ctx, tx)
	var before int32
	var target string
	var id = productID
	var pid = &productID
	var vid *int64
	if request.VariantID != nil {
		target = "variants"
		id = *request.VariantID
		vid = request.VariantID

	} else {
		target = "products"
	}
	q := `SELECT COALESCE(stock_quantity,0) FROM ` + target + ` WHERE `
	if target == "products" {
		q += `id=$1 AND deleted_at IS NULL FOR UPDATE`
	} else {
		// Lock the parent as well as the variant. Product archival takes a
		// FOR UPDATE lock on that parent, so this makes an adjustment and an
		// archive serialize instead of allowing stock history after archival.
		q = `SELECT COALESCE(v.stock_quantity,0) FROM variants v JOIN products p ON p.id=v.product_id WHERE v.variant_id=$1 AND v.product_id=$2 AND p.deleted_at IS NULL FOR UPDATE OF v, p`
	}
	if e = func() pgx.Row {
		if target == "variants" {
			return tx.QueryRow(ctx, q, id, productID)
		}
		return tx.QueryRow(ctx, q, id)
	}().Scan(&before); e != nil {
		if errors.Is(e, pgx.ErrNoRows) {
			return models.StockTransaction{}, r.productLifecycleError(ctx, productID)
		}
		return models.StockTransaction{}, fmt.Errorf("lock stock: %w", e)
	}
	after := before + request.QuantityChange
	if after < 0 {
		return models.StockTransaction{}, fmt.Errorf("insufficient stock")
	}
	if target == "products" {
		_, e = tx.Exec(ctx, `UPDATE products SET stock_quantity=$2,updated_at=CURRENT_TIMESTAMP WHERE id=$1 AND deleted_at IS NULL`, id, after)
	} else {
		_, e = tx.Exec(ctx, `UPDATE variants v SET stock_quantity=$2 FROM products p WHERE v.variant_id=$1 AND p.id=v.product_id AND p.deleted_at IS NULL`, id, after)
	}
	if e != nil {
		return models.StockTransaction{}, fmt.Errorf("update stock: %w", e)
	}
	typ := "INCREMENT"
	if request.QuantityChange < 0 {
		typ = "DECREMENT"
	}
	var out models.StockTransaction
	e = tx.QueryRow(ctx, `
		WITH inserted AS (
			INSERT INTO stock_transactions(product_id,variant_id,transaction_type,quantity_change,quantity_before,quantity_after,reason,notes,reference_type,reference_id,created_by,confirmation_image,operator_name)
			VALUES($1,$2,$3::stock_transaction_type,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
			RETURNING *
		)
		SELECT st.transaction_id,st.product_id,st.variant_id,st.transaction_type::text,
			st.quantity_change,st.quantity_before,st.quantity_after,st.reason,st.notes,
			st.reference_type,st.reference_id,
			COALESCE(st.operator_name, NULLIF(btrim(u.full_name), ''), u.email, st.created_by),
			st.confirmation_image,st.created_at
		FROM inserted st LEFT JOIN users u ON u.user_id::text=st.created_by`, pid, vid, typ, request.QuantityChange, before, after, request.Reason, request.Notes, request.ReferenceType, request.ReferenceID, actor, request.ConfirmationImage, request.OperatorName).Scan(&out.TransactionID, &out.ProductID, &out.VariantID, &out.TransactionType, &out.QuantityChange, &out.QuantityBefore, &out.QuantityAfter, &out.Reason, &out.Notes, &out.ReferenceType, &out.ReferenceID, &out.CreatedBy, &out.ConfirmationImage, &out.CreatedAt)
	if e != nil {
		return models.StockTransaction{}, fmt.Errorf("record stock transaction: %w", e)
	}
	if e = tx.Commit(ctx); e != nil {
		return models.StockTransaction{}, fmt.Errorf("commit stock adjustment: %w", e)
	}
	return out, nil
}

func (r *InventoryRepository) productLifecycleError(ctx context.Context, productID int64) error {
	var archived bool
	err := r.pool.QueryRow(ctx, `SELECT deleted_at IS NOT NULL FROM products WHERE id=$1`, productID).Scan(&archived)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrProductNotFound
	}
	if err != nil {
		return fmt.Errorf("read product lifecycle: %w", err)
	}
	if archived {
		return ErrProductArchived
	}
	return ErrProductNotFound
}

func (r *InventoryRepository) ListProductTransactions(ctx context.Context, productID int64, offset, limit int32) (models.StockTransactionListResponse, error) {
	return r.list(ctx, `WHERE st.product_id=$1`, []any{productID}, offset, limit)
}

func (r *InventoryRepository) ListVariantTransactions(ctx context.Context, variantID int64, offset, limit int32) (models.StockTransactionListResponse, error) {
	return r.list(ctx, `WHERE st.variant_id=$1`, []any{variantID}, offset, limit)
}

func (r *InventoryRepository) ListTransactions(ctx context.Context, offset, limit int32) (models.StockTransactionListResponse, error) {
	return r.list(ctx, ``, nil, offset, limit)
}

func (r *InventoryRepository) list(ctx context.Context, where string, args []any, offset, limit int32) (models.StockTransactionListResponse, error) {
	base := ` FROM stock_transactions st LEFT JOIN products p ON p.id=st.product_id LEFT JOIN variants v ON v.variant_id=st.variant_id LEFT JOIN products vp ON vp.id=v.product_id `
	var total int64
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*)`+base+where, args...).Scan(&total); err != nil {
		return models.StockTransactionListResponse{}, fmt.Errorf("count stock transactions: %w", err)
	}
	// ponytail: compare as text so legacy actor labels never need a numeric cast.
	base += ` LEFT JOIN users u ON u.user_id::text=st.created_by `
	// ponytail: reuse the unique order-item link for old and new POS transactions;
	// client-supplied reference IDs must not select another order's operator.
	base += ` LEFT JOIN order_items oi ON oi.inventory_transaction_id=st.transaction_id LEFT JOIN orders o ON o.order_id=oi.order_id `
	query := `SELECT st.transaction_id,st.product_id,st.variant_id,st.transaction_type::text,st.quantity_change,st.quantity_before,st.quantity_after,st.reason,st.notes,st.reference_type,st.reference_id,COALESCE(st.operator_name, o.operator_name, o.staff_full_name, NULLIF(btrim(u.full_name), ''), u.email, st.created_by),st.confirmation_image,st.created_at,COALESCE(p.name,vp.name)` + base + where + ` ORDER BY st.transaction_id DESC OFFSET $` + fmt.Sprint(len(args)+1) + ` LIMIT $` + fmt.Sprint(len(args)+2)
	queryArgs := append(append([]any{}, args...), offset, limit)
	rows, err := r.pool.Query(ctx, query, queryArgs...)
	if err != nil {
		return models.StockTransactionListResponse{}, fmt.Errorf("list stock transactions: %w", err)
	}
	defer rows.Close()
	out := models.StockTransactionListResponse{Transactions: make([]models.StockTransaction, 0), Total: total, Page: offset/limit + 1, PageSize: limit}
	if total > 0 {
		out.TotalPages = (int32(total) + limit - 1) / limit
	}
	for rows.Next() {
		var tx models.StockTransaction
		if err := rows.Scan(&tx.TransactionID, &tx.ProductID, &tx.VariantID, &tx.TransactionType, &tx.QuantityChange, &tx.QuantityBefore, &tx.QuantityAfter, &tx.Reason, &tx.Notes, &tx.ReferenceType, &tx.ReferenceID, &tx.CreatedBy, &tx.ConfirmationImage, &tx.CreatedAt, &tx.ProductName); err != nil {
			return models.StockTransactionListResponse{}, fmt.Errorf("scan stock transaction: %w", err)
		}
		out.Transactions = append(out.Transactions, tx)
	}
	if err := rows.Err(); err != nil {
		return models.StockTransactionListResponse{}, fmt.Errorf("iterate stock transactions: %w", err)
	}
	return out, nil
}
