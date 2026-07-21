// Package upload implements the UploadService use case: validate a raw file,
// store it in R2, create the Document + Lesson records, and publish
// UPLOAD_COMPLETED to kick off the indexing pipeline.
// See docs/04-indexing-pipeline.md#upload-sequence.
package upload

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"

	"course-assistant/internal/domain/entities"
	"course-assistant/internal/domain/provider"
	"course-assistant/internal/domain/repository"
)

const (
	// PipelineVersion tags every Job so we can tell which pipeline definition
	// it ran under — docs/04-indexing-pipeline.md#versioning-strategy.
	PipelineVersion = "1.0"

	// uploadStream is the Redis Stream name for UPLOAD_COMPLETED events.
	uploadStream = "pipeline:upload"
)

// Service owns the upload use case.
type Service struct {
	courses   repository.CourseRepository
	lessons   repository.LessonRepository
	documents repository.DocumentRepository
	jobs      repository.JobRepository
	objects   provider.ObjectStore
	queue     provider.Queue
	ids       provider.IDGenerator
}

// NewService wires the Service with its dependencies. All concrete
// implementations are injected at startup — see apps/api/cmd/api/main.go.
func NewService(
	courses repository.CourseRepository,
	lessons repository.LessonRepository,
	documents repository.DocumentRepository,
	jobs repository.JobRepository,
	objects provider.ObjectStore,
	queue provider.Queue,
	ids provider.IDGenerator,
) *Service {
	return &Service{
		courses:   courses,
		lessons:   lessons,
		documents: documents,
		jobs:      jobs,
		objects:   objects,
		queue:     queue,
		ids:       ids,
	}
}

// UploadResult holds the IDs returned to the caller (Go API handler) after a
// successful upload. Processing is async — the caller immediately returns 202.
type UploadResult struct {
	CourseID    string
	DocumentIDs []string
}

// Upload validates, stores, and queues one raw file for indexing.
// traceID is propagated through all Redis events for distributed tracing.
func (s *Service) Upload(
	ctx context.Context,
	ws repository.WorkspaceID,
	projectID, courseID string,
	filename string,
	data []byte,
	traceID string,
) (*UploadResult, error) {

	// Validate MIME / extension
	sourceType, err := detectSourceType(filename)
	if err != nil {
		return nil, err
	}

	// Fetch course (validates workspace isolation)
	course, err := s.courses.GetByID(ctx, ws, courseID)
	if err != nil {
		return nil, fmt.Errorf("upload: course: %w", err)
	}

	// Transition course to UPLOADING
	if err := course.TransitionTo(entities.CourseStatusUploading); err != nil {
		return nil, fmt.Errorf("upload: transition: %w", err)
	}
	if err := s.courses.Update(ctx, ws, course); err != nil {
		return nil, fmt.Errorf("upload: update course status: %w", err)
	}

	// Compute checksum for dedup / versioning
	sum := sha256.Sum256(data)
	checksum := hex.EncodeToString(sum[:])

	// Create Lesson (1:1 with file in MVP)
	lessonID := s.ids.New()
	lesson := &entities.Lesson{
		ID:       lessonID,
		CourseID: courseID,
		Title:    strings.TrimSuffix(filename, filepath.Ext(filename)),
	}
	if err := s.lessons.Create(ctx, lesson); err != nil {
		return nil, fmt.Errorf("upload: lesson: %w", err)
	}

	// Store raw file in R2 at raw/<course_id>/<lesson_id>/<filename>
	storageKey := fmt.Sprintf("raw/%s/%s/%s", courseID, lessonID, filename)
	if err := s.objects.Put(ctx, storageKey, data, "application/octet-stream"); err != nil {
		return nil, fmt.Errorf("upload: r2 put: %w", err)
	}

	// Create Document record
	docID := s.ids.New()
	doc := &entities.Document{
		ID:               docID,
		LessonID:         lessonID,
		CourseID:         courseID,
		SourceType:       sourceType,
		StoragePath:      storageKey,
		OriginalFilename: filename,
		Checksum:         checksum,
	}
	if err := s.documents.Create(ctx, doc); err != nil {
		return nil, fmt.Errorf("upload: document: %w", err)
	}

	// Create manifest Job
	jobID := s.ids.New()
	job := &entities.Job{
		ID:              jobID,
		CourseID:        courseID,
		Stage:           entities.JobStageManifest,
		Status:          entities.JobStatusQueued,
		MaxAttempts:     3,
		PipelineVersion: PipelineVersion,
	}
	if err := s.jobs.Create(ctx, job); err != nil {
		return nil, fmt.Errorf("upload: job: %w", err)
	}

	// Transition course to UPLOADED
	if err := course.TransitionTo(entities.CourseStatusUploaded); err != nil {
		return nil, fmt.Errorf("upload: transition uploaded: %w", err)
	}
	if err := s.courses.Update(ctx, ws, course); err != nil {
		return nil, fmt.Errorf("upload: update course uploaded: %w", err)
	}

	// Publish UPLOAD_COMPLETED — consumed by Manifest Worker
	event := provider.Event{
		Name: "UPLOAD_COMPLETED",
		Payload: map[string]any{
			"course_id":    courseID,
			"document_ids": []string{docID},
			"job_id":       jobID,
		},
		TraceID: traceID,
	}
	if err := s.queue.Publish(ctx, uploadStream, event); err != nil {
		return nil, fmt.Errorf("upload: publish: %w", err)
	}

	return &UploadResult{CourseID: courseID, DocumentIDs: []string{docID}}, nil
}

func detectSourceType(filename string) (entities.SourceType, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".srt":
		return entities.SourceTypeSRT, nil
	case ".pdf":
		return entities.SourceTypePDF, nil
	case ".docx":
		return entities.SourceTypeDOCX, nil
	default:
		return "", fmt.Errorf("upload: unsupported file type %q (MVP supports .srt)", ext)
	}
}
