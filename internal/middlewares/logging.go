package middlewares

import (
	"log/slog"
	"net/http"
	"time"
)

// Logging writes one structured event for each completed HTTP request.
func Logging(logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			startedAt := time.Now()
			recorder := &responseRecorder{ResponseWriter: writer, status: http.StatusOK}
			next.ServeHTTP(recorder, request)

			logger.Info("HTTP request",
				"request_id", ID(request.Context()),
				"method", request.Method,
				"path", request.URL.Path,
				"status", recorder.status,
				"bytes", recorder.bytes,
				"duration", time.Since(startedAt),
			)
		})
	}
}

type responseRecorder struct {
	http.ResponseWriter
	status      int
	bytes       int
	wroteHeader bool
}

func (writer *responseRecorder) WriteHeader(status int) {
	if !writer.wroteHeader {
		writer.status = status
		writer.wroteHeader = true
	}
	writer.ResponseWriter.WriteHeader(status)
}

func (writer *responseRecorder) Write(body []byte) (int, error) {
	if !writer.wroteHeader {
		writer.WriteHeader(http.StatusOK)
	}
	bytesWritten, err := writer.ResponseWriter.Write(body)
	writer.bytes += bytesWritten
	return bytesWritten, err
}
