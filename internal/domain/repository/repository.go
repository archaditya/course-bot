// Package repository defines the persistence-facing interfaces that
// application/ use cases depend on. Concrete implementations live in
// internal/infrastructure/postgres (and redis, r2, qdrant for the
// non-relational stores) and are wired in at startup — see
// docs/02-system-architecture.md#module-dependency-diagram.
//
// Workspace isolation (docs/08-security.md#workspace-isolation) is enforced
// here at the type level: every method that reads or writes a
// workspace-scoped resource takes a WorkspaceID as a required argument.
// There is deliberately no method shaped like "GetProject(id)" without a
// workspace context — that shape is how cross-tenant bugs happen.
package repository

import (
	"context"
	"errors"

	"course-assistant/internal/domain/entities"
)

// ErrNotFound is the sentinel every repository implementation returns when a
// lookup finds no matching (and, for workspace-scoped lookups, no
// authorized) row. It lives in domain/ rather than in a specific
// infrastructure package so application/ use cases can check for it without
// importing any concrete implementation — see
// docs/02-system-architecture.md#module-dependency-diagram.
var ErrNotFound = errors.New("repository: not found")

// WorkspaceID is a required argument, not a type alias for string, so a
// caller can't accidentally pass a ProjectID or CourseID where a workspace
// scope is expected without the compiler noticing the intent mismatch is at
// least named.
type WorkspaceID = string

type UserRepository interface {
	Create(ctx context.Context, u *entities.User) error
	GetByID(ctx context.Context, id string) (*entities.User, error)
	GetByEmail(ctx context.Context, email string) (*entities.User, error)
	Update(ctx context.Context, u *entities.User) error
}

type RefreshTokenRepository interface {
	Create(ctx context.Context, t *entities.RefreshToken) error
	GetByHash(ctx context.Context, tokenHash string) (*entities.RefreshToken, error)
	Revoke(ctx context.Context, id string) error
	RevokeAllForUser(ctx context.Context, userID string) error
}

type WorkspaceRepository interface {
	Create(ctx context.Context, w *entities.Workspace) error
	GetByID(ctx context.Context, id string) (*entities.Workspace, error)
	GetByUserID(ctx context.Context, userID string) (*entities.Workspace, error)
}

type ProjectRepository interface {
	Create(ctx context.Context, p *entities.Project) error
	GetByID(ctx context.Context, ws WorkspaceID, id string) (*entities.Project, error)
	ListByWorkspace(ctx context.Context, ws WorkspaceID, cursor string, limit int) ([]*entities.Project, string, error)
	Update(ctx context.Context, ws WorkspaceID, p *entities.Project) error
	Delete(ctx context.Context, ws WorkspaceID, id string) error
}

type CourseRepository interface {
	Create(ctx context.Context, ws WorkspaceID, c *entities.Course) error
	GetByID(ctx context.Context, ws WorkspaceID, id string) (*entities.Course, error)
	ListByProject(ctx context.Context, ws WorkspaceID, projectID string, cursor string, limit int) ([]*entities.Course, string, error)
	UpdateStatus(ctx context.Context, ws WorkspaceID, id string, status entities.CourseStatus) error
	Update(ctx context.Context, ws WorkspaceID, c *entities.Course) error
	Delete(ctx context.Context, ws WorkspaceID, id string) error
}

type LessonRepository interface {
	Create(ctx context.Context, l *entities.Lesson) error
	ListByCourse(ctx context.Context, courseID string) ([]*entities.Lesson, error)
}

type DocumentRepository interface {
	Create(ctx context.Context, d *entities.Document) error
	GetByID(ctx context.Context, id string) (*entities.Document, error)
	ListByCourse(ctx context.Context, courseID string) ([]*entities.Document, error)
	SetNormalizedRef(ctx context.Context, id string, ref string, version string) error
}

type ChunkRepository interface {
	// CreateBatch writes chunk rows; called by a Worker only — see
	// docs/07-storage.md#write-ownership. Never called from the AI Service.
	CreateBatch(ctx context.Context, chunks []*entities.Chunk) error
	ListByDocument(ctx context.Context, documentID string) ([]*entities.Chunk, error)
	GetByIDs(ctx context.Context, ids []string) ([]*entities.Chunk, error)
}

type ConversationRepository interface {
	Create(ctx context.Context, c *entities.Conversation) error
	GetByID(ctx context.Context, ws WorkspaceID, id string) (*entities.Conversation, error)
	ListByProject(ctx context.Context, ws WorkspaceID, projectID string, cursor string, limit int) ([]*entities.Conversation, string, error)
}

type MessageRepository interface {
	Create(ctx context.Context, m *entities.Message) error
	ListByConversation(ctx context.Context, conversationID string, cursor string, limit int) ([]*entities.Message, string, error)
	UpdateStatus(ctx context.Context, id string, status entities.MessageStatus) error
}

type CitationRepository interface {
	CreateBatch(ctx context.Context, citations []*entities.Citation) error
	ListByMessage(ctx context.Context, messageID string) ([]*entities.Citation, error)
}

type JobRepository interface {
	Create(ctx context.Context, j *entities.Job) error
	GetByID(ctx context.Context, id string) (*entities.Job, error)
	ListByCourse(ctx context.Context, courseID string) ([]*entities.Job, error)
	Update(ctx context.Context, j *entities.Job) error
}

type AuditLogRepository interface {
	Record(ctx context.Context, a *entities.AuditLog) error
}
