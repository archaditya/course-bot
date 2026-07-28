package postgres

import "archadilm/internal/domain/repository"

// Compile-time assertions that each repository actually implements the
// domain interface application/ will depend on.
var (
	_ repository.UserRepository         = (*UserRepository)(nil)
	_ repository.RefreshTokenRepository = (*RefreshTokenRepository)(nil)
	_ repository.WorkspaceRepository    = (*WorkspaceRepository)(nil)
)
