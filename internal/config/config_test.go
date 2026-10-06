package config

import (
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		env     map[string]string
		wantErr bool
		check   func(t *testing.T, cfg Config)
	}{
		"uses defaults": {
			env: map[string]string{"DATABASE_URL": "postgres://postgres:postgres@localhost:5432/intania_shop"},
			check: func(t *testing.T, cfg Config) {
				t.Helper()
				if cfg.Server.Address != defaultAddress {
					t.Fatalf("address = %q, want %q", cfg.Server.Address, defaultAddress)
				}
				if cfg.Database.MaxConns != 10 {
					t.Fatalf("max connections = %d, want 10", cfg.Database.MaxConns)
				}
				if cfg.Server.WriteTimeout != 30*time.Second {
					t.Fatalf("write timeout = %s, want 30s", cfg.Server.WriteTimeout)
				}
			},
		},
		"reads overrides": {
			env: map[string]string{
				"DATABASE_URL":            "postgres://postgres:postgres@localhost:5432/intania_shop",
				"SERVER_ADDR":             ":9000",
				"DB_MAX_CONNS":            "20",
				"DB_MIN_CONNS":            "2",
				"SERVER_SHUTDOWN_TIMEOUT": "10s",
				"CORS_ALLOWED_ORIGINS":    "https://shop.example, https://admin.example",
				"LOG_LEVEL":               "debug",
			},
			check: func(t *testing.T, cfg Config) {
				t.Helper()
				if cfg.Server.Address != ":9000" || cfg.Database.MinConns != 2 || cfg.Database.MaxConns != 20 {
					t.Fatalf("unexpected overrides: %#v", cfg)
				}
				if cfg.Server.ShutdownTimeout != 10*time.Second {
					t.Fatalf("shutdown timeout = %s, want 10s", cfg.Server.ShutdownTimeout)
				}
				if len(cfg.CORS.AllowedOrigins) != 2 || cfg.Log.Level != "DEBUG" {
					t.Fatalf("unexpected CORS or log config: %#v", cfg)
				}
			},
		},
		"requires database URL": {wantErr: true},
		"rejects invalid duration": {
			env:     map[string]string{"DATABASE_URL": "postgres://postgres:postgres@localhost:5432/intania_shop", "SERVER_READ_TIMEOUT": "later"},
			wantErr: true,
		},
		"rejects invalid connection range": {
			env:     map[string]string{"DATABASE_URL": "postgres://postgres:postgres@localhost:5432/intania_shop", "DB_MAX_CONNS": "1", "DB_MIN_CONNS": "2"},
			wantErr: true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cfg, err := load(func(key string) string { return test.env[key] })
			if test.wantErr {
				if err == nil {
					t.Fatal("load() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("load() error = %v", err)
			}
			test.check(t, cfg)
		})
	}
}

func TestAuthConfigValidateForServer(t *testing.T) {
	t.Parallel()

	valid := AuthConfig{
		JWTSecret:           "12345678901234567890123456789012",
		GoogleClientID:      "client-id",
		GoogleClientSecret:  "client-secret",
		GoogleRedirectURL:   "https://api.example.com/auth/google/callback",
		FrontendCallbackURL: "https://app.example.com/auth/callback",
	}

	tests := map[string]struct {
		config    AuthConfig
		wantError bool
	}{
		"valid":                      {config: valid},
		"missing frontend callback":  {config: func() AuthConfig { cfg := valid; cfg.FrontendCallbackURL = ""; return cfg }(), wantError: true},
		"relative frontend callback": {config: func() AuthConfig { cfg := valid; cfg.FrontendCallbackURL = "/auth/callback"; return cfg }(), wantError: true},
		"frontend callback with fragment": {config: func() AuthConfig {
			cfg := valid
			cfg.FrontendCallbackURL = "https://app.example.com/auth/callback#token"
			return cfg
		}(), wantError: true},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := test.config.ValidateForServer()
			if test.wantError && err == nil {
				t.Fatal("ValidateForServer() error = nil, want error")
			}
			if !test.wantError && err != nil {
				t.Fatalf("ValidateForServer() error = %v", err)
			}
		})
	}
}

func TestConfigValidateForServerRequiresFrontendCORSOrigin(t *testing.T) {
	t.Parallel()

	config := Config{
		CORS: CORSConfig{AllowedOrigins: []string{"https://app.example.com"}},
		Auth: AuthConfig{
			JWTSecret:           "12345678901234567890123456789012",
			GoogleClientID:      "client-id",
			GoogleClientSecret:  "client-secret",
			GoogleRedirectURL:   "https://api.example.com/auth/google/callback",
			FrontendCallbackURL: "https://app.example.com/auth/callback",
		},
	}
	if err := config.ValidateForServer(); err != nil {
		t.Fatalf("ValidateForServer() error = %v", err)
	}

	config.CORS.AllowedOrigins = []string{"https://other.example.com"}
	if err := config.ValidateForServer(); err == nil {
		t.Fatal("ValidateForServer() error = nil, want error when frontend origin is not in CORS_ALLOWED_ORIGINS")
	}
}
