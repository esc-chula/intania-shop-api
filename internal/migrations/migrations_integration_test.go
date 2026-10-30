//go:build integration

package migrations

import (
	"context"
	"database/sql"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"reflect"
	"strings"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

func TestCleanBaselineUpAndDown(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	if err := Apply(ctx, dsn, logger); err != nil {
		t.Fatalf("apply clean baseline: %v", err)
	}

	database, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	assertNames(t, database, `
		SELECT tablename FROM pg_tables
		WHERE schemaname='public' AND tablename <> 'goose_db_version'
		ORDER BY tablename`, []string{
		"oauth_login_codes", "order_items", "order_payments", "order_promotions", "orders", "payment_slips",
		"products", "project_products", "projects", "promotion_items", "promotions",
		"stock_transactions", "users", "variants",
	})
	assertNames(t, database, `
		SELECT t.typname FROM pg_type t
		JOIN pg_namespace n ON n.oid=t.typnamespace
		WHERE n.nspname='public' AND t.typtype='e'
		ORDER BY t.typname`, []string{
		"pos_buyer_gender", "pos_order_status", "pos_payment_method",
		"product_status", "product_type", "stock_transaction_type", "user_role",
	})

	migrationFS, err := fs.Sub(files, "sql")
	if err != nil {
		t.Fatal(err)
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, database, migrationFS, goose.WithSlog(logger))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = provider.DownTo(ctx, 0); err != nil {
		t.Fatalf("roll back clean baseline: %v", err)
	}
	assertNames(t, database, `
		SELECT tablename FROM pg_tables
		WHERE schemaname='public' AND tablename <> 'goose_db_version'
		ORDER BY tablename`, nil)
	assertNames(t, database, `
		SELECT t.typname FROM pg_type t
		JOIN pg_namespace n ON n.oid=t.typnamespace
		WHERE n.nspname='public' AND t.typtype='e'
		ORDER BY t.typname`, nil)
}

func TestProjectStatusOverrideRollbackPreservesManualChoices(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	if err := Apply(ctx, dsn, logger); err != nil {
		t.Fatal(err)
	}
	database, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	var projectID int64
	if err := database.QueryRowContext(ctx, `
		INSERT INTO projects(name, start_date, end_date, status_override)
		VALUES('Migration override test', '2099-01-01', '2099-01-02', 'ACTIVE')
		RETURNING project_id`).Scan(&projectID); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := database.Exec(`DELETE FROM projects WHERE project_id = $1`, projectID); err != nil {
			t.Error(err)
		}
	}()
	migrationFS, err := fs.Sub(files, "sql")
	if err != nil {
		t.Fatal(err)
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, database, migrationFS, goose.WithSlog(logger))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.DownTo(ctx, 9); err == nil || !strings.Contains(err.Error(), "while overrides exist") {
		t.Fatalf("rollback with override = %v, want safeguard error", err)
	}
	var status string
	if err := database.QueryRowContext(ctx, `SELECT status_override FROM projects WHERE project_id = $1`, projectID).Scan(&status); err != nil || status != "ACTIVE" {
		t.Fatalf("override after rejected rollback = %q, error = %v", status, err)
	}
	if _, err := database.ExecContext(ctx, `UPDATE projects SET status_override = NULL WHERE project_id = $1`, projectID); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.DownTo(ctx, 9); err != nil {
		t.Fatalf("rollback after clearing override: %v", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("reapply override migration: %v", err)
	}
	var automatic bool
	if err := database.QueryRowContext(ctx, `SELECT status_override IS NULL FROM projects WHERE project_id = $1`, projectID).Scan(&automatic); err != nil || !automatic {
		t.Fatalf("existing project should remain automatic: %t, error = %v", automatic, err)
	}
}

func TestOrderOperatorNameMigrationPreservesExistingSnapshots(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	if err := Apply(ctx, dsn, logger); err != nil {
		t.Fatal(err)
	}
	database, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	migrationFS, err := fs.Sub(files, "sql")
	if err != nil {
		t.Fatal(err)
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, database, migrationFS, goose.WithSlog(logger))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.DownTo(ctx, 10); err != nil {
		t.Fatal(err)
	}
	assertNames(t, database, `SELECT column_name FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = 'orders' AND column_name = 'operator_name'`, nil)
	// Insert a pre-migration row without using the new column.
	var orderID int64
	if err := database.QueryRowContext(ctx, `INSERT INTO orders
		(order_number, staff_full_name, subtotal, discount, net_total)
		VALUES ('operator-migration-test', 'Original account', 0, 0, 0) RETURNING order_id`).Scan(&orderID); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := database.ExecContext(ctx, `DELETE FROM orders WHERE order_id = $1`, orderID); err != nil {
			t.Error(err)
		}
	}()
	if _, err := provider.Up(ctx); err != nil {
		t.Fatal(err)
	}
	var raw sql.NullString
	var resolved string
	if err := database.QueryRowContext(ctx, `SELECT operator_name, COALESCE(operator_name, staff_full_name)
		FROM orders WHERE order_id = $1`, orderID).Scan(&raw, &resolved); err != nil || raw.Valid || resolved != "Original account" {
		t.Fatalf("legacy operator = %+v, resolved = %q, error = %v", raw, resolved, err)
	}
	if _, err := database.ExecContext(ctx, `UPDATE orders SET operator_name = 'มายด์' WHERE order_id = $1`, orderID); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.DownTo(ctx, 10); err != nil {
		t.Fatal(err)
	}
	assertNames(t, database, `SELECT column_name FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = 'orders' AND column_name = 'operator_name'`, nil)
	if _, err := provider.Up(ctx); err != nil {
		t.Fatal(err)
	}
	if err := database.QueryRowContext(ctx, `SELECT operator_name, staff_full_name
		FROM orders WHERE order_id = $1`, orderID).Scan(&raw, &resolved); err != nil || raw.Valid || resolved != "Original account" {
		t.Fatalf("snapshot after rollback/reapply = %+v, %q, error = %v", raw, resolved, err)
	}
}

