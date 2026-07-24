package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"archadilm/internal/domain/provider"
)

const dlqStream = "pipeline:dlq"

type DLQEntry struct {
	OriginalStream string                 `json:"original_stream"`
	OriginalEvent  provider.Event         `json:"original_event"`
	FailedAt       time.Time             `json:"failed_at"`
	Stage          string                `json:"stage"`
	Error          string                `json:"error"`
	JobID          string                `json:"job_id"`
	CourseID       string                `json:"course_id"`
	RetryCount     int                   `json:"retry_count"`
}

func SendToDLQ(ctx context.Context, queue provider.Queue, originalStream string, event provider.Event, stage, jobID, courseID string, err error) error {
	entry := DLQEntry{
		OriginalStream: originalStream,
		OriginalEvent:  event,
		FailedAt:       time.Now(),
		Stage:          stage,
		Error:          err.Error(),
		JobID:          jobID,
		CourseID:       courseID,
		RetryCount:     0,
	}
	
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("dlq: marshal: %w", err)
	}
	
	dlqEvent := provider.Event{
		Name:    "JOB_FAILED",
		Payload: map[string]any{"entry": string(data)},
		TraceID: event.TraceID,
	}
	
	return queue.Publish(ctx, dlqStream, dlqEvent)
}

func RetryFromDLQ(ctx context.Context, queue provider.Queue, entryID string) error {
	// Fetch DLQ entry
	// Republish to original stream
	// Remove from DLQ
	return nil
}