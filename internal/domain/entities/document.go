package entities

import "time"

// SourceType enumerates supported raw content types. MVP is SRT only; the
// rest are the roadmap in docs/04-indexing-pipeline.md#roadmap. Adding a new
// type here plus one DocumentParser implementation (see
// internal/domain/provider) is the entire cost of onboarding a new format.
type SourceType string

const (
	SourceTypeSRT   SourceType = "srt"
	SourceTypeVideo SourceType = "video"
	SourceTypePDF   SourceType = "pdf"
	SourceTypeDOCX  SourceType = "docx"
	SourceTypeGit   SourceType = "github"
	SourceTypeURL   SourceType = "url"
)

// Document is the raw uploaded artifact tied to a Lesson, plus a pointer to
// its parsed (NormalizedDocument) form. See docs/03-domain-model.md and
// docs/04-indexing-pipeline.md#internal-document-format.
type Document struct {
	ID          string
	LessonID    string
	CourseID    string // denormalized for query convenience; Lesson->Course is authoritative
	SourceType  SourceType
	StoragePath string // pointer into R2 `raw/`, immutable — see docs/07-storage.md
	// NormalizedRef points at the processed NormalizedDocument in R2
	// `processed/`, produced by the Parser Worker. Nil until PARSING succeeds.
	NormalizedRef        *string
	NormalizationVersion *string
	OriginalFilename     string
	Checksum             string
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// NormalizedDocument is the shared intermediate shape every parser converges
// on before anything downstream (chunking, embedding, retrieval) touches it.
// See docs/04-indexing-pipeline.md#internal-document-format and
// docs/decisions/ADR-004-normalized-document.md. This is an in-flight/transfer
// shape (stored as processed/ bytes in R2, referenced by Document.NormalizedRef)
// rather than a Postgres-backed entity of its own.
type NormalizedDocument struct {
	Metadata struct {
		SourceType       SourceType
		OriginalFilename string
		Checksum         string
	}
	Language             string // detected or declared
	SourceRef            string // pointer back to raw file in R2
	Timeline             bool   // true for srt/video, false for pdf/docx
	NormalizationVersion string
	Segments             []Segment
}

// Segment is one entry in a NormalizedDocument's segments[]. The Chunk Worker
// only ever reads segments — never the raw file — which is the seam that
// keeps adding a content type isolated to a single parser.
type Segment struct {
	SegmentID string
	Text      string
	StartTS   *int // nullable
	EndTS     *int // nullable
	Speaker   *string
	Page      *int
}
