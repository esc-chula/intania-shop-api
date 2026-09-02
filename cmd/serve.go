package cmd

import (
	"fmt"

	"github.com/esc-chula/intania-shop-api/internal/auth"
	"github.com/esc-chula/intania-shop-api/internal/config"
	"github.com/esc-chula/intania-shop-api/internal/database"
	"github.com/esc-chula/intania-shop-api/internal/handlers"
	"github.com/esc-chula/intania-shop-api/internal/repositories"
	"github.com/esc-chula/intania-shop-api/internal/server"
	"github.com/esc-chula/intania-shop-api/internal/storage"
	"github.com/esc-chula/intania-shop-api/internal/usecases"
	"github.com/spf13/cobra"
)

func newServeCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Run the HTTP API server",
		RunE:  runServe,
	}
}

func runServe(cmd *cobra.Command, _ []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	if err := cfg.Auth.ValidateForServer(); err != nil {
		return fmt.Errorf("validate authentication configuration: %w", err)
	}
	tokens, err := usecases.NewTokenManager(cfg.Auth.JWTSecret, cfg.Auth.JWTIssuer, cfg.Auth.JWTTTL)
	if err != nil {
		return fmt.Errorf("initialize token manager: %w", err)
	}
	oauth, err := auth.NewGoogleOAuth(auth.GoogleConfig{
		ClientID: cfg.Auth.GoogleClientID, ClientSecret: cfg.Auth.GoogleClientSecret,
		RedirectURL: cfg.Auth.GoogleRedirectURL, CookieSecret: cfg.Auth.JWTSecret, CookieSecure: cfg.Auth.CookieSecure,
	})
	if err != nil {
		return fmt.Errorf("initialize Google OAuth: %w", err)
	}

	logger := server.NewLogger(cfg.Log.Level)
	pool, err := database.Open(cmd.Context(), cfg.Database)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer pool.Close()
	uploader, err := storage.NewGCSUploader(cmd.Context(), cfg.Storage.Bucket)
	if err != nil {
		return fmt.Errorf("initialize GCS uploader: %w", err)
	}
	defer func() { _ = uploader.Close() }()
	users := repositories.NewUserRepository(pool)
	authHandler := handlers.NewAuthHandler(oauth, usecases.NewAuthService(users, tokens))
	productHandler := handlers.NewProductHandler(usecases.NewProductService(repositories.NewProductRepository(pool)))
	productAdminHandler := handlers.NewProductAdminHandler(usecases.NewProductAdminService(repositories.NewProductRepository(pool)))
	inventoryHandler := handlers.NewInventoryHandler(usecases.NewInventoryService(repositories.NewInventoryRepository(pool)))
	projectHandler := handlers.NewProjectHandler(usecases.NewProjectService(repositories.NewProjectRepository(pool)))
	uploadHandler := handlers.NewUploadHandler(uploader)

	handler := server.NewHandler(server.Dependencies{
		Logger:              logger,
		Database:            pool,
		CORSAllowedOrigins:  cfg.CORS.AllowedOrigins,
		AuthHandler:         authHandler,
		ProductHandler:      productHandler,
		ProductAdminHandler: productAdminHandler,
		InventoryHandler:    inventoryHandler,
		ProjectHandler:      projectHandler,
		UploadHandler:       uploadHandler,
		TokenVerifier:       tokens,
	})
	httpServer := server.NewHTTPServer(cfg.Server, handler)

	logger.Info("starting API server", "address", cfg.Server.Address)
	if err := server.Run(cmd.Context(), httpServer, cfg.Server.ShutdownTimeout); err != nil {
		return fmt.Errorf("run server: %w", err)
	}
	logger.Info("API server stopped")
	return nil
}