func TestPromotionApplicationCountMigrationPreservesHistoricalSnapshots(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	if err := Apply(ctx, dsn, logger); err != nil {
		t.Fatal(err)
	}
	database, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	migrationFS, err := fs.Sub(files, "sql")
	if err != nil {
		t.Fatal(err)
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, database, migrationFS, goose.WithSlog(logger))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.DownTo(ctx, 12); err != nil {
		t.Fatal(err)
	}
	var orderID int64
	if err := database.QueryRowContext(ctx, `INSERT INTO orders
		(order_number, staff_full_name, subtotal, discount, net_total)
		VALUES ('promotion-count-migration-test', 'Original staff', 1600, 800, 800)
		RETURNING order_id`).Scan(&orderID); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := database.ExecContext(ctx, `DELETE FROM orders WHERE order_id = $1`, orderID); err != nil {
			t.Error(err)
		}
	}()
	if _, err := database.ExecContext(ctx, `INSERT INTO order_promotions
		(order_id, promotion_id, name, original_bundle_price, promotion_price, discount)
		VALUES ($1, 1, 'Historical bundle', 1600, 800, 800)`, orderID); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatal(err)
	}
	var count sql.NullInt64
	if err := database.QueryRowContext(ctx, `SELECT application_count FROM order_promotions WHERE order_id = $1`, orderID).Scan(&count); err != nil || count.Valid {
		t.Fatalf("historical count = %+v, error = %v; want NULL", count, err)
	}
	for _, invalid := range []int64{0, -1} {
		if _, err := database.ExecContext(ctx, `UPDATE order_promotions SET application_count = $1 WHERE order_id = $2`, invalid, orderID); err == nil {
			t.Fatalf("application_count %d should violate check constraint", invalid)
		}
	}
	if _, err := database.ExecContext(ctx, `UPDATE order_promotions SET application_count = 2 WHERE order_id = $1`, orderID); err != nil {
		t.Fatal(err)
	}
	if err := database.QueryRowContext(ctx, `SELECT application_count FROM order_promotions WHERE order_id = $1`, orderID).Scan(&count); err != nil || !count.Valid || count.Int64 != 2 {
		t.Fatalf("stored count = %+v, error = %v; want 2", count, err)
	}
	if _, err := provider.DownTo(ctx, 12); err != nil {
		t.Fatal(err)
	}
	assertNames(t, database, `SELECT column_name FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = 'order_promotions' AND column_name = 'application_count'`, nil)
	if _, err := provider.Up(ctx); err != nil {
		t.Fatal(err)
	}
	var discount string
	if err := database.QueryRowContext(ctx, `SELECT application_count, discount::text FROM order_promotions WHERE order_id = $1`, orderID).Scan(&count, &discount); err != nil || count.Valid || discount != "800.00" {
		t.Fatalf("snapshot after rollback/reapply: count = %+v, discount = %q, error = %v", count, discount, err)
	}
}

func assertNames(t *testing.T, database *sql.DB, query string, want []string) {
	t.Helper()
	rows, err := database.Query(query)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		got = append(got, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("names=%v want=%v", got, want)
	}
}
