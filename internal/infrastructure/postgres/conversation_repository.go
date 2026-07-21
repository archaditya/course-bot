package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"course-assistant/internal/domain/entities"
	"course-assistant/internal/domain/repository"
)

type conversationRepository struct{ db *sql.DB }

func NewConversationRepository(db *sql.DB) repository.ConversationRepository {
	return &conversationRepository{db: db}
}

func (r *conversationRepository) Create(ctx context.Context, c *entities.Conversation) error {
	const q = `INSERT INTO conversations (id, project_id, title) VALUES ($1,$2,$3)`
	_, err := r.db.ExecContext(ctx, q, c.ID, c.ProjectID, c.Title)
	if err != nil {
		return fmt.Errorf("conversation: create: %w", err)
	}
	return nil
}

func (r *conversationRepository) GetByID(ctx context.Context, ws repository.WorkspaceID, id string) (*entities.Conversation, error) {
	const q = `
		SELECT c.id, c.project_id, c.title, c.created_at, c.updated_at
		FROM conversations c
		JOIN projects p ON p.id = c.project_id
		WHERE c.id = $1 AND p.workspace_id = $2`
	row := r.db.QueryRowContext(ctx, q, id, ws)
	var conv entities.Conversation
	err := row.Scan(&conv.ID, &conv.ProjectID, &conv.Title, &conv.CreatedAt, &conv.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("conversation: get: %w", err)
	}
	return &conv, nil
}

func (r *conversationRepository) ListByProject(ctx context.Context, ws repository.WorkspaceID, projectID, cursor string, limit int) ([]*entities.Conversation, string, error) {
	args := []interface{}{projectID, ws}
	q := `
		SELECT c.id, c.project_id, c.title, c.created_at, c.updated_at
		FROM conversations c
		JOIN projects p ON p.id = c.project_id
		WHERE c.project_id = $1 AND p.workspace_id = $2`
	if cursor != "" {
		q += ` AND c.id > $3`
		args = append(args, cursor)
	}
	q += fmt.Sprintf(` ORDER BY c.created_at DESC LIMIT %d`, limit+1)

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, "", fmt.Errorf("conversation: list: %w", err)
	}
	defer rows.Close()

	var convs []*entities.Conversation
	for rows.Next() {
		var c entities.Conversation
		if err := rows.Scan(&c.ID, &c.ProjectID, &c.Title, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, "", err
		}
		convs = append(convs, &c)
	}
	var next string
	if len(convs) > limit {
		next = convs[limit].ID
		convs = convs[:limit]
	}
	return convs, next, rows.Err()
}
