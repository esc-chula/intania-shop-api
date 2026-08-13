// Package auth implements external authentication protocol details.
package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const oauthStateCookieName = "intania_shop_oauth_state"

var (
	// ErrInvalidOAuthState indicates a callback lacks a valid state and verifier pair.
	ErrInvalidOAuthState = errors.New("invalid or expired OAuth state")
	// ErrUnverifiedGoogleEmail indicates Google did not attest to the email address.
	ErrUnverifiedGoogleEmail = errors.New("google email is not verified")
)

// GoogleConfig contains the Google client settings needed for OAuth authorization-code flow.
type GoogleConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	CookieSecret string
	CookieSecure bool
}

// GoogleUser is the verified profile returned by Google's user-info endpoint.
type GoogleUser struct {
	ID            string  `json:"id"`
	Email         string  `json:"email"`
	VerifiedEmail bool    `json:"verified_email"`
	Name          string  `json:"name"`
	Picture       *string `json:"picture"`
}

// GoogleOAuth implements PKCE and CSRF validation around Google's OAuth flow.
type GoogleOAuth struct {
	config       oauth2.Config
	cookieSecret []byte
	cookieSecure bool
	httpClient   *http.Client
	now          func() time.Time
}

type statePayload struct {
	State     string `json:"state"`
	Verifier  string `json:"verifier"`
	ExpiresAt int64  `json:"expires_at"`
}

// NewGoogleOAuth creates a Google authorization-code client with PKCE support.
func NewGoogleOAuth(config GoogleConfig) (*GoogleOAuth, error) {
	if strings.TrimSpace(config.ClientID) == "" || strings.TrimSpace(config.ClientSecret) == "" || strings.TrimSpace(config.RedirectURL) == "" {
		return nil, fmt.Errorf("google OAuth client ID, secret, and redirect URL are required")
	}
	if len(config.CookieSecret) < 32 {
		return nil, fmt.Errorf("OAuth cookie secret must be at least 32 bytes")
	}
	if _, err := url.ParseRequestURI(config.RedirectURL); err != nil {
		return nil, fmt.Errorf("parse Google OAuth redirect URL: %w", err)
	}

	return &GoogleOAuth{
		config: oauth2.Config{
			ClientID:     config.ClientID,
			ClientSecret: config.ClientSecret,
			RedirectURL:  config.RedirectURL,
			Endpoint:     google.Endpoint,
			Scopes:       []string{"openid", "email", "profile"},
		},
		cookieSecret: []byte(config.CookieSecret),
		cookieSecure: config.CookieSecure,
		httpClient:   &http.Client{Timeout: 10 * time.Second},
		now:          time.Now,
	}, nil
}

// Begin returns Google's authorization URL and a signed, short-lived PKCE state cookie.
func (oauth *GoogleOAuth) Begin() (string, *http.Cookie, error) {
	state, err := randomURLValue(32)
	if err != nil {
		return "", nil, fmt.Errorf("generate OAuth state: %w", err)
	}
	verifier, err := randomURLValue(48)
	if err != nil {
		return "", nil, fmt.Errorf("generate PKCE verifier: %w", err)
	}

	payload := statePayload{State: state, Verifier: verifier, ExpiresAt: oauth.now().Add(10 * time.Minute).Unix()}
	cookieValue, err := oauth.signState(payload)
	if err != nil {
		return "", nil, fmt.Errorf("sign OAuth state: %w", err)
	}

	authURL := oauth.config.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.S256ChallengeOption(verifier))
	return authURL, &http.Cookie{
		Name:     oauthStateCookieName,
		Value:    cookieValue,
		Path:     "/auth/google",
		MaxAge:   600,
		HttpOnly: true,
		Secure:   oauth.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	}, nil
}

// Complete validates the browser state, exchanges code for a token, and reads a verified Google profile.
func (oauth *GoogleOAuth) Complete(request *http.Request, code, state string) (GoogleUser, error) {
	payload, err := oauth.readState(request)
	if err != nil || !hmac.Equal([]byte(payload.State), []byte(state)) {
		return GoogleUser{}, ErrInvalidOAuthState
	}

	token, err := oauth.config.Exchange(request.Context(), code, oauth2.VerifierOption(payload.Verifier))
	if err != nil {
		return GoogleUser{}, fmt.Errorf("exchange Google authorization code: %w", err)
	}

	profile, err := oauth.fetchUser(request.Context(), token.AccessToken)
	if err != nil {
		return GoogleUser{}, err
	}
	if !profile.VerifiedEmail {
		return GoogleUser{}, ErrUnverifiedGoogleEmail
	}
	return profile, nil
}

// ClearStateCookie returns an expired cookie for use after an OAuth callback.
func (oauth *GoogleOAuth) ClearStateCookie() *http.Cookie {
	return &http.Cookie{Name: oauthStateCookieName, Value: "", Path: "/auth/google", MaxAge: -1, HttpOnly: true, Secure: oauth.cookieSecure, SameSite: http.SameSiteLaxMode}
}

func (oauth *GoogleOAuth) fetchUser(requestContext context.Context, accessToken string) (GoogleUser, error) {
	request, err := http.NewRequestWithContext(requestContext, http.MethodGet, "https://www.googleapis.com/oauth2/v2/userinfo", nil)
	if err != nil {
		return GoogleUser{}, fmt.Errorf("create Google user-info request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+accessToken)
	response, err := oauth.httpClient.Do(request)
	if err != nil {
		return GoogleUser{}, fmt.Errorf("request Google user-info: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return GoogleUser{}, fmt.Errorf("google user-info returned status %d", response.StatusCode)
	}

	var user GoogleUser
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&user); err != nil {
		return GoogleUser{}, fmt.Errorf("decode Google user-info: %w", err)
	}
	if strings.TrimSpace(user.ID) == "" || strings.TrimSpace(user.Email) == "" {
		return GoogleUser{}, fmt.Errorf("google user-info response lacks ID or email")
	}
	return user, nil
}

func (oauth *GoogleOAuth) signState(payload statePayload) (string, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	message := base64.RawURLEncoding.EncodeToString(encoded)
	mac := hmac.New(sha256.New, oauth.cookieSecret)
	if _, err := mac.Write([]byte(message)); err != nil {
		return "", err
	}
	return message + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func (oauth *GoogleOAuth) readState(request *http.Request) (statePayload, error) {
	cookie, err := request.Cookie(oauthStateCookieName)
	if err != nil {
		return statePayload{}, ErrInvalidOAuthState
	}
	parts := strings.Split(cookie.Value, ".")
	if len(parts) != 2 {
		return statePayload{}, ErrInvalidOAuthState
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return statePayload{}, ErrInvalidOAuthState
	}
	mac := hmac.New(sha256.New, oauth.cookieSecret)
	if _, err := mac.Write([]byte(parts[0])); err != nil {
		return statePayload{}, fmt.Errorf("verify OAuth state: %w", err)
	}
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return statePayload{}, ErrInvalidOAuthState
	}
	encoded, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return statePayload{}, ErrInvalidOAuthState
	}
	var payload statePayload
	if err := json.Unmarshal(encoded, &payload); err != nil || payload.ExpiresAt <= oauth.now().Unix() || payload.State == "" || payload.Verifier == "" {
		return statePayload{}, ErrInvalidOAuthState
	}
	return payload, nil
}

func randomURLValue(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}
