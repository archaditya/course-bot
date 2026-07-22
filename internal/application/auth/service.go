// Package auth implements the signup/login/refresh use cases from
// docs/01-product-requirements.md#authentication. Authorization decisions
// (who can do what) live here, not in interfaces/http, per
// docs/08-security.md#authorization — HTTP handlers only translate
// requests/responses.
package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"archadilm/internal/domain/entities"
	"archadilm/internal/domain/repository"
	"archadilm/internal/infrastructure/security"
)

var (
	ErrEmailTaken         = errors.New("auth: email already registered")
	ErrInvalidCredentials = errors.New("auth: invalid email or password")
	ErrInvalidRefresh     = errors.New("auth: invalid or expired refresh token")
)

// AccessTokenTTL and RefreshTokenTTL match docs/08-security.md#jwt-rotation:
// access tokens are short-lived, refresh tokens long-lived but rotated on
// every use.
const (
	AccessTokenTTL  = 15 * time.Minute
	RefreshTokenTTL = 30 * 24 * time.Hour
)

type Tokens struct {
	AccessToken  string
	RefreshToken string
}

type Service struct {
	users         repository.UserRepository
	workspaces    repository.WorkspaceRepository
	refreshTokens repository.RefreshTokenRepository
	jwtSigningKey string
}

func NewService(
	users repository.UserRepository,
	workspaces repository.WorkspaceRepository,
	refreshTokens repository.RefreshTokenRepository,
	jwtSigningKey string,
) *Service {
	return &Service{
		users:         users,
		workspaces:    workspaces,
		refreshTokens: refreshTokens,
		jwtSigningKey: jwtSigningKey,
	}
}

// SignUp creates a User plus its 1:1 Workspace (MVP — see
// docs/03-domain-model.md#entities) for email/password auth. Google OAuth
// signup is a separate, not-yet-implemented path (docs/01-product-requirements.md#authentication);
// this only covers AuthProviderPassword.
func (s *Service) SignUp(ctx context.Context, email, password string) (*entities.User, error) {
	if _, err := s.users.GetByEmail(ctx, email); err == nil {
		return nil, ErrEmailTaken
	} else if !errors.Is(err, repository.ErrNotFound) {
		return nil, fmt.Errorf("auth: checking existing email: %w", err)
	}

	hash, err := security.HashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("auth: hashing password: %w", err)
	}

	user := &entities.User{
		Email:        email,
		PasswordHash: hash,
		AuthProvider: entities.AuthProviderPassword,
	}
	if err := s.users.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("auth: creating user: %w", err)
	}

	workspace := &entities.Workspace{UserID: user.ID, Name: "Personal Workspace"}
	if err := s.workspaces.Create(ctx, workspace); err != nil {
		return nil, fmt.Errorf("auth: creating workspace: %w", err)
	}

	return user, nil
}

// Login verifies credentials and issues a fresh access/refresh token pair.
func (s *Service) Login(ctx context.Context, email, password string) (*entities.User, Tokens, error) {
	user, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, Tokens{}, ErrInvalidCredentials
		}
		return nil, Tokens{}, fmt.Errorf("auth: looking up user: %w", err)
	}
	if user.AuthProvider != entities.AuthProviderPassword {
		return nil, Tokens{}, ErrInvalidCredentials
	}
	ok, err := security.VerifyPassword(user.PasswordHash, password)
	if err != nil {
		return nil, Tokens{}, fmt.Errorf("auth: verifying password: %w", err)
	}
	if !ok {
		return nil, Tokens{}, ErrInvalidCredentials
	}

	workspace, err := s.workspaces.GetByUserID(ctx, user.ID)
	if err != nil {
		return nil, Tokens{}, fmt.Errorf("auth: looking up workspace: %w", err)
	}

	tokens, err := s.issueTokens(ctx, user.ID, workspace.ID)
	if err != nil {
		return nil, Tokens{}, err
	}
	return user, tokens, nil
}

// Refresh rotates a refresh token: the presented token is revoked and a new
// pair is issued, per docs/08-security.md#jwt-rotation ("each refresh
// invalidates the previous refresh token").
func (s *Service) Refresh(ctx context.Context, refreshToken string) (Tokens, error) {
	hash := security.HashRefreshToken(refreshToken)
	stored, err := s.refreshTokens.GetByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return Tokens{}, ErrInvalidRefresh
		}
		return Tokens{}, fmt.Errorf("auth: looking up refresh token: %w", err)
	}
	if stored.RevokedAt != nil || time.Now().After(stored.ExpiresAt) {
		return Tokens{}, ErrInvalidRefresh
	}

	workspace, err := s.workspaces.GetByUserID(ctx, stored.UserID)
	if err != nil {
		return Tokens{}, fmt.Errorf("auth: looking up workspace: %w", err)
	}

	if err := s.refreshTokens.Revoke(ctx, stored.ID); err != nil {
		return Tokens{}, fmt.Errorf("auth: revoking used refresh token: %w", err)
	}

	return s.issueTokens(ctx, stored.UserID, workspace.ID)
}

func (s *Service) issueTokens(ctx context.Context, userID, workspaceID string) (Tokens, error) {
	access, err := security.SignAccessToken(s.jwtSigningKey, userID, workspaceID, AccessTokenTTL)
	if err != nil {
		return Tokens{}, fmt.Errorf("auth: signing access token: %w", err)
	}

	refresh, refreshHash, err := security.GenerateRefreshToken()
	if err != nil {
		return Tokens{}, fmt.Errorf("auth: generating refresh token: %w", err)
	}
	record := &entities.RefreshToken{
		UserID:    userID,
		TokenHash: refreshHash,
		ExpiresAt: time.Now().Add(RefreshTokenTTL),
	}
	if err := s.refreshTokens.Create(ctx, record); err != nil {
		return Tokens{}, fmt.Errorf("auth: storing refresh token: %w", err)
	}

	return Tokens{AccessToken: access, RefreshToken: refresh}, nil
}
