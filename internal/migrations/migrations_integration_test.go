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
		"orders", "products", "project_products", "projects", "promotion_items", "promotions",
		"stock_transactions", "users", "variants",
	})
	assertNames(t, database, `
		SELECT t.typname FROM pg_type t
		JOIN pg_namespace n ON n.oid=t.typnamespace
		WHERE n.nspname='public' AND t.typtype='e'
		ORDER BY t.typname`, []string{"product_status", "product_type", "stock_transaction_type", "user_role"})

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
