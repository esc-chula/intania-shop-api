package middlewares

import "net/http"

// Middleware wraps an HTTP handler.
type Middleware func(http.Handler) http.Handler
