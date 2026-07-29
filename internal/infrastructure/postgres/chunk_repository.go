package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"archadilm/internal/domain/entities"
	"archadilm/internal/domain/repository"
)

type chunkRepository struct{ db *sql.DB }

func NewChunkRepository(db *sql.DB) repository.ChunkRepository {
	return &chunkRepository{db: db}
}

func (r *chunkRepository) CreateBatch(ctx context.Context, chunks []*entities.Chunk) error {
	if len(chunks) == 0 {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("chunk: begin tx: %w", err)
	}
	defer tx.Rollback()

	const q = `
		INSERT INTO chunks
			(id, document_id, conversation_id, start_timestamp, end_timestamp, page_number,
			 title, summary, content, token_count, embedding_version, vector_ref)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`

	for _, c := range chunks {
		_, err := tx.ExecContext(ctx, q,
			c.ID, c.DocumentID, c.ConversationID,
			nullInt(c.StartTimestamp), nullInt(c.EndTimestamp), nullInt(c.PageNumber),
			c.Title, c.Summary, c.Content, c.TokenCount,
			c.EmbeddingVersion, c.VectorRef,
		)
		if err != nil {
			return fmt.Errorf("chunk: insert %s: %w", c.ID, err)
		}
	}
	return tx.Commit()
}

func (r *chunkRepository) ListByDocument(ctx context.Context, documentID string) ([]*entities.Chunk, error) {
	const q = `
		SELECT c.id, c.document_id, c.conversation_id, c.start_timestamp, c.end_timestamp, c.page_number,
		       c.title, c.summary, c.content, c.token_count, c.embedding_version, c.vector_ref, c.created_at,
		       COALESCE(d.original_filename, ''), COALESCE(d.source_type, ''), COALESCE(d.source_url, '')
		FROM chunks c
		LEFT JOIN documents d ON c.document_id = d.id
		WHERE c.document_id = $1 ORDER BY c.created_at`
	rows, err := r.db.QueryContext(ctx, q, documentID)
	if err != nil {
		return nil, fmt.Errorf("chunk: list: %w", err)
	}
	defer rows.Close()
	return scanChunks(rows)
}

func (r *chunkRepository) GetByIDs(ctx context.Context, ids []string) ([]*entities.Chunk, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}
	q := fmt.Sprintf(`
		SELECT c.id, c.document_id, c.conversation_id, c.start_timestamp, c.end_timestamp, c.page_number,
		       c.title, c.summary, c.content, c.token_count, c.embedding_version, c.vector_ref, c.created_at,
		       COALESCE(d.original_filename, ''), COALESCE(d.source_type, ''), COALESCE(d.source_url, '')
		FROM chunks c
		LEFT JOIN documents d ON c.document_id = d.id
		WHERE c.id IN (%s)`, strings.Join(placeholders, ","))
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("chunk: get by ids: %w", err)
	}
	defer rows.Close()
	return scanChunks(rows)
}

func scanChunks(rows *sql.Rows) ([]*entities.Chunk, error) {
	var chunks []*entities.Chunk
	for rows.Next() {
		var c entities.Chunk
		var startTS, endTS, pageNum sql.NullInt64
		var docName, sourceType, sourceURL string
		err := rows.Scan(
			&c.ID, &c.DocumentID, &c.ConversationID,
			&startTS, &endTS, &pageNum,
			&c.Title, &c.Summary, &c.Content, &c.TokenCount,
			&c.EmbeddingVersion, &c.VectorRef, &c.CreatedAt,
			&docName, &sourceType, &sourceURL,
		)
		if err != nil {
			return nil, fmt.Errorf("chunk: scan: %w", err)
		}
		c.DocumentName = docName
		c.SourceType = sourceType
		c.SourceURL = sourceURL

		if startTS.Valid {
			v := int(startTS.Int64)
			c.StartTimestamp = &v
		}
		if endTS.Valid {
			v := int(endTS.Int64)
			c.EndTimestamp = &v
		}
		if pageNum.Valid {
			v := int(pageNum.Int64)
			c.PageNumber = &v
		}
		chunks = append(chunks, &c)
	}
	return chunks, rows.Err()
}

func (r *chunkRepository) GetByID(ctx context.Context, id string) (*entities.Chunk, error) {
	chunks, err := r.GetByIDs(ctx, []string{id})
	if err != nil {
		return nil, err
	}
	if len(chunks) == 0 {
		return nil, repository.ErrNotFound
	}
	return chunks[0], nil
}

func nullInt(v *int) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*v), Valid: true}
}
