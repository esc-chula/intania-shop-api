package middlewares

import (
	"log/slog"
	"net/http"
	"runtime/debug"
)

func Recovery(logger *slog.Logger) Middleware { // middleware factory
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			defer func() {
				if recovered := recover(); recovered != nil {
					logger.Error("panic while handling request",
						"request_id", ID(request.Context()),
						"panic", recovered,
						"stack", string(debug.Stack()),
					)
					http.Error(writer, "Internal server error", http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(writer, request)
		})
	}
}
