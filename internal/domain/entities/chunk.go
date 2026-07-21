package entities

import "time"

// Chunk is a retrievable slice of a Document, with its own timestamp range,
// embedding pointer, and generated metadata. Postgres stores this row in
// full; Qdrant stores only the vector plus a minimal filter payload
// (chunk_id, course_id, start_timestamp) — see docs/07-storage.md and
// docs/04-indexing-pipeline.md#chunk-schema.
type Chunk struct {
	ID               string
	DocumentID       string
	CourseID         string // denormalized: needed as a Qdrant payload filter for workspace/course isolation
	StartTimestamp   *int   // nullable — page-based sources have no timestamp
	EndTimestamp     *int   // nullable
	PageNumber       *int   // nullable — timeline-based sources have no page
	Title            string // short, generated — used in citation UI
	Summary          string // 1-2 sentence generated summary, used for reranking context
	Content          string // the actual retrievable text
	TokenCount       int
	EmbeddingVersion string // see docs/04-indexing-pipeline.md#versioning-strategy
	VectorRef        string // pointer to the vector's ID in Qdrant, not the vector itself
	CreatedAt        time.Time
}
