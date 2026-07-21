package http

import (
	"encoding/json"
	"log"
	"net/http"

	"course-assistant/internal/application/project"
	"course-assistant/internal/domain/entities"
)

type ProjectHandler struct {
	svc *project.Service
}

func NewProjectHandler(svc *project.Service) *ProjectHandler {
	return &ProjectHandler{svc: svc}
}

func (h *ProjectHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /projects", h.create)
	mux.HandleFunc("GET /projects", h.list)
	mux.HandleFunc("GET /projects/{id}", h.get)
	mux.HandleFunc("PATCH /projects/{id}", h.rename)
	mux.HandleFunc("DELETE /projects/{id}", h.delete)
}

type projectResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func toProjectResponse(p *entities.Project) projectResponse {
	return projectResponse{ID: p.ID, Name: p.Name}
}

type createProjectRequest struct {
	Name string `json:"name"`
}

func (h *ProjectHandler) create(w http.ResponseWriter, r *http.Request) {
	claims, ok := ClaimsFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Missing access token.")
		return
	}

	var req createProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		WriteError(w, http.StatusBadRequest, "INVALID_REQUEST", "name is required.")
		return
	}

	p, err := h.svc.Create(r.Context(), claims.WorkspaceID, req.Name)
	if err != nil {
		log.Printf("api: create project error: %v", err)
		WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Could not create project.")
		return
	}
	writeJSON(w, http.StatusCreated, toProjectResponse(p))
}

func (h *ProjectHandler) list(w http.ResponseWriter, r *http.Request) {
	claims, ok := ClaimsFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Missing access token.")
		return
	}

	cursor := r.URL.Query().Get("cursor")
	limit := parseLimit(r.URL.Query().Get("limit"))

	projects, next, err := h.svc.List(r.Context(), claims.WorkspaceID, cursor, limit)
	if err != nil {
		log.Printf("api: list projects error: %v", err)
		WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Could not list projects.")
		return
	}

	items := make([]projectResponse, 0, len(projects))
	for _, p := range projects {
		items = append(items, toProjectResponse(p))
	}
	writeJSON(w, http.StatusOK, listResponse[projectResponse]{Items: items, NextCursor: next})
}

func (h *ProjectHandler) get(w http.ResponseWriter, r *http.Request) {
	claims, ok := ClaimsFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Missing access token.")
		return
	}

	p, err := h.svc.Get(r.Context(), claims.WorkspaceID, r.PathValue("id"))
	if err != nil {
		notFoundOrInternal(w, err, "PROJECT_NOT_FOUND", "Project not found.")
		return
	}
	writeJSON(w, http.StatusOK, toProjectResponse(p))
}

func (h *ProjectHandler) rename(w http.ResponseWriter, r *http.Request) {
	claims, ok := ClaimsFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Missing access token.")
		return
	}

	var req createProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		WriteError(w, http.StatusBadRequest, "INVALID_REQUEST", "name is required.")
		return
	}

	p, err := h.svc.Rename(r.Context(), claims.WorkspaceID, r.PathValue("id"), req.Name)
	if err != nil {
		notFoundOrInternal(w, err, "PROJECT_NOT_FOUND", "Project not found.")
		return
	}
	writeJSON(w, http.StatusOK, toProjectResponse(p))
}

func (h *ProjectHandler) delete(w http.ResponseWriter, r *http.Request) {
	claims, ok := ClaimsFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Missing access token.")
		return
	}

	if err := h.svc.Delete(r.Context(), claims.WorkspaceID, r.PathValue("id")); err != nil {
		notFoundOrInternal(w, err, "PROJECT_NOT_FOUND", "Project not found.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
