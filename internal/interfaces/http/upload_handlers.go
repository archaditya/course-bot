package http

import (
	"io"
	"net/http"

	"course-assistant/internal/application/upload"
)

// UploadHandler handles POST /courses/:id/upload per
// docs/10-api-contracts.md#upload. Returns 202 immediately; processing is
// async via Redis Streams → Worker pipeline.
type UploadHandler struct {
	svc *upload.Service
}

func NewUploadHandler(svc *upload.Service) *UploadHandler {
	return &UploadHandler{svc: svc}
}

func (h *UploadHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /courses/{id}/upload", h.upload)
}

func (h *UploadHandler) upload(w http.ResponseWriter, r *http.Request) {
	claims, ok := ClaimsFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Missing access token.")
		return
	}

	// 50 MB limit per file; multipart form parse
	if err := r.ParseMultipartForm(50 << 20); err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_REQUEST", "Multipart form parse failed.")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_REQUEST", "Missing file field in form.")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Could not read uploaded file.")
		return
	}

	// trace_id threads through all downstream events for distributed tracing
	traceID := r.Header.Get("X-Trace-Id")
	if traceID == "" {
		traceID = r.PathValue("id") + "-" + header.Filename
	}

	result, err := h.svc.Upload(r.Context(),
		claims.WorkspaceID,
		r.FormValue("project_id"),
		r.PathValue("id"),
		header.Filename,
		data,
		traceID,
	)
	if err != nil {
		switch err.Error() {
		case "upload: unsupported file type":
			WriteError(w, http.StatusBadRequest, "UNSUPPORTED_FILE_TYPE",
				"Only .srt files are supported in this version.")
		default:
			notFoundOrInternal(w, err, "COURSE_NOT_FOUND", "Course not found.")
		}
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]any{
		"course_id":    result.CourseID,
		"document_ids": result.DocumentIDs,
	})
}
