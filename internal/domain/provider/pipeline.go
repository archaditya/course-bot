package provider

import "course-assistant/internal/domain/entities"

// FileMeta describes the raw uploaded file being parsed.
type FileMeta struct {
	OriginalFilename string
	SourceType       entities.SourceType
	Checksum         string
}

// DocumentParser converts one raw source format into the shared
// NormalizedDocument shape. Adding a new content type (PDF, DOCX, video,
// GitHub, URL — see docs/04-indexing-pipeline.md#roadmap) is exactly one new
// implementation of this interface; nothing else in the pipeline changes.
// See docs/decisions/ADR-004-normalized-document.md.
type DocumentParser interface {
	Parse(raw []byte, meta FileMeta) (*entities.NormalizedDocument, error)
	SupportsType(t entities.SourceType) bool
}

// Chunker splits a NormalizedDocument's segments into retrievable Chunks.
// It only ever reads doc.Segments — never the original raw bytes.
type Chunker interface {
	Chunk(doc *entities.NormalizedDocument) ([]entities.Chunk, error)
}
