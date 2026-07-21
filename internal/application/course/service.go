// Package course implements the Course CRUD use cases from
// docs/10-api-contracts.md#courses and docs/01-product-requirements.md#course-management
// (rename/delete). Re-indexing and the job-status list on
// GET /courses/:id/status are deferred until the indexing pipeline
// (docs/04-indexing-pipeline.md) and JobRepository wiring land.
package course

import (
	"context"
	"fmt"

	"course-assistant/internal/domain/entities"
	"course-assistant/internal/domain/repository"
)

type Service struct {
	courses repository.CourseRepository
}

func NewService(courses repository.CourseRepository) *Service {
	return &Service{courses: courses}
}

// Create makes a new Course in CREATED status — see the lifecycle state
// machine in docs/03-domain-model.md#course-lifecycle. It always starts
// there; nothing is allowed to construct a Course in any other status.
func (s *Service) Create(ctx context.Context, workspaceID, projectID, title string) (*entities.Course, error) {
	c := &entities.Course{
		ProjectID: projectID,
		Title:     title,
		Status:    entities.CourseStatusCreated,
	}
	if err := s.courses.Create(ctx, workspaceID, c); err != nil {
		return nil, fmt.Errorf("course: create: %w", err)
	}
	return c, nil
}

func (s *Service) Get(ctx context.Context, workspaceID, id string) (*entities.Course, error) {
	c, err := s.courses.GetByID(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (s *Service) ListByProject(ctx context.Context, workspaceID, projectID, cursor string, limit int) ([]*entities.Course, string, error) {
	return s.courses.ListByProject(ctx, workspaceID, projectID, cursor, limit)
}

func (s *Service) Rename(ctx context.Context, workspaceID, id, title string) (*entities.Course, error) {
	c, err := s.courses.GetByID(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	c.Title = title
	if err := s.courses.Update(ctx, workspaceID, c); err != nil {
		return nil, fmt.Errorf("course: rename: %w", err)
	}
	return c, nil
}

func (s *Service) Delete(ctx context.Context, workspaceID, id string) error {
	if err := s.courses.Delete(ctx, workspaceID, id); err != nil {
		return fmt.Errorf("course: delete: %w", err)
	}
	return nil
}
