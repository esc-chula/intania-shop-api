package repositories

import (
	"context"
	"fmt"

	"github.com/esc-chula/intania-shop-api/internal/auth"
	"github.com/esc-chula/intania-shop-api/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// UserRepository persists OAuth-backed user accounts.
type UserRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

// UpsertGoogleUser finds an existing Google identity, links a matching email, or creates a user account.
func (repository *UserRepository) UpsertGoogleUser(ctx context.Context, googleUser auth.GoogleUser) (models.User, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return models.User{}, fmt.Errorf("begin Google user upsert: %w", err)
	}
	defer rollback(ctx, tx)

	user, found, err := scanUser(ctx, tx, `
		SELECT user_id, COALESCE(full_name, ''), email, role::text, profile_picture
		FROM users WHERE google_id = $1 FOR UPDATE`, googleUser.ID)
	if err != nil {
		return models.User{}, err
	}
	if found {
		if err := tx.Commit(ctx); err != nil {
			return models.User{}, fmt.Errorf("commit Google user lookup: %w", err)
		}
		return user, nil
	}

	user, found, err = scanUser(ctx, tx, `
		SELECT user_id, COALESCE(full_name, ''), email, role::text, profile_picture
		FROM users WHERE email = $1 FOR UPDATE`, googleUser.Email)
	if err != nil {
		return models.User{}, err
	}
	if found {
		user, _, err = scanUser(ctx, tx, `
			UPDATE users SET google_id = $1, profile_picture = $2
			WHERE user_id = $3
			RETURNING user_id, COALESCE(full_name, ''), email, role::text, profile_picture`, googleUser.ID, googleUser.Picture, user.ID)
		if err != nil {
			return models.User{}, err
		}
	} else {
		user, _, err = scanUser(ctx, tx, `
			INSERT INTO users (full_name, email, role, google_id, profile_picture)
			VALUES ($1, $2, 'USER', $3, $4)
			RETURNING user_id, COALESCE(full_name, ''), email, role::text, profile_picture`, googleUser.Name, googleUser.Email, googleUser.ID, googleUser.Picture)
		if err != nil {
			return models.User{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return models.User{}, fmt.Errorf("commit Google user upsert: %w", err)
	}
	return user, nil
}

type rowQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func scanUser(ctx context.Context, querier rowQuerier, query string, arguments ...any) (models.User, bool, error) {
	var user models.User
	var role string
	err := querier.QueryRow(ctx, query, arguments...).Scan(&user.ID, &user.FullName, &user.Email, &role, &user.ProfilePicture)
	if err == pgx.ErrNoRows {
		return models.User{}, false, nil
	}
	if err != nil {
		return models.User{}, false, fmt.Errorf("query user: %w", err)
	}
	user.Role = models.ParseRole(role)
	if !user.Role.Valid() {
		return models.User{}, false, fmt.Errorf("query user: invalid role %q", role)
	}
	return user, true, nil
}
