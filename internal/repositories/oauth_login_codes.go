package repositories

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/esc-chula/intania-shop-api/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// OAuthLoginCodeRepository persists only hashes of short-lived, single-use browser handoff codes.
type OAuthLoginCodeRepository struct {
	pool *pgxpool.Pool
}

// NewOAuthLoginCodeRepository constructs an OAuth login-code repository.
func NewOAuthLoginCodeRepository(pool *pgxpool.Pool) *OAuthLoginCodeRepository {
	return &OAuthLoginCodeRepository{pool: pool}
}

// Create creates a random login code for a user. The database receives only its SHA-256 hash.
func (repository *OAuthLoginCodeRepository) Create(ctx context.Context, userID int64) (string, error) {
	if userID <= 0 {
		return "", fmt.Errorf("user ID must be positive")
	}

	rawCode, err := randomOAuthLoginCode()
	if err != nil {
		return "", err
	}
	hash := oauthLoginCodeHash(rawCode)
	_, err = repository.pool.Exec(ctx, `
		WITH expired AS (
			DELETE FROM oauth_login_codes WHERE expires_at <= CURRENT_TIMESTAMP
		)
		INSERT INTO oauth_login_codes (code_hash, user_id, expires_at)
		VALUES ($1, $2, CURRENT_TIMESTAMP + INTERVAL '5 minutes')`, hash, userID)
	if err != nil {
		return "", fmt.Errorf("store OAuth login code: %w", err)
	}
	return rawCode, nil
}

// Consume atomically removes an unexpired login code and returns its user.
func (repository *OAuthLoginCodeRepository) Consume(ctx context.Context, rawCode string) (models.User, bool, error) {
	hash := oauthLoginCodeHash(rawCode)
	var user models.User
	var role string
	err := repository.pool.QueryRow(ctx, `
		WITH consumed AS (
			DELETE FROM oauth_login_codes
			WHERE code_hash = $1 AND expires_at > CURRENT_TIMESTAMP
			RETURNING user_id
		)
		SELECT users.user_id, COALESCE(users.full_name, ''), users.email, users.role::text, users.profile_picture
		FROM users
		JOIN consumed ON consumed.user_id = users.user_id`, hash).Scan(
		&user.ID, &user.FullName, &user.Email, &role, &user.ProfilePicture,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.User{}, false, nil
	}
	if err != nil {
		return models.User{}, false, fmt.Errorf("consume OAuth login code: %w", err)
	}
	user.Role = models.ParseRole(role)
	if !user.Role.Valid() {
		return models.User{}, false, fmt.Errorf("consume OAuth login code: invalid role %q", role)
	}
	return user, true, nil
}

func randomOAuthLoginCode() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate OAuth login code: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

func oauthLoginCodeHash(rawCode string) []byte {
	hash := sha256.Sum256([]byte(rawCode))
	return hash[:]
}
