// Package migrations applies embedded PostgreSQL schema migrations.
package migrations

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

//go:embed sql/*.sql
var files embed.FS

// Apply applies every pending migration exactly once.
func Apply(ctx context.Context, databaseURL string, logger *slog.Logger) error {
	database, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return fmt.Errorf("open PostgreSQL connection: %w", err)
	}
	defer func() { _ = database.Close() }()

	if err := database.PingContext(ctx); err != nil {
		return fmt.Errorf("ping PostgreSQL: %w", err)
	}

	migrationFS, err := fs.Sub(files, "sql")
	if err != nil {
		return fmt.Errorf("open embedded migrations: %w", err)
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, database, migrationFS, goose.WithSlog(logger))
	if err != nil {
		return fmt.Errorf("create migration provider: %w", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("apply pending migrations: %w", err)
	}
	return nil
}

// AdoptBaseline marks the legacy baseline as applied without executing its schema DDL.
// Call it only after the operator has compared the existing database schema to the baseline.
func AdoptBaseline(ctx context.Context, databaseURL string) error {
	database, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return fmt.Errorf("open PostgreSQL connection: %w", err)
	}
	defer func() { _ = database.Close() }()
	if err := database.PingContext(ctx); err != nil {
		return fmt.Errorf("ping PostgreSQL: %w", err)
	}
	if _, err := database.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS goose_db_version (id BIGSERIAL PRIMARY KEY, version_id BIGINT NOT NULL, is_applied BOOLEAN NOT NULL, tstamp TIMESTAMP NOT NULL DEFAULT NOW())`); err != nil {
		return fmt.Errorf("create migration version table: %w", err)
	}
	var count int
	if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM goose_db_version`).Scan(&count); err != nil {
		return fmt.Errorf("inspect migration version table: %w", err)
	}
	if count != 0 {
		return fmt.Errorf("refusing baseline adoption: goose_db_version already contains migration history")
	}
	if _, err := database.ExecContext(ctx, `INSERT INTO goose_db_version(version_id,is_applied) VALUES(1,TRUE)`); err != nil {
		return fmt.Errorf("record baseline adoption: %w", err)
	}
	return nil
}
