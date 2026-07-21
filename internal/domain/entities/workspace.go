package entities

import "time"

// Workspace is the billing/ownership boundary for a user (or, later, a team).
// Reserved for the multi-team phase; in MVP it's 1:1 with User.
// See docs/03-domain-model.md and docs/08-security.md#workspace-isolation.
type Workspace struct {
	ID        string
	UserID    string // owner, MVP: 1:1
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}
