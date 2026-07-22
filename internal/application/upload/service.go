// Package upload implements the UploadService use case: validate a raw file,
// store it in R2, create the Document + Lesson records, and publish
// UPLOAD_COMPLETED to kick off the indexing pipeline.
// See docs/04-indexing-pipeline.md#upload-sequence.
package upload

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"path/filepath"
	"strings"

	"archadilm/internal/domain/entities"
	"archadilm/internal/domain/provider"
	"archadilm/internal/domain/repository"
)

const (
	// PipelineVersion tags every Job so we can tell which pipeline definition
	// it ran under — docs/04-indexing-pipeline.md#versioning-strategy.
	PipelineVersion = "1.0"

	// uploadStream is the Redis Stream name for UPLOAD_COMPLETED events.
	uploadStream = "pipeline:upload"

	// maxZipSize is the maximum allowed uncompressed size (200 MB).
	maxZipSize = 200 << 20
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
	CourseID    string   `json:"course_id"`
	DocumentIDs []string `json:"document_ids"`
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

// AddSource handles non-file sources: URLs and pasted text. Instead of
// storing raw bytes in R2, it either stores the text content or records the
// URL for the parser to fetch later. Kicks off the same pipeline as Upload.
func (s *Service) AddSource(
	ctx context.Context,
	ws repository.WorkspaceID,
	courseID string,
	sourceType string,
	url string,
	content string,
	title string,
) (*UploadResult, error) {

	// Fetch course (validates workspace isolation)
	course, err := s.courses.GetByID(ctx, ws, courseID)
	if err != nil {
		return nil, fmt.Errorf("add-source: course: %w", err)
	}

	// Transition course to UPLOADING → UPLOADED
	if err := course.TransitionTo(entities.CourseStatusUploading); err != nil {
		return nil, fmt.Errorf("add-source: transition: %w", err)
	}
	if err := s.courses.Update(ctx, ws, course); err != nil {
		return nil, fmt.Errorf("add-source: update course: %w", err)
	}

	lessonTitle := title
	if lessonTitle == "" {
		lessonTitle = url
		if lessonTitle == "" {
			lessonTitle = "Pasted text"
		}
	}

	lessonID := s.ids.New()
	lesson := &entities.Lesson{
		ID:       lessonID,
		CourseID: courseID,
		Title:    lessonTitle,
	}
	if err := s.lessons.Create(ctx, lesson); err != nil {
		return nil, fmt.Errorf("add-source: lesson: %w", err)
	}

	docID := s.ids.New()
	doc := &entities.Document{
		ID:               docID,
		LessonID:         lessonID,
		CourseID:         courseID,
		OriginalFilename: lessonTitle,
	}

	// Map source type string to domain SourceType
	switch sourceType {
	case "url":
		doc.SourceType = entities.SourceTypeURL
		doc.SourceURL = url
		// URL sources don't store raw data — the parser will fetch the page
		sum := sha256.Sum256([]byte(url))
		doc.Checksum = hex.EncodeToString(sum[:])

	case "video_url":
		doc.SourceType = entities.SourceTypeVideo
		doc.SourceURL = url
		sum := sha256.Sum256([]byte(url))
		doc.Checksum = hex.EncodeToString(sum[:])

	case "text":
		doc.SourceType = entities.SourceTypeText
		// Text sources store the pasted content in R2 (it still needs to go
		// through the parser/chunker pipeline like everything else)
		sum := sha256.Sum256([]byte(content))
		doc.Checksum = hex.EncodeToString(sum[:])

		storageKey := fmt.Sprintf("raw/%s/%s/content.txt", courseID, lessonID)
		if err := s.objects.Put(ctx, storageKey, []byte(content), "text/plain"); err != nil {
			return nil, fmt.Errorf("add-source: r2 put text: %w", err)
		}
		doc.StoragePath = storageKey

	default:
		return nil, fmt.Errorf("add-source: unsupported source type %q", sourceType)
	}

	if err := s.documents.Create(ctx, doc); err != nil {
		return nil, fmt.Errorf("add-source: document: %w", err)
	}

	// Create manifest Job and publish
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
		return nil, fmt.Errorf("add-source: job: %w", err)
	}

	// Transition to UPLOADED
	if err := course.TransitionTo(entities.CourseStatusUploaded); err != nil {
		return nil, fmt.Errorf("add-source: transition uploaded: %w", err)
	}
	if err := s.courses.Update(ctx, ws, course); err != nil {
		return nil, fmt.Errorf("add-source: update course uploaded: %w", err)
	}

	event := provider.Event{
		Name: "UPLOAD_COMPLETED",
		Payload: map[string]any{
			"course_id":    courseID,
			"document_ids": []string{docID},
			"job_id":       jobID,
		},
		TraceID: courseID + "-source",
	}
	if err := s.queue.Publish(ctx, uploadStream, event); err != nil {
		return nil, fmt.Errorf("add-source: publish: %w", err)
	}

	return &UploadResult{CourseID: courseID, DocumentIDs: []string{docID}}, nil
}

