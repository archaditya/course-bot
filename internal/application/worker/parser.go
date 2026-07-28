package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"archadilm/internal/domain/entities"
	"archadilm/internal/domain/provider"
	"archadilm/internal/domain/repository"
	"archadilm/internal/infrastructure/llm"
)

// NormalizationVersion tracks the SRT parser logic version.
const NormalizationVersion = "1.0"

// ParserWorker consumes MANIFEST_READY, parses SRT → NormalizedDocument,
// stores it in R2, updates Document.NormalizedRef, and publishes NORMALIZED.
type ParserWorker struct {
	base
	documents repository.DocumentRepository
	objects   provider.ObjectStore
	aiClient  *llm.Client
	allowedURLDomains []string
}

func NewParserWorker(
	jobs repository.JobRepository,
	documents repository.DocumentRepository,
	objects provider.ObjectStore,
	queue provider.Queue,
	ids provider.IDGenerator,
	aiClient *llm.Client,
	allowedURLDomains []string,
) *ParserWorker {
	return &ParserWorker{
		base:              base{documents: documents, jobs: jobs, queue: queue, ids: ids},
		documents:         documents,
		objects:           objects,
		aiClient:          aiClient,
		allowedURLDomains: allowedURLDomains,
	}
}

func (w *ParserWorker) Run(ctx context.Context) error {
	const (
		stream = "pipeline:manifest"
		group  = "parser-workers"
	)
	ch, err := w.queue.Consume(ctx, stream, group)
	if err != nil {
		return fmt.Errorf("parser: consume: %w", err)
	}
	log.Println("parser worker: listening on", stream)
	for {
		select {
		case <-ctx.Done():
			return nil
		case qe, ok := <-ch:
			if !ok {
				return nil
			}
			if qe.Name != "MANIFEST_READY" {
				_ = qe.Ack(ctx)
				continue
			}
			w.handle(ctx, qe)
			_ = qe.Ack(ctx)
		}
	}
}

func (w *ParserWorker) handle(ctx context.Context, qe provider.QueuedEvent) {
	conversationID, _ := qe.Payload["conversation_id"].(string)
	docID, _ := qe.Payload["document_id"].(string)
	jobID, _ := qe.Payload["job_id"].(string)

	job, err := w.jobs.GetByID(ctx, jobID)
	if err != nil {
		log.Printf("parser: get job %s: %v", jobID, err)
		return
	}

	for attempt := 1; attempt <= job.MaxAttempts; attempt++ {
		if err := w.startJob(ctx, job); err != nil {
			log.Printf("parser: start job: %v", err)
			return
		}
		if err := w.process(ctx, conversationID, docID, qe.TraceID); err == nil {
			if err := w.succeedJob(ctx, job, entities.DocumentStatusChunking); err != nil {
				log.Printf("metadata: complete job %s: %v", job.ID, err)
				return
			}
			return
		} else {
			w.failJob(ctx, job, "parsing", conversationID, qe.TraceID, err)
			if job.Status == entities.JobStatusDeadLettered {
				return
			}
		}
	}
}

func (w *ParserWorker) process(ctx context.Context, conversationID, docID, traceID string) error {
	doc, err := w.documents.GetByIDInternal(ctx, docID)
	if err != nil {
		return fmt.Errorf("parser: get document: %w", err)
	}

	var normalized *entities.NormalizedDocument

	switch doc.SourceType {
	case entities.SourceTypeSRT:
		rawData, err := w.objects.Get(ctx, doc.StoragePath)
		if err != nil {
			return fmt.Errorf("parser: get raw file: %w", err)
		}
		normalized, err = parseSRT(rawData, doc)
		if err != nil {
			return fmt.Errorf("parser: srt: %w", err)
		}

	case entities.SourceTypeVTT:
		rawData, err := w.objects.Get(ctx, doc.StoragePath)
		if err != nil {
			return fmt.Errorf("parser: get raw file: %w", err)
		}
		normalized, err = parseVTT(rawData, doc)
		if err != nil {
			return fmt.Errorf("parser: vtt: %w", err)
		}

	case entities.SourceTypePDF:
		rawData, err := w.objects.Get(ctx, doc.StoragePath)
		if err != nil {
			return fmt.Errorf("parser: get raw file: %w", err)
		}
		normalized, err = parsePDF(rawData, doc, w.aiClient)
		if err != nil {
			return fmt.Errorf("parser: pdf: %w", err)
		}

	case entities.SourceTypeURL, entities.SourceTypeVideo:
		normalized, err = parseURL(doc, w.aiClient, w.allowedURLDomains)
		if err != nil {
			return fmt.Errorf("parser: url: %w", err)
		}

	case entities.SourceTypeText:
		rawData, err := w.objects.Get(ctx, doc.StoragePath)
		if err != nil {
			return fmt.Errorf("parser: get raw file: %w", err)
		}
		normalized, err = parseText(rawData, doc)
		if err != nil {
			return fmt.Errorf("parser: text: %w", err)
		}

	default:
		return fmt.Errorf("parser: unsupported source type %q", doc.SourceType)
	}

	data, err := json.Marshal(normalized)
	if err != nil {
		return fmt.Errorf("parser: marshal: %w", err)
	}
	normalizedKey := fmt.Sprintf("processed/%s/%s/normalized.json", conversationID, docID)
	if err := w.objects.Put(ctx, normalizedKey, data, "application/json"); err != nil {
		return fmt.Errorf("parser: put processed: %w", err)
	}

	if err := w.documents.SetNormalizedRef(ctx, docID, normalizedKey, NormalizationVersion); err != nil {
		return fmt.Errorf("parser: set normalized ref: %w", err)
	}

	chunkJobID := w.ids.New()
	chunkJob := &entities.Job{
		ID:              chunkJobID,
		ConversationID:  conversationID,
		DocumentID:      docID,
		Stage:           entities.JobStageChunk,
		Status:          entities.JobStatusQueued,
		MaxAttempts:     3,
		PipelineVersion: PipelineVersion,
	}
	if err := w.jobs.Create(ctx, chunkJob); err != nil {
		return fmt.Errorf("parser: create chunk job: %w", err)
	}

	return w.queue.Publish(ctx, "pipeline:parse", provider.Event{
		Name: "NORMALIZED",
		Payload: map[string]any{
			"conversation_id": conversationID,
			"document_id":     docID,
			"normalized_ref":  normalizedKey,
			"job_id":          chunkJobID,
		},
		TraceID: traceID,
	})
}

