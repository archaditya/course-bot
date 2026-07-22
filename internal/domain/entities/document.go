package entities

import "time"

// SourceType enumerates supported raw content types.
type SourceType string

const (
	SourceTypeSRT   SourceType = "srt"
	SourceTypeVTT   SourceType = "vtt"
	SourceTypeVideo SourceType = "video"
	SourceTypePDF   SourceType = "pdf"
	SourceTypeDOCX  SourceType = "docx"
	SourceTypeGit   SourceType = "github"
	SourceTypeURL   SourceType = "url"
	SourceTypeText  SourceType = "text"
)

// Document is the raw uploaded artifact tied to a Lesson, plus a pointer to
// its parsed (NormalizedDocument) form. See docs/03-domain-model.md and
// docs/04-indexing-pipeline.md#internal-document-format.
type Document struct {
	ID          string
	LessonID    string
	CourseID    string // denormalized for query convenience; Lesson→Course is authoritative
	SourceType  SourceType
	StoragePath string // pointer into R2 `raw/`, immutable — see docs/07-storage.md
	// SourceURL is set for URL-based sources (video URL, web URL). Mutually
	// exclusive with file-upload StoragePath — one or the other is populated.
	SourceURL string
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
// See docs/04-indexing-pipeline.md#internal-document-format.
type NormalizedDocument struct {
	Metadata struct {
		SourceType       SourceType
		OriginalFilename string
		Checksum         string
	}
	Language             string // detected or declared
	SourceRef            string // pointer back to raw file in R2
	Timeline             bool   // true for srt/video/vtt, false for pdf/docx/url/text
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
