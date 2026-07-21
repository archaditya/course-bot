package http

import (
	"encoding/json"
	"fmt"
	"net/http"

	"course-assistant/internal/application/chat"
)

// ChatHandler handles conversation and message endpoints. Streaming responses
// use Server-Sent Events (SSE) — the frontend reads via EventSource.
// See docs/10-api-contracts.md#chat.
type ChatHandler struct {
	svc *chat.Service
}

func NewChatHandler(svc *chat.Service) *ChatHandler {
	return &ChatHandler{svc: svc}
}

func (h *ChatHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /conversations", h.createConversation)
	mux.HandleFunc("POST /conversations/{id}/messages", h.sendMessage)
}

type createConversationRequest struct {
	ProjectID string `json:"project_id"`
	CourseID  string `json:"course_id"`
}

func (h *ChatHandler) createConversation(w http.ResponseWriter, r *http.Request) {
	claims, ok := ClaimsFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Missing access token.")
		return
	}
	var req createConversationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ProjectID == "" {
		WriteError(w, http.StatusBadRequest, "INVALID_REQUEST", "project_id is required.")
		return
	}
	conv, err := h.svc.CreateConversation(r.Context(), claims.WorkspaceID, req.ProjectID)
	if err != nil {
		notFoundOrInternal(w, err, "PROJECT_NOT_FOUND", "Project not found.")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":         conv.ID,
		"project_id": conv.ProjectID,
		"title":      conv.Title,
		"created_at": conv.CreatedAt,
	})
}

type sendMessageRequest struct {
	Content  string `json:"content"`
	CourseID string `json:"course_id"`
}

// sendMessage streams the AI response using SSE.
// The final JSON object (citations, confidence) is sent as a special
// data: [RESULT] {...} event that the frontend parses separately.
func (h *ChatHandler) sendMessage(w http.ResponseWriter, r *http.Request) {
	claims, ok := ClaimsFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Missing access token.")
		return
	}
	var req sendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Content == "" {
		WriteError(w, http.StatusBadRequest, "INVALID_REQUEST", "content is required.")
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // disable nginx buffering
	flusher, ok := w.(http.Flusher)
	if !ok {
		WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Streaming not supported.")
		return
	}

	// Token channel — the service writes here as tokens arrive
	tokenCh := make(chan chat.StreamToken, 128)

	// Run pipeline in background; we forward tokens to SSE below
	var result *chat.MessageResult
	var pipeErr error
	done := make(chan struct{})
	go func() {
		defer close(done)
		result, pipeErr = h.svc.Send(
			r.Context(),
			claims.WorkspaceID,
			r.PathValue("id"),
			req.CourseID,
			req.Content,
			tokenCh,
		)
	}()

	// Forward tokens as SSE events
	for token := range tokenCh {
		if token.Done {
			break
		}
		fmt.Fprintf(w, "data: %s\n\n", token.Text)
		flusher.Flush()
	}
	<-done

	if pipeErr != nil {
		fmt.Fprintf(w, "data: [ERROR: %s]\n\n", pipeErr.Error())
		flusher.Flush()
		return
	}

	// Send final result event with citations
	if result != nil {
		resultJSON, _ := json.Marshal(result)
		fmt.Fprintf(w, "data: [RESULT] %s\n\n", string(resultJSON))
		flusher.Flush()
	}
	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()
}
