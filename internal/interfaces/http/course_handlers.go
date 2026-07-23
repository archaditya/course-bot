package http

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"archadilm/internal/application/course"
	"archadilm/internal/domain/entities"
)

type CourseHandler struct{ svc *course.Service }

func NewCourseHandler(svc *course.Service) *CourseHandler { return &CourseHandler{svc: svc} }
func (h *CourseHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /projects/{project_id}/collections", h.create)
	mux.HandleFunc("GET /projects/{project_id}/collections", h.list)
	mux.HandleFunc("GET /collections/{id}", h.get)
	mux.HandleFunc("PATCH /collections/{id}", h.rename)
	mux.HandleFunc("DELETE /collections/{id}", h.delete)
	mux.HandleFunc("POST /projects/{project_id}/courses", h.create)
	mux.HandleFunc("GET /projects/{project_id}/courses", h.list)
	mux.HandleFunc("GET /courses/{id}", h.get)
	mux.HandleFunc("PATCH /courses/{id}", h.rename)
	mux.HandleFunc("DELETE /courses/{id}", h.delete)
}

type courseResponse struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func toCourseResponse(c *entities.Course) courseResponse {
	return courseResponse{ID: c.ID, Title: c.Title, Status: string(c.Status), CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt}
}

type createCourseRequest struct {
	Title string `json:"title"`
}

func (h *CourseHandler) create(w http.ResponseWriter, r *http.Request) {
	claims, ok := ClaimsFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Missing access token.")
		return
	}
	var req createCourseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Title == "" {
		WriteError(w, http.StatusBadRequest, "INVALID_REQUEST", "title is required.")
		return
	}
	c, err := h.svc.Create(r.Context(), claims.WorkspaceID, r.PathValue("project_id"), req.Title)
	if err != nil {
		notFoundOrInternal(w, err, "PROJECT_NOT_FOUND", "Project not found.")
		return
	}
	writeJSON(w, http.StatusCreated, toCourseResponse(c))
}
func (h *CourseHandler) list(w http.ResponseWriter, r *http.Request) {
	claims, ok := ClaimsFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Missing access token.")
		return
	}
	courses, next, err := h.svc.ListByProject(r.Context(), claims.WorkspaceID, r.PathValue("project_id"), r.URL.Query().Get("cursor"), parseLimit(r.URL.Query().Get("limit")))
	if err != nil {
		log.Printf("api: list collections error: %v", err)
		WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Could not list collections.")
		return
	}
	items := make([]courseResponse, 0, len(courses))
	for _, c := range courses {
		items = append(items, toCourseResponse(c))
	}
	writeJSON(w, http.StatusOK, listResponse[courseResponse]{Items: items, NextCursor: next})
}
func (h *CourseHandler) get(w http.ResponseWriter, r *http.Request) {
	claims, ok := ClaimsFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Missing access token.")
		return
	}
	c, err := h.svc.Get(r.Context(), claims.WorkspaceID, r.PathValue("id"))
	if err != nil {
		notFoundOrInternal(w, err, "COLLECTION_NOT_FOUND", "Collection not found.")
		return
	}
	writeJSON(w, http.StatusOK, toCourseResponse(c))
}
func (h *CourseHandler) rename(w http.ResponseWriter, r *http.Request) {
	claims, ok := ClaimsFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Missing access token.")
		return
	}
	var req createCourseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Title == "" {
		WriteError(w, http.StatusBadRequest, "INVALID_REQUEST", "title is required.")
		return
	}
	c, err := h.svc.Rename(r.Context(), claims.WorkspaceID, r.PathValue("id"), req.Title)
	if err != nil {
		notFoundOrInternal(w, err, "COLLECTION_NOT_FOUND", "Collection not found.")
		return
	}
	writeJSON(w, http.StatusOK, toCourseResponse(c))
}
func (h *CourseHandler) delete(w http.ResponseWriter, r *http.Request) {
	claims, ok := ClaimsFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Missing access token.")
		return
	}
	if err := h.svc.Delete(r.Context(), claims.WorkspaceID, r.PathValue("id")); err != nil {
		notFoundOrInternal(w, err, "COLLECTION_NOT_FOUND", "Collection not found.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
