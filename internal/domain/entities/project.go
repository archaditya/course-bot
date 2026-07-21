package entities

import "time"

// Project is a folder-like grouping of Courses and Chats/Conversations,
// e.g. "Machine Learning Bootcamp". See docs/03-domain-model.md.
type Project struct {
	ID          string
	WorkspaceID string
	Name        string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
