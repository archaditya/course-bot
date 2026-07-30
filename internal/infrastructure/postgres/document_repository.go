package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"archadilm/internal/domain/entities"
	"archadilm/internal/domain/repository"
)

type documentRepository struct{ db *sql.DB }

func NewDocumentRepository(db *sql.DB) repository.DocumentRepository {
	return &documentRepository{db: db}
}

// Create verifies the target conversation belongs to the given workspace
// before inserting, so a caller can't attach a document to a conversation it
// doesn't own by guessing a conversation_id.
func (r *documentRepository) Create(ctx context.Context, ws string, d *entities.Document) error {
	const q = `
		INSERT INTO documents (id, conversation_id, source_type, status, storage_path, source_url, original_filename, checksum)
		SELECT $1, $2, $3, $4, $5, $6, $7, $8
		WHERE EXISTS (SELECT 1 FROM conversations WHERE id = $2 AND workspace_id = $9)`
	res, err := r.db.ExecContext(ctx, q, d.ID, d.ConversationID, string(d.SourceType), string(d.Status), d.StoragePath, nullableString(d.SourceURL), d.OriginalFilename, d.Checksum, ws)
	if err != nil {
		return fmt.Errorf("document: create: %w", err)
	}
	return checkRowsAffected(res)
}

const documentSelect = `
	SELECT d.id, d.conversation_id, d.source_type, d.status, d.storage_path, d.source_url,
	       d.normalized_ref, d.normalization_version, d.original_filename, d.checksum,
	       d.ai_summary, d.ai_questions, d.ai_overview, d.created_at, d.updated_at
	FROM documents d`

func (r *documentRepository) GetByID(ctx context.Context, ws string, id string) (*entities.Document, error) {
	const q = documentSelect + `
		JOIN conversations c ON c.id = d.conversation_id
		WHERE d.id = $1 AND c.workspace_id = $2`
	return scanDocument(r.db.QueryRowContext(ctx, q, id, ws))
}

// GetByIDInternal is the unscoped read path for trusted internal callers
// (pipeline workers).
func (r *documentRepository) GetByIDInternal(ctx context.Context, id string) (*entities.Document, error) {
	return scanDocument(r.db.QueryRowContext(ctx, documentSelect+` WHERE d.id = $1`, id))
}

func (r *documentRepository) ListByConversation(ctx context.Context, ws string, conversationID string) ([]*entities.Document, error) {
	const q = documentSelect + `
		JOIN conversations c ON c.id = d.conversation_id
		WHERE d.conversation_id = $1 AND c.workspace_id = $2
		ORDER BY d.created_at`
	rows, err := r.db.QueryContext(ctx, q, conversationID, ws)
	if err != nil {
		return nil, fmt.Errorf("document: list: %w", err)
	}
	defer rows.Close()
	var docs []*entities.Document
	for rows.Next() {
		d, err := scanDocumentRow(rows)
		if err != nil {
			return nil, err
		}
		docs = append(docs, d)
	}
	return docs, rows.Err()
}

// UpdateStatus is the narrow write path used by pipeline workers as
// PARSING/CHUNKING/.../INDEXED/FAILED transitions occur. Background workers
// are trusted internal callers — they don't carry a browser workspace
// claim, so this is intentionally unscoped by workspace (mirrors the old
// CourseRepository.UpdateStatus internal path).
func (r *documentRepository) UpdateStatus(ctx context.Context, id string, status entities.DocumentStatus) error {
	res, err := r.db.ExecContext(ctx, `UPDATE documents SET status = $1, updated_at = now() WHERE id = $2`, string(status), id)
	if err != nil {
		return fmt.Errorf("document: update status: %w", err)
	}
	return checkRowsAffected(res)
}

func (r *documentRepository) SetNormalizedRef(ctx context.Context, id, ref, version string) error {
	if _, err := r.db.ExecContext(ctx, `UPDATE documents SET normalized_ref=$1, normalization_version=$2, updated_at=now() WHERE id=$3`, ref, version, id); err != nil {
		return fmt.Errorf("document: set normalized ref: %w", err)
	}
	return nil
}

type documentScanner interface{ Scan(dest ...any) error }

func scanDocument(row *sql.Row) (*entities.Document, error) {
	d, err := scanDocumentFields(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, repository.ErrNotFound
	}
	return d, err
}
func scanDocumentRow(rows *sql.Rows) (*entities.Document, error) { return scanDocumentFields(rows) }
func scanDocumentFields(s documentScanner) (*entities.Document, error) {
	var d entities.Document
	var sourceType, status string
	var sourceURL, normalizedRef, normalizationVersion sql.NullString
	var aiQuestionsJSON []byte
	err := s.Scan(
		&d.ID, &d.ConversationID, &sourceType, &status, &d.StoragePath, &sourceURL,
		&normalizedRef, &normalizationVersion, &d.OriginalFilename, &d.Checksum,
		&d.AISummary, &aiQuestionsJSON, &d.AIOverview, &d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	d.SourceType = entities.SourceType(sourceType)
	d.Status = entities.DocumentStatus(status)
	if sourceURL.Valid {
		d.SourceURL = sourceURL.String
	}
	if normalizedRef.Valid {
		d.NormalizedRef = &normalizedRef.String
	}
	if normalizationVersion.Valid {
		d.NormalizationVersion = &normalizationVersion.String
	}
	if len(aiQuestionsJSON) > 0 {
		_ = json.Unmarshal(aiQuestionsJSON, &d.AIQuestions)
	}
	return &d, nil
}

func (r *documentRepository) SetNormalizedData(ctx context.Context, id string, data []byte, version string) error {
	const query = `
		UPDATE documents
		SET normalized_data = $2, normalization_version = $3, updated_at = NOW()
		WHERE id = $1
	`
	_, err := r.db.ExecContext(ctx, query, id, data, version)
	return err
}

func (r *documentRepository) GetNormalizedData(ctx context.Context, id string) ([]byte, string, error) {
	const query = `
		SELECT normalized_data, normalization_version
		FROM documents
		WHERE id = $1
	`
	var data []byte
	var version string
	err := r.db.QueryRowContext(ctx, query, id).Scan(&data, &version)
	return data, version, err
}

func (r *documentRepository) UpdateOriginalFilename(ctx context.Context, id string, filename string) error {
	const query = `
		UPDATE documents
		SET original_filename = $2, updated_at = NOW()
		WHERE id = $1
	`
	_, err := r.db.ExecContext(ctx, query, id, filename)
	return err
}

func (r *documentRepository) UpdateIntel(ctx context.Context, id string, summary string, questions []string, overview string) error {
	qJSON, err := json.Marshal(questions)
	if err != nil {
		return fmt.Errorf("document: marshal questions: %w", err)
	}
	const query = `
		UPDATE documents
		SET ai_summary = $2, ai_questions = $3, ai_overview = $4, updated_at = NOW()
		WHERE id = $1
	`
	_, err = r.db.ExecContext(ctx, query, id, summary, qJSON, overview)
	return err
}

func (r *documentRepository) Delete(ctx context.Context, ws string, id string) error {
	const q = `
		DELETE FROM documents d
		USING conversations c
		WHERE d.id = $1 AND d.conversation_id = c.id AND c.workspace_id = $2`
	res, err := r.db.ExecContext(ctx, q, id, ws)
	if err != nil {
		return fmt.Errorf("document: delete: %w", err)
	}
	return checkRowsAffected(res)
}
