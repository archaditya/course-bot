package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"archadilm/internal/domain/provider"
	redisinfra "archadilm/internal/infrastructure/redis"
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

// RetryFromDLQ replays one dead-lettered entry: it looks the entry up by its
// Redis Stream ID (entryID — the ID XRANGE/XREAD would report, e.g.
// "1700000000000-0", not the JobID), republishes the original event to the
// stream it originally failed on, and then removes it from the DLQ.
//
// This takes the concrete *redisinfra.Queue rather than the generic
// provider.Queue interface — looking up and deleting one specific stream
// entry by ID isn't part of that interface (the regular pipeline only ever
// publishes and consumes), and this is an operator-triggered replay path,
// not something the pipeline itself calls.
func RetryFromDLQ(ctx context.Context, queue *redisinfra.Queue, entryID string) error {
	dlqEvent, err := queue.FetchStreamEntry(ctx, dlqStream, entryID)
	if err != nil {
		return fmt.Errorf("dlq: fetch entry %s: %w", entryID, err)
	}

	rawEntry, _ := dlqEvent.Payload["entry"].(string)
	if rawEntry == "" {
		return fmt.Errorf("dlq: entry %s has no payload", entryID)
	}

	var entry DLQEntry
	if err := json.Unmarshal([]byte(rawEntry), &entry); err != nil {
		return fmt.Errorf("dlq: unmarshal entry %s: %w", entryID, err)
	}

	if entry.OriginalStream == "" {
		return fmt.Errorf("dlq: entry %s has no original stream recorded", entryID)
	}

	if err := queue.Publish(ctx, entry.OriginalStream, entry.OriginalEvent); err != nil {
		return fmt.Errorf("dlq: republish entry %s to %s: %w", entryID, entry.OriginalStream, err)
	}

	if err := queue.DeleteStreamEntry(ctx, dlqStream, entryID); err != nil {
		// The entry was already republished at this point — log rather than
		// fail the call, since returning an error here could make a caller
		// retry and republish the same event a second time.
		log.Printf("dlq: entry %s replayed but failed to remove from DLQ: %v", entryID, err)
	}

	return nil
}