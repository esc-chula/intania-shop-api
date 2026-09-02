// Package handlers contains HTTP transport adapters.
package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/esc-chula/intania-shop-api/internal/middlewares"
	"github.com/esc-chula/intania-shop-api/internal/models"
)

func writeSuccess(writer http.ResponseWriter, status int, data any) {
	writeJSON(writer, status, map[string]any{"success": true, "data": data})
}

func writeError(writer http.ResponseWriter, status int, message string) {
	writeJSON(writer, status, map[string]any{"success": false, "error": message})
}

// writeProjectError writes the Project/POS error envelope, which carries a
// machine-readable code and the request identifier alongside the message.
func writeProjectError(writer http.ResponseWriter, request *http.Request, status int, code models.ProjectAPIErrorCode, message string) {
	writeJSON(writer, status, map[string]any{
		"success":    false,
		"error":      message,
		"code":       code,
		"request_id": middlewares.ID(request.Context()),
	})
}

func writeJSON(writer http.ResponseWriter, status int, body any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(body)
}
