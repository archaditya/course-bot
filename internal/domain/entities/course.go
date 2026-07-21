package entities

import (
	"fmt"
	"time"
)

// CourseStatus mirrors the Course Lifecycle state machine in
// docs/03-domain-model.md#course-lifecycle.
type CourseStatus string

const (
	CourseStatusCreated     CourseStatus = "CREATED"
	CourseStatusUploading   CourseStatus = "UPLOADING"
	CourseStatusUploaded    CourseStatus = "UPLOADED"
	CourseStatusParsing     CourseStatus = "PARSING"
	CourseStatusNormalizing CourseStatus = "NORMALIZING"
	CourseStatusChunking    CourseStatus = "CHUNKING"
	CourseStatusEmbedding   CourseStatus = "EMBEDDING"
	CourseStatusIndexed     CourseStatus = "INDEXED"
	CourseStatusFailed      CourseStatus = "FAILED"
)

// validCourseTransitions encodes the allowed edges of the state diagram in
// docs/03-domain-model.md#course-lifecycle. Kept here (not in application/)
// because "what transitions are legal" is a property of the entity itself,
// not of any particular use case.
var validCourseTransitions = map[CourseStatus][]CourseStatus{
	CourseStatusCreated:     {CourseStatusUploading},
	CourseStatusUploading:   {CourseStatusUploaded, CourseStatusFailed},
	CourseStatusUploaded:    {CourseStatusParsing},
	CourseStatusParsing:     {CourseStatusNormalizing, CourseStatusFailed},
	CourseStatusNormalizing: {CourseStatusChunking, CourseStatusFailed},
	CourseStatusChunking:    {CourseStatusEmbedding, CourseStatusFailed},
	CourseStatusEmbedding:   {CourseStatusIndexed, CourseStatusFailed},
	CourseStatusIndexed:     {},
	// FAILED --> retry goes back to PARSING per the diagram.
	CourseStatusFailed: {CourseStatusParsing},
}

// Course is a single body of material (e.g. one course's transcripts).
// See docs/03-domain-model.md.
type Course struct {
	ID        string
	ProjectID string
	Title     string
	Status    CourseStatus
	CreatedAt time.Time
	UpdatedAt time.Time
}

// CanTransitionTo reports whether moving from the Course's current status to
// `next` is a legal edge in the lifecycle state machine.
func (c *Course) CanTransitionTo(next CourseStatus) bool {
	for _, allowed := range validCourseTransitions[c.Status] {
		if allowed == next {
			return true
		}
	}
	return false
}

// TransitionTo mutates the Course's status if the transition is legal,
// otherwise returns an error. Callers in application/ should always go
// through this rather than assigning Status directly.
func (c *Course) TransitionTo(next CourseStatus) error {
	if !c.CanTransitionTo(next) {
		return fmt.Errorf("illegal course status transition: %s -> %s", c.Status, next)
	}
	c.Status = next
	return nil
}

// Module is an optional logical grouping within a course (e.g. "Week 3").
// Reserved in the model for future use, not required for MVP.
type Module struct {
	ID        string
	CourseID  string
	Title     string
	Position  int
	CreatedAt time.Time
}

// Lesson is the unit that maps to one uploaded source file.
// A Course contains many Lessons; a Lesson optionally belongs to a Module.
type Lesson struct {
	ID        string
	CourseID  string
	ModuleID  *string // nullable: MVP may not use Modules
	Title     string
	Position  int
	CreatedAt time.Time
}
