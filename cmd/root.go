package cmd

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
)

var rootCmd = newRootCommand()

func newRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "intania-shop-api",
		Short:         "Intania Shop API",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE:          runServe,
	}
	cmd.AddCommand(newServeCommand(), newMigrateCommand())
	return cmd
}

// Execute runs the command-line interface with signal-aware cancellation.
func Execute() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return rootCmd.ExecuteContext(ctx)
}
