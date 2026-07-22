package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"archadilm/internal/domain/entities"
)

type ProjectRepository struct{ db *sql.DB }

func NewProjectRepository(db *sql.DB) *ProjectRepository { return &ProjectRepository{db: db} }

func (r *ProjectRepository) Create(ctx context.Context, p *entities.Project) error {
	const q = `
		INSERT INTO projects (workspace_id, name)
		VALUES ($1, $2)
		RETURNING id, created_at, updated_at`
	err := r.db.QueryRowContext(ctx, q, p.WorkspaceID, p.Name).Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return fmt.Errorf("postgres: create project: %w", err)
	}
	return nil
}

// GetByID is scoped by workspace so a project belonging to another
// workspace is indistinguishable from a nonexistent one — see
// docs/08-security.md#workspace-isolation.
func (r *ProjectRepository) GetByID(ctx context.Context, ws string, id string) (*entities.Project, error) {
	const q = `SELECT id, workspace_id, name, created_at, updated_at
		FROM projects WHERE id = $1 AND workspace_id = $2`
	var p entities.Project
	err := r.db.QueryRowContext(ctx, q, id, ws).Scan(&p.ID, &p.WorkspaceID, &p.Name, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("postgres: get project: %w", err)
	}
	return &p, nil
}

func (r *ProjectRepository) ListByWorkspace(ctx context.Context, ws string, cursor string, limit int) ([]*entities.Project, string, error) {
	limit = normalizeLimit(limit)
	after, afterID, hasCursor, err := decodeCursor(cursor)
	if err != nil {
		return nil, "", err
	}

	// Two query shapes rather than one with an embedded NULL check: binding
	// an empty string as a $N::uuid parameter fails Postgres's type check
	// before the query even runs, regardless of whether a WHERE branch
	// would skip it at execution time — so the first page (no cursor yet)
	// must be a genuinely different query, not just an unmatched predicate.
	var rows *sql.Rows
	if hasCursor {
		const q = `
			SELECT id, workspace_id, name, created_at, updated_at
			FROM projects
			WHERE workspace_id = $1 AND (created_at, id) > ($2, $3)
			ORDER BY created_at ASC, id ASC
			LIMIT $4`
		rows, err = r.db.QueryContext(ctx, q, ws, after, afterID, limit)
	} else {
		const q = `
			SELECT id, workspace_id, name, created_at, updated_at
			FROM projects
			WHERE workspace_id = $1
			ORDER BY created_at ASC, id ASC
			LIMIT $2`
		rows, err = r.db.QueryContext(ctx, q, ws, limit)
	}
	if err != nil {
		return nil, "", fmt.Errorf("postgres: list projects: %w", err)
	}
	defer rows.Close()

	var out []*entities.Project
	for rows.Next() {
		var p entities.Project
		if err := rows.Scan(&p.ID, &p.WorkspaceID, &p.Name, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, "", fmt.Errorf("postgres: scan project: %w", err)
		}
		out = append(out, &p)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}

	var nextCursor string
	if len(out) == limit {
		last := out[len(out)-1]
		nextCursor = encodeCursor(last.CreatedAt, last.ID)
	}
	return out, nextCursor, nil
}

func (r *ProjectRepository) Update(ctx context.Context, ws string, p *entities.Project) error {
	const q = `UPDATE projects SET name = $1, updated_at = now() WHERE id = $2 AND workspace_id = $3`
	res, err := r.db.ExecContext(ctx, q, p.Name, p.ID, ws)
	if err != nil {
		return fmt.Errorf("postgres: update project: %w", err)
	}
	return checkRowsAffected(res)
}

func (r *ProjectRepository) Delete(ctx context.Context, ws string, id string) error {
	const q = `DELETE FROM projects WHERE id = $1 AND workspace_id = $2`
	res, err := r.db.ExecContext(ctx, q, id, ws)
	if err != nil {
		return fmt.Errorf("postgres: delete project: %w", err)
	}
	return checkRowsAffected(res)
}
