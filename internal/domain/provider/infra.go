package provider

import (
	"context"
	"time"
)

// Event is one message on the pipeline event backbone. Payload is kept as a
// map rather than per-event structs so the Queue interface itself doesn't
// need to change every time docs/04-indexing-pipeline.md#event-contracts
// gains a new event — event-specific (de)serialization lives in
// application/, right next to the event contract table it implements.
type Event struct {
	Name    string // e.g. "UPLOAD_COMPLETED" — see event-contracts table
	Payload map[string]any
	TraceID string
}

// Queue is the event backbone abstraction from docs/decisions/ADR-003-redis-streams.md.
// application/ code depends only on this interface; internal/infrastructure/redis
// is the only place that ever imports a Redis client. Swapping the backbone
// (e.g. to Kafka at higher scale, per the ADR's "Revisit if") touches only
// that one infrastructure package.
type Queue interface {
	Publish(ctx context.Context, stream string, e Event) error
	// Consume starts a consumer-group reader for `stream`/`group` and returns
	// a channel of events plus an ack function the caller must invoke after
	// successful processing (consumer-group semantics per ADR-003).
	Consume(ctx context.Context, stream string, group string) (<-chan QueuedEvent, error)
}

type QueuedEvent struct {
	Event
	Ack func(ctx context.Context) error
}

// VectorPoint is what gets written to the vector store: a vector plus the
// minimal payload needed for filtered search (course_id, timestamp) — see
// docs/07-storage.md and docs/decisions/ADR-002-qdrant.md. Only Go Workers
// ever call Upsert (docs/07-storage.md#access-patterns); the AI Service
// reads via Search at query time through a Go proxy, never writes.
type VectorPoint struct {
	ChunkID        string
	CourseID       string
	StartTimestamp *int
	Vector         Vector
}

type VectorSearchResult struct {
	ChunkID string
	Score   float64
}

// VectorStore is the Qdrant abstraction. Treated as a derived, rebuildable
// index (ADR-002): safe to wipe and rebuild from Postgres + R2 at any time,
// which is why Upsert takes full points rather than supporting partial
// mutation.
type VectorStore interface {
	Upsert(ctx context.Context, points []VectorPoint) error
	Search(ctx context.Context, courseID string, query Vector, topK int) ([]VectorSearchResult, error)
	DeleteByCourse(ctx context.Context, courseID string) error
}

// ObjectStore is the R2 abstraction (docs/07-storage.md). Raw files in
// `raw/` are immutable; `processed/` holds normalized documents and other
// generated artifacts. The frontend never talks to this directly — the Go
// API issues short-lived signed URLs (docs/08-security.md#r2-signed-urls)
// and the browser uses those instead.
type ObjectStore interface {
	Put(ctx context.Context, key string, data []byte, contentType string) error
	Get(ctx context.Context, key string) ([]byte, error)
	// SignedPutURL issues a short-lived, single-use upload URL scoped to one
	// object key, per docs/08-security.md#r2-signed-urls.
	SignedPutURL(ctx context.Context, key string, expiry time.Duration) (string, error)
	SignedGetURL(ctx context.Context, key string, expiry time.Duration) (string, error)
}
