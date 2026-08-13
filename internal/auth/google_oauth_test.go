package auth

import (
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

const testCookieSecret = "12345678901234567890123456789012"

func TestGoogleOAuthState(t *testing.T) {
	t.Parallel()

	oauth, err := NewGoogleOAuth(GoogleConfig{
		ClientID: "client", ClientSecret: "secret", RedirectURL: "https://api.example/auth/google/callback", CookieSecret: testCookieSecret,
	})
	if err != nil {
		t.Fatalf("NewGoogleOAuth() error = %v", err)
	}
	now := time.Date(2026, 8, 8, 0, 0, 0, 0, time.UTC)
	oauth.now = func() time.Time { return now }

	authorizationURL, cookie, err := oauth.Begin()
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}
	parsedURL, err := url.Parse(authorizationURL)
	if err != nil {
		t.Fatalf("parse authorization URL: %v", err)
	}
	state := parsedURL.Query().Get("state")
	if state == "" || parsedURL.Query().Get("code_challenge") == "" || cookie.HttpOnly != true {
		t.Fatalf("Begin() returned incomplete OAuth values: %s", authorizationURL)
	}

	tests := map[string]struct {
		cookieValue string
		state       string
		wantErr     error
	}{
		"valid state reaches exchange": {cookieValue: cookie.Value, state: state},
		"mismatched state":             {cookieValue: cookie.Value, state: "other", wantErr: ErrInvalidOAuthState},
		"tampered cookie":              {cookieValue: cookie.Value + "x", state: state, wantErr: ErrInvalidOAuthState},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			request := httptest.NewRequest("GET", "/auth/google/callback", nil)
			request.AddCookie(cookie)
			if test.cookieValue != cookie.Value {
				request.Header.Set("Cookie", cookie.Name+"="+test.cookieValue)
			}
			_, err := oauth.Complete(request, "code", test.state)
			if test.wantErr != nil {
				if err != test.wantErr {
					t.Fatalf("Complete() error = %v, want %v", err, test.wantErr)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), "exchange Google authorization code") {
				t.Fatalf("Complete() error = %v, want exchange error after valid state", err)
			}
		})
	}
}
