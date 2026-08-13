// Package handlers contains HTTP transport adapters.
package handlers

import (
	"encoding/json"
	"net/http"
)

func writeSuccess(writer http.ResponseWriter, status int, data any) {
	writeJSON(writer, status, map[string]any{"success": true, "data": data})
}

func writeError(writer http.ResponseWriter, status int, message string) {
	writeJSON(writer, status, map[string]any{"success": false, "error": message})
}

func writeJSON(writer http.ResponseWriter, status int, body any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(body)
}
