package http

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"archadilm/internal/application/upload"
	"archadilm/internal/infrastructure/observability"
	sentry "github.com/getsentry/sentry-go"
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
	// Collection is the public resource name. Course endpoints remain aliases
	// while existing clients migrate.
	mux.HandleFunc("POST /collections/{id}/upload", h.upload)
	mux.HandleFunc("POST /collections/{courseID}/sources", h.handleAddSource)
	mux.HandleFunc("POST /courses/{id}/upload", h.upload)
	mux.HandleFunc("POST /courses/{courseID}/sources", h.handleAddSource)
}

func (h *UploadHandler) upload(w http.ResponseWriter, r *http.Request) {
	claims, ok := ClaimsFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Missing access token.")
		return
	}

	if h.svc == nil {
		observability.CaptureMessage("Upload service unavailable", sentry.LevelWarning)
		WriteError(w, http.StatusServiceUnavailable, "UPLOAD_DISABLED", "Upload service unavailable.")
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

	// Auto-detect ZIP uploads and route to the ZIP handler
	if strings.HasSuffix(strings.ToLower(header.Filename), ".zip") {
		result, err := h.svc.UploadZip(r.Context(),
			claims.WorkspaceID,
			r.FormValue("project_id"),
			r.PathValue("id"),
			data,
			traceID,
		)
		if err != nil {
			observability.CaptureException(err)
			notFoundOrInternal(w, err, "COURSE_NOT_FOUND", "Course not found.")
			return
		}
		writeJSON(w, http.StatusAccepted, result)
		return
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
		observability.CaptureException(err)
		if strings.Contains(err.Error(), "unsupported file type") {
			WriteError(w, http.StatusBadRequest, "UNSUPPORTED_FILE_TYPE",
				"Supported: .pdf, .docx, .txt, .md, .srt, .vtt, or .zip")
		} else {
			notFoundOrInternal(w, err, "COURSE_NOT_FOUND", "Course not found.")
		}
		return
	}

	writeJSON(w, http.StatusAccepted, result)
}

// handleAddSource handles URL-based and text-based source additions.
// For file uploads, the existing upload endpoint is used instead.
func (h *UploadHandler) handleAddSource(w http.ResponseWriter, r *http.Request) {
	claims, ok := ClaimsFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Missing access token.")
		return
	}

	if h.svc == nil {
		WriteError(w, http.StatusServiceUnavailable, "UPLOAD_DISABLED", "Upload service unavailable.")
		return
	}

	courseID := r.PathValue("courseID")

	var req struct {
		SourceType string `json:"source_type"` // "url" | "text" | "video_url"
		URL        string `json:"url,omitempty"`
		Content    string `json:"content,omitempty"`
		Title      string `json:"title,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_BODY", "Invalid JSON body.")
		return
	}

	switch req.SourceType {
	case "url":
		if req.URL == "" {
			WriteError(w, http.StatusBadRequest, "MISSING_URL", "url field is required for URL sources.")
			return
		}
	case "text":
		if req.Content == "" {
			WriteError(w, http.StatusBadRequest, "MISSING_CONTENT", "content field is required for text sources.")
			return
		}
	case "video_url":
		if req.URL == "" {
			WriteError(w, http.StatusBadRequest, "MISSING_URL", "url field is required for video URL sources.")
			return
		}
	default:
		WriteError(w, http.StatusBadRequest, "INVALID_SOURCE_TYPE", "source_type must be url, text, or video_url.")
		return
	}

	result, err := h.svc.AddSource(r.Context(), claims.WorkspaceID, courseID, req.SourceType, req.URL, req.Content, req.Title)
	if err != nil {
		observability.CaptureException(err)
		notFoundOrInternal(w, err, "COURSE_NOT_FOUND", "Course not found.")
		return
	}

	writeJSON(w, http.StatusAccepted, result)
}
