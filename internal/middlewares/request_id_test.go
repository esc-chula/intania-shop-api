package middlewares

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestID(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	tests := map[string]struct {
		incoming string
		want     string
	}{
		"uses valid incoming ID":    {incoming: "request-123", want: "request-123"},
		"replaces invalid incoming": {incoming: "invalid\nID"},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			handler := RequestID(logger)(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if got := ID(request.Context()); got != writer.Header().Get(requestIDHeader) {
					t.Fatalf("context ID = %q, response ID = %q", got, writer.Header().Get(requestIDHeader))
				}
			}))
			request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
			request.Header.Set(requestIDHeader, test.incoming)
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)
			got := response.Header().Get(requestIDHeader)
			if test.want != "" && got != test.want {
				t.Fatalf("request ID = %q, want %q", got, test.want)
			}
			if test.want == "" && !validRequestID(got) {
				t.Fatalf("generated request ID %q is invalid", got)
			}
		})
	}
}
