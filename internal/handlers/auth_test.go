package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/esc-chula/intania-shop-api/internal/auth"
	"github.com/esc-chula/intania-shop-api/internal/models"
	"github.com/esc-chula/intania-shop-api/internal/usecases"
)

type fakeOAuthFlow struct {
	googleUser  auth.GoogleUser
	completeErr error
}

func (fake *fakeOAuthFlow) Begin() (string, *http.Cookie, error) {
	return "https://accounts.google.com/o/oauth2/auth", &http.Cookie{Name: "state", Value: "signed"}, nil
}

func (fake *fakeOAuthFlow) Complete(*http.Request, string, string) (auth.GoogleUser, error) {
	return fake.googleUser, fake.completeErr
}

func (fake *fakeOAuthFlow) ClearStateCookie() *http.Cookie {
	return &http.Cookie{Name: "state", MaxAge: -1}
}

type fakeGoogleLogin struct {
	code          string
	completeErr   error
	response      usecases.LoginResponse
	exchangeErr   error
	exchangedCode string
}

func (fake *fakeGoogleLogin) CompleteGoogleLogin(context.Context, auth.GoogleUser) (string, error) {
	return fake.code, fake.completeErr
}

func (fake *fakeGoogleLogin) ExchangeOAuthLoginCode(_ context.Context, code string) (usecases.LoginResponse, error) {
	fake.exchangedCode = code
	return fake.response, fake.exchangeErr
}

func newAuthHandlerForTest(t *testing.T, login GoogleLogin) *AuthHandler {
	t.Helper()
	handler, err := NewAuthHandler(&fakeOAuthFlow{googleUser: auth.GoogleUser{ID: "google-id", Email: "admin@example.com"}}, login, "https://app.example.com/auth/callback?source=google")
	if err != nil {
		t.Fatalf("NewAuthHandler() error = %v", err)
	}
	return handler
}

func TestAuthHandlerCompleteGoogleRedirectsWithOneTimeCode(t *testing.T) {
	t.Parallel()

	handler := newAuthHandlerForTest(t, &fakeGoogleLogin{code: "one-time-code"})
	request := httptest.NewRequest(http.MethodGet, "/auth/google/callback?code=google-code&state=state", nil)
	recorder := httptest.NewRecorder()

	handler.completeGoogle(recorder, request)

	response := recorder.Result()
	if response.StatusCode != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusSeeOther)
	}
	location, err := url.Parse(response.Header.Get("Location"))
	if err != nil {
		t.Fatalf("parse Location: %v", err)
	}
	if location.String() != "https://app.example.com/auth/callback?code=one-time-code&source=google" {
		t.Fatalf("Location = %q", location.String())
	}
}

func TestAuthHandlerExchangeOAuthLoginCode(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		body       string
		login      *fakeGoogleLogin
		wantStatus int
	}{
		"returns JWT after valid exchange": {
			body:       `{"code":"one-time-code"}`,
			login:      &fakeGoogleLogin{response: usecases.LoginResponse{Token: "jwt", User: models.User{ID: 7, Role: models.RoleAdmin}, Message: "Login successful"}},
			wantStatus: http.StatusOK,
		},
		"rejects used code": {
			body:       `{"code":"used-code"}`,
			login:      &fakeGoogleLogin{exchangeErr: usecases.ErrInvalidOAuthLoginCode},
			wantStatus: http.StatusUnauthorized,
		},
		"rejects invalid JSON": {
			body:       `{`,
			login:      &fakeGoogleLogin{},
			wantStatus: http.StatusBadRequest,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			handler := newAuthHandlerForTest(t, test.login)
			request := httptest.NewRequest(http.MethodPost, "/auth/exchange", strings.NewReader(test.body))
			recorder := httptest.NewRecorder()

			handler.exchangeOAuthLoginCode(recorder, request)

			if recorder.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, test.wantStatus)
			}
			if test.wantStatus == http.StatusOK {
				var body struct {
					Success bool                   `json:"success"`
					Data    usecases.LoginResponse `json:"data"`
				}
				if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
					t.Fatalf("decode response: %v", err)
				}
				if !body.Success || body.Data.Token != "jwt" || test.login.exchangedCode != "one-time-code" {
					t.Fatalf("response = %#v, exchanged code = %q", body, test.login.exchangedCode)
				}
			}
		})
	}
}

var _ OAuthFlow = (*fakeOAuthFlow)(nil)
var _ GoogleLogin = (*fakeGoogleLogin)(nil)
