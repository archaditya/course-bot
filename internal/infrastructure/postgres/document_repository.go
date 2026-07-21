package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"course-assistant/internal/domain/entities"
	"course-assistant/internal/domain/repository"
)

type documentRepository struct{ db *sql.DB }

func NewDocumentRepository(db *sql.DB) repository.DocumentRepository {
	return &documentRepository{db: db}
}

func (r *documentRepository) Create(ctx context.Context, d *entities.Document) error {
	const q = `
		INSERT INTO documents
			(id, lesson_id, course_id, source_type, storage_path, original_filename, checksum)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`
	_, err := r.db.ExecContext(ctx, q,
		d.ID, d.LessonID, d.CourseID,
		string(d.SourceType), d.StoragePath,
		d.OriginalFilename, d.Checksum,
	)
	if err != nil {
		return fmt.Errorf("document: create: %w", err)
	}
	return nil
}

func (r *documentRepository) GetByID(ctx context.Context, id string) (*entities.Document, error) {
	const q = `
		SELECT id, lesson_id, course_id, source_type, storage_path,
		       normalized_ref, normalization_version, original_filename, checksum,
		       created_at, updated_at
		FROM documents WHERE id = $1`
	row := r.db.QueryRowContext(ctx, q, id)
	return scanDocument(row)
}

func (r *documentRepository) ListByCourse(ctx context.Context, courseID string) ([]*entities.Document, error) {
	const q = `
		SELECT id, lesson_id, course_id, source_type, storage_path,
		       normalized_ref, normalization_version, original_filename, checksum,
		       created_at, updated_at
		FROM documents WHERE course_id = $1 ORDER BY created_at`
	rows, err := r.db.QueryContext(ctx, q, courseID)
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

func (r *documentRepository) SetNormalizedRef(ctx context.Context, id, ref, version string) error {
	const q = `UPDATE documents SET normalized_ref=$1, normalization_version=$2, updated_at=now() WHERE id=$3`
	_, err := r.db.ExecContext(ctx, q, ref, version, id)
	if err != nil {
		return fmt.Errorf("document: set normalized ref: %w", err)
	}
	return nil
}

// scanDocument/scanDocumentRow handle nullable columns.
type documentScanner interface {
	Scan(dest ...any) error
}

func scanDocument(row *sql.Row) (*entities.Document, error) {
	d, err := scanDocumentFields(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, repository.ErrNotFound
	}
	return d, err
}

func scanDocumentRow(rows *sql.Rows) (*entities.Document, error) {
	return scanDocumentFields(rows)
}

func scanDocumentFields(s documentScanner) (*entities.Document, error) {
	var d entities.Document
	var sourceType string
	var normalizedRef, normalizationVersion sql.NullString
	err := s.Scan(
		&d.ID, &d.LessonID, &d.CourseID, &sourceType, &d.StoragePath,
		&normalizedRef, &normalizationVersion,
		&d.OriginalFilename, &d.Checksum,
		&d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	d.SourceType = entities.SourceType(sourceType)
	if normalizedRef.Valid {
		d.NormalizedRef = &normalizedRef.String
	}
	if normalizationVersion.Valid {
		d.NormalizationVersion = &normalizationVersion.String
	}
	return &d, nil
}
