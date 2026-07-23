package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"archadilm/internal/domain/entities"
	"archadilm/internal/domain/repository"
)

var ErrNotFound = repository.ErrNotFound

type UserRepository struct{ db *sql.DB }

func NewUserRepository(db *sql.DB) *UserRepository { return &UserRepository{db: db} }

func (r *UserRepository) Create(ctx context.Context, u *entities.User) error {
	const q = `INSERT INTO users (full_name, email, password_hash, auth_provider)
		VALUES ($1, $2, $3, $4) RETURNING id, created_at, updated_at`
	if err := r.db.QueryRowContext(ctx, q, u.FullName, u.Email, nullableString(u.PasswordHash), u.AuthProvider).
		Scan(&u.ID, &u.CreatedAt, &u.UpdatedAt); err != nil {
		return fmt.Errorf("postgres: create user: %w", err)
	}
	return nil
}

func (r *UserRepository) GetByID(ctx context.Context, id string) (*entities.User, error) {
	return scanUser(r.db.QueryRowContext(ctx, `SELECT id, full_name, email, COALESCE(password_hash, ''), auth_provider, created_at, updated_at FROM users WHERE id = $1`, id))
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*entities.User, error) {
	return scanUser(r.db.QueryRowContext(ctx, `SELECT id, full_name, email, COALESCE(password_hash, ''), auth_provider, created_at, updated_at FROM users WHERE email = $1`, email))
}

func (r *UserRepository) Update(ctx context.Context, u *entities.User) error {
	const q = `UPDATE users SET full_name = $1, email = $2, password_hash = $3, auth_provider = $4, updated_at = now() WHERE id = $5`
	res, err := r.db.ExecContext(ctx, q, u.FullName, u.Email, nullableString(u.PasswordHash), u.AuthProvider, u.ID)
	if err != nil {
		return fmt.Errorf("postgres: update user: %w", err)
	}
	return checkRowsAffected(res)
}

type rowScanner interface{ Scan(dest ...any) error }

func scanUser(row rowScanner) (*entities.User, error) {
	var u entities.User
	if err := row.Scan(&u.ID, &u.FullName, &u.Email, &u.PasswordHash, &u.AuthProvider, &u.CreatedAt, &u.UpdatedAt); err != nil {
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
