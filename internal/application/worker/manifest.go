package worker

import (
	"context"
	"fmt"
	"log"

	"archadilm/internal/domain/entities"
	"archadilm/internal/domain/provider"
	"archadilm/internal/domain/repository"
)

// ManifestWorker consumes UPLOAD_COMPLETED, creates a parsing job per
// document, and publishes MANIFEST_READY to trigger the Parser Worker.
type ManifestWorker struct {
	base
	documents repository.DocumentRepository
}

func NewManifestWorker(
	courses repository.CourseRepository,
	jobs repository.JobRepository,
	documents repository.DocumentRepository,
	queue provider.Queue,
	ids provider.IDGenerator,
) *ManifestWorker {
	return &ManifestWorker{
		base:      base{courses: courses, jobs: jobs, queue: queue, ids: ids},
		documents: documents,
	}
}

func (w *ManifestWorker) Run(ctx context.Context) error {
	const (
		stream = "pipeline:upload"
		group  = "manifest-workers"
	)
	ch, err := w.queue.Consume(ctx, stream, group)
	if err != nil {
		return fmt.Errorf("manifest: consume: %w", err)
	}
	log.Println("manifest worker: listening on", stream)
	for {
		select {
		case <-ctx.Done():
			return nil
		case qe, ok := <-ch:
			if !ok {
				return nil
			}
			if qe.Name != "UPLOAD_COMPLETED" {
				_ = qe.Ack(ctx)
				continue
			}
			w.handle(ctx, qe)
			_ = qe.Ack(ctx)
		}
	}
}

func (w *ManifestWorker) handle(ctx context.Context, qe provider.QueuedEvent) {
	courseID, _ := qe.Payload["course_id"].(string)
	jobID, _ := qe.Payload["job_id"].(string)

	job, err := w.jobs.GetByID(ctx, jobID)
	if err != nil {
		log.Printf("manifest: get job %s: %v", jobID, err)
		return
	}

	if err := w.startJob(ctx, job); err != nil {
		log.Printf("manifest: start job: %v", err)
		return
	}

	docs, err := w.documents.ListByCourse(ctx, courseID)
	if err != nil {
		w.failJob(ctx, "", job, "manifest", courseID, qe.TraceID, err)
		return
	}

	for _, doc := range docs {
		parserJobID := w.ids.New()
		parserJob := &entities.Job{
			ID:              parserJobID,
			CourseID:        courseID,
			DocumentID:      &doc.ID,
			Stage:           entities.JobStageParsing,
			Status:          entities.JobStatusQueued,
			MaxAttempts:     3,
			PipelineVersion: PipelineVersion,
		}
		if err := w.jobs.Create(ctx, parserJob); err != nil {
			w.failJob(ctx, "", job, "manifest", courseID, qe.TraceID, err)
			return
		}

		if err := w.queue.Publish(ctx, "pipeline:manifest", provider.Event{
			Name: "MANIFEST_READY",
			Payload: map[string]any{
				"course_id":   courseID,
				"document_id": doc.ID,
				"job_id":      parserJobID,
			},
			TraceID: qe.TraceID,
		}); err != nil {
			w.failJob(ctx, "", job, "manifest", courseID, qe.TraceID, err)
			return
		}
	}

	if err := job.TransitionTo(entities.JobStatusSucceeded); err == nil {
		_ = w.jobs.Update(ctx, job)
	}
}
