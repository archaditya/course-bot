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
    "archadilm/internal/infrastructure/resilience"
    redisinfra "archadilm/internal/infrastructure/redis"
)

// PipelineVersion tags every Job created by any worker stage.
const PipelineVersion = "1.0"

// base holds the dependencies shared by all pipeline stages.
type base struct {
	courses repository.CourseRepository
	jobs    repository.JobRepository
	jobStore *redis.JobStore
	queue   provider.Queue
	ids     provider.IDGenerator
}

// startJob marks a Job RUNNING, updating Postgres.
func (b *base) startJob(ctx context.Context, job *entities.Job) error {
    retryConfig := resilience.RetryConfig{
        MaxAttempts:  3,
        InitialDelay: 100 * time.Millisecond,
        MaxDelay:     1 * time.Second,
        Multiplier:   2.0,
    }

	// Update in Redis first (fast)
    if b.jobStore != nil {
        _ = b.jobStore.UpdateJobStatus(ctx, job.ID, entities.JobStatusInProgress)
    }
    
    return resilience.RetryWithContext(ctx, retryConfig, func() error {
        return job.TransitionTo(entities.JobStatusInProgress)
    })
}

// succeedJob marks a Job SUCCEEDED and advances the Course state to `next`.
func (b *base) succeedJob(ctx context.Context, ws repository.WorkspaceID, job *entities.Job, nextStatus entities.CourseStatus) error {
    retryConfig := resilience.RetryConfig{
        MaxAttempts:  3,
        InitialDelay: 100 * time.Millisecond,
        MaxDelay:     1 * time.Second,
        Multiplier:   2.0,
    }
    
    var err error
    err = resilience.RetryWithContext(ctx, retryConfig, func() error {
        return job.TransitionTo(entities.JobStatusSucceeded)
    })
    if err != nil {
        return err
    }
    
    err = resilience.RetryWithContext(ctx, retryConfig, func() error {
        return b.jobs.Update(ctx, job)
    })
    if err != nil {
        return err
    }
    
    if ws != "" {
        return resilience.RetryWithContext(ctx, retryConfig, func() error {
            return b.courses.UpdateStatus(ctx, ws, job.CourseID, nextStatus)
        })
    } else {
        return resilience.RetryWithContext(ctx, retryConfig, func() error {
            return b.courses.UpdateStatusByID(ctx, job.CourseID, nextStatus)
        })
    }
}

// failJob handles retryable vs. non-retryable failures per
func (b *base) failJob(ctx context.Context, ws repository.WorkspaceID, job *entities.Job, stage string, courseID, traceID string, originalErr error) {
    retryConfig := resilience.RetryConfig{
        MaxAttempts:  3,
        InitialDelay: 100 * time.Millisecond,
        MaxDelay:     1 * time.Second,
        Multiplier:   2.0,
    }
    
    job.Attempts++
    if job.Attempts >= job.MaxAttempts {
        _ = job.TransitionTo(entities.JobStatusDeadLettered)

		// Send to DLQ
        event := provider.Event{
            Name:    "MANIFEST_READY", // or whatever the event was
            Payload: map[string]any{"course_id": courseID},
            TraceID: traceID,
        }
        _ = SendToDLQ(ctx, b.queue, "pipeline:manifest", event, stage, job.ID, courseID, originalErr)
    } else {
        _ = job.TransitionTo(entities.JobStatusFailed)
    }
    
    _ = resilience.RetryWithContext(ctx, retryConfig, func() error {
        return b.jobs.Update(ctx, job)
    })
    
    log.Printf("%s: job %s failed (attempt %d/%d): %v", stage, job.ID, job.Attempts, job.MaxAttempts, originalErr)
}
