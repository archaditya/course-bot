package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"archadilm/internal/domain/entities"
	"archadilm/internal/domain/repository"
)

type citationRepository struct{ db *sql.DB }

func NewCitationRepository(db *sql.DB) repository.CitationRepository {
	return &citationRepository{db: db}
}

func (r *citationRepository) CreateBatch(ctx context.Context, citations []*entities.Citation) error {
	if len(citations) == 0 {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("citation: begin tx: %w", err)
	}
	defer tx.Rollback()
	const q = `INSERT INTO citations (id, message_id, chunk_id, start_timestamp, page_number) VALUES ($1,$2,$3,$4,$5)`
	for _, c := range citations {
		_, err := tx.ExecContext(ctx, q,
			c.ID, c.MessageID, c.ChunkID,
			nullInt(c.StartTimestamp), nullInt(c.PageNumber),
		)
		if err != nil {
			return fmt.Errorf("citation: insert: %w", err)
		}
	}
	return tx.Commit()
}

func (r *citationRepository) ListByMessage(ctx context.Context, messageID string) ([]*entities.Citation, error) {
	const q = `SELECT id, message_id, chunk_id, start_timestamp, page_number FROM citations WHERE message_id=$1`
	rows, err := r.db.QueryContext(ctx, q, messageID)
	if err != nil {
		return nil, fmt.Errorf("citation: list: %w", err)
	}
	defer rows.Close()
	var citations []*entities.Citation
	for rows.Next() {
		var c entities.Citation
		var startTS, pageNum sql.NullInt64
		if err := rows.Scan(&c.ID, &c.MessageID, &c.ChunkID, &startTS, &pageNum); err != nil {
			return nil, err
		}
		if startTS.Valid {
			v := int(startTS.Int64); c.StartTimestamp = &v
		}
		if pageNum.Valid {
			v := int(pageNum.Int64); c.PageNumber = &v
		}
		citations = append(citations, &c)
	}
	return citations, rows.Err()
}
