package http

import (
	"net/http"

	"archadilm/internal/domain/repository"
)

type StatusHandler struct {
	courses repository.CourseRepository
	jobs    repository.JobRepository
}

func NewStatusHandler(courses repository.CourseRepository, jobs repository.JobRepository) *StatusHandler {
	return &StatusHandler{courses: courses, jobs: jobs}
}
func (h *StatusHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /courses/{courseID}/status", h.handleGetStatus)
	mux.HandleFunc("GET /collections/{courseID}/status", h.handleGetStatus)
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
	claims, ok := ClaimsFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Missing access token.")
		return
	}
	course, err := h.courses.GetByID(r.Context(), claims.WorkspaceID, r.PathValue("courseID"))
	if err != nil {
		WriteError(w, http.StatusNotFound, "NOT_FOUND", "Collection not found.")
		return
	}
	jobs, err := h.jobs.ListByCourse(r.Context(), course.ID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Could not load indexing status.")
		return
	}
	items := make([]jobStatusItem, len(jobs))
	for i, job := range jobs {
		items[i] = jobStatusItem{ID: job.ID, Stage: string(job.Stage), Status: string(job.Status), Attempts: job.Attempts, LastError: job.LastError}
	}
	writeJSON(w, http.StatusOK, courseStatusResponse{CourseID: course.ID, Status: string(course.Status), Jobs: items})
}
