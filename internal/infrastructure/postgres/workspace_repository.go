package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"course-assistant/internal/domain/entities"
)

type WorkspaceRepository struct{ db *sql.DB }

func NewWorkspaceRepository(db *sql.DB) *WorkspaceRepository { return &WorkspaceRepository{db: db} }

func (r *WorkspaceRepository) Create(ctx context.Context, w *entities.Workspace) error {
	const q = `
		INSERT INTO workspaces (user_id, name)
		VALUES ($1, $2)
		RETURNING id, created_at, updated_at`
	err := r.db.QueryRowContext(ctx, q, w.UserID, w.Name).Scan(&w.ID, &w.CreatedAt, &w.UpdatedAt)
	if err != nil {
		return fmt.Errorf("postgres: create workspace: %w", err)
	}
	return nil
}

func (r *WorkspaceRepository) GetByID(ctx context.Context, id string) (*entities.Workspace, error) {
	const q = `SELECT id, user_id, name, created_at, updated_at FROM workspaces WHERE id = $1`
	return scanWorkspace(r.db.QueryRowContext(ctx, q, id))
}

func (r *WorkspaceRepository) GetByUserID(ctx context.Context, userID string) (*entities.Workspace, error) {
	const q = `SELECT id, user_id, name, created_at, updated_at FROM workspaces WHERE user_id = $1`
	return scanWorkspace(r.db.QueryRowContext(ctx, q, userID))
}

func scanWorkspace(row rowScanner) (*entities.Workspace, error) {
	var w entities.Workspace
	err := row.Scan(&w.ID, &w.UserID, &w.Name, &w.CreatedAt, &w.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("postgres: scan workspace: %w", err)
	}
	return &w, nil
}
