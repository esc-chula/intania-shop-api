package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/esc-chula/intania-shop-api/internal/config"
	"github.com/esc-chula/intania-shop-api/internal/handlers"
	"github.com/esc-chula/intania-shop-api/internal/middlewares"
	"github.com/esc-chula/intania-shop-api/internal/models"
	"github.com/go-chi/chi/v5"
)

type Pinger interface {
	Ping(context.Context) error
}

type Dependencies struct {
	Logger              *slog.Logger
	Database            Pinger
	CORSAllowedOrigins  []string
	AuthHandler         *handlers.AuthHandler
	ProductHandler      *handlers.ProductHandler
	ProductAdminHandler *handlers.ProductAdminHandler
	InventoryHandler    *handlers.InventoryHandler
	UploadHandler       *handlers.UploadHandler
	TokenVerifier       middlewares.TokenVerifier
}

func NewLogger(level string) *slog.Logger {
	logLevel := new(slog.LevelVar)
	if err := logLevel.UnmarshalText([]byte(level)); err != nil {
		logLevel.Set(slog.LevelInfo)
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}))
}

func NewHandler(dependencies Dependencies) http.Handler {
	if dependencies.Logger == nil {
		dependencies.Logger = slog.Default()
	}

	router := chi.NewRouter()
	router.Use(
		middlewares.Recovery(dependencies.Logger),
		middlewares.RequestID(dependencies.Logger),
		middlewares.Logging(dependencies.Logger),
		middlewares.CORS(dependencies.CORSAllowedOrigins),
	)
	router.Get("/", root)
	router.Get("/health", health(dependencies.Database))
	router.Get("/openapi.yaml", openAPIDocument)
	router.Get("/docs", scalarReference)

	if dependencies.AuthHandler != nil {
		dependencies.AuthHandler.Register(router)
	}
	if dependencies.TokenVerifier != nil {
		authenticated := router.With(middlewares.Authenticate(dependencies.TokenVerifier))
		admin := authenticated.With(middlewares.RequireRole(models.RoleAdmin))
		if dependencies.ProductHandler != nil {
			dependencies.ProductHandler.Register(admin)
		}
		if dependencies.ProductAdminHandler != nil {
			dependencies.ProductAdminHandler.Register(admin)
		}
		if dependencies.InventoryHandler != nil {
			dependencies.InventoryHandler.Register(admin)
		}
		if dependencies.UploadHandler != nil {
			dependencies.UploadHandler.Register(admin)
		}
	}

	return router
}

func NewHTTPServer(cfg config.ServerConfig, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              cfg.Address,
		Handler:           handler,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}
}

func Run(ctx context.Context, httpServer *http.Server, shutdownTimeout time.Duration) error {
	errCh := make(chan error, 1)
	go func() {
		errCh <- httpServer.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown HTTP server: %w", err)
		}
		if err := <-errCh; err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}
}
