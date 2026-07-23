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

// EmbeddingWorker consumes METADATA_DONE, calls the AI Service for embeddings,
// writes vectors to Qdrant, writes Chunk rows to Postgres, marks the course
// INDEXED, and publishes INDEXED.
//
// This is the only place in the pipeline that writes to Qdrant.
// docs/02-system-architecture.md#component-ownership: AI Service never writes.
type EmbeddingWorker struct {
	base
	chunks   repository.ChunkRepository
	vectors  provider.VectorStore
	aiClient *llm.Client
}

func NewEmbeddingWorker(
	courses repository.CourseRepository,
	jobs repository.JobRepository,
	chunks repository.ChunkRepository,
	vectors provider.VectorStore,
	queue provider.Queue,
	aiClient *llm.Client,
) *EmbeddingWorker {
	return &EmbeddingWorker{
		base:     base{courses: courses, jobs: jobs, queue: queue},
		chunks:   chunks,
		vectors:  vectors,
		aiClient: aiClient,
	}
}

func (w *EmbeddingWorker) Run(ctx context.Context) error {
	const (
		stream = "pipeline:metadata"
		group  = "embedding-workers"
	)
	ch, err := w.queue.Consume(ctx, stream, group)
	if err != nil {
		return fmt.Errorf("embedding: consume: %w", err)
	}
	log.Println("embedding worker: listening on", stream)
	for {
		select {
		case <-ctx.Done():
			return nil
		case qe, ok := <-ch:
			if !ok {
				return nil
			}
			if qe.Name != "METADATA_DONE" {
				_ = qe.Ack(ctx)
				continue
			}
			w.handle(ctx, qe)
			_ = qe.Ack(ctx)
		}
	}
}

func (w *EmbeddingWorker) handle(ctx context.Context, qe provider.QueuedEvent) {
	courseID, _ := qe.Payload["course_id"].(string)
	chunksJSON, _ := qe.Payload["chunks"].(string)
	jobID, _ := qe.Payload["job_id"].(string)

	job, err := w.jobs.GetByID(ctx, jobID)
	if err != nil {
		log.Printf("embedding: get job %s: %v", jobID, err)
		return
	}

	for attempt := 1; attempt <= job.MaxAttempts; attempt++ {
		if err := w.startJob(ctx, job); err != nil {
			log.Printf("embedding: start job: %v", err)
			return
		}
		if err := w.courses.UpdateStatus(
			ctx,
			"",
			courseID,
			entities.CourseStatusEmbedding,
		); err != nil {
			w.failJob(ctx, "", job, "embedding-status", courseID, qe.TraceID, err)
			return
		}
		if err := w.process(ctx, courseID, chunksJSON, qe.TraceID, job); err == nil {
			// Mark course INDEXED — terminal success state
			if err := w.succeedJob(ctx, "", job, entities.CourseStatusIndexed); err != nil {
				log.Printf("embedding: complete job %s: %v", job.ID, err)
				return
			}
			// Publish INDEXED so Go API status updater pushes WebSocket notification
			_ = w.queue.Publish(ctx, "pipeline:status", provider.Event{
				Name:    "INDEXED",
				Payload: map[string]any{"course_id": courseID},
				TraceID: qe.TraceID,
			})
			return
		} else {
			w.failJob(ctx, "", job, "embedding", courseID, qe.TraceID, err)
			if job.Status == entities.JobStatusDeadLettered {
				return
			}
		}
	}
}

func (w *EmbeddingWorker) process(ctx context.Context, courseID, chunksJSON, traceID string, job *entities.Job) error {
	var chunks []entities.Chunk
	if err := json.Unmarshal([]byte(chunksJSON), &chunks); err != nil {
		return fmt.Errorf("embedding: unmarshal: %w", err)
	}

	// Extract texts for batch embedding
	texts := make([]string, len(chunks))
	for i, c := range chunks {
		texts[i] = c.Content
	}

	// Call AI Service — stateless compute, returns vectors only
	vecs, err := w.aiClient.Embed(ctx, texts)
	if err != nil {
		return fmt.Errorf("embedding: embed: %w", err)
	}
	if len(vecs) != len(chunks) {
		return fmt.Errorf("embedding: expected %d vectors, got %d", len(chunks), len(vecs))
	}

	// Build Qdrant points
	points := make([]provider.VectorPoint, len(chunks))
	for i, c := range chunks {
		points[i] = provider.VectorPoint{
			ChunkID:        c.ID,
			CourseID:       courseID,
			StartTimestamp: c.StartTimestamp,
			Vector:         vecs[i],
		}
		chunks[i].VectorRef = c.ID // Qdrant point ID = chunk ID
	}

	// Write vectors to Qdrant
	if err := w.vectors.Upsert(ctx, points); err != nil {
		return fmt.Errorf("embedding: upsert qdrant: %w", err)
	}

	// Write Chunk rows to Postgres (the only place this happens — ADR: workers own writes)
	chunkPtrs := make([]*entities.Chunk, len(chunks))
	for i := range chunks {
		chunkPtrs[i] = &chunks[i]
	}
	if err := w.chunks.CreateBatch(ctx, chunkPtrs); err != nil {
		return fmt.Errorf("embedding: write chunks to postgres: %w", err)
	}

	return nil
}
