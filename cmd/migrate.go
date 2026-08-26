package cmd

import (
	"fmt"

	"github.com/esc-chula/intania-shop-api/internal/config"
	"github.com/esc-chula/intania-shop-api/internal/migrations"
	"github.com/esc-chula/intania-shop-api/internal/server"
	"github.com/spf13/cobra"
)

func newMigrateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "migrate",
		Short: "Apply pending PostgreSQL migrations",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("load configuration: %w", err)
			}

			logger := server.NewLogger(cfg.Log.Level)
			if err := migrations.Apply(cmd.Context(), cfg.Database.URL, logger); err != nil {
				return fmt.Errorf("apply migrations: %w", err)
			}
			return nil
		},
	}
}
