//go:build integration

package repositories_test

import (
	"context"
	"io"
	"log/slog"
	"os"
	"sync"
	"testing"

	"github.com/esc-chula/intania-shop-api/internal/migrations"
	"github.com/esc-chula/intania-shop-api/internal/models"
	"github.com/esc-chula/intania-shop-api/internal/repositories"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestInventoryAdjustConcurrentDecrement(t *testing.T) {
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
	if _, err = pool.Exec(ctx, `TRUNCATE stock_transactions,variants,products,users RESTART IDENTITY CASCADE`); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO users(email,role) VALUES($1,$2)`, "inventory@example.test", "USER"); err != nil {
		t.Fatal(err)
	}
	var productID int64
	if err = pool.QueryRow(ctx, `INSERT INTO products(name,price,status,stock_quantity) VALUES($1,$2,$3,$4) RETURNING id`, "test", 10, "IN_STOCK", 7).Scan(&productID); err != nil {
		t.Fatal(err)
	}
	repository := repositories.NewInventoryRepository(pool)
	var wait sync.WaitGroup
	successes := make(chan struct{}, 10)
	failures := make(chan error, 10)
	for range 10 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, err := repository.Adjust(ctx, productID, models.AdjustStockRequest{QuantityChange: -1, Reason: "STORE_SALE"}, "test")
			if err != nil {
				failures <- err
				return
			}
			successes <- struct{}{}
		}()
	}
	wait.Wait()
	close(successes)
	close(failures)
	var successCount, failureCount int
	for range successes {
		successCount++
	}
	for range failures {
		failureCount++
	}
	if successCount != 7 || failureCount != 3 {
		t.Fatalf("successes=%d failures=%d, want 7 and 3", successCount, failureCount)
	}
	var stock int
	if err = pool.QueryRow(ctx, `SELECT stock_quantity FROM products WHERE id=$1`, productID).Scan(&stock); err != nil {
		t.Fatal(err)
	}
	if stock != 0 {
		t.Fatalf("stock=%d want 0", stock)
	}
	var transactions int
	if err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM stock_transactions WHERE product_id=$1`, productID).Scan(&transactions); err != nil {
		t.Fatal(err)
	}
	if transactions != 7 {
		t.Fatalf("transactions=%d want 7", transactions)
	}
}

func TestProductMutationRoundTrip(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err = pool.Exec(ctx, `TRUNCATE stock_transactions,variants,products RESTART IDENTITY CASCADE`); err != nil {
		t.Fatal(err)
	}
	name, price := "shirt", "199.50"
	status := models.ProductStatusInStock
	product, err := repositories.NewProductRepository(pool).Create(ctx, models.ProductInput{Name: &name, BasePrice: &price, Status: &status})
	if err != nil {
		t.Fatalf("create product: %v", err)
	}
	if product.ProductID == 0 || product.BasePrice != price || product.Name != name {
		t.Fatalf("unexpected created product: %#v", product)
	}
	updated := "shirt updated"
	product, err = repositories.NewProductRepository(pool).Update(ctx, product.ProductID, models.ProductInput{Name: &updated})
	if err != nil {
		t.Fatalf("update product: %v", err)
	}
	if product.Name != updated {
		t.Fatalf("name=%q want %q", product.Name, updated)
	}
	variant, err := repositories.NewProductRepository(pool).CreateVariant(ctx, product.ProductID, models.VariantInput{Price: &price})
	if err != nil {
		t.Fatalf("create variant: %v", err)
	}
	if variant.ProductID != product.ProductID {
		t.Fatalf("variant product ID=%d want %d", variant.ProductID, product.ProductID)
	}
	if err = repositories.NewProductRepository(pool).DeleteVariant(ctx, variant.VariantID); err != nil {
		t.Fatalf("delete variant: %v", err)
	}
}
