package http

import (
	"encoding/json"
	"net/http"

	"archadilm/internal/domain/repository"
)

// StatusHandler serves polling endpoints for course processing status.
type StatusHandler struct {
	courses repository.CourseRepository
	jobs    repository.JobRepository
}

// NewStatusHandler creates a StatusHandler.
func NewStatusHandler(courses repository.CourseRepository, jobs repository.JobRepository) *StatusHandler {
	return &StatusHandler{courses: courses, jobs: jobs}
}

// Register mounts the status routes on the given mux.
func (h *StatusHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /courses/{courseID}/status", h.handleGetStatus)
}

type courseStatusResponse struct {
	CourseID string          `json:"course_id"`
	Status   string          `json:"status"`
	Jobs     []jobStatusItem `json:"jobs"`
}

type jobStatusItem struct {
	ID        string `json:"id"`
	Stage     string `json:"stage"`
	Status    string `json:"status"`
	Attempts  int    `json:"attempts"`
	LastError string `json:"last_error,omitempty"`
}

func (h *StatusHandler) handleGetStatus(w http.ResponseWriter, r *http.Request) {
	courseID := r.PathValue("courseID")
	ws := r.Header.Get("X-Workspace-ID")

	course, err := h.courses.GetByID(r.Context(), ws, courseID)
	if err != nil {
		WriteError(w, http.StatusNotFound, "NOT_FOUND", "course not found")
		return
	}

	jobs, err := h.jobs.ListByCourse(r.Context(), courseID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load jobs")
		return
	}

	jobItems := make([]jobStatusItem, len(jobs))
	for i, j := range jobs {
		jobItems[i] = jobStatusItem{
			ID:        j.ID,
			Stage:     string(j.Stage),
			Status:    string(j.Status),
			Attempts:  j.Attempts,
			LastError: j.LastError,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(courseStatusResponse{
		CourseID: course.ID,
		Status:   string(course.Status),
		Jobs:     jobItems,
	})
}
