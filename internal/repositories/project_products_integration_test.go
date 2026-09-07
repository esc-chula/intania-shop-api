//go:build integration

package repositories_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/esc-chula/intania-shop-api/internal/migrations"
	"github.com/esc-chula/intania-shop-api/internal/models"
	"github.com/esc-chula/intania-shop-api/internal/repositories"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestProjectProductReplacement(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	if err := migrations.Apply(ctx, dsn, slog.New(slog.NewTextHandler(io.Discard, nil))); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err = pool.Exec(ctx, `TRUNCATE project_products, variants, products, orders, projects RESTART IDENTITY CASCADE`); err != nil {
		t.Fatal(err)
	}
	var projectID, completedID, singleID, variantProductID, variantID int64
	for _, row := range []struct {
		name, start, end string
		id               *int64
	}{
		{"active", "2020-01-01", "2099-01-01", &projectID}, {"completed", "1999-01-01", "2000-01-01", &completedID},
	} {
		if err := pool.QueryRow(ctx, `INSERT INTO projects(name,start_date,end_date) VALUES($1,$2,$3) RETURNING project_id`, row.name, row.start, row.end).Scan(row.id); err != nil {
			t.Fatal(err)
		}
	}
	if err := pool.QueryRow(ctx, `INSERT INTO products(name,price,status) VALUES('single',10,'IN_STOCK') RETURNING id`).Scan(&singleID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO products(name,price,status,product_type) VALUES('multi',10,'IN_STOCK','MULTIPLE') RETURNING id`).Scan(&variantProductID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO variants(product_id,size,price) VALUES($1,'M',12) RETURNING variant_id`, variantProductID).Scan(&variantID); err != nil {
		t.Fatal(err)
	}
	repo := repositories.NewProjectRepository(pool)
	today := models.NewDate(time.Date(2026, 9, 6, 0, 0, 0, 0, time.UTC))
	items := []models.ProjectProductAssignmentInput{{ProductID: singleID, ProjectPrice: "9.00"}, {ProductID: variantProductID, VariantID: &variantID, ProjectPrice: "11.00"}}
	if got, err := repo.ReplaceProducts(ctx, today, projectID, items); err != nil || len(got) != 2 {
		t.Fatalf("replace got=%v err=%v", got, err)
	}
	if _, err := repo.ReplaceProducts(ctx, today, projectID, []models.ProjectProductAssignmentInput{{ProductID: singleID, ProjectPrice: "8.00"}}); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM project_products WHERE project_id=$1`, projectID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("count=%d err=%v", count, err)
	}
	if _, err := repo.ReplaceProducts(ctx, today, projectID, []models.ProjectProductAssignmentInput{{ProductID: singleID, ProjectPrice: "8.00"}, {ProductID: variantProductID, ProjectPrice: "1.00"}}); !errors.Is(err, repositories.ErrProjectProductInvalid) {
		t.Fatalf("invalid err=%v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM project_products WHERE project_id=$1`, projectID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("rollback count=%d err=%v", count, err)
	}
	if _, err := repo.ReplaceProducts(ctx, today, completedID, items[:1]); !errors.Is(err, repositories.ErrProjectCompleted) {
		t.Fatalf("completed err=%v", err)
	}
}
