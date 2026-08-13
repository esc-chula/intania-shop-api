package usecases

import (
	"context"
	"fmt"

	"github.com/esc-chula/intania-shop-api/internal/auth"
	"github.com/esc-chula/intania-shop-api/internal/models"
)

// GoogleUserUpserter creates or links a local account for a verified Google profile.
type GoogleUserUpserter interface {
	UpsertGoogleUser(context.Context, auth.GoogleUser) (models.User, error)
}

// AccessTokenIssuer creates an API access token for an authenticated identity.
type AccessTokenIssuer interface {
	Issue(models.Identity) (string, error)
}

// AuthService applies local account rules after Google has verified the external identity.
type AuthService struct {
	users  GoogleUserUpserter
	tokens AccessTokenIssuer
}

// LoginResponse is the legacy-compatible success payload from Google sign-in.
type LoginResponse struct {
	User    models.User `json:"user"`
	Token   string      `json:"token"`
	Message string      `json:"message"`
}

// NewAuthService constructs the Google-login use case.
func NewAuthService(users GoogleUserUpserter, tokens AccessTokenIssuer) *AuthService {
	return &AuthService{users: users, tokens: tokens}
}

// LoginWithGoogle links the verified profile to a local account and issues an access token.
func (service *AuthService) LoginWithGoogle(ctx context.Context, googleUser auth.GoogleUser) (LoginResponse, error) {
	user, err := service.users.UpsertGoogleUser(ctx, googleUser)
	if err != nil {
		return LoginResponse{}, fmt.Errorf("upsert Google user: %w", err)
	}
	token, err := service.tokens.Issue(models.Identity{UserID: user.ID, Role: user.Role})
	if err != nil {
		return LoginResponse{}, fmt.Errorf("issue access token: %w", err)
	}
	return LoginResponse{User: user, Token: token, Message: "Login successful"}, nil
}
