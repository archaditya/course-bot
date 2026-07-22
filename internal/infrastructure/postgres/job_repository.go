package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"archadilm/internal/domain/entities"
	"archadilm/internal/domain/repository"
)

type jobRepository struct{ db *sql.DB }

func NewJobRepository(db *sql.DB) repository.JobRepository {
	return &jobRepository{db: db}
}

func (r *jobRepository) Create(ctx context.Context, j *entities.Job) error {
	const q = `
		INSERT INTO jobs
			(id, course_id, document_id, stage, status, attempts, max_attempts, pipeline_version, last_error)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`
	_, err := r.db.ExecContext(ctx, q,
		j.ID, j.CourseID, nullString(j.DocumentID),
		string(j.Stage), string(j.Status),
		j.Attempts, j.MaxAttempts, j.PipelineVersion, j.LastError,
	)
	if err != nil {
		return fmt.Errorf("job: create: %w", err)
	}
	return nil
}

func (r *jobRepository) GetByID(ctx context.Context, id string) (*entities.Job, error) {
	const q = `
		SELECT id, course_id, document_id, stage, status, attempts, max_attempts,
		       pipeline_version, last_error, created_at, updated_at
		FROM jobs WHERE id = $1`
	row := r.db.QueryRowContext(ctx, q, id)
	j, err := scanJob(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, repository.ErrNotFound
	}
	return j, err
}

func (r *jobRepository) ListByCourse(ctx context.Context, courseID string) ([]*entities.Job, error) {
	const q = `
		SELECT id, course_id, document_id, stage, status, attempts, max_attempts,
		       pipeline_version, last_error, created_at, updated_at
		FROM jobs WHERE course_id = $1 ORDER BY created_at`
	rows, err := r.db.QueryContext(ctx, q, courseID)
	if err != nil {
		return nil, fmt.Errorf("job: list: %w", err)
	}
	defer rows.Close()
	var jobs []*entities.Job
	for rows.Next() {
		j, err := scanJobRow(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}

func (r *jobRepository) Update(ctx context.Context, j *entities.Job) error {
	const q = `
		UPDATE jobs SET status=$1, attempts=$2, last_error=$3, updated_at=now()
		WHERE id=$4`
	_, err := r.db.ExecContext(ctx, q, string(j.Status), j.Attempts, j.LastError, j.ID)
	if err != nil {
		return fmt.Errorf("job: update: %w", err)
	}
	return nil
}

func scanJob(row *sql.Row) (*entities.Job, error) {
	return scanJobFields(row)
}

func scanJobRow(rows *sql.Rows) (*entities.Job, error) {
	return scanJobFields(rows)
}

type jobScanner interface{ Scan(dest ...any) error }

func scanJobFields(s jobScanner) (*entities.Job, error) {
	var j entities.Job
	var stage, status, lastError string
	var docID sql.NullString
	err := s.Scan(
		&j.ID, &j.CourseID, &docID, &stage, &status,
		&j.Attempts, &j.MaxAttempts, &j.PipelineVersion, &lastError,
		&j.CreatedAt, &j.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	j.Stage = entities.JobStage(stage)
	j.Status = entities.JobStatus(status)
	j.LastError = lastError
	if docID.Valid {
		j.DocumentID = &docID.String
	}
	return &j, nil
}

func nullString(s *string) sql.NullString {
	if s == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *s, Valid: true}
}
