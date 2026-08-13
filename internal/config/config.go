// Package config loads API configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const defaultAddress = "0.0.0.0:8080"

// Config contains all application configuration.
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	CORS     CORSConfig
	Log      LogConfig
	Auth     AuthConfig
	Storage  StorageConfig
}

// ServerConfig controls the HTTP server lifecycle.
type ServerConfig struct {
	Address           string
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ShutdownTimeout   time.Duration
}

// DatabaseConfig controls PostgreSQL connection pooling.
type DatabaseConfig struct {
	URL             string
	MaxConns        int32
	MinConns        int32
	MaxConnLifetime time.Duration
	MaxConnIdleTime time.Duration
}

// CORSConfig controls browser origins permitted to call the API.
type CORSConfig struct {
	AllowedOrigins []string
}

// LogConfig controls structured logging.
type LogConfig struct {
	Level string
}

// StorageConfig controls object storage.
type StorageConfig struct {
	Bucket string
}

// AuthConfig controls access-token and Google OAuth settings.
type AuthConfig struct {
	JWTSecret          string
	JWTIssuer          string
	JWTTTL             time.Duration
	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string
	CookieSecure       bool
}

// ValidateForServer confirms all security-sensitive server configuration is present.
func (config AuthConfig) ValidateForServer() error {
	if len(config.JWTSecret) < 32 {
		return fmt.Errorf("JWT_SECRET must be at least 32 bytes")
	}
	if strings.TrimSpace(config.GoogleClientID) == "" || strings.TrimSpace(config.GoogleClientSecret) == "" || strings.TrimSpace(config.GoogleRedirectURL) == "" {
		return fmt.Errorf("GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET, and GOOGLE_REDIRECT_URL are required")
	}
	return nil
}

// Load reads and validates configuration from the process environment.
func Load() (Config, error) {
	return load(os.Getenv)
}

func load(getenv func(string) string) (Config, error) {
	databaseURL := strings.TrimSpace(getenv("DATABASE_URL"))
	if databaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}

	maxConns, err := parseInt32(getenv("DB_MAX_CONNS"), 10, "DB_MAX_CONNS")
	if err != nil {
		return Config{}, err
	}
	minConns, err := parseInt32(getenv("DB_MIN_CONNS"), 0, "DB_MIN_CONNS")
	if err != nil {
		return Config{}, err
	}
	if minConns > maxConns {
		return Config{}, fmt.Errorf("DB_MIN_CONNS cannot exceed DB_MAX_CONNS")
	}

	readHeaderTimeout, err := parseDuration(getenv("SERVER_READ_HEADER_TIMEOUT"), 5*time.Second, "SERVER_READ_HEADER_TIMEOUT")
	if err != nil {
		return Config{}, err
	}
	readTimeout, err := parseDuration(getenv("SERVER_READ_TIMEOUT"), 15*time.Second, "SERVER_READ_TIMEOUT")
	if err != nil {
		return Config{}, err
	}
	writeTimeout, err := parseDuration(getenv("SERVER_WRITE_TIMEOUT"), 30*time.Second, "SERVER_WRITE_TIMEOUT")
	if err != nil {
		return Config{}, err
	}
	idleTimeout, err := parseDuration(getenv("SERVER_IDLE_TIMEOUT"), 60*time.Second, "SERVER_IDLE_TIMEOUT")
	if err != nil {
		return Config{}, err
	}
	shutdownTimeout, err := parseDuration(getenv("SERVER_SHUTDOWN_TIMEOUT"), 15*time.Second, "SERVER_SHUTDOWN_TIMEOUT")
	if err != nil {
		return Config{}, err
	}
	maxConnLifetime, err := parseDuration(getenv("DB_MAX_CONN_LIFETIME"), 30*time.Minute, "DB_MAX_CONN_LIFETIME")
	if err != nil {
		return Config{}, err
	}
	maxConnIdleTime, err := parseDuration(getenv("DB_MAX_CONN_IDLE_TIME"), 5*time.Minute, "DB_MAX_CONN_IDLE_TIME")
	if err != nil {
		return Config{}, err
	}
	jwtTTL, err := parseDuration(getenv("JWT_TTL"), 24*time.Hour, "JWT_TTL")
	if err != nil {
		return Config{}, err
	}

	return Config{
		Server: ServerConfig{
			Address:           firstNonEmpty(getenv("SERVER_ADDR"), defaultAddress),
			ReadHeaderTimeout: readHeaderTimeout,
			ReadTimeout:       readTimeout,
			WriteTimeout:      writeTimeout,
			IdleTimeout:       idleTimeout,
			ShutdownTimeout:   shutdownTimeout,
		},
		Database: DatabaseConfig{
			URL:             databaseURL,
			MaxConns:        maxConns,
			MinConns:        minConns,
			MaxConnLifetime: maxConnLifetime,
			MaxConnIdleTime: maxConnIdleTime,
		},
		CORS:    CORSConfig{AllowedOrigins: parseOrigins(getenv("CORS_ALLOWED_ORIGINS"))},
		Storage: StorageConfig{Bucket: strings.TrimSpace(getenv("GCS_BUCKET"))},
		Log:     LogConfig{Level: strings.ToUpper(firstNonEmpty(getenv("LOG_LEVEL"), "INFO"))},
		Auth: AuthConfig{
			JWTSecret:          strings.TrimSpace(getenv("JWT_SECRET")),
			JWTIssuer:          firstNonEmpty(getenv("JWT_ISSUER"), "intania-shop-api"),
			JWTTTL:             jwtTTL,
			GoogleClientID:     strings.TrimSpace(getenv("GOOGLE_CLIENT_ID")),
			GoogleClientSecret: strings.TrimSpace(getenv("GOOGLE_CLIENT_SECRET")),
			GoogleRedirectURL:  strings.TrimSpace(getenv("GOOGLE_REDIRECT_URL")),
			CookieSecure:       firstNonEmpty(getenv("AUTH_COOKIE_SECURE"), "true") != "false",
		},
	}, nil
}

func parseInt32(value string, fallback int32, name string) (int32, error) {
	if strings.TrimSpace(value) == "" {
		return fallback, nil
	}

	parsed, err := strconv.ParseInt(value, 10, 32)
	if err != nil || parsed < 0 {
		return 0, fmt.Errorf("%s must be a non-negative integer", name)
	}
	return int32(parsed), nil
}

func parseDuration(value string, fallback time.Duration, name string) (time.Duration, error) {
	if strings.TrimSpace(value) == "" {
		return fallback, nil
	}

	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration", name)
	}
	return parsed, nil
}

func parseOrigins(value string) []string {
	if strings.TrimSpace(value) == "" {
		return []string{
			"http://localhost:3000",
			"http://localhost:8080",
			"http://127.0.0.1:3000",
			"http://127.0.0.1:8080",
		}
	}

	values := strings.Split(value, ",")
	origins := make([]string, 0, len(values))
	for _, origin := range values {
		if trimmed := strings.TrimSpace(origin); trimmed != "" {
			origins = append(origins, trimmed)
		}
	}
	return origins
}

func firstNonEmpty(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
