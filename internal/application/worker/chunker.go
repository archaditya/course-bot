
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
)

const (
	defaultWindowSize = 20
	defaultOverlap    = 2
)

// ChunkWorker consumes NORMALIZED, applies sliding-window chunking, and
// publishes CHUNKED. It does NOT write to Postgres.
type ChunkWorker struct {
	base
	documents repository.DocumentRepository
	objects   provider.ObjectStore
}

func NewChunkWorker(
	courses repository.CourseRepository,
	jobs repository.JobRepository,
	documents repository.DocumentRepository,
	objects provider.ObjectStore,
	queue provider.Queue,
	ids provider.IDGenerator,
) *ChunkWorker {
	return &ChunkWorker{
		base:      base{courses: courses, jobs: jobs, queue: queue, ids: ids},
		documents: documents,
		objects:   objects,
	}
}

func (w *ChunkWorker) Run(ctx context.Context) error {
	const (
		stream = "pipeline:parse"
		group  = "chunk-workers"
	)
	ch, err := w.queue.Consume(ctx, stream, group)
	if err != nil {
		return fmt.Errorf("chunker: consume: %w", err)
	}
	log.Println("chunk worker: listening on", stream)
	for {
		select {
		case <-ctx.Done():
			return nil
		case qe, ok := <-ch:
			if !ok {
				return nil
			}
			if qe.Name != "NORMALIZED" {
				_ = qe.Ack(ctx)
				continue
			}
			w.handle(ctx, qe)
			_ = qe.Ack(ctx)
		}
	}
}

func (w *ChunkWorker) handle(ctx context.Context, qe provider.QueuedEvent) {
	courseID, _ := qe.Payload["course_id"].(string)
	docID, _ := qe.Payload["document_id"].(string)
	normalizedRef, _ := qe.Payload["normalized_ref"].(string)
	jobID, _ := qe.Payload["job_id"].(string)

	job, err := w.jobs.GetByID(ctx, jobID)
	if err != nil {
		log.Printf("chunker: get job %s: %v", jobID, err)
		return
	}

	for attempt := 1; attempt <= job.MaxAttempts; attempt++ {
		if err := w.startJob(ctx, job); err != nil {
			log.Printf("chunker: start job: %v", err)
			return
		}
		if err := w.process(ctx, courseID, docID, normalizedRef, qe.TraceID); err == nil {
			if err := w.succeedJob(ctx, "", job, entities.CourseStatusNormalizing); err != nil {
				log.Printf("chunker: complete job %s: %v", job.ID, err)
				return
			}
			return
		} else {
			w.failJob(ctx, "", job, "chunking", courseID, qe.TraceID, err)
			if job.Status == entities.JobStatusDeadLettered {
				return
			}
		}
	}
}

func (w *ChunkWorker) process(ctx context.Context, courseID, docID, normalizedRef, traceID string) error {
	data, err := w.objects.Get(ctx, normalizedRef)
	if err != nil {
		return fmt.Errorf("chunker: get normalized: %w", err)
	}
	var nd entities.NormalizedDocument
	if err := json.Unmarshal(data, &nd); err != nil {
		return fmt.Errorf("chunker: unmarshal: %w", err)
	}

	chunks := w.slidingWindowChunk(nd.Segments, courseID, docID)

	chunkData, err := json.Marshal(chunks)
	if err != nil {
		return fmt.Errorf("chunker: marshal chunks: %w", err)
	}

	metaJobID := w.ids.New()
	metaJob := &entities.Job{
		ID:              metaJobID,
		CourseID:        courseID,
		DocumentID:      &docID,
		Stage:           entities.JobStageMetadata,
		Status:          entities.JobStatusQueued,
		MaxAttempts:     3,
		PipelineVersion: PipelineVersion,
	}
	if err := w.jobs.Create(ctx, metaJob); err != nil {
		return fmt.Errorf("chunker: create metadata job: %w", err)
	}

	return w.queue.Publish(ctx, "pipeline:chunk", provider.Event{
		Name: "CHUNKED",
		Payload: map[string]any{
			"course_id":   courseID,
			"document_id": docID,
			"chunks":      string(chunkData),
			"job_id":      metaJobID,
		},
		TraceID: traceID,
	})
}

func (w *ChunkWorker) slidingWindowChunk(segs []entities.Segment, courseID, docID string) []entities.Chunk {
	if len(segs) == 0 {
		return nil
	}
	var chunks []entities.Chunk
	step := defaultWindowSize - defaultOverlap
	for i := 0; i < len(segs); i += step {
		end := i + defaultWindowSize
		if end > len(segs) {
			end = len(segs)
		}
		window := segs[i:end]
		texts := make([]string, len(window))
		for j, s := range window {
			texts[j] = s.Text
		}
		content := strings.Join(texts, " ")
		c := entities.Chunk{
			ID:               w.ids.New(),
			DocumentID:       docID,
			CourseID:         courseID,
			Content:          content,
			TokenCount:       len(content) / 4,
			EmbeddingVersion: "text-embedding-3-small-v1",
		}
		if window[0].StartTS != nil {
			c.StartTimestamp = window[0].StartTS
		}
		if window[len(window)-1].EndTS != nil {
			c.EndTimestamp = window[len(window)-1].EndTS
		}
		chunks = append(chunks, c)
		if end == len(segs) {
			break
		}
	}
	return chunks
}
