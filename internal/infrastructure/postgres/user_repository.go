package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"course-assistant/internal/domain/entities"
	"course-assistant/internal/domain/repository"
)

// ErrNotFound re-exports the domain-level sentinel (defined in
// internal/domain/repository so application/ never has to import this
// infrastructure package directly) for convenience within this package's
// own files.
var ErrNotFound = repository.ErrNotFound

type UserRepository struct{ db *sql.DB }

func NewUserRepository(db *sql.DB) *UserRepository { return &UserRepository{db: db} }

func (r *UserRepository) Create(ctx context.Context, u *entities.User) error {
	const q = `
		INSERT INTO users (email, password_hash, auth_provider)
		VALUES ($1, $2, $3)
		RETURNING id, created_at, updated_at`
	err := r.db.QueryRowContext(ctx, q, u.Email, nullableString(u.PasswordHash), u.AuthProvider).
		Scan(&u.ID, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return fmt.Errorf("postgres: create user: %w", err)
	}
	return nil
}

func (r *UserRepository) GetByID(ctx context.Context, id string) (*entities.User, error) {
	const q = `SELECT id, email, COALESCE(password_hash, ''), auth_provider, created_at, updated_at
		FROM users WHERE id = $1`
	return scanUser(r.db.QueryRowContext(ctx, q, id))
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*entities.User, error) {
	const q = `SELECT id, email, COALESCE(password_hash, ''), auth_provider, created_at, updated_at
		FROM users WHERE email = $1`
	return scanUser(r.db.QueryRowContext(ctx, q, email))
}

func (r *UserRepository) Update(ctx context.Context, u *entities.User) error {
	const q = `UPDATE users SET email = $1, password_hash = $2, auth_provider = $3, updated_at = now()
		WHERE id = $4`
	res, err := r.db.ExecContext(ctx, q, u.Email, nullableString(u.PasswordHash), u.AuthProvider, u.ID)
	if err != nil {
		return fmt.Errorf("postgres: update user: %w", err)
	}
	return checkRowsAffected(res)
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanUser(row rowScanner) (*entities.User, error) {
	var u entities.User
	err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.AuthProvider, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("postgres: scan user: %w", err)
	}
	return &u, nil
}

func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func checkRowsAffected(res sql.Result) error {
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("postgres: rows affected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
