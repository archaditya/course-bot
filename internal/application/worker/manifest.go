package worker

import (
	"context"
	"fmt"
	"log"

	"archadilm/internal/domain/entities"
	"archadilm/internal/domain/provider"
	"archadilm/internal/domain/repository"
)

// ManifestWorker turns one UPLOAD_COMPLETED event (one document) into a
// parse Job for that document. Each source is queued and processed
// independently — there is no fan-out across a collection anymore.
type ManifestWorker struct {
	base
}

func NewManifestWorker(documents repository.DocumentRepository, jobs repository.JobRepository, queue provider.Queue, ids provider.IDGenerator) *ManifestWorker {
	return &ManifestWorker{base: base{documents: documents, jobs: jobs, queue: queue, ids: ids}}
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
	conversationID, _ := qe.Payload["conversation_id"].(string)
	documentID, _ := qe.Payload["document_id"].(string)
	jobID, _ := qe.Payload["job_id"].(string)
	if conversationID == "" || documentID == "" || jobID == "" {
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

	parserJobID := w.ids.New()
	parserJob := &entities.Job{ID: parserJobID, ConversationID: conversationID, DocumentID: documentID, Stage: entities.JobStageParsing, Status: entities.JobStatusQueued, MaxAttempts: 3, PipelineVersion: PipelineVersion}
	if err := w.jobs.Create(ctx, parserJob); err != nil {
		w.failJob(ctx, job, "manifest", conversationID, qe.TraceID, err)
		return
	}
	if err := w.queue.Publish(ctx, "pipeline:manifest", provider.Event{Name: "MANIFEST_READY", Payload: map[string]any{"conversation_id": conversationID, "document_id": documentID, "job_id": parserJobID}, TraceID: qe.TraceID}); err != nil {
		w.failJob(ctx, job, "manifest", conversationID, qe.TraceID, err)
		return
	}

	if err := job.TransitionTo(entities.JobStatusSucceeded); err != nil {
		log.Printf("manifest: complete job %s: %v", job.ID, err)
		return
	}
	if err := w.jobs.Update(ctx, job); err != nil {
		log.Printf("manifest: save job %s: %v", job.ID, err)
	}
}
