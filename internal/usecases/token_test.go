package usecases

import (
	"testing"
	"time"

	"github.com/esc-chula/intania-shop-api/internal/models"
	"github.com/golang-jwt/jwt/v5"
)

const testJWTSecret = "12345678901234567890123456789012"

func TestTokenManagerVerify(t *testing.T) {
	t.Parallel()

	manager, err := NewTokenManager(testJWTSecret, "intania-shop-api", time.Hour)
	if err != nil {
		t.Fatalf("NewTokenManager() error = %v", err)
	}
	now := time.Date(2026, 8, 8, 0, 0, 0, 0, time.UTC)
	manager.now = func() time.Time { return now }
	validToken, err := manager.Issue(models.Identity{UserID: 42, Role: models.RoleUser})
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	wrongAlgorithm, err := jwt.NewWithClaims(jwt.SigningMethodHS512, jwt.MapClaims{
		"iss": "intania-shop-api", "sub": "42", "role": "USER", "exp": now.Add(time.Hour).Unix(),
	}).SignedString([]byte(testJWTSecret))
	if err != nil {
		t.Fatalf("sign wrong algorithm token: %v", err)
	}

	tests := map[string]struct {
		token   string
		wantErr bool
	}{
		"valid":           {token: validToken},
		"tampered":        {token: validToken + "x", wantErr: true},
		"wrong algorithm": {token: wrongAlgorithm, wantErr: true},
		"empty":           {token: "", wantErr: true},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			identity, err := manager.Verify(test.token)
			if test.wantErr {
				if err == nil {
					t.Fatal("Verify() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("Verify() error = %v", err)
			}
			if identity != (models.Identity{UserID: 42, Role: models.RoleUser}) {
				t.Fatalf("Verify() identity = %#v", identity)
			}
		})
	}
}
