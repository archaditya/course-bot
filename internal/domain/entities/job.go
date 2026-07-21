package entities

import (
	"fmt"
	"time"
)

// JobStage identifies which pipeline stage a Job tracks. See
// docs/04-indexing-pipeline.md#pipeline.
type JobStage string

const (
	JobStageManifest  JobStage = "manifest"
	JobStageParsing   JobStage = "parsing"
	JobStageNormalize JobStage = "normalizing"
	JobStageChunk     JobStage = "chunking"
	JobStageMetadata  JobStage = "metadata"
	JobStageEmbedding JobStage = "embedding"
	JobStageIndex     JobStage = "indexing"
)

// JobStatus mirrors the Job Lifecycle state machine in
// docs/03-domain-model.md#job-lifecycle.
type JobStatus string

const (
	JobStatusQueued       JobStatus = "QUEUED"
	JobStatusRunning      JobStatus = "RUNNING"
	JobStatusSucceeded    JobStatus = "SUCCEEDED"
	JobStatusRetrying     JobStatus = "RETRYING"
	JobStatusDeadLettered JobStatus = "DEAD_LETTERED"
)

var validJobTransitions = map[JobStatus][]JobStatus{
	JobStatusQueued:       {JobStatusRunning},
	JobStatusRunning:      {JobStatusSucceeded, JobStatusRetrying},
	JobStatusRetrying:     {JobStatusRunning, JobStatusDeadLettered},
	JobStatusSucceeded:    {},
	JobStatusDeadLettered: {},
}

// Job is a unit of background work (parse, chunk, embed, etc.) tracked
// through its own lifecycle, independent of Course state. See
// docs/03-domain-model.md and docs/09-deployment.md#error-handling.
type Job struct {
	ID              string
	CourseID        string
	DocumentID      *string // nullable: manifest-level jobs aren't per-document
	Stage           JobStage
	Status          JobStatus
	Attempts        int
	MaxAttempts     int // see docs/09-deployment.md#non-functional-requirements: retry cap of 3
	PipelineVersion string
	LastError       string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// CanTransitionTo reports whether moving from the Job's current status to
// `next` is a legal edge in docs/03-domain-model.md#job-lifecycle.
func (j *Job) CanTransitionTo(next JobStatus) bool {
	for _, allowed := range validJobTransitions[j.Status] {
		if allowed == next {
			return true
		}
	}
	return false
}

// TransitionTo mutates the Job's status if the transition is legal.
func (j *Job) TransitionTo(next JobStatus) error {
	if !j.CanTransitionTo(next) {
		return fmt.Errorf("illegal job status transition: %s -> %s", j.Status, next)
	}
	j.Status = next
	return nil
}
