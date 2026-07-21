package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"course-assistant/internal/domain/entities"
	"course-assistant/internal/domain/repository"
)

type messageRepository struct{ db *sql.DB }

func NewMessageRepository(db *sql.DB) repository.MessageRepository {
	return &messageRepository{db: db}
}

func (r *messageRepository) Create(ctx context.Context, m *entities.Message) error {
	const q = `INSERT INTO messages (id, conversation_id, role, content, status) VALUES ($1,$2,$3,$4,$5)`
	_, err := r.db.ExecContext(ctx, q, m.ID, m.ConversationID, string(m.Role), m.Content, string(m.Status))
	if err != nil {
		return fmt.Errorf("message: create: %w", err)
	}
	return nil
}

func (r *messageRepository) ListByConversation(ctx context.Context, conversationID, cursor string, limit int) ([]*entities.Message, string, error) {
	args := []interface{}{conversationID}
	q := `SELECT id, conversation_id, role, content, status, created_at FROM messages WHERE conversation_id = $1`
	if cursor != "" {
		q += ` AND id > $2`
		args = append(args, cursor)
	}
	q += fmt.Sprintf(` ORDER BY created_at LIMIT %d`, limit+1)

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, "", fmt.Errorf("message: list: %w", err)
	}
	defer rows.Close()

	var msgs []*entities.Message
	for rows.Next() {
		var m entities.Message
		var role, status string
		if err := rows.Scan(&m.ID, &m.ConversationID, &role, &m.Content, &status, &m.CreatedAt); err != nil {
			return nil, "", err
		}
		m.Role = entities.MessageRole(role)
		m.Status = entities.MessageStatus(status)
		msgs = append(msgs, &m)
	}
	var next string
	if len(msgs) > limit {
		next = msgs[limit].ID
		msgs = msgs[:limit]
	}
	return msgs, next, rows.Err()
}

func (r *messageRepository) UpdateStatus(ctx context.Context, id string, status entities.MessageStatus) error {
	const q = `UPDATE messages SET status=$1 WHERE id=$2`
	_, err := r.db.ExecContext(ctx, q, string(status), id)
	if err != nil {
		return fmt.Errorf("message: update status: %w", err)
	}
	return nil
}
