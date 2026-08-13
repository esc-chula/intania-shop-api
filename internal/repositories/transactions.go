package repositories

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// rollback closes a transaction on every exit path; a committed transaction returns ErrTxClosed, which is expected.
func rollback(ctx context.Context, tx pgx.Tx) {
	_ = tx.Rollback(ctx)
}
