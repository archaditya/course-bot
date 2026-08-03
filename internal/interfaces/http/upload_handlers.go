package http

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"archadilm/internal/application/upload"
	"archadilm/internal/domain/repository"
	"archadilm/internal/infrastructure/observability"
	rediscache "archadilm/internal/infrastructure/redis"
	sentry "github.com/getsentry/sentry-go"
)

// UploadHandler handles adding sources (PDF/SRT/VTT/DOCX/TXT file, ZIP,
// YouTube/web URL, or pasted text) to a conversation's own knowledge base,
// and listing/removing them. Upload returns 202 immediately; processing is
// async via Redis Streams -> Worker pipeline, and each source's own status
// is polled from GET /documents/{id}/status (see status_handler.go) or from
// the list endpoint below.
type UploadHandler struct {
	svc       *upload.Service
	documents repository.DocumentRepository
	chunks    repository.ChunkRepository
	cache     *rediscache.Cache
}

func NewUploadHandler(svc *upload.Service, documents repository.DocumentRepository, chunks repository.ChunkRepository, cache *rediscache.Cache) *UploadHandler {
	return &UploadHandler{svc: svc, documents: documents, chunks: chunks, cache: cache}
}

func (h *UploadHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /conversations/{id}/documents", h.upload)
	mux.HandleFunc("POST /conversations/{id}/documents/source", h.handleAddSource)
	mux.HandleFunc("GET /conversations/{id}/documents", h.listDocuments)
	mux.HandleFunc("DELETE /conversations/{id}/documents/{docID}", h.deleteDocument)
	mux.HandleFunc("GET /documents/{docID}/chunks", h.listChunks)
}

// upload accepts a multipart file upload — a single PDF/SRT/VTT/DOCX/TXT
// file, or a ZIP archive (auto-detected and fanned out into one Document
// per supported file inside it).
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

	conversationID := r.PathValue("id")

	traceID := r.Header.Get("X-Trace-Id")
	if traceID == "" {
		traceID = conversationID + "-" + header.Filename
	}

	// Auto-detect ZIP uploads and route to the ZIP handler
	if strings.HasSuffix(strings.ToLower(header.Filename), ".zip") {
		result, err := h.svc.UploadZip(r.Context(), claims.WorkspaceID, conversationID, data, traceID)
		if err != nil {
			observability.CaptureException(err)
			notFoundOrInternal(w, err, "CONVERSATION_NOT_FOUND", "Conversation not found.")
			return
		}
		writeJSON(w, http.StatusAccepted, result)
		return
	}

	result, err := h.svc.Upload(r.Context(), claims.WorkspaceID, conversationID, header.Filename, data, traceID)
	if err != nil {
		observability.CaptureException(err)
		if strings.Contains(err.Error(), "unsupported file type") {
			WriteError(w, http.StatusBadRequest, "UNSUPPORTED_FILE_TYPE",
				"Supported: .pdf, .docx, .txt, .md, .srt, .vtt, or .zip")
		} else {
			notFoundOrInternal(w, err, "CONVERSATION_NOT_FOUND", "Conversation not found.")
		}
		return
	}

	if h.cache != nil {
		h.cache.InvalidateConversation(r.Context(), conversationID)
	}

	writeJSON(w, http.StatusAccepted, result)
}

// handleAddSource handles the other two tabs of the "Add Document" modal:
// a YouTube/web URL, or pasted text. File uploads (including ZIP) go
// through upload() above instead.
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

	conversationID := r.PathValue("id")

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
		if len(req.Content) > 50000 {
			WriteError(w, http.StatusBadRequest, "TEXT_TOO_LONG", "Text content exceeds the 50,000 character limit. Please shorten it and try again.")
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

	result, err := h.svc.AddSource(r.Context(), claims.WorkspaceID, conversationID, req.SourceType, req.URL, req.Content, req.Title)
	if err != nil {
		observability.CaptureException(err)
		notFoundOrInternal(w, err, "CONVERSATION_NOT_FOUND", "Conversation not found.")
		return
	}

	if h.cache != nil {
		h.cache.InvalidateConversation(r.Context(), conversationID)
	}

	writeJSON(w, http.StatusAccepted, result)
}

// listDocuments powers the right-hand sidebar: every source that has been
// added to this conversation, each with its own indexing status.
func (h *UploadHandler) listDocuments(w http.ResponseWriter, r *http.Request) {
	claims, ok := ClaimsFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Missing access token.")
		return
	}
	docs, err := h.documents.ListByConversation(r.Context(), claims.WorkspaceID, r.PathValue("id"))
	if err != nil {
		notFoundOrInternal(w, err, "CONVERSATION_NOT_FOUND", "Conversation not found.")
		return
	}
	items := make([]map[string]any, len(docs))
	for i, d := range docs {
		items[i] = map[string]any{
			"id":                d.ID,
			"original_filename": d.OriginalFilename,
			"source_type":       d.SourceType,
			"source_url":        d.SourceURL,
			"status":            d.Status,
			"created_at":        d.CreatedAt,
			"ai_summary":        d.AISummary,
			"ai_questions":      d.AIQuestions,
			"ai_overview":       d.AIOverview,
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *UploadHandler) deleteDocument(w http.ResponseWriter, r *http.Request) {
	claims, ok := ClaimsFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Missing access token.")
		return
	}
	conversationID := r.PathValue("id")
	if err := h.documents.Delete(r.Context(), claims.WorkspaceID, r.PathValue("docID")); err != nil {
		notFoundOrInternal(w, err, "DOCUMENT_NOT_FOUND", "Document not found.")
		return
	}
	if h.cache != nil && conversationID != "" {
		h.cache.InvalidateConversation(r.Context(), conversationID)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *UploadHandler) listChunks(w http.ResponseWriter, r *http.Request) {
	docID := r.PathValue("docID")
	chunks, err := h.chunks.ListByDocument(r.Context(), docID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Could not fetch document chunks.")
		return
	}
	items := make([]map[string]any, len(chunks))
	for i, c := range chunks {
		items[i] = map[string]any{
			"id":                c.ID,
			"document_id":       c.DocumentID,
			"conversation_id":   c.ConversationID,
			"start_timestamp":   c.StartTimestamp,
			"end_timestamp":     c.EndTimestamp,
			"page_number":       c.PageNumber,
			"title":             c.Title,
			"summary":           c.Summary,
			"content":           c.Content,
			"token_count":       c.TokenCount,
			"embedding_version": c.EmbeddingVersion,
			"created_at":        c.CreatedAt,
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}
