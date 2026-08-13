package middlewares

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"
)

const requestIDHeader = "X-Request-ID"

type requestIDKey struct{}

var fallbackRequestID uint64

// RequestID adds a request identifier to the request context and response.
func RequestID(logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			requestID := request.Header.Get(requestIDHeader)
			if !validRequestID(requestID) {
				var err error
				requestID, err = newRequestID()
				if err != nil {
					requestID = fallbackID()
					logger.Error("generate request ID", "error", err)
				}
			}

			writer.Header().Set(requestIDHeader, requestID)
			contextWithID := context.WithValue(request.Context(), requestIDKey{}, requestID)
			next.ServeHTTP(writer, request.WithContext(contextWithID))
		})
	}
}

// ID returns the request identifier stored by RequestID.
func ID(ctx context.Context) string {
	requestID, _ := ctx.Value(requestIDKey{}).(string)
	return requestID
}

func newRequestID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func fallbackID() string {
	sequence := atomic.AddUint64(&fallbackRequestID, 1)
	return strconv.FormatInt(time.Now().UTC().UnixNano(), 36) + "-" + strconv.FormatUint(sequence, 36)
}

func validRequestID(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for _, character := range value {
		if character < 33 || character > 126 {
			return false
		}
	}
	return true
}
