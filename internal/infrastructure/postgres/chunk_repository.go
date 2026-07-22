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

// CreateBatch inserts chunk rows in a single transaction. Called by the
// Embedding Worker only — see docs/07-storage.md#write-ownership.
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
			(id, document_id, course_id, start_timestamp, end_timestamp, page_number,
			 title, summary, content, token_count, embedding_version, vector_ref)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`

	for _, c := range chunks {
		_, err := tx.ExecContext(ctx, q,
			c.ID, c.DocumentID, c.CourseID,
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
		SELECT id, document_id, course_id, start_timestamp, end_timestamp, page_number,
		       title, summary, content, token_count, embedding_version, vector_ref, created_at
		FROM chunks WHERE document_id = $1 ORDER BY created_at`
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
		SELECT id, document_id, course_id, start_timestamp, end_timestamp, page_number,
		       title, summary, content, token_count, embedding_version, vector_ref, created_at
		FROM chunks WHERE id IN (%s)`, strings.Join(placeholders, ","))
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
		err := rows.Scan(
			&c.ID, &c.DocumentID, &c.CourseID,
			&startTS, &endTS, &pageNum,
			&c.Title, &c.Summary, &c.Content, &c.TokenCount,
			&c.EmbeddingVersion, &c.VectorRef, &c.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("chunk: scan: %w", err)
		}
		if startTS.Valid {
			v := int(startTS.Int64); c.StartTimestamp = &v
		}
		if endTS.Valid {
			v := int(endTS.Int64); c.EndTimestamp = &v
		}
		if pageNum.Valid {
			v := int(pageNum.Int64); c.PageNumber = &v
		}
		chunks = append(chunks, &c)
	}
	return chunks, rows.Err()
}

func nullInt(v *int) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*v), Valid: true}
}
