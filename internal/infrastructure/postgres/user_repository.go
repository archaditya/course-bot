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

func NewUserRepository(db *sql.DB) repository.UserRepository { return &UserRepository{db: db} }

func (r *UserRepository) Create(ctx context.Context, u *entities.User) error {
	role := u.Role
	if role == "" {
		role = entities.UserRoleUser
	}
	// Check if ID is pre-generated (e.g. by seed script) or let DB generate it
	if u.ID != "" {
		const q = `INSERT INTO users (id, full_name, email, password_hash, auth_provider, role, is_disabled)
			VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING created_at, updated_at`
		if err := r.db.QueryRowContext(ctx, q, u.ID, u.FullName, u.Email, nullableString(u.PasswordHash), u.AuthProvider, string(role), u.IsDisabled).
			Scan(&u.CreatedAt, &u.UpdatedAt); err != nil {
			return fmt.Errorf("postgres: create user: %w", err)
		}
		return nil
	}

	const q = `INSERT INTO users (full_name, email, password_hash, auth_provider, role, is_disabled)
		VALUES ($1, $2, $3, $4, $5, $6) RETURNING id, created_at, updated_at`
	if err := r.db.QueryRowContext(ctx, q, u.FullName, u.Email, nullableString(u.PasswordHash), u.AuthProvider, string(role), u.IsDisabled).
		Scan(&u.ID, &u.CreatedAt, &u.UpdatedAt); err != nil {
		return fmt.Errorf("postgres: create user: %w", err)
	}
	return nil
}

const userSelect = `SELECT id, full_name, email, COALESCE(password_hash, ''), auth_provider, role, is_disabled, created_at, updated_at FROM users`

func (r *UserRepository) GetByID(ctx context.Context, id string) (*entities.User, error) {
	return scanUser(r.db.QueryRowContext(ctx, userSelect+` WHERE id = $1`, id))
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*entities.User, error) {
	return scanUser(r.db.QueryRowContext(ctx, userSelect+` WHERE email = $1`, email))
}

func (r *UserRepository) Update(ctx context.Context, u *entities.User) error {
	const q = `UPDATE users SET full_name = $1, email = $2, password_hash = $3, auth_provider = $4, role = $5, is_disabled = $6, updated_at = now() WHERE id = $7`
	res, err := r.db.ExecContext(ctx, q, u.FullName, u.Email, nullableString(u.PasswordHash), u.AuthProvider, string(u.Role), u.IsDisabled, u.ID)
	if err != nil {
		return fmt.Errorf("postgres: update user: %w", err)
	}
	return checkRowsAffected(res)
}

func (r *UserRepository) UpdateRole(ctx context.Context, userID string, role entities.UserRole) error {
	const q = `UPDATE users SET role = $1, updated_at = now() WHERE id = $2`
	res, err := r.db.ExecContext(ctx, q, string(role), userID)
	if err != nil {
		return fmt.Errorf("postgres: update user role: %w", err)
	}
	return checkRowsAffected(res)
}

func (r *UserRepository) UpdateStatus(ctx context.Context, userID string, isDisabled bool) error {
	const q = `UPDATE users SET is_disabled = $1, updated_at = now() WHERE id = $2`
	res, err := r.db.ExecContext(ctx, q, isDisabled, userID)
	if err != nil {
		return fmt.Errorf("postgres: update user status: %w", err)
	}
	return checkRowsAffected(res)
}

func (r *UserRepository) ListUsersWithUsageStats(ctx context.Context) ([]*entities.UserUsageStat, error) {
	const q = `
		SELECT u.id, u.email, u.full_name, u.role, u.is_disabled, u.created_at,
		       COALESCE((SELECT COUNT(*) FROM conversations c JOIN workspaces w ON w.id = c.workspace_id WHERE w.user_id = u.id), 0) AS conversation_count,
		       COALESCE((SELECT COUNT(*) FROM documents d JOIN conversations c ON c.id = d.conversation_id JOIN workspaces w ON w.id = c.workspace_id WHERE w.user_id = u.id), 0) AS document_count,
		       COALESCE((SELECT COUNT(*) FROM messages m JOIN conversations c ON c.id = m.conversation_id JOIN workspaces w ON w.id = c.workspace_id WHERE w.user_id = u.id), 0) AS message_count,
		       COALESCE((SELECT COUNT(*) FROM chunks ch JOIN conversations c ON c.id = ch.conversation_id JOIN workspaces w ON w.id = c.workspace_id WHERE w.user_id = u.id), 0) AS chunk_count
		FROM users u
		ORDER BY u.created_at DESC`

	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("postgres: list user usage stats: %w", err)
	}
	defer rows.Close()

	var stats []*entities.UserUsageStat
	for rows.Next() {
		var s entities.UserUsageStat
		var role string
		if err := rows.Scan(
			&s.ID, &s.Email, &s.FullName, &role, &s.IsDisabled, &s.CreatedAt,
			&s.ConversationCount, &s.DocumentCount, &s.MessageCount, &s.ChunkCount,
		); err != nil {
			return nil, fmt.Errorf("postgres: scan user usage stat: %w", err)
		}
		s.Role = entities.UserRole(role)
		stats = append(stats, &s)
	}
	return stats, rows.Err()
}

func (r *UserRepository) GetSystemStats(ctx context.Context) (*entities.SystemStats, error) {
	const q = `
		SELECT
			(SELECT COUNT(*) FROM users) AS total_users,
			(SELECT COUNT(*) FROM users WHERE is_disabled = false) AS active_users,
			(SELECT COUNT(*) FROM users WHERE is_disabled = true) AS restricted_users,
			(SELECT COUNT(*) FROM conversations) AS total_conversations,
			(SELECT COUNT(*) FROM documents) AS total_documents,
			(SELECT COUNT(*) FROM messages) AS total_messages,
			(SELECT COUNT(*) FROM chunks) AS total_chunks`

	var s entities.SystemStats
	if err := r.db.QueryRowContext(ctx, q).Scan(
		&s.TotalUsers, &s.ActiveUsers, &s.RestrictedUsers,
		&s.TotalConversations, &s.TotalDocuments, &s.TotalMessages, &s.TotalChunks,
	); err != nil {
		return nil, fmt.Errorf("postgres: get system stats: %w", err)
	}
	return &s, nil
}

type rowScanner interface{ Scan(dest ...any) error }

func scanUser(row rowScanner) (*entities.User, error) {
	var u entities.User
	var role string
	if err := row.Scan(&u.ID, &u.FullName, &u.Email, &u.PasswordHash, &u.AuthProvider, &role, &u.IsDisabled, &u.CreatedAt, &u.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("postgres: scan user: %w", err)
	}
	u.Role = entities.UserRole(role)
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
