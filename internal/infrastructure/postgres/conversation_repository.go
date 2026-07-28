package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"archadilm/internal/domain/entities"
	"archadilm/internal/domain/repository"
)

type conversationRepository struct{ db *sql.DB }

func NewConversationRepository(db *sql.DB) repository.ConversationRepository {
	return &conversationRepository{db: db}
}

func (r *conversationRepository) Create(ctx context.Context, c *entities.Conversation) error {
	const q = `INSERT INTO conversations (id, workspace_id, title) VALUES ($1,$2,$3)`
	_, err := r.db.ExecContext(ctx, q, c.ID, c.WorkspaceID, c.Title)
	if err != nil {
		return fmt.Errorf("conversation: create: %w", err)
	}
	return nil
}

func (r *conversationRepository) GetByID(ctx context.Context, ws repository.WorkspaceID, id string) (*entities.Conversation, error) {
	const q = `
		SELECT id, workspace_id, title, created_at, updated_at
		FROM conversations
		WHERE id = $1 AND workspace_id = $2`
	row := r.db.QueryRowContext(ctx, q, id, ws)
	var conv entities.Conversation
	err := row.Scan(&conv.ID, &conv.WorkspaceID, &conv.Title, &conv.CreatedAt, &conv.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("conversation: get: %w", err)
	}
	return &conv, nil
}

func (r *conversationRepository) ListByWorkspace(ctx context.Context, ws repository.WorkspaceID, cursor string, limit int) ([]*entities.Conversation, string, error) {
	limit = normalizeLimit(limit)
	args := []interface{}{ws}
	q := `
		SELECT id, workspace_id, title, created_at, updated_at
		FROM conversations
		WHERE workspace_id = $1`
	if cursor != "" {
		q += ` AND id > $2`
		args = append(args, cursor)
	}
	q += fmt.Sprintf(` ORDER BY updated_at DESC LIMIT %d`, limit+1)

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, "", fmt.Errorf("conversation: list: %w", err)
	}
	defer rows.Close()

	var convs []*entities.Conversation
	for rows.Next() {
		var c entities.Conversation
		if err := rows.Scan(&c.ID, &c.WorkspaceID, &c.Title, &c.CreatedAt, &c.UpdatedAt); err != nil {
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

func (r *conversationRepository) UpdateTitle(ctx context.Context, ws repository.WorkspaceID, id, title string) error {
	const q = `UPDATE conversations SET title = $1, updated_at = NOW() WHERE id = $2 AND workspace_id = $3`
	res, err := r.db.ExecContext(ctx, q, title, id, ws)
	if err != nil {
		return fmt.Errorf("conversation: update title: %w", err)
	}
	return checkRowsAffected(res)
}

func (r *conversationRepository) Delete(ctx context.Context, ws repository.WorkspaceID, id string) error {
	const q = `DELETE FROM conversations WHERE id = $1 AND workspace_id = $2`
	res, err := r.db.ExecContext(ctx, q, id, ws)
	if err != nil {
		return fmt.Errorf("conversation: delete: %w", err)
	}
	return checkRowsAffected(res)
}
