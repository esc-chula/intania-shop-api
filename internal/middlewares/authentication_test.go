package middlewares

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/esc-chula/intania-shop-api/internal/models"
)

type fakeVerifier struct {
	identity models.Identity
	err      error
}

func (verifier fakeVerifier) Verify(string) (models.Identity, error) {
	return verifier.identity, verifier.err
}

func TestAuthenticateAndRequireRole(t *testing.T) {
	t.Parallel()

	next := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		identity, ok := IdentityFromContext(request.Context())
		if !ok || identity.UserID != 7 {
			t.Error("authenticated identity missing")
		}
		writer.WriteHeader(http.StatusNoContent)
	})

	tests := map[string]struct {
		header string
		role   models.Role
		err    error
		want   int
	}{
		"missing header": {want: http.StatusUnauthorized},
		"invalid token":  {header: "Bearer bad", err: errors.New("bad"), want: http.StatusUnauthorized},
		"user forbidden": {header: "Bearer token", role: models.RoleUser, want: http.StatusForbidden},
		"admin allowed":  {header: "Bearer token", role: models.RoleAdmin, want: http.StatusNoContent},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			handler := Authenticate(fakeVerifier{identity: models.Identity{UserID: 7, Role: test.role}, err: test.err})(RequireRole(models.RoleAdmin)(next))
			request := httptest.NewRequest(http.MethodGet, "/", nil)
			request.Header.Set("Authorization", test.header)
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			if recorder.Code != test.want {
				t.Fatalf("status = %d, want %d", recorder.Code, test.want)
			}
		})
	}
}
