package usecases

import (
	"context"
	"errors"
	"testing"

	"github.com/esc-chula/intania-shop-api/internal/auth"
	"github.com/esc-chula/intania-shop-api/internal/models"
)

type fakeGoogleUserUpserter struct {
	user models.User
	err  error
}

func (fake *fakeGoogleUserUpserter) UpsertGoogleUser(context.Context, auth.GoogleUser) (models.User, error) {
	return fake.user, fake.err
}

type fakeTokenIssuer struct {
	identity models.Identity
	token    string
	err      error
}

func (fake *fakeTokenIssuer) Issue(identity models.Identity) (string, error) {
	fake.identity = identity
	return fake.token, fake.err
}

type fakeOAuthLoginCodeStore struct {
	createdFor   int64
	createdCode  string
	createErr    error
	consumedCode string
	consumedUser models.User
	found        bool
	consumeErr   error
}

func (fake *fakeOAuthLoginCodeStore) Create(_ context.Context, userID int64) (string, error) {
	fake.createdFor = userID
	return fake.createdCode, fake.createErr
}

func (fake *fakeOAuthLoginCodeStore) Consume(_ context.Context, code string) (models.User, bool, error) {
	fake.consumedCode = code
	return fake.consumedUser, fake.found, fake.consumeErr
}

func TestAuthServiceCompleteGoogleLogin(t *testing.T) {
	t.Parallel()

	users := &fakeGoogleUserUpserter{user: models.User{ID: 7, Role: models.RoleAdmin}}
	tokens := &fakeTokenIssuer{token: "must-not-be-issued"}
	codes := &fakeOAuthLoginCodeStore{createdCode: "single-use-code"}
	service := NewAuthService(users, tokens, codes)

	code, err := service.CompleteGoogleLogin(context.Background(), auth.GoogleUser{ID: "google-user", Email: "admin@example.com"})
	if err != nil {
		t.Fatalf("CompleteGoogleLogin() error = %v", err)
	}
	if code != "single-use-code" || codes.createdFor != 7 {
		t.Fatalf("CompleteGoogleLogin() = %q, createdFor = %d", code, codes.createdFor)
	}
	if tokens.identity.UserID != 0 {
		t.Fatalf("CompleteGoogleLogin() issued an access token for %+v", tokens.identity)
	}
}

func TestAuthServiceExchangeOAuthLoginCode(t *testing.T) {
	t.Parallel()

	user := models.User{ID: 7, Email: "admin@example.com", Role: models.RoleAdmin}
	storeErr := errors.New("database unavailable")
	tests := map[string]struct {
		store     *fakeOAuthLoginCodeStore
		issuer    *fakeTokenIssuer
		wantError error
		wantToken string
	}{
		"issues token for consumed code": {
			store:     &fakeOAuthLoginCodeStore{consumedUser: user, found: true},
			issuer:    &fakeTokenIssuer{token: "jwt"},
			wantToken: "jwt",
		},
		"rejects missing or used code": {
			store:     &fakeOAuthLoginCodeStore{},
			issuer:    &fakeTokenIssuer{},
			wantError: ErrInvalidOAuthLoginCode,
		},
		"returns store failure": {
			store:     &fakeOAuthLoginCodeStore{consumeErr: storeErr},
			issuer:    &fakeTokenIssuer{},
			wantError: storeErr,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			service := NewAuthService(&fakeGoogleUserUpserter{}, test.issuer, test.store)

			response, err := service.ExchangeOAuthLoginCode(context.Background(), "single-use-code")
			if test.wantError != nil {
				if err == nil || !errors.Is(err, test.wantError) {
					t.Fatalf("ExchangeOAuthLoginCode() error = %v, want %v", err, test.wantError)
				}
				return
			}
			if err != nil {
				t.Fatalf("ExchangeOAuthLoginCode() error = %v", err)
			}
			if response.Token != test.wantToken || test.issuer.identity != (models.Identity{UserID: 7, Role: models.RoleAdmin}) {
				t.Fatalf("ExchangeOAuthLoginCode() response = %#v, identity = %#v", response, test.issuer.identity)
			}
		})
	}
}
