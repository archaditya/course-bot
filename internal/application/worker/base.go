// Package worker contains the Go background workers that own the entire
// indexing pipeline. Each worker consumes one Redis Stream event, does its
// work, and publishes the next event. See docs/04-indexing-pipeline.md.
//
// Workers never serve HTTP requests. All persistence (Postgres, Qdrant) is
// done here — the Python AI Service only returns compute results.
package worker

import (
	"context"
	"fmt"
	"log"
	"time"

	"course-assistant/internal/domain/entities"
	"course-assistant/internal/domain/provider"
	"course-assistant/internal/domain/repository"
)

// PipelineVersion tags every Job created by any worker stage.
const PipelineVersion = "1.0"

// base holds the dependencies shared by all pipeline stages.
type base struct {
	courses repository.CourseRepository
	jobs    repository.JobRepository
	queue   provider.Queue
	ids     provider.IDGenerator
}

// startJob marks a Job RUNNING, updating Postgres.
func (b *base) startJob(ctx context.Context, job *entities.Job) error {
	if err := job.TransitionTo(entities.JobStatusRunning); err != nil {
		return fmt.Errorf("start job: %w", err)
	}
	job.Attempts++
	return b.jobs.Update(ctx, job)
}

// succeedJob marks a Job SUCCEEDED and advances the Course state to `next`.
func (b *base) succeedJob(ctx context.Context, ws repository.WorkspaceID, job *entities.Job, next entities.CourseStatus) error {
	if err := job.TransitionTo(entities.JobStatusSucceeded); err != nil {
		return fmt.Errorf("succeed job: %w", err)
	}
	if err := b.jobs.Update(ctx, job); err != nil {
		return err
	}
	return b.courses.UpdateStatus(ctx, ws, job.CourseID, next)
}

// failJob handles retryable vs. non-retryable failures per
// docs/09-deployment.md#error-handling. On max retries it dead-letters the
// job, marks the course FAILED, and publishes a FAILED event.
func (b *base) failJob(ctx context.Context, ws repository.WorkspaceID, job *entities.Job, stage, courseID, traceID string, err error) {
	errMsg := err.Error()
	log.Printf("worker: %s failed (attempt %d/%d) course=%s: %v",
		stage, job.Attempts, job.MaxAttempts, courseID, err)

	if job.Attempts >= job.MaxAttempts {
		job.Status = entities.JobStatusDeadLettered
		job.LastError = errMsg
		_ = b.jobs.Update(ctx, job)
		_ = b.courses.UpdateStatus(ctx, ws, courseID, entities.CourseStatusFailed)

		_ = b.queue.Publish(ctx, "pipeline:status", provider.Event{
			Name: "FAILED",
			Payload: map[string]any{
				"course_id": courseID,
				"job_id":    job.ID,
				"stage":     stage,
				"error":     errMsg,
			},
			TraceID: traceID,
		})
		return
	}

	job.Status = entities.JobStatusRetrying
	job.LastError = errMsg
	_ = b.jobs.Update(ctx, job)

	backoff := time.Duration(job.Attempts*job.Attempts) * 2 * time.Second
	log.Printf("worker: %s retrying in %s", stage, backoff)
	time.Sleep(backoff)
}
