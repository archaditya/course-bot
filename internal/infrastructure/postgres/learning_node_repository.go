package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"archadilm/internal/domain/entities"
	"archadilm/internal/domain/repository"
)

type LearningNodeRepository struct {
	db *sql.DB
}

func NewLearningNodeRepository(db *sql.DB) *LearningNodeRepository {
	return &LearningNodeRepository{db: db}
}

func (r *LearningNodeRepository) Create(ctx context.Context, node *entities.LearningNode) error {
	now := time.Now()
	node.CreatedAt = now
	node.UpdatedAt = now
	if node.Content == nil {
		node.Content = json.RawMessage("{}")
	}

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO learning_nodes (id, conversation_id, tool_type, title, content, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		node.ID, node.ConversationID, node.ToolType, node.Title, node.Content, node.Status, node.CreatedAt, node.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("learning_node_repo: create: %w", err)
	}
	return nil
}

func (r *LearningNodeRepository) ListByConversation(ctx context.Context, conversationID string) ([]*entities.LearningNode, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, conversation_id, tool_type, title, content, status, created_at, updated_at
		 FROM learning_nodes
		 WHERE conversation_id = $1
		 ORDER BY created_at DESC`,
		conversationID,
	)
	if err != nil {
		return nil, fmt.Errorf("learning_node_repo: list: %w", err)
	}
	defer rows.Close()

	var nodes []*entities.LearningNode
	for rows.Next() {
		n := &entities.LearningNode{}
		if err := rows.Scan(&n.ID, &n.ConversationID, &n.ToolType, &n.Title, &n.Content, &n.Status, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, fmt.Errorf("learning_node_repo: scan: %w", err)
		}
		nodes = append(nodes, n)
	}
	return nodes, rows.Err()
}

func (r *LearningNodeRepository) GetByID(ctx context.Context, id string) (*entities.LearningNode, error) {
	n := &entities.LearningNode{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id, conversation_id, tool_type, title, content, status, created_at, updated_at
		 FROM learning_nodes WHERE id = $1`, id,
	).Scan(&n.ID, &n.ConversationID, &n.ToolType, &n.Title, &n.Content, &n.Status, &n.CreatedAt, &n.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("learning_node_repo: get: %w", err)
	}
	return n, nil
}

func (r *LearningNodeRepository) UpdateContent(ctx context.Context, id string, content json.RawMessage, status string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE learning_nodes SET content = $1, status = $2, updated_at = NOW() WHERE id = $3`,
		content, status, id,
	)
	if err != nil {
		return fmt.Errorf("learning_node_repo: update: %w", err)
	}
	return nil
}

func (r *LearningNodeRepository) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM learning_nodes WHERE id = $1`, id,
	)
	if err != nil {
		return fmt.Errorf("learning_node_repo: delete: %w", err)
	}
	return nil
}
