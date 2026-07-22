package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"archadilm/internal/domain/entities"
)

type CourseRepository struct{ db *sql.DB }

func NewCourseRepository(db *sql.DB) *CourseRepository { return &CourseRepository{db: db} }

// Create verifies the target project belongs to the given workspace before
// inserting, so a caller can't create a course under a project it doesn't
// own by guessing a project_id — see docs/08-security.md#workspace-isolation.
func (r *CourseRepository) Create(ctx context.Context, ws string, c *entities.Course) error {
	const q = `
		INSERT INTO courses (project_id, title, status)
		SELECT $1, $2, $3
		WHERE EXISTS (SELECT 1 FROM projects WHERE id = $1 AND workspace_id = $4)
		RETURNING id, created_at, updated_at`
	err := r.db.QueryRowContext(ctx, q, c.ProjectID, c.Title, c.Status, ws).
		Scan(&c.ID, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound // project doesn't exist or isn't in this workspace
		}
		return fmt.Errorf("postgres: create course: %w", err)
	}
	return nil
}

func (r *CourseRepository) GetByID(ctx context.Context, ws string, id string) (*entities.Course, error) {
	const q = `
		SELECT c.id, c.project_id, c.title, c.status, c.created_at, c.updated_at
		FROM courses c
		JOIN projects p ON p.id = c.project_id
		WHERE c.id = $1 AND p.workspace_id = $2`
	var c entities.Course
	err := r.db.QueryRowContext(ctx, q, id, ws).
		Scan(&c.ID, &c.ProjectID, &c.Title, &c.Status, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("postgres: get course: %w", err)
	}
	return &c, nil
}

func (r *CourseRepository) ListByProject(ctx context.Context, ws string, projectID string, cursor string, limit int) ([]*entities.Course, string, error) {
	limit = normalizeLimit(limit)
	after, afterID, hasCursor, err := decodeCursor(cursor)
	if err != nil {
		return nil, "", err
	}

	// See the comment in project_repository.go's ListByWorkspace: an empty
	// string can't bind as a $N::uuid parameter, so the no-cursor (first
	// page) case is a separate query rather than a NULL-guarded predicate.
	var rows *sql.Rows
	if hasCursor {
		const q = `
			SELECT c.id, c.project_id, c.title, c.status, c.created_at, c.updated_at
			FROM courses c
			JOIN projects p ON p.id = c.project_id
			WHERE c.project_id = $1 AND p.workspace_id = $2 AND (c.created_at, c.id) > ($3, $4)
			ORDER BY c.created_at ASC, c.id ASC
			LIMIT $5`
		rows, err = r.db.QueryContext(ctx, q, projectID, ws, after, afterID, limit)
	} else {
		const q = `
			SELECT c.id, c.project_id, c.title, c.status, c.created_at, c.updated_at
			FROM courses c
			JOIN projects p ON p.id = c.project_id
			WHERE c.project_id = $1 AND p.workspace_id = $2
			ORDER BY c.created_at ASC, c.id ASC
			LIMIT $3`
		rows, err = r.db.QueryContext(ctx, q, projectID, ws, limit)
	}
	if err != nil {
		return nil, "", fmt.Errorf("postgres: list courses: %w", err)
	}
	defer rows.Close()

	var out []*entities.Course
	for rows.Next() {
		var c entities.Course
		if err := rows.Scan(&c.ID, &c.ProjectID, &c.Title, &c.Status, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, "", fmt.Errorf("postgres: scan course: %w", err)
		}
		out = append(out, &c)
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

// UpdateStatus is the narrow write path used by the status-updater consumer
// (docs/04-indexing-pipeline.md#event-contracts) as INDEXED/FAILED events
// arrive — separate from the general Update method since pipeline status
// transitions don't go through the same call path as a user renaming a
// course.
func (r *CourseRepository) UpdateStatus(ctx context.Context, ws string, id string, status entities.CourseStatus) error {
	const q = `
		UPDATE courses c SET status = $1, updated_at = now()
		FROM projects p
		WHERE c.id = $2 AND c.project_id = p.id AND p.workspace_id = $3`
	res, err := r.db.ExecContext(ctx, q, status, id, ws)
	if err != nil {
		return fmt.Errorf("postgres: update course status: %w", err)
	}
	return checkRowsAffected(res)
}

func (r *CourseRepository) Update(ctx context.Context, ws string, c *entities.Course) error {
	const q = `
		UPDATE courses cr SET title = $1, updated_at = now()
		FROM projects p
		WHERE cr.id = $2 AND cr.project_id = p.id AND p.workspace_id = $3`
	res, err := r.db.ExecContext(ctx, q, c.Title, c.ID, ws)
	if err != nil {
		return fmt.Errorf("postgres: update course: %w", err)
	}
	return checkRowsAffected(res)
}

func (r *CourseRepository) Delete(ctx context.Context, ws string, id string) error {
	const q = `
		DELETE FROM courses cr
		USING projects p
		WHERE cr.id = $1 AND cr.project_id = p.id AND p.workspace_id = $2`
	res, err := r.db.ExecContext(ctx, q, id, ws)
	if err != nil {
		return fmt.Errorf("postgres: delete course: %w", err)
	}
	return checkRowsAffected(res)
}
