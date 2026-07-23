package worker

import (
	"context"
	"fmt"
	"log"

	"archadilm/internal/domain/entities"
	"archadilm/internal/domain/provider"
	"archadilm/internal/domain/repository"
)

// ManifestWorker turns the document IDs carried by one upload event into one
// parse job per document. It must never query every document in a collection:
// ZIP uploads otherwise create an N×N pipeline fan-out.
type ManifestWorker struct {
	base
	documents repository.DocumentRepository
}

func NewManifestWorker(courses repository.CourseRepository, jobs repository.JobRepository, documents repository.DocumentRepository, queue provider.Queue, ids provider.IDGenerator) *ManifestWorker {
	return &ManifestWorker{base: base{courses: courses, jobs: jobs, queue: queue, ids: ids}, documents: documents}
}

func (w *ManifestWorker) Run(ctx context.Context) error {
	ch, err := w.queue.Consume(ctx, "pipeline:upload", "manifest-workers")
	if err != nil {
		return fmt.Errorf("manifest: consume: %w", err)
	}
	log.Println("manifest worker: listening on pipeline:upload")
	for {
		select {
		case <-ctx.Done():
			return nil
		case qe, ok := <-ch:
			if !ok {
				return nil
			}
			if qe.Name == "UPLOAD_COMPLETED" {
				w.handle(ctx, qe)
			}
			_ = qe.Ack(ctx)
		}
	}
}

func (w *ManifestWorker) handle(ctx context.Context, qe provider.QueuedEvent) {
	courseID, _ := qe.Payload["course_id"].(string)
	jobID, _ := qe.Payload["job_id"].(string)
	documentIDs := payloadStrings(qe.Payload["document_ids"])
	if courseID == "" || jobID == "" || len(documentIDs) == 0 {
		log.Printf("manifest: invalid upload event %v", qe.Payload)
		return
	}
	job, err := w.jobs.GetByID(ctx, jobID)
	if err != nil {
		log.Printf("manifest: get job %s: %v", jobID, err)
		return
	}
	if err := w.startJob(ctx, job); err != nil {
		log.Printf("manifest: start job: %v", err)
		return
	}
	for _, documentID := range documentIDs {
		doc, err := w.documents.GetByID(ctx, documentID)
		if err != nil || doc.CourseID != courseID {
			w.failJob(ctx, "", job, "manifest", courseID, qe.TraceID, fmt.Errorf("document %s is missing or outside collection", documentID))
			return
		}
		parserJobID := w.ids.New()
		parserJob := &entities.Job{ID: parserJobID, CourseID: courseID, DocumentID: &documentID, Stage: entities.JobStageParsing, Status: entities.JobStatusQueued, MaxAttempts: 3, PipelineVersion: PipelineVersion}
		if err := w.jobs.Create(ctx, parserJob); err != nil {
			w.failJob(ctx, "", job, "manifest", courseID, qe.TraceID, err)
			return
		}
		if err := w.queue.Publish(ctx, "pipeline:manifest", provider.Event{Name: "MANIFEST_READY", Payload: map[string]any{"course_id": courseID, "document_id": documentID, "job_id": parserJobID}, TraceID: qe.TraceID}); err != nil {
			w.failJob(ctx, "", job, "manifest", courseID, qe.TraceID, err)
			return
		}
	}
	if err := job.TransitionTo(entities.JobStatusSucceeded); err != nil {
		log.Printf("manifest: complete job %s: %v", job.ID, err)
		return
	}
	if err := w.jobs.Update(ctx, job); err != nil {
		log.Printf("manifest: save job %s: %v", job.ID, err)
	}
}

func payloadStrings(value any) []string {
	switch ids := value.(type) {
	case []string:
		return ids
	case []any:
		out := make([]string, 0, len(ids))
		for _, id := range ids {
			if text, ok := id.(string); ok && text != "" {
				out = append(out, text)
			}
		}
		return out
	default:
		return nil
	}
}
