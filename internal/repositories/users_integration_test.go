//go:build integration

package repositories_test

import (
	"context"
	"os"
	"testing"

	"github.com/esc-chula/intania-shop-api/internal/auth"
	"github.com/esc-chula/intania-shop-api/internal/models"
	"github.com/esc-chula/intania-shop-api/internal/repositories"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestOAuthOnlyUserUpsert(t *testing.T) {
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
	if _, err = pool.Exec(ctx, `TRUNCATE users RESTART IDENTITY CASCADE`); err != nil {
		t.Fatal(err)
	}

	repository := repositories.NewUserRepository(pool)
	picture := "https://example.test/new.png"
	created, err := repository.UpsertGoogleUser(ctx, auth.GoogleUser{ID: "google-new", Email: "new@example.test", Name: "New User", Picture: &picture})
	if err != nil {
		t.Fatalf("create Google user: %v", err)
	}
	if created.ID == 0 || created.Role != models.RoleUser || created.Email != "new@example.test" {
		t.Fatalf("unexpected created user: %#v", created)
	}

	existing, err := repository.UpsertGoogleUser(ctx, auth.GoogleUser{ID: "google-new", Email: "changed@example.test", Name: "Changed"})
	if err != nil {
		t.Fatalf("find existing Google ID: %v", err)
	}
	if existing.ID != created.ID || existing.Email != created.Email {
		t.Fatalf("existing Google identity changed: %#v", existing)
	}

	var matchingID int64
	if err = pool.QueryRow(ctx, `INSERT INTO users(full_name,email,role) VALUES($1,$2,'ADMIN') RETURNING user_id`, "Existing Admin", "admin@example.test").Scan(&matchingID); err != nil {
		t.Fatal(err)
	}
	linked, err := repository.UpsertGoogleUser(ctx, auth.GoogleUser{ID: "google-admin", Email: "admin@example.test", Name: "Google Name"})
	if err != nil {
		t.Fatalf("link matching email: %v", err)
	}
	if linked.ID != matchingID || linked.Role != models.RoleAdmin {
		t.Fatalf("matching email was not linked: %#v", linked)
	}
	var googleID string
	if err = pool.QueryRow(ctx, `SELECT google_id FROM users WHERE user_id=$1`, matchingID).Scan(&googleID); err != nil {
		t.Fatal(err)
	}
	if googleID != "google-admin" {
		t.Fatalf("google_id=%q want google-admin", googleID)
	}
}
