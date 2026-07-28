package entities

import "time"

// Chunk is a retrievable slice of a Document, with its own timestamp range,
// embedding pointer, and generated metadata. Postgres stores this row in
// full; Qdrant stores only the vector plus a minimal filter payload
// (chunk_id, conversation_id, document_id, start_timestamp). Retrieval for
// a conversation only ever searches chunks stamped with that conversation's
// own ID — never another conversation's.
type Chunk struct {
	ID               string
	DocumentID       string
	DocumentName     string // e.g. "03_props-style-props_epm.vtt"
	SourceType       string // e.g. "vtt", "pdf", "url"
	SourceURL        string
	ConversationID   string // denormalized: the Qdrant payload filter key for per-conversation isolation
	StartTimestamp   *int   // nullable — page-based sources have no timestamp
	EndTimestamp     *int   // nullable
	PageNumber       *int   // nullable — timeline-based sources have no page
	Title            string // short, generated — used in citation UI
	Summary          string // 1-2 sentence generated summary, used for reranking context
	Content          string // the actual retrievable text
	TokenCount       int
	EmbeddingVersion string
	VectorRef        string // pointer to the vector's ID in Qdrant, not the vector itself
	CreatedAt        time.Time
}
