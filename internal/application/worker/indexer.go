package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"archadilm/internal/domain/entities"
	"archadilm/internal/domain/provider"
	"archadilm/internal/domain/repository"
	"archadilm/internal/infrastructure/llm"
	"archadilm/internal/infrastructure/observability"
)

// IndexerWorker combines Metadata and Embedding stages
type IndexerWorker struct {
	base
	chunks   repository.ChunkRepository
	vectors  provider.VectorStore
	aiClient *llm.Client
}

func NewIndexerWorker(
    courses repository.CourseRepository,
    jobs repository.JobRepository,
    chunks repository.ChunkRepository,
    vectors provider.VectorStore,
    queue provider.Queue,
    ids provider.IDGenerator,
    aiClient *llm.Client,
) *IndexerWorker {
    return &IndexerWorker{
        base:     base{courses: courses, jobs: jobs, queue: queue, ids: ids},  // ADD ids
        chunks:   chunks,
        vectors:  vectors,
        aiClient: aiClient,
    }
}

func (w *IndexerWorker) Run(ctx context.Context) error {
	const (
		stream = "pipeline:text-processed"
		group  = "indexer-workers"
	)
	ch, err := w.queue.Consume(ctx, stream, group)
	if err != nil {
		return fmt.Errorf("indexer: consume: %w", err)
	}
	log.Println("indexer worker: listening on", stream)
	
	for {
		select {
		case <-ctx.Done():
			return nil
		case qe, ok := <-ch:
			if !ok {
				return nil
			}
			if qe.Name != "TEXT_PROCESSED" {
				_ = qe.Ack(ctx)
				continue
			}
			w.handle(ctx, qe)
			_ = qe.Ack(ctx)
		}
	}
}

func (w *IndexerWorker) handle(ctx context.Context, qe provider.QueuedEvent) {
	courseID, _ := qe.Payload["course_id"].(string)
	chunksJSON, _ := qe.Payload["chunks"].(string)
	jobID, _ := qe.Payload["job_id"].(string)
	
	job, err := w.jobs.GetByID(ctx, jobID)
	if err != nil {
		log.Printf("indexer: get job %s: %v", jobID, err)
		return
	}
	
	for attempt := 1; attempt <= job.MaxAttempts; attempt++ {
		if err := w.startJob(ctx, job); err != nil {
			log.Printf("indexer: start job: %v", err)
			return
		}
		if err := w.process(ctx, courseID, chunksJSON, qe.TraceID, job); err == nil {
			if err := w.succeedJob(ctx, "", job, entities.CourseStatusIndexed); err != nil {
				log.Printf("indexer: complete job %s: %v", job.ID, err)
				return
			}
			// Publish INDEXED event
			_ = w.queue.Publish(ctx, "pipeline:status", provider.Event{
				Name:    "INDEXED",
				Payload: map[string]any{"course_id": courseID},
				TraceID: qe.TraceID,
			})
			return
		} else {
			w.failJob(ctx, "", job, "indexing", courseID, qe.TraceID, err)
			if job.Status == entities.JobStatusDeadLettered {
				return
			}
		}
	}
}

func (w *IndexerWorker) process(ctx context.Context, courseID, chunksJSON, traceID string, job *entities.Job) error {
	var chunks []entities.Chunk
	if err := json.Unmarshal([]byte(chunksJSON), &chunks); err != nil {
		return fmt.Errorf("indexer: unmarshal: %w", err)
	}
	
	// Step 1: Add metadata (local, no AI call)
	metadataStart := time.Now()
	for i := range chunks {
		chunks[i].Title, chunks[i].Summary = localMetadata(chunks[i].Content)
	}
	observability.RecordProcessingTime("metadata", time.Since(metadataStart))
	
	// Step 2: Batch embeddings
	embeddingStart := time.Now()
	texts := make([]string, len(chunks))
	for i, c := range chunks {
		texts[i] = c.Content
	}
	
	vecs, err := w.aiClient.Embed(ctx, texts)
	if err != nil {
		observability.RecordError("embedding")
		return fmt.Errorf("indexer: embed: %w", err)
	}
	if len(vecs) != len(chunks) {
		observability.RecordError("embedding")
		return fmt.Errorf("indexer: expected %d vectors, got %d", len(chunks), len(vecs))
	}
	observability.RecordProcessingTime("embedding", time.Since(embeddingStart))
	
	// Step 3: Build Qdrant points
	points := make([]provider.VectorPoint, len(chunks))
	for i, c := range chunks {
		points[i] = provider.VectorPoint{
			ChunkID:        c.ID,
			CourseID:       courseID,
			StartTimestamp: c.StartTimestamp,
			Vector:         vecs[i],
		}
		chunks[i].VectorRef = c.ID
	}
	
	// Step 4: Upsert to Qdrant
	if err := w.vectors.Upsert(ctx, points); err != nil {
		return fmt.Errorf("indexer: upsert qdrant: %w", err)
	}
	
	// Step 5: Write chunks to Postgres
	chunkPtrs := make([]*entities.Chunk, len(chunks))
	for i := range chunks {
		chunkPtrs[i] = &chunks[i]
	}
	if err := w.chunks.CreateBatch(ctx, chunkPtrs); err != nil {
		return fmt.Errorf("indexer: write chunks to postgres: %w", err)
	}
	
	return nil
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