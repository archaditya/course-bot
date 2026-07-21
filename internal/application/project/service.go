// Package project implements the Project CRUD use cases from
// docs/10-api-contracts.md#projects. Every method requires a workspace
// context (docs/08-security.md#workspace-isolation) — there is no method
// shaped like "GetByID(id)" without one.
package project

import (
	"context"
	"fmt"

	"course-assistant/internal/domain/entities"
	"course-assistant/internal/domain/repository"
)

type Service struct {
	projects repository.ProjectRepository
}

func NewService(projects repository.ProjectRepository) *Service {
	return &Service{projects: projects}
}

func (s *Service) Create(ctx context.Context, workspaceID, name string) (*entities.Project, error) {
	p := &entities.Project{WorkspaceID: workspaceID, Name: name}
	if err := s.projects.Create(ctx, p); err != nil {
		return nil, fmt.Errorf("project: create: %w", err)
	}
	return p, nil
}

func (s *Service) Get(ctx context.Context, workspaceID, id string) (*entities.Project, error) {
	p, err := s.projects.GetByID(ctx, workspaceID, id)
	if err != nil {
		return nil, err // callers check errors.Is(err, repository.ErrNotFound)
	}
	return p, nil
}

func (s *Service) List(ctx context.Context, workspaceID, cursor string, limit int) ([]*entities.Project, string, error) {
	return s.projects.ListByWorkspace(ctx, workspaceID, cursor, limit)
}

func (s *Service) Rename(ctx context.Context, workspaceID, id, name string) (*entities.Project, error) {
	p, err := s.projects.GetByID(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	p.Name = name
	if err := s.projects.Update(ctx, workspaceID, p); err != nil {
		return nil, fmt.Errorf("project: rename: %w", err)
	}
	return p, nil
}

func (s *Service) Delete(ctx context.Context, workspaceID, id string) error {
	if err := s.projects.Delete(ctx, workspaceID, id); err != nil {
		return fmt.Errorf("project: delete: %w", err)
	}
	return nil
}
