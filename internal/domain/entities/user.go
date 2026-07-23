package entities

import "time"

// AuthProvider identifies how a User authenticated.
type AuthProvider string

const (
	AuthProviderGoogle   AuthProvider = "google"
	AuthProviderPassword AuthProvider = "password"
)

// User is a person with an account. See docs/03-domain-model.md.
// A User authenticates via Google or email/password, and owns Workspaces.
type User struct {
	ID           string
	FullName     string
	Email        string
	PasswordHash string // empty when AuthProvider is google
	AuthProvider AuthProvider
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
