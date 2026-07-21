package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"course-assistant/internal/domain/entities"
	"course-assistant/internal/domain/repository"
)

type lessonRepository struct{ db *sql.DB }

func NewLessonRepository(db *sql.DB) repository.LessonRepository {
	return &lessonRepository{db: db}
}

func (r *lessonRepository) Create(ctx context.Context, l *entities.Lesson) error {
	const q = `INSERT INTO lessons (id, course_id, module_id, title, position) VALUES ($1,$2,$3,$4,$5)`
	_, err := r.db.ExecContext(ctx, q, l.ID, l.CourseID, nil, l.Title, l.Position)
	if err != nil {
		return fmt.Errorf("lesson: create: %w", err)
	}
	return nil
}

func (r *lessonRepository) ListByCourse(ctx context.Context, courseID string) ([]*entities.Lesson, error) {
	const q = `SELECT id, course_id, module_id, title, position, created_at FROM lessons WHERE course_id=$1 ORDER BY position`
	rows, err := r.db.QueryContext(ctx, q, courseID)
	if err != nil {
		return nil, fmt.Errorf("lesson: list: %w", err)
	}
	defer rows.Close()
	var lessons []*entities.Lesson
	for rows.Next() {
		var l entities.Lesson
		var moduleID sql.NullString
		if err := rows.Scan(&l.ID, &l.CourseID, &moduleID, &l.Title, &l.Position, &l.CreatedAt); err != nil {
			return nil, err
		}
		if moduleID.Valid {
			l.ModuleID = &moduleID.String
		}
		lessons = append(lessons, &l)
	}
	return lessons, rows.Err()
}
