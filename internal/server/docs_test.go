package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDocumentationEndpoints(t *testing.T) {
	t.Parallel()

	handler := NewHandler(Dependencies{
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		Database: fakePinger{},
	})

	tests := map[string]struct {
		path        string
		contentType string
		body        string
	}{
		"OpenAPI": {
			path:        "/openapi.yaml",
			contentType: "application/yaml",
			body:        "openapi: 3.1.0",
		},
		"Scalar": {
			path:        "/docs",
			contentType: "text/html",
			body:        "Scalar.createApiReference",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			request := httptest.NewRequest(http.MethodGet, test.path, nil)
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
			}
			if contentType := response.Header().Get("Content-Type"); !strings.HasPrefix(contentType, test.contentType) {
				t.Fatalf("content type = %q, want prefix %q", contentType, test.contentType)
			}
			if body := response.Body.String(); !strings.Contains(body, test.body) {
				t.Fatalf("body does not contain %q", test.body)
			}
		})
	}
}
