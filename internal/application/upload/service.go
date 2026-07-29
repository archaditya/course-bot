// Package upload implements the "add a source" use case: validate a raw
// file (or URL / pasted text), store it, create the Document record under
// the target Conversation's own knowledge base, and publish
// UPLOAD_COMPLETED to kick off the indexing pipeline for that one source.
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
	"net/http"
	"path/filepath"
	"strings"

	"archadilm/internal/domain/entities"
	"archadilm/internal/domain/provider"
	"archadilm/internal/domain/repository"
)

const (
	// PipelineVersion tags every Job so we can tell which pipeline definition
	// it ran under.
	PipelineVersion = "1.0"

	// uploadStream is the Redis Stream name for UPLOAD_COMPLETED events.
	uploadStream = "pipeline:upload"

	// maxZipSize is the maximum allowed uncompressed size (200 MB).
	maxZipSize = 200 << 20
)

// Service owns the "add source" use case.
type Service struct {
	conversations repository.ConversationRepository
	documents     repository.DocumentRepository
	jobs          repository.JobRepository
	objects       provider.ObjectStore
	queue         provider.Queue
	ids           provider.IDGenerator
}

func NewService(
	conversations repository.ConversationRepository,
	documents repository.DocumentRepository,
	jobs repository.JobRepository,
	objects provider.ObjectStore,
	queue provider.Queue,
	ids provider.IDGenerator,
) *Service {
	return &Service{
		conversations: conversations,
		documents:     documents,
		jobs:          jobs,
		objects:       objects,
		queue:         queue,
		ids:           ids,
	}
}

// UploadResult holds the IDs returned to the caller (Go API handler) after a
// successful upload. Processing is async — the caller immediately returns 202.
type UploadResult struct {
	ConversationID string   `json:"conversation_id"`
	DocumentIDs    []string `json:"document_ids"`
}

// Upload validates, stores, and queues one raw file for indexing under the
// given conversation's knowledge base.
func (s *Service) Upload(
	ctx context.Context,
	ws repository.WorkspaceID,
	conversationID string,
	filename string,
	data []byte,
	traceID string,
) (*UploadResult, error) {
	sourceType, err := detectSourceType(filename)
	if err != nil {
		return nil, err
	}

	detectedType := http.DetectContentType(data)
	if strings.Contains(detectedType, "executable") || strings.Contains(detectedType, "x-dosexec") {
		return nil, fmt.Errorf("upload: security error: malicious executable file header detected")
	}

	// Verify the conversation belongs to this workspace before attaching
	// anything to it.
	if _, err := s.conversations.GetByID(ctx, ws, conversationID); err != nil {
		return nil, fmt.Errorf("upload: conversation access denied: %w", err)
	}

	sum := sha256.Sum256(data)
	checksum := hex.EncodeToString(sum[:])

	docID := s.ids.New()
	storageKey := fmt.Sprintf("raw/%s/%s/%s", conversationID, docID, filename)
	if err := s.objects.Put(ctx, storageKey, data, "application/octet-stream"); err != nil {
		return nil, fmt.Errorf("upload: r2 put: %w", err)
	}

	doc := &entities.Document{
		ID:               docID,
		ConversationID:   conversationID,
		SourceType:       sourceType,
		Status:           entities.DocumentStatusUploading,
		StoragePath:      storageKey,
		OriginalFilename: filename,
		Checksum:         checksum,
	}
	if err := s.documents.Create(ctx, ws, doc); err != nil {
		return nil, fmt.Errorf("upload: document: %w", err)
	}

	return s.queueForIndexing(ctx, conversationID, docID, traceID)
}

// AddSource handles non-file sources: URLs and pasted text.
func (s *Service) AddSource(
	ctx context.Context,
	ws repository.WorkspaceID,
	conversationID string,
	sourceType string,
	url string,
	content string,
	title string,
) (*UploadResult, error) {
	if _, err := s.conversations.GetByID(ctx, ws, conversationID); err != nil {
		return nil, fmt.Errorf("add-source: conversation access denied: %w", err)
	}

	docTitle := title
	if docTitle == "" {
		docTitle = url
		if docTitle == "" {
			docTitle = "Pasted text"
		}
	}

	docID := s.ids.New()
	doc := &entities.Document{
		ID:               docID,
		ConversationID:   conversationID,
		Status:           entities.DocumentStatusUploading,
		OriginalFilename: docTitle,
	}

	switch sourceType {
	case "url":
		doc.SourceType = entities.SourceTypeURL
		doc.SourceURL = url
		sum := sha256.Sum256([]byte(url))
		doc.Checksum = hex.EncodeToString(sum[:])

	case "video_url":
		doc.SourceType = entities.SourceTypeVideo
		doc.SourceURL = url
		sum := sha256.Sum256([]byte(url))
		doc.Checksum = hex.EncodeToString(sum[:])

	case "text":
		doc.SourceType = entities.SourceTypeText
		sum := sha256.Sum256([]byte(content))
		doc.Checksum = hex.EncodeToString(sum[:])

		storageKey := fmt.Sprintf("raw/%s/%s/content.txt", conversationID, docID)
		if err := s.objects.Put(ctx, storageKey, []byte(content), "text/plain"); err != nil {
			return nil, fmt.Errorf("add-source: r2 put text: %w", err)
		}
		doc.StoragePath = storageKey

	default:
		return nil, fmt.Errorf("add-source: unsupported source type %q", sourceType)
	}

	if err := s.documents.Create(ctx, ws, doc); err != nil {
		return nil, fmt.Errorf("add-source: document: %w", err)
	}

	return s.queueForIndexing(ctx, conversationID, docID, conversationID+"-source")
}

