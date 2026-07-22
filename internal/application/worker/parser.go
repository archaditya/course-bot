package worker

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"

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
}

func NewParserWorker(
	courses repository.CourseRepository,
	jobs repository.JobRepository,
	documents repository.DocumentRepository,
	objects provider.ObjectStore,
	queue provider.Queue,
	ids provider.IDGenerator,
	aiClient *llm.Client,
) *ParserWorker {
	return &ParserWorker{
		base:      base{courses: courses, jobs: jobs, queue: queue, ids: ids},
		documents: documents,
		objects:   objects,
		aiClient:  aiClient,
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
	courseID, _ := qe.Payload["course_id"].(string)
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
		if err := w.process(ctx, courseID, docID, qe.TraceID); err == nil {
			_ = w.succeedJob(ctx, "", job, entities.CourseStatusParsing)
			return
		} else {
			w.failJob(ctx, "", job, "parsing", courseID, qe.TraceID, err)
			if job.Status == entities.JobStatusDeadLettered {
				return
			}
		}
	}
}

func (w *ParserWorker) process(ctx context.Context, courseID, docID, traceID string) error {
	doc, err := w.documents.GetByID(ctx, docID)
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

	case entities.SourceTypeURL:
		normalized, err = parseURL(doc, w.aiClient)
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
	normalizedKey := fmt.Sprintf("processed/%s/%s/normalized.json", courseID, docID)
	if err := w.objects.Put(ctx, normalizedKey, data, "application/json"); err != nil {
		return fmt.Errorf("parser: put processed: %w", err)
	}

	if err := w.documents.SetNormalizedRef(ctx, docID, normalizedKey, NormalizationVersion); err != nil {
		return fmt.Errorf("parser: set normalized ref: %w", err)
	}

	chunkJobID := w.ids.New()
	chunkJob := &entities.Job{
		ID:              chunkJobID,
		CourseID:        courseID,
		DocumentID:      &docID,
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
			"course_id":      courseID,
			"document_id":    docID,
			"normalized_ref": normalizedKey,
			"job_id":         chunkJobID,
		},
		TraceID: traceID,
	})
}

// parseSRT parses a SubRip (.srt) transcript into a NormalizedDocument.
func parseSRT(data []byte, doc *entities.Document) (*entities.NormalizedDocument, error) {
	nd := &entities.NormalizedDocument{
		Language:             "en",
		SourceRef:            doc.StoragePath,
		Timeline:             true,
		NormalizationVersion: NormalizationVersion,
	}
	nd.Metadata.SourceType = doc.SourceType
	nd.Metadata.OriginalFilename = doc.OriginalFilename
	nd.Metadata.Checksum = doc.Checksum

	scanner := bufio.NewScanner(bytes.NewReader(data))
	var (
		segIdx  int
		state   int
		seg     entities.Segment
		textBuf []string
	)

	flush := func() {
		if len(textBuf) > 0 {
			seg.Text = strings.TrimSpace(strings.Join(textBuf, " "))
			nd.Segments = append(nd.Segments, seg)
			textBuf = nil
		}
	}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		switch state {
		case 0:
			if line == "" {
				continue
			}
			idx, err := strconv.Atoi(line)
			if err != nil {
				continue
			}
			segIdx = idx
			seg = entities.Segment{SegmentID: fmt.Sprintf("seg-%d", segIdx)}
			state = 1

		case 1:
			if line == "" {
				state = 0
				continue
			}
			parts := strings.Split(line, " --> ")
			if len(parts) != 2 {
				return nil, fmt.Errorf("srt: malformed timing at segment %d: %q", segIdx, line)
			}
			startSec, err := parseSRTTime(parts[0])
			if err != nil {
				return nil, fmt.Errorf("srt: start time at %d: %w", segIdx, err)
			}
			endSec, err := parseSRTTime(parts[1])
			if err != nil {
				return nil, fmt.Errorf("srt: end time at %d: %w", segIdx, err)
			}
			seg.StartTS = &startSec
			seg.EndTS = &endSec
			state = 2

		case 2:
			if line == "" {
				flush()
				state = 0
				continue
			}
			textBuf = append(textBuf, line)
		}
	}
	flush()

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("srt: scan: %w", err)
	}
	if len(nd.Segments) == 0 {
		return nil, fmt.Errorf("srt: no segments found; file may be empty or corrupt")
	}
	return nd, nil
}

func parseSRTTime(s string) (int, error) {
	s = strings.TrimSpace(strings.ReplaceAll(s, ",", "."))
	parts := strings.SplitN(s, ":", 3)
	if len(parts) != 3 {
		return 0, fmt.Errorf("expected HH:MM:SS,mmm got %q", s)
	}
	h, _ := strconv.Atoi(parts[0])
	m, _ := strconv.Atoi(parts[1])
	secParts := strings.SplitN(parts[2], ".", 2)
	sec, _ := strconv.Atoi(secParts[0])
	return h*3600 + m*60 + sec, nil
}
