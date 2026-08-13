package handlers

import (
	"context"
	"errors"
	"github.com/go-chi/chi/v5"
	"net/http"
	"strings"

	"github.com/esc-chula/intania-shop-api/internal/auth"
	"github.com/esc-chula/intania-shop-api/internal/usecases"
)

// OAuthFlow is the Google protocol capability needed by the authentication handler.
type OAuthFlow interface {
	Begin() (string, *http.Cookie, error)
	Complete(*http.Request, string, string) (auth.GoogleUser, error)
	ClearStateCookie() *http.Cookie
}

// GoogleLogin performs local account linking after Google authentication succeeds.
type GoogleLogin interface {
	LoginWithGoogle(context.Context, auth.GoogleUser) (usecases.LoginResponse, error)
}

// AuthHandler serves the approved Google OAuth endpoints.
type AuthHandler struct {
	oauth OAuthFlow
	login GoogleLogin
}

// NewAuthHandler constructs an OAuth HTTP adapter.
func NewAuthHandler(oauth OAuthFlow, login GoogleLogin) *AuthHandler {
	return &AuthHandler{oauth: oauth, login: login}
}

func (handler *AuthHandler) Register(router chi.Router) {
	router.Get("/auth/google", handler.startGoogle)
	router.Get("/auth/google/redirect", handler.redirectGoogle)
	router.Get("/auth/google/callback", handler.completeGoogle)
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
		writeError(writer, http.StatusBadRequest, providerError+": "+request.URL.Query().Get("error_description"))
		return
	}
	code := strings.TrimSpace(request.URL.Query().Get("code"))
	state := strings.TrimSpace(request.URL.Query().Get("state"))
	if code == "" || state == "" {
		writeError(writer, http.StatusBadRequest, "Missing OAuth code or state")
		return
	}
	googleUser, err := handler.oauth.Complete(request, code, state)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidOAuthState) || errors.Is(err, auth.ErrUnverifiedGoogleEmail) {
			writeError(writer, http.StatusUnauthorized, err.Error())
			return
		}
		writeError(writer, http.StatusBadGateway, "Google OAuth failed")
		return
	}
	response, err := handler.login.LoginWithGoogle(request.Context(), googleUser)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "Unable to complete sign-in")
		return
	}
	writeSuccess(writer, http.StatusOK, response)
}
