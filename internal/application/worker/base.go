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

	"archadilm/internal/domain/entities"
	"archadilm/internal/domain/provider"
	"archadilm/internal/domain/repository"
	redisinfra "archadilm/internal/infrastructure/redis"
	"archadilm/internal/infrastructure/resilience"
)

// PipelineVersion tags every Job created by any worker stage.
const PipelineVersion = "1.0"

// dbRetryConfig governs the short, fast retries used for Postgres/state
// writes within a single pipeline step. It is intentionally small — the
// job-level retry loop (see indexer.go/parser.go/text_processor.go) is what
// handles slow, expensive retries like a failed AI-service call.
var dbRetryConfig = resilience.RetryConfig{
	MaxAttempts:  3,
	InitialDelay: 100 * time.Millisecond,
	MaxDelay:     1 * time.Second,
	Multiplier:   2.0,
}

// base holds the dependencies shared by all pipeline stages.
type base struct {
	courses  repository.CourseRepository
	jobs     repository.JobRepository
	jobStore *redisinfra.JobStore
	queue    provider.Queue
	ids      provider.IDGenerator
}

// SetJobStore attaches the Redis-backed job status cache used for fast
// status reads. Optional — callers that don't set one just skip the
// best-effort Redis updates in startJob/succeedJob/failJob.
func (b *base) SetJobStore(js *redisinfra.JobStore) {
	b.jobStore = js
}

// startJob marks a Job RUNNING, updating Postgres (and, best-effort, the
// Redis job-status cache used for fast status reads).
func (b *base) startJob(ctx context.Context, job *entities.Job) error {
	if err := job.TransitionTo(entities.JobStatusRunning); err != nil {
		return fmt.Errorf("start job: %w", err)
	}
	job.Attempts++

	// Update Redis first (fast path for status reads). Best-effort: Redis
	// being unavailable must never block the pipeline.
	if b.jobStore != nil {
		_ = b.jobStore.UpdateJobStatus(ctx, job.ID, entities.JobStatusRunning)
	}

	return resilience.RetryWithContext(ctx, dbRetryConfig, func() error {
		return b.jobs.Update(ctx, job)
	})
}

// succeedJob marks a Job SUCCEEDED and advances the Course state to `next`.
//
// ws is always "" for pipeline workers (they're trusted internal callers,
// not scoped to a browser's workspace claim) — CourseRepository.UpdateStatus
// already has an internal, unscoped update path for that case, so there is
// no separate "by id" method to call here.
func (b *base) succeedJob(ctx context.Context, ws repository.WorkspaceID, job *entities.Job, nextStatus entities.CourseStatus) error {
	if err := job.TransitionTo(entities.JobStatusSucceeded); err != nil {
		return fmt.Errorf("succeed job: %w", err)
	}

	if err := resilience.RetryWithContext(ctx, dbRetryConfig, func() error {
		return b.jobs.Update(ctx, job)
	}); err != nil {
		return err
	}

	if b.jobStore != nil {
		_ = b.jobStore.UpdateJobStatus(ctx, job.ID, entities.JobStatusSucceeded)
	}

	return resilience.RetryWithContext(ctx, dbRetryConfig, func() error {
		return b.courses.UpdateStatus(ctx, ws, job.CourseID, nextStatus)
	})
}

// failJob handles retryable vs. non-retryable failures per
// docs/09-deployment.md#error-handling. On max retries it dead-letters the
// job, marks the course FAILED, publishes a FAILED event, and sends the
// original event to the DLQ stream so it can be inspected/replayed later.
func (b *base) failJob(ctx context.Context, ws repository.WorkspaceID, job *entities.Job, stage, courseID, traceID string, originalErr error) {
	errMsg := originalErr.Error()
	log.Printf("worker: %s failed (attempt %d/%d) course=%s: %v",
		stage, job.Attempts, job.MaxAttempts, courseID, originalErr)

	if job.Attempts >= job.MaxAttempts {
		_ = job.TransitionTo(entities.JobStatusDeadLettered)
		job.LastError = errMsg
		_ = resilience.RetryWithContext(ctx, dbRetryConfig, func() error {
			return b.jobs.Update(ctx, job)
		})
		_ = resilience.RetryWithContext(ctx, dbRetryConfig, func() error {
			return b.courses.UpdateStatus(ctx, ws, courseID, entities.CourseStatusFailed)
		})

		if b.jobStore != nil {
			_ = b.jobStore.UpdateJobStatus(ctx, job.ID, entities.JobStatusDeadLettered)
		}

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

		event := provider.Event{
			Name:    "FAILED",
			Payload: map[string]any{"course_id": courseID, "job_id": job.ID},
			TraceID: traceID,
		}
		_ = SendToDLQ(ctx, b.queue, "pipeline:status", event, stage, job.ID, courseID, originalErr)
		return
	}

	_ = job.TransitionTo(entities.JobStatusRetrying)
	job.LastError = errMsg
	_ = resilience.RetryWithContext(ctx, dbRetryConfig, func() error {
		return b.jobs.Update(ctx, job)
	})
	if b.jobStore != nil {
		_ = b.jobStore.UpdateJobStatus(ctx, job.ID, entities.JobStatusRetrying)
	}

	backoff := time.Duration(job.Attempts*job.Attempts) * 2 * time.Second
	log.Printf("worker: %s retrying in %s", stage, backoff)
	time.Sleep(backoff)
}
