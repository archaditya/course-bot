package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"course-assistant/internal/domain/entities"
)

type RefreshTokenRepository struct{ db *sql.DB }

func NewRefreshTokenRepository(db *sql.DB) *RefreshTokenRepository {
	return &RefreshTokenRepository{db: db}
}

func (r *RefreshTokenRepository) Create(ctx context.Context, t *entities.RefreshToken) error {
	const q = `
		INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
		RETURNING id, created_at`
	err := r.db.QueryRowContext(ctx, q, t.UserID, t.TokenHash, t.ExpiresAt).Scan(&t.ID, &t.CreatedAt)
	if err != nil {
		return fmt.Errorf("postgres: create refresh token: %w", err)
	}
	return nil
}

func (r *RefreshTokenRepository) GetByHash(ctx context.Context, tokenHash string) (*entities.RefreshToken, error) {
	const q = `SELECT id, user_id, token_hash, expires_at, revoked_at, created_at
		FROM refresh_tokens WHERE token_hash = $1`
	var t entities.RefreshToken
	err := r.db.QueryRowContext(ctx, q, tokenHash).
		Scan(&t.ID, &t.UserID, &t.TokenHash, &t.ExpiresAt, &t.RevokedAt, &t.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("postgres: get refresh token: %w", err)
	}
	return &t, nil
}

// Revoke marks a single refresh token used, per the rotate-on-every-use
// policy in docs/08-security.md#jwt-rotation.
func (r *RefreshTokenRepository) Revoke(ctx context.Context, id string) error {
	const q = `UPDATE refresh_tokens SET revoked_at = now() WHERE id = $1 AND revoked_at IS NULL`
	res, err := r.db.ExecContext(ctx, q, id)
	if err != nil {
		return fmt.Errorf("postgres: revoke refresh token: %w", err)
	}
	return checkRowsAffected(res)
}

// RevokeAllForUser is used on password change / suspected compromise to cut
// off every outstanding refresh token at once.
func (r *RefreshTokenRepository) RevokeAllForUser(ctx context.Context, userID string) error {
	const q = `UPDATE refresh_tokens SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL`
	_, err := r.db.ExecContext(ctx, q, userID)
	if err != nil {
		return fmt.Errorf("postgres: revoke all refresh tokens: %w", err)
	}
	return nil
}
