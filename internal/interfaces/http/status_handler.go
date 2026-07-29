package http

import (
	"net/http"

	"archadilm/internal/domain/repository"
)

// StatusHandler exposes indexing status for a single Document — every
// source tracks its own status independently, so there is no separate
// "collection" status to roll up.
type StatusHandler struct {
	documents repository.DocumentRepository
	jobs      repository.JobRepository
}

func NewStatusHandler(documents repository.DocumentRepository, jobs repository.JobRepository) *StatusHandler {
	return &StatusHandler{documents: documents, jobs: jobs}
}
func (h *StatusHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /documents/{documentID}/status", h.handleGetStatus)
}

type documentStatusResponse struct {
	DocumentID string          `json:"document_id"`
	Status     string          `json:"status"`
	Jobs       []jobStatusItem `json:"jobs"`
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
	doc, err := h.documents.GetByID(r.Context(), claims.WorkspaceID, r.PathValue("documentID"))
	if err != nil {
		WriteError(w, http.StatusNotFound, "NOT_FOUND", "Document not found.")
		return
	}
	jobs, err := h.jobs.ListByDocument(r.Context(), doc.ID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Could not load indexing status.")
		return
	}
	items := make([]jobStatusItem, len(jobs))
	for i, job := range jobs {
		items[i] = jobStatusItem{ID: job.ID, Stage: string(job.Stage), Status: string(job.Status), Attempts: job.Attempts, LastError: job.LastError}
	}
	writeJSON(w, http.StatusOK, documentStatusResponse{DocumentID: doc.ID, Status: string(doc.Status), Jobs: items})
}
