package usecases

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/esc-chula/intania-shop-api/internal/models"
	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrInvalidToken    = errors.New("invalid token")
	ErrInvalidIdentity = errors.New("invalid identity")
)

// TokenManager issues and validates short-lived HS256 access tokens.
type TokenManager struct {
	secret []byte
	issuer string
	ttl    time.Duration
	now    func() time.Time
}

type tokenClaims struct {
	Role models.Role `json:"role"`
	jwt.RegisteredClaims
}

// NewTokenManager constructs a token manager using a shared secret.
func NewTokenManager(secret, issuer string, ttl time.Duration) (*TokenManager, error) {
	if len(secret) < 32 {
		return nil, fmt.Errorf("JWT secret must be at least 32 bytes")
	}
	if strings.TrimSpace(issuer) == "" {
		return nil, fmt.Errorf("JWT issuer is required")
	}
	if ttl <= 0 {
		return nil, fmt.Errorf("JWT TTL must be positive")
	}

	return &TokenManager{secret: []byte(secret), issuer: issuer, ttl: ttl, now: time.Now}, nil
}

// Issue creates a signed access token for identity.
func (manager *TokenManager) Issue(identity models.Identity) (string, error) {
	if identity.UserID <= 0 || !identity.Role.Valid() {
		return "", ErrInvalidIdentity
	}

	now := manager.now().UTC()
	claims := tokenClaims{
		Role: identity.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    manager.issuer,
			Subject:   strconv.FormatInt(identity.UserID, 10),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(manager.ttl)),
		},
	}

	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(manager.secret)
}

// Verify validates an access token and returns its authenticated identity.
func (manager *TokenManager) Verify(rawToken string) (models.Identity, error) {
	claims := new(tokenClaims)
	token, err := jwt.ParseWithClaims(
		rawToken,
		claims,
		func(token *jwt.Token) (any, error) {
			if token.Method != jwt.SigningMethodHS256 {
				return nil, ErrInvalidToken
			}
			return manager.secret, nil
		},
		jwt.WithIssuer(manager.issuer),
		jwt.WithExpirationRequired(),
		jwt.WithTimeFunc(manager.now),
	)
	if err != nil || !token.Valid {
		return models.Identity{}, ErrInvalidToken
	}

	userID, err := strconv.ParseInt(claims.Subject, 10, 64)
	if err != nil || userID <= 0 || !claims.Role.Valid() {
		return models.Identity{}, ErrInvalidToken
	}
	return models.Identity{UserID: userID, Role: claims.Role}, nil
}
