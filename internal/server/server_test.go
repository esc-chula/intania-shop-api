package server

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakePinger struct {
	err error
}

func (pinger fakePinger) Ping(context.Context) error {
	return pinger.err
}

func TestHandler(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	tests := map[string]struct {
		path       string
		pinger     fakePinger
		wantStatus int
		wantBody   string
	}{
		"root":      {path: "/", wantStatus: http.StatusOK, wantBody: "intania-shop-api"},
		"healthy":   {path: "/health", wantStatus: http.StatusOK, wantBody: "ok"},
		"unhealthy": {path: "/health", pinger: fakePinger{err: errors.New("unavailable")}, wantStatus: http.StatusServiceUnavailable, wantBody: "unavailable"},
		"not found": {path: "/missing", wantStatus: http.StatusNotFound},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			handler := NewHandler(Dependencies{
				Logger:             logger,
				Database:           test.pinger,
				CORSAllowedOrigins: []string{"http://localhost:3000"},
			})
			request := httptest.NewRequest(http.MethodGet, test.path, nil)
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.wantStatus)
			}
			if test.wantBody != "" && response.Body.String() != test.wantBody {
				t.Fatalf("body = %q, want %q", response.Body.String(), test.wantBody)
			}
			if response.Header().Get("X-Request-ID") == "" {
				t.Fatal("response is missing X-Request-ID")
			}
		})
	}
}
