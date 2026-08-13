package middlewares

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/esc-chula/intania-shop-api/internal/models"
)

// TokenVerifier validates a bearer token into an authenticated identity.
type TokenVerifier interface {
	Verify(string) (models.Identity, error)
}

type identityContextKey struct{}

// Authenticate requires a valid bearer token and attaches its identity to the request context.
func Authenticate(verifier TokenVerifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			header := request.Header.Get("Authorization")
			token, ok := strings.CutPrefix(header, "Bearer ")
			if !ok || strings.TrimSpace(token) == "" {
				writeAuthError(writer, http.StatusUnauthorized, "Missing or invalid authorization header")
				return
			}

			identity, err := verifier.Verify(strings.TrimSpace(token))
			if err != nil {
				writeAuthError(writer, http.StatusUnauthorized, "Invalid or expired token")
				return
			}
			next.ServeHTTP(writer, request.WithContext(context.WithValue(request.Context(), identityContextKey{}, identity)))
		})
	}
}

// RequireRole permits only identities with one of roles. It expects Authenticate to run first.
func RequireRole(roles ...models.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			identity, ok := IdentityFromContext(request.Context())
			if !ok {
				writeAuthError(writer, http.StatusUnauthorized, "Authentication required")
				return
			}
			for _, role := range roles {
				if identity.Role == role {
					next.ServeHTTP(writer, request)
					return
				}
			}
			writeAuthError(writer, http.StatusForbidden, "Insufficient permissions")
		})
	}
}

// IdentityFromContext returns the authenticated request identity, if present.
func IdentityFromContext(ctx context.Context) (models.Identity, bool) {
	identity, ok := ctx.Value(identityContextKey{}).(models.Identity)
	return identity, ok
}

func writeAuthError(writer http.ResponseWriter, status int, message string) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(map[string]any{"success": false, "error": message})
}