// UploadZip extracts a ZIP archive, auto-detects the type of each file inside,
// creates one Document per supported file, and queues them all for indexing
// under the same Course. Unsupported files are skipped with a warning log.
//
// This is what powers the "Upload ZIP" card in the frontend — the user drops
// a zip of SRT/VTT/PDF/TXT files and everything gets indexed in parallel.
func (s *Service) UploadZip(
	ctx context.Context,
	ws repository.WorkspaceID,
	projectID, courseID string,
	zipData []byte,
	traceID string,
) (*UploadResult, error) {

	// Fetch course (validates workspace isolation)
	course, err := s.courses.GetByID(ctx, ws, courseID)
	if err != nil {
		return nil, fmt.Errorf("upload-zip: course: %w", err)
	}

	if err := course.TransitionTo(entities.CourseStatusUploading); err != nil {
		return nil, fmt.Errorf("upload-zip: transition: %w", err)
	}
	if err := s.courses.Update(ctx, ws, course); err != nil {
		return nil, fmt.Errorf("upload-zip: update course: %w", err)
	}

	// Open ZIP from memory
	reader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return nil, fmt.Errorf("upload-zip: open archive: %w", err)
	}

	var docIDs []string
	var totalUncompressed int64

	for _, f := range reader.File {
		// Skip directories and hidden files
		if f.FileInfo().IsDir() || strings.HasPrefix(filepath.Base(f.Name), ".") {
			continue
		}

		// Guard against zip bombs
		totalUncompressed += int64(f.UncompressedSize64)
		if totalUncompressed > maxZipSize {
			return nil, fmt.Errorf("upload-zip: total uncompressed size exceeds %d MB limit", maxZipSize>>20)
		}

		// Detect source type from filename
		sourceType, err := detectSourceType(f.Name)
		if err != nil {
			log.Printf("upload-zip: skipping %s: %v", f.Name, err)
			continue
		}

		// Read file content
		rc, err := f.Open()
		if err != nil {
			log.Printf("upload-zip: open %s: %v", f.Name, err)
			continue
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			log.Printf("upload-zip: read %s: %v", f.Name, err)
			continue
		}

		// Compute checksum
		sum := sha256.Sum256(data)
		checksum := hex.EncodeToString(sum[:])

		// Create Lesson
		basename := filepath.Base(f.Name)
		lessonID := s.ids.New()
		lesson := &entities.Lesson{
			ID:       lessonID,
			CourseID: courseID,
			Title:    strings.TrimSuffix(basename, filepath.Ext(basename)),
		}
		if err := s.lessons.Create(ctx, lesson); err != nil {
			log.Printf("upload-zip: create lesson for %s: %v", f.Name, err)
			continue
		}

		// Store in R2
		storageKey := fmt.Sprintf("raw/%s/%s/%s", courseID, lessonID, basename)
		if err := s.objects.Put(ctx, storageKey, data, "application/octet-stream"); err != nil {
			log.Printf("upload-zip: r2 put %s: %v", f.Name, err)
			continue
		}

		// Create Document
		docID := s.ids.New()
		doc := &entities.Document{
			ID:               docID,
			LessonID:         lessonID,
			CourseID:         courseID,
			SourceType:       sourceType,
			StoragePath:      storageKey,
			OriginalFilename: basename,
			Checksum:         checksum,
		}
		if err := s.documents.Create(ctx, doc); err != nil {
			log.Printf("upload-zip: create document for %s: %v", f.Name, err)
			continue
		}

		// Create Job
		jobID := s.ids.New()
		job := &entities.Job{
			ID:              jobID,
			CourseID:        courseID,
			DocumentID:      &docID,
			Stage:           entities.JobStageManifest,
			Status:          entities.JobStatusQueued,
			MaxAttempts:     3,
			PipelineVersion: PipelineVersion,
		}
		if err := s.jobs.Create(ctx, job); err != nil {
			log.Printf("upload-zip: create job for %s: %v", f.Name, err)
			continue
		}

		// Publish UPLOAD_COMPLETED for each file — they process in parallel
		event := provider.Event{
			Name: "UPLOAD_COMPLETED",
			Payload: map[string]any{
				"course_id":    courseID,
				"document_ids": []string{docID},
				"job_id":       jobID,
			},
			TraceID: traceID + "-" + basename,
		}
		if err := s.queue.Publish(ctx, uploadStream, event); err != nil {
			log.Printf("upload-zip: publish for %s: %v", f.Name, err)
			continue
		}

		docIDs = append(docIDs, docID)
		log.Printf("upload-zip: queued %s (type=%s, doc=%s)", basename, sourceType, docID)
	}

	if len(docIDs) == 0 {
		return nil, fmt.Errorf("upload-zip: no supported files found in archive")
	}

	// Transition to UPLOADED
	if err := course.TransitionTo(entities.CourseStatusUploaded); err != nil {
		return nil, fmt.Errorf("upload-zip: transition uploaded: %w", err)
	}
	if err := s.courses.Update(ctx, ws, course); err != nil {
		return nil, fmt.Errorf("upload-zip: update course uploaded: %w", err)
	}

	return &UploadResult{CourseID: courseID, DocumentIDs: docIDs}, nil
}

func detectSourceType(filename string) (entities.SourceType, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".srt":
		return entities.SourceTypeSRT, nil
	case ".vtt":
		return entities.SourceTypeVTT, nil
	case ".pdf":
		return entities.SourceTypePDF, nil
	case ".docx":
		return entities.SourceTypeDOCX, nil
	case ".txt", ".md", ".markdown":
		return entities.SourceTypeText, nil
	default:
		return "", fmt.Errorf("upload: unsupported file type %q (supported: .srt, .vtt, .pdf, .docx, .txt, .md)", ext)
	}
}
