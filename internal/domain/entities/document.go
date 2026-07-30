package entities

import (
	"fmt"
	"time"
)

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

// DocumentStatus is the indexing lifecycle of one uploaded source. Every
// Document tracks its own status independently — there is no course-level
// grouping status above it. A user watching their sidebar sees each source
// go UPLOADING -> ... -> INDEXED (or FAILED) on its own, and the chat
// becomes usable against that source as soon as it alone reaches INDEXED.
type DocumentStatus string

const (
	DocumentStatusUploading   DocumentStatus = "UPLOADING"
	DocumentStatusUploaded    DocumentStatus = "UPLOADED"
	DocumentStatusParsing     DocumentStatus = "PARSING"
	DocumentStatusNormalizing DocumentStatus = "NORMALIZING"
	DocumentStatusChunking    DocumentStatus = "CHUNKING"
	DocumentStatusEmbedding   DocumentStatus = "EMBEDDING"
	DocumentStatusIndexed     DocumentStatus = "INDEXED"
	DocumentStatusFailed      DocumentStatus = "FAILED"
)

// validDocumentTransitions encodes the allowed edges of the pipeline state
// diagram. Kept on the entity (not in application/) because "what
// transitions are legal" is a property of the Document itself.
var validDocumentTransitions = map[DocumentStatus][]DocumentStatus{
	DocumentStatusUploading:   {DocumentStatusUploaded, DocumentStatusFailed},
	DocumentStatusUploaded:    {DocumentStatusParsing},
	DocumentStatusParsing:     {DocumentStatusNormalizing, DocumentStatusFailed},
	DocumentStatusNormalizing: {DocumentStatusChunking, DocumentStatusFailed},
	DocumentStatusChunking:    {DocumentStatusEmbedding, DocumentStatusFailed},
	DocumentStatusEmbedding:   {DocumentStatusIndexed, DocumentStatusFailed},
	DocumentStatusIndexed:     {},
	// FAILED --> retry goes back to PARSING.
	DocumentStatusFailed: {DocumentStatusParsing},
}

// CanTransitionTo reports whether moving from the Document's current status
// to `next` is a legal edge in the lifecycle state machine.
func (d *Document) CanTransitionTo(next DocumentStatus) bool {
	for _, allowed := range validDocumentTransitions[d.Status] {
		if allowed == next {
			return true
		}
	}
	return false
}

// TransitionTo mutates the Document's status if the transition is legal,
// otherwise returns an error. Callers in application/ should always go
// through this rather than assigning Status directly.
func (d *Document) TransitionTo(next DocumentStatus) error {
	if !d.CanTransitionTo(next) {
		return fmt.Errorf("illegal document status transition: %s -> %s", d.Status, next)
	}
	d.Status = next
	return nil
}

// Document is one uploaded/added source (a PDF, a pasted text, a YouTube
// URL, or one file extracted from a ZIP), scoped directly to the
// Conversation it was added to. Each conversation is its own notebook: its
// own sources, and retrieval that only ever searches within that
// conversation's own documents — never another conversation's.
type Document struct {
	ID             string
	ConversationID string
	SourceType     SourceType
	Status         DocumentStatus
	StoragePath    string // pointer into R2 `raw/`, immutable
	// SourceURL is set for URL-based sources (video URL, web URL). Mutually
	// exclusive with file-upload StoragePath — one or the other is populated.
	SourceURL string
	// NormalizedRef points at the processed NormalizedDocument in R2
	// `processed/`, produced by the Parser Worker. Nil until PARSING succeeds.
	NormalizedRef        *string
	NormalizationVersion *string
	OriginalFilename     string
	Checksum             string
	// AI-generated source intelligence — populated after indexing completes.
	AISummary   string   // NotebookLM-style 2-paragraph Source Guide
	AIQuestions []string // content-specific suggested questions
	AIOverview  string   // friendly welcome message for the user
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// NormalizedDocument is the shared intermediate shape every parser converges
// on before anything downstream (chunking, embedding, retrieval) touches it.
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

// Segment is one entry in a NormalizedDocument's segments[]. The Chunk
// Worker only ever reads segments — never the raw file — which is the seam
// that keeps adding a content type isolated to a single parser.
type Segment struct {
	SegmentID string
	Text      string
	StartTS   *int // nullable
	EndTS     *int // nullable
	Speaker   *string
	Page      *int
}
