package usecases

import (
	"context"
	"errors"
	"fmt"

	"github.com/esc-chula/intania-shop-api/internal/auth"
	"github.com/esc-chula/intania-shop-api/internal/models"
)

// ErrInvalidOAuthLoginCode indicates that a browser login handoff code is invalid, expired, or already used.
var ErrInvalidOAuthLoginCode = errors.New("invalid, expired, or already used OAuth login code")

// GoogleUserUpserter creates or links a local account for a verified Google profile.
type GoogleUserUpserter interface {
	UpsertGoogleUser(context.Context, auth.GoogleUser) (models.User, error)
}

// AccessTokenIssuer creates an API access token for an authenticated identity.
type AccessTokenIssuer interface {
	Issue(models.Identity) (string, error)
}

// OAuthLoginCodeStore creates and consumes short-lived one-time browser handoff codes.
type OAuthLoginCodeStore interface {
	Create(context.Context, int64) (string, error)
	Consume(context.Context, string) (models.User, bool, error)
}

// AuthService applies local account rules after Google has verified the external identity.
type AuthService struct {
	users  GoogleUserUpserter
	tokens AccessTokenIssuer
	codes  OAuthLoginCodeStore
}

// LoginResponse is the success payload from Google sign-in.
type LoginResponse struct {
	User    models.User `json:"user"`
	Token   string      `json:"token"`
	Message string      `json:"message"`
}

// NewAuthService constructs the Google-login use case.
func NewAuthService(users GoogleUserUpserter, tokens AccessTokenIssuer, codes OAuthLoginCodeStore) *AuthService {
	return &AuthService{users: users, tokens: tokens, codes: codes}
}

// CompleteGoogleLogin links the verified profile to a local account and creates a one-time browser handoff code.
func (service *AuthService) CompleteGoogleLogin(ctx context.Context, googleUser auth.GoogleUser) (string, error) {
	user, err := service.users.UpsertGoogleUser(ctx, googleUser)
	if err != nil {
		return "", fmt.Errorf("upsert Google user: %w", err)
	}
	code, err := service.codes.Create(ctx, user.ID)
	if err != nil {
		return "", fmt.Errorf("create OAuth login code: %w", err)
	}
	return code, nil
}

// ExchangeOAuthLoginCode consumes a one-time browser handoff code and issues an access token.
func (service *AuthService) ExchangeOAuthLoginCode(ctx context.Context, code string) (LoginResponse, error) {
	user, found, err := service.codes.Consume(ctx, code)
	if err != nil {
		return LoginResponse{}, fmt.Errorf("consume OAuth login code: %w", err)
	}
	if !found {
		return LoginResponse{}, ErrInvalidOAuthLoginCode
	}
	token, err := service.tokens.Issue(models.Identity{UserID: user.ID, Role: user.Role})
	if err != nil {
		return LoginResponse{}, fmt.Errorf("issue access token: %w", err)
	}
	return LoginResponse{User: user, Token: token, Message: "Login successful"}, nil
}
