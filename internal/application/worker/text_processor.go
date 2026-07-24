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

// TextProcessorWorker combines Parser and Chunker stages
type TextProcessorWorker struct {
	base
	documents repository.DocumentRepository
	objects   provider.ObjectStore
	aiClient  *llm.Client
	allowedURLDomains []string
}

func NewTextProcessorWorker(
	courses repository.CourseRepository,
	jobs repository.JobRepository,
	documents repository.DocumentRepository,
	objects provider.ObjectStore,
	queue provider.Queue,
	ids provider.IDGenerator,
	aiClient *llm.Client,
	allowedURLDomains []string,
) *TextProcessorWorker {
	return &TextProcessorWorker{
		base:              base{courses: courses, jobs: jobs, queue: queue, ids: ids},
		documents:         documents,
		objects:           objects,
		aiClient:          aiClient,
		allowedURLDomains: allowedURLDomains,
	}
}

func (w *TextProcessorWorker) Run(ctx context.Context) error {
	const (
		stream = "pipeline:manifest"
		group  = "text-processor-workers"
	)
	ch, err := w.queue.Consume(ctx, stream, group)
	if err != nil {
		return fmt.Errorf("text-processor: consume: %w", err)
	}
	log.Println("text processor worker: listening on", stream)
	
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

func (w *TextProcessorWorker) handle(ctx context.Context, qe provider.QueuedEvent) {
	courseID, _ := qe.Payload["course_id"].(string)
	docID, _ := qe.Payload["document_id"].(string)
	jobID, _ := qe.Payload["job_id"].(string)
	
	job, err := w.jobs.GetByID(ctx, jobID)
	if err != nil {
		log.Printf("text-processor: get job %s: %v", jobID, err)
		return
	}
	
	for attempt := 1; attempt <= job.MaxAttempts; attempt++ {
		if err := w.startJob(ctx, job); err != nil {
			log.Printf("text-processor: start job: %v", err)
			return
		}
		if err := w.process(ctx, courseID, docID, qe.TraceID); err == nil {
			if err := w.succeedJob(ctx, "", job, entities.CourseStatusIndexing); err != nil {
				log.Printf("text-processor: complete job %s: %v", job.ID, err)
				return
			}
			return
		} else {
			w.failJob(ctx, "", job, "text-processing", courseID, qe.TraceID, err)
			if job.Status == entities.JobStatusDeadLettered {
				return
			}
		}
	}
}

func (w *TextProcessorWorker) process(ctx context.Context, courseID, docID, traceID string) error {
	doc, err := w.documents.GetByID(ctx, docID)
	if err != nil {
		return fmt.Errorf("text-processor: get document: %w", err)
	}
	
	// Step 1: Extract text (was Parser)
	var normalized *entities.NormalizedDocument
	switch doc.SourceType {
	case entities.SourceTypeSRT:
		rawData, err := w.objects.Get(ctx, doc.StoragePath)
		if err != nil {
			return fmt.Errorf("text-processor: get raw file: %w", err)
		}
		normalized, err = parseSRT(rawData, doc)
		if err != nil {
			return fmt.Errorf("text-processor: srt: %w", err)
		}
	case entities.SourceTypeVTT:
		rawData, err := w.objects.Get(ctx, doc.StoragePath)
		if err != nil {
			return fmt.Errorf("text-processor: get raw file: %w", err)
		}
		normalized, err = parseVTT(rawData, doc)
		if err != nil {
			return fmt.Errorf("text-processor: vtt: %w", err)
		}
	case entities.SourceTypeURL, entities.SourceTypeVideo:
		normalized, err = parseURL(doc, w.aiClient, w.allowedURLDomains)
		if err != nil {
			return fmt.Errorf("text-processor: url: %w", err)
		}
	case entities.SourceTypeText:
		rawData, err := w.objects.Get(ctx, doc.StoragePath)
		if err != nil {
			return fmt.Errorf("text-processor: get raw file: %w", err)
		}
		normalized, err = parseText(rawData, doc)
		if err != nil {
			return fmt.Errorf("text-processor: text: %w", err)
		}
	// ... handle other source types ...
	default:
		return fmt.Errorf("text-processor: unsupported source type %q", doc.SourceType)
	}

	// Store normalized in Postgres instead of R2
    data, err := json.Marshal(normalized)
    if err != nil {
        return fmt.Errorf("text-processor: marshal: %w", err)
    }
    
    if err := w.documents.SetNormalizedData(ctx, docID, data, NormalizationVersion); err != nil {
        return fmt.Errorf("text-processor: set normalized data: %w", err)
    }
	
	// Step 2: Chunk immediately (was Chunker)
	chunks := w.slidingWindowChunk(normalized.Segments, courseID, docID)
	
	// Step 3: Store chunks in Postgres (skip R2 intermediate)
	chunkPtrs := make([]*entities.Chunk, len(chunks))
	for i := range chunks {
		chunkPtrs[i] = &chunks[i]
	}
	
	// Create indexer job
	indexerJobID := w.ids.New()
	indexerJob := &entities.Job{
		ID:              indexerJobID,
		CourseID:        courseID,
		DocumentID:      &docID,
		Stage:           entities.JobStageIndexing,
		Status:          entities.JobStatusQueued,
		MaxAttempts:     3,
		PipelineVersion: PipelineVersion,
	}
	if err := w.jobs.Create(ctx, indexerJob); err != nil {
		return fmt.Errorf("text-processor: create indexer job: %w", err)
	}
	
	// Publish TEXT_PROCESSED event with chunks
	chunkData, err := json.Marshal(chunks)
	if err != nil {
		return fmt.Errorf("text-processor: marshal chunks: %w", err)
	}
	
	return w.queue.Publish(ctx, "pipeline:text-processed", provider.Event{
		Name: "TEXT_PROCESSED",
		Payload: map[string]any{
			"course_id":   courseID,
			"document_id": docID,
			"chunks":      string(chunkData),
			"job_id":      indexerJobID,
		},
		TraceID: traceID,
	})
}

func (w *TextProcessorWorker) slidingWindowChunk(segs []entities.Segment, courseID, docID string) []entities.Chunk {
	// Same chunking logic from chunker.go
	const defaultWindowSize = 20
	const defaultOverlap = 2
	step := defaultWindowSize - defaultOverlap
	
	var chunks []entities.Chunk
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
	}
	return chunks
}