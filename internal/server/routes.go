package server

import (
	"context"
	"net/http"
	"time"
)

const healthTimeout = 2 * time.Second

func root(writer http.ResponseWriter, _ *http.Request) {
	writeText(writer, http.StatusOK, "intania-shop-api")
}

func health(database Pinger) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if database == nil {
			writeText(writer, http.StatusServiceUnavailable, "unavailable")
			return
		}

		ctx, cancel := context.WithTimeout(request.Context(), healthTimeout)
		defer cancel()
		if err := database.Ping(ctx); err != nil {
			writeText(writer, http.StatusServiceUnavailable, "unavailable")
			return
		}
		writeText(writer, http.StatusOK, "ok")
	}
}

func writeText(writer http.ResponseWriter, status int, body string) {
	writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
	writer.WriteHeader(status)
	_, _ = writer.Write([]byte(body))
}
