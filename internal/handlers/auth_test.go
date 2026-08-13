package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/esc-chula/intania-shop-api/internal/auth"
	"github.com/esc-chula/intania-shop-api/internal/models"
	"github.com/esc-chula/intania-shop-api/internal/usecases"
	"github.com/go-chi/chi/v5"
)

type fakeOAuthFlow struct {
	completeError error
}

func (flow fakeOAuthFlow) Begin() (string, *http.Cookie, error) {
	return "https://accounts.example/authorize", &http.Cookie{Name: "state", Value: "signed", HttpOnly: true}, nil
}

func (flow fakeOAuthFlow) Complete(*http.Request, string, string) (auth.GoogleUser, error) {
	if flow.completeError != nil {
		return auth.GoogleUser{}, flow.completeError
	}
	return auth.GoogleUser{ID: "google-user", Email: "user@example.com", VerifiedEmail: true, Name: "User"}, nil
}

func (flow fakeOAuthFlow) ClearStateCookie() *http.Cookie {
	return &http.Cookie{Name: "state", MaxAge: -1}
}

type fakeGoogleLogin struct{}

func (fakeGoogleLogin) LoginWithGoogle(context.Context, auth.GoogleUser) (usecases.LoginResponse, error) {
	return usecases.LoginResponse{User: models.User{ID: 1, Role: models.RoleUser}, Token: "token", Message: "Login successful"}, nil
}

func TestAuthHandler(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		url      string
		oauthErr error
		want     int
	}{
		"starts OAuth":                        {url: "/auth/google", want: http.StatusOK},
		"rejects missing callback parameters": {url: "/auth/google/callback", want: http.StatusBadRequest},
		"rejects invalid state":               {url: "/auth/google/callback?code=code&state=state", oauthErr: auth.ErrInvalidOAuthState, want: http.StatusUnauthorized},
		"completes sign in":                   {url: "/auth/google/callback?code=code&state=state", want: http.StatusOK},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			mux := chi.NewRouter()
			NewAuthHandler(fakeOAuthFlow{completeError: test.oauthErr}, fakeGoogleLogin{}).Register(mux)
			recorder := httptest.NewRecorder()
			mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, test.url, nil))
			if recorder.Code != test.want {
				t.Fatalf("status = %d, want %d; body = %s", recorder.Code, test.want, recorder.Body.String())
			}
		})
	}
}

func TestAuthHandlerProviderError(t *testing.T) {
	t.Parallel()
	mux := chi.NewRouter()
	NewAuthHandler(fakeOAuthFlow{completeError: errors.New("unexpected")}, fakeGoogleLogin{}).Register(mux)
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/auth/google/callback?error=access_denied", nil))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}
