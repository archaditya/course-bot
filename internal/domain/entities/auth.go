package entities

import "time"

// RefreshToken backs JWT rotation per docs/08-security.md#jwt-rotation:
// long-lived, rotated on every use, stored hashed (never plaintext).
// Each refresh invalidates the previous token by marking it RevokedAt.
type RefreshToken struct {
	ID        string
	UserID    string
	TokenHash string
	ExpiresAt time.Time
	RevokedAt *time.Time
	CreatedAt time.Time
}

// AuditLog is an append-only record of who did what (delete course,
// re-index, login), separate from application logs. See
// docs/09-deployment.md#observability.
type AuditLog struct {
	ID        string
	UserID    string
	Action    string
	Resource  string // e.g. "course:<uuid>"
	Metadata  string // JSON blob, kept opaque at the domain layer
	CreatedAt time.Time
}