// UploadZip extracts a ZIP archive, auto-detects the type of each file
// inside, creates one Document per supported file under the given
// conversation, and queues them all for indexing in parallel. Unsupported
// files are skipped with a warning log.
func (s *Service) UploadZip(
	ctx context.Context,
	ws repository.WorkspaceID,
	conversationID string,
	zipData []byte,
	traceID string,
) (*UploadResult, error) {
	if _, err := s.conversations.GetByID(ctx, ws, conversationID); err != nil {
		return nil, fmt.Errorf("upload-zip: conversation access denied: %w", err)
	}

	reader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return nil, fmt.Errorf("upload-zip: open archive: %w", err)
	}

	var docIDs []string
	var totalUncompressed int64

	for _, f := range reader.File {
		if f.FileInfo().IsDir() || strings.HasPrefix(filepath.Base(f.Name), ".") {
			continue
		}

		totalUncompressed += int64(f.UncompressedSize64)
		if totalUncompressed > maxZipSize {
			return nil, fmt.Errorf("upload-zip: total uncompressed size exceeds %d MB limit", maxZipSize>>20)
		}

		sourceType, err := detectSourceType(f.Name)
		if err != nil {
			log.Printf("upload-zip: skipping %s: %v", f.Name, err)
			continue
		}

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

		sum := sha256.Sum256(data)
		checksum := hex.EncodeToString(sum[:])
		basename := filepath.Base(f.Name)

		docID := s.ids.New()
		storageKey := fmt.Sprintf("raw/%s/%s/%s", conversationID, docID, basename)
		if err := s.objects.Put(ctx, storageKey, data, "application/octet-stream"); err != nil {
			log.Printf("upload-zip: r2 put %s: %v", f.Name, err)
			continue
		}

		doc := &entities.Document{
			ID:               docID,
			ConversationID:   conversationID,
			SourceType:       sourceType,
			Status:           entities.DocumentStatusUploading,
			StoragePath:      storageKey,
			OriginalFilename: basename,
			Checksum:         checksum,
		}
		if err := s.documents.Create(ctx, ws, doc); err != nil {
			log.Printf("upload-zip: create document for %s: %v", f.Name, err)
			continue
		}

		if _, err := s.queueForIndexing(ctx, conversationID, docID, traceID+"-"+basename); err != nil {
			log.Printf("upload-zip: queue %s: %v", f.Name, err)
			continue
		}

		docIDs = append(docIDs, docID)
		log.Printf("upload-zip: queued %s (type=%s, doc=%s)", basename, sourceType, docID)
	}

	if len(docIDs) == 0 {
		return nil, fmt.Errorf("upload-zip: no supported files found in archive")
	}

	return &UploadResult{ConversationID: conversationID, DocumentIDs: docIDs}, nil
}

// queueForIndexing transitions the freshly created Document to UPLOADED,
// creates its manifest Job, and publishes UPLOAD_COMPLETED so the pipeline
// picks it up. Every source is queued independently — indexing one source
// never blocks or depends on any other source in the same conversation.
func (s *Service) queueForIndexing(ctx context.Context, conversationID, docID, traceID string) (*UploadResult, error) {
	jobID := s.ids.New()
	job := &entities.Job{
		ID:              jobID,
		ConversationID:  conversationID,
		DocumentID:      docID,
		Stage:           entities.JobStageManifest,
		Status:          entities.JobStatusQueued,
		MaxAttempts:     3,
		PipelineVersion: PipelineVersion,
	}
	if err := s.jobs.Create(ctx, job); err != nil {
		return nil, fmt.Errorf("upload: job: %w", err)
	}

	if err := s.documents.UpdateStatus(ctx, docID, entities.DocumentStatusUploaded); err != nil {
		return nil, fmt.Errorf("upload: update document status: %w", err)
	}

	event := provider.Event{
		Name: "UPLOAD_COMPLETED",
		Payload: map[string]any{
			"conversation_id": conversationID,
			"document_id":     docID,
			"job_id":          jobID,
		},
		TraceID: traceID,
	}
	if err := s.queue.Publish(ctx, uploadStream, event); err != nil {
		return nil, fmt.Errorf("upload: publish: %w", err)
	}

	return &UploadResult{ConversationID: conversationID, DocumentIDs: []string{docID}}, nil
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
