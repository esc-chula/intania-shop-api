package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/esc-chula/intania-shop-api/internal/auth"
	"github.com/esc-chula/intania-shop-api/internal/usecases"
	"github.com/go-chi/chi/v5"
)

// OAuthFlow is the Google protocol capability needed by the authentication handler.
type OAuthFlow interface {
	Begin() (string, *http.Cookie, error)
	Complete(*http.Request, string, string) (auth.GoogleUser, error)
	ClearStateCookie() *http.Cookie
}

// GoogleLogin completes Google sign-in and exchanges the resulting browser handoff code.
type GoogleLogin interface {
	CompleteGoogleLogin(context.Context, auth.GoogleUser) (string, error)
	ExchangeOAuthLoginCode(context.Context, string) (usecases.LoginResponse, error)
}

// AuthHandler serves the approved Google OAuth endpoints.
type AuthHandler struct {
	oauth               OAuthFlow
	login               GoogleLogin
	frontendCallbackURL *url.URL
}

// NewAuthHandler constructs an OAuth HTTP adapter.
func NewAuthHandler(oauth OAuthFlow, login GoogleLogin, frontendCallbackURL string) (*AuthHandler, error) {
	callbackURL, err := url.Parse(frontendCallbackURL)
	if err != nil {
		return nil, fmt.Errorf("parse frontend callback URL: %w", err)
	}
	return &AuthHandler{oauth: oauth, login: login, frontendCallbackURL: callbackURL}, nil
}

func (handler *AuthHandler) Register(router chi.Router) {
	router.Get("/auth/google", handler.startGoogle)
	router.Get("/auth/google/redirect", handler.redirectGoogle)
	router.Get("/auth/google/callback", handler.completeGoogle)
	router.Post("/auth/exchange", handler.exchangeOAuthLoginCode)
}

func (handler *AuthHandler) startGoogle(writer http.ResponseWriter, request *http.Request) {
	authorizationURL, cookie, err := handler.oauth.Begin()
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "Unable to start Google OAuth")
		return
	}
	http.SetCookie(writer, cookie)
	writeSuccess(writer, http.StatusOK, map[string]string{"auth_url": authorizationURL})
}

func (handler *AuthHandler) redirectGoogle(writer http.ResponseWriter, request *http.Request) {
	authorizationURL, cookie, err := handler.oauth.Begin()
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "Unable to start Google OAuth")
		return
	}
	http.SetCookie(writer, cookie)
	http.Redirect(writer, request, authorizationURL, http.StatusTemporaryRedirect)
}

func (handler *AuthHandler) completeGoogle(writer http.ResponseWriter, request *http.Request) {
	http.SetCookie(writer, handler.oauth.ClearStateCookie())
	if providerError := strings.TrimSpace(request.URL.Query().Get("error")); providerError != "" {
		handler.redirectToFrontendCallback(writer, request, url.Values{"error": {"oauth_" + providerError}})
		return
	}
	code := strings.TrimSpace(request.URL.Query().Get("code"))
	state := strings.TrimSpace(request.URL.Query().Get("state"))
	if code == "" || state == "" {
		handler.redirectToFrontendCallback(writer, request, url.Values{"error": {"oauth_invalid_callback"}})
		return
	}
	googleUser, err := handler.oauth.Complete(request, code, state)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidOAuthState) || errors.Is(err, auth.ErrUnverifiedGoogleEmail) {
			handler.redirectToFrontendCallback(writer, request, url.Values{"error": {"oauth_unauthorized"}})
			return
		}
		handler.redirectToFrontendCallback(writer, request, url.Values{"error": {"oauth_failed"}})
		return
	}
	loginCode, err := handler.login.CompleteGoogleLogin(request.Context(), googleUser)
	if err != nil {
		handler.redirectToFrontendCallback(writer, request, url.Values{"error": {"oauth_server_error"}})
		return
	}
	handler.redirectToFrontendCallback(writer, request, url.Values{"code": {loginCode}})
}

func (handler *AuthHandler) redirectToFrontendCallback(writer http.ResponseWriter, request *http.Request, values url.Values) {
	frontendCallbackURL := *handler.frontendCallbackURL
	query := frontendCallbackURL.Query()
	for key, value := range values {
		query.Del(key)
		for _, item := range value {
			query.Add(key, item)
		}
	}
	frontendCallbackURL.RawQuery = query.Encode()
	http.Redirect(writer, request, frontendCallbackURL.String(), http.StatusSeeOther)
}

func (handler *AuthHandler) exchangeOAuthLoginCode(writer http.ResponseWriter, request *http.Request) {
	var body struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
		writeError(writer, http.StatusBadRequest, "Invalid JSON body")
		return
	}
	response, err := handler.login.ExchangeOAuthLoginCode(request.Context(), strings.TrimSpace(body.Code))
	if err != nil {
		if errors.Is(err, usecases.ErrInvalidOAuthLoginCode) {
			writeError(writer, http.StatusUnauthorized, "Invalid, expired, or already used login code")
			return
		}
		writeError(writer, http.StatusInternalServerError, "Unable to exchange login code")
		return
	}
	writeSuccess(writer, http.StatusOK, response)
}
