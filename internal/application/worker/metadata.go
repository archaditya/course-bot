package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"archadilm/internal/domain/entities"
	"archadilm/internal/domain/provider"
	"archadilm/internal/domain/repository"
	"archadilm/internal/infrastructure/llm"
)

// MetadataWorker consumes CHUNKED, calls the AI Service for title+summary
// per chunk, and publishes METADATA_DONE.
type MetadataWorker struct {
	base
	aiClient *llm.Client
}

func NewMetadataWorker(
	courses repository.CourseRepository,
	jobs repository.JobRepository,
	queue provider.Queue,
	ids provider.IDGenerator,
	aiClient *llm.Client,
) *MetadataWorker {
	return &MetadataWorker{
		base:     base{courses: courses, jobs: jobs, queue: queue, ids: ids},
		aiClient: aiClient,
	}
}

func (w *MetadataWorker) Run(ctx context.Context) error {
	const (
		stream = "pipeline:chunk"
		group  = "metadata-workers"
	)
	ch, err := w.queue.Consume(ctx, stream, group)
	if err != nil {
		return fmt.Errorf("metadata: consume: %w", err)
	}
	log.Println("metadata worker: listening on", stream)
	for {
		select {
		case <-ctx.Done():
			return nil
		case qe, ok := <-ch:
			if !ok {
				return nil
			}
			if qe.Name != "CHUNKED" {
				_ = qe.Ack(ctx)
				continue
			}
			w.handle(ctx, qe)
			_ = qe.Ack(ctx)
		}
	}
}

func (w *MetadataWorker) handle(ctx context.Context, qe provider.QueuedEvent) {
	courseID, _ := qe.Payload["course_id"].(string)
	docID, _ := qe.Payload["document_id"].(string)
	chunksJSON, _ := qe.Payload["chunks"].(string)
	jobID, _ := qe.Payload["job_id"].(string)

	job, err := w.jobs.GetByID(ctx, jobID)
	if err != nil {
		log.Printf("metadata: get job %s: %v", jobID, err)
		return
	}

	for attempt := 1; attempt <= job.MaxAttempts; attempt++ {
		if err := w.startJob(ctx, job); err != nil {
			log.Printf("metadata: start job: %v", err)
			return
		}
		if err := w.process(ctx, courseID, docID, chunksJSON, qe.TraceID); err == nil {
			if err := w.succeedJob(ctx, "", job, entities.CourseStatusChunking); err != nil {
				log.Printf("metadata: complete job %s: %v", job.ID, err)
				return
			}
			return
		} else {
			w.failJob(ctx, "", job, "metadata", courseID, qe.TraceID, err)
			if job.Status == entities.JobStatusDeadLettered {
				return
			}
		}
	}
}

func (w *MetadataWorker) process(ctx context.Context, courseID, docID, chunksJSON, traceID string) error {
	var chunks []entities.Chunk
	if err := json.Unmarshal([]byte(chunksJSON), &chunks); err != nil {
		return fmt.Errorf("metadata: unmarshal chunks: %w", err)
	}
	// Indexing must be predictable and bounded. Do not make paid completion
	// requests for every chunk; generation happens only when a user asks chat.
	for i := range chunks {
		chunks[i].Title, chunks[i].Summary = localMetadata(chunks[i].Content)
	}
	enriched, err := json.Marshal(chunks)
	if err != nil {
		return fmt.Errorf("metadata: marshal chunks: %w", err)
	}
	embedJobID := w.ids.New()
	embedJob := &entities.Job{ID: embedJobID, CourseID: courseID, DocumentID: &docID, Stage: entities.JobStageEmbedding, Status: entities.JobStatusQueued, MaxAttempts: 3, PipelineVersion: PipelineVersion}
	if err := w.jobs.Create(ctx, embedJob); err != nil {
		return fmt.Errorf("metadata: create embedding job: %w", err)
	}
	return w.queue.Publish(ctx, "pipeline:metadata", provider.Event{Name: "METADATA_DONE", Payload: map[string]any{"course_id": courseID, "document_id": docID, "chunks": string(enriched), "job_id": embedJobID}, TraceID: traceID})
}

func localMetadata(content string) (string, string) {
	clean := strings.Join(strings.Fields(content), " ")
	if clean == "" {
		return "Untitled source", ""
	}
	words := strings.Fields(clean)
	titleWords := words
	if len(titleWords) > 10 {
		titleWords = titleWords[:10]
	}
	title := strings.Join(titleWords, " ")
	if len(words) > 10 {
		title += "…"
	}
	runes := []rune(clean)
	if len(runes) > 360 {
		return title, string(runes[:360]) + "…"
	}
	return title, clean
}
