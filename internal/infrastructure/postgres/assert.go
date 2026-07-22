package postgres

import "archadilm/internal/domain/repository"

// Compile-time assertions that each repository actually implements the
// domain interface application/ will depend on. If one of these breaks, the
// mismatch is caught here at build time — not as a runtime wiring surprise
// in apps/api/cmd/main.go.
var (
	_ repository.UserRepository         = (*UserRepository)(nil)
	_ repository.RefreshTokenRepository = (*RefreshTokenRepository)(nil)
	_ repository.WorkspaceRepository    = (*WorkspaceRepository)(nil)
	_ repository.ProjectRepository      = (*ProjectRepository)(nil)
	_ repository.CourseRepository       = (*CourseRepository)(nil)
)
