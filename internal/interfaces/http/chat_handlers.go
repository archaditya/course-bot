package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"unicode/utf8"

	"archadilm/internal/application/chat"
)

const (
	MaxChatPayloadBytes = 1 << 20 // 1 MB max request body
	MaxChatMessageRunes = 4000   // 4,000 characters max per user message
)

type ChatHandler struct {
	svc *chat.Service
}

func NewChatHandler(svc *chat.Service) *ChatHandler {
	return &ChatHandler{svc: svc}
}

func (h *ChatHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /conversations", h.createConversation)
	mux.HandleFunc("GET /conversations", h.listConversations)
	mux.HandleFunc("POST /conversations/{id}/messages", h.sendMessage)
	mux.HandleFunc("GET /chunks/{id}", h.getChunk)
	mux.HandleFunc("GET /conversations/{id}/messages", h.getConversationMessages)
	mux.HandleFunc("DELETE /conversations/{id}", h.deleteConversation)
	mux.HandleFunc("PATCH /conversations/{id}", h.updateConversationTitle)
}

func (h *ChatHandler) createConversation(w http.ResponseWriter, r *http.Request) {
	claims, ok := ClaimsFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Missing access token.")
		return
	}
	conv, err := h.svc.CreateConversation(r.Context(), claims.WorkspaceID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Could not create conversation.")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":         conv.ID,
		"title":      conv.Title,
		"created_at": conv.CreatedAt,
	})
}

type sendMessageRequest struct {
	Content string `json:"content"`
}

type updateConversationTitleRequest struct {
	Title string `json:"title"`
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

	// 1. Cap raw HTTP body size to prevent memory exhaustion DoS
	r.Body = http.MaxBytesReader(w, r.Body, MaxChatPayloadBytes)

	var req sendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Content == "" {
		WriteError(w, http.StatusBadRequest, "INVALID_REQUEST", "content is required.")
		return
	}

	// 2. Validate input bounds
	contentLength := utf8.RuneCountInString(req.Content)
	if contentLength == 0 {
		WriteError(w, http.StatusBadRequest, "INVALID_REQUEST", "content is required.")
		return
	}
	if contentLength > MaxChatMessageRunes {
		WriteError(w, http.StatusBadRequest, "INVALID_REQUEST", 
			fmt.Sprintf("content exceeds maximum length of %d characters.", MaxChatMessageRunes))
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // disable nginx buffering
	flusher, ok := w.(http.Flusher)
	if !ok {
		WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Streaming not supported.")
		return
	}

	tokenCh := make(chan chat.StreamToken, 128)

	var result *chat.MessageResult
	var pipeErr error
	done := make(chan struct{})
	go func() {
		defer close(done)
		result, pipeErr = h.svc.Send(
			r.Context(),
			claims.WorkspaceID,
			r.PathValue("id"),
			req.Content,
			tokenCh,
		)
	}()

	for token := range tokenCh {
		if token.Text != "" {
			payload, _ := json.Marshal(map[string]string{"text": token.Text})
			fmt.Fprintf(w, "data: %s\n\n", string(payload))
			flusher.Flush()
		}
		if token.Done {
			break
		}
	}
	<-done

	if pipeErr != nil {
		fmt.Fprintf(w, "data: [ERROR: %s]\n\n", pipeErr.Error())
		flusher.Flush()
		return
	}

	if result != nil {
		resultJSON, _ := json.Marshal(result)
		fmt.Fprintf(w, "data: [RESULT] %s\n\n", string(resultJSON))
		flusher.Flush()
	}
	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()
}

func (h *ChatHandler) listConversations(w http.ResponseWriter, r *http.Request) {
	claims, ok := ClaimsFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Missing access token.")
		return
	}
	convs, _, err := h.svc.ListConversations(r.Context(), claims.WorkspaceID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Could not load conversations.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": convs})
}

func (h *ChatHandler) getChunk(w http.ResponseWriter, r *http.Request) {
	_, ok := ClaimsFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Missing access token.")
		return
	}
	chunkID := r.PathValue("id")
	chunk, err := h.svc.GetChunk(r.Context(), chunkID)
	if err != nil {
		notFoundOrInternal(w, err, "CHUNK_NOT_FOUND", "Chunk not found.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":              chunk.ID,
		"document_id":     chunk.DocumentID,
		"document_name":   chunk.DocumentName,
		"source_type":     chunk.SourceType,
		"source_url":      chunk.SourceURL,
		"content":         chunk.Content,
		"title":           chunk.Title,
		"start_timestamp": chunk.StartTimestamp,
		"end_timestamp":   chunk.EndTimestamp,
		"page_number":     chunk.PageNumber,
	})
}

func (h *ChatHandler) getConversationMessages(w http.ResponseWriter, r *http.Request) {
	_, ok := ClaimsFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Missing access token.")
		return
	}
	convID := r.PathValue("id")
	msgs, next, err := h.svc.GetConversationMessages(r.Context(), convID)
	if err != nil {
		notFoundOrInternal(w, err, "CONVERSATION_NOT_FOUND", "Conversation not found.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": msgs, "next_cursor": next})
}

func (h *ChatHandler) deleteConversation(w http.ResponseWriter, r *http.Request) {
	claims, ok := ClaimsFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Missing access token.")
		return
	}
	convID := r.PathValue("id")
	if err := h.svc.DeleteConversation(r.Context(), claims.WorkspaceID, convID); err != nil {
		notFoundOrInternal(w, err, "CONVERSATION_NOT_FOUND", "Conversation not found.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ChatHandler) updateConversationTitle(w http.ResponseWriter, r *http.Request) {
	claims, ok := ClaimsFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Missing access token.")
		return
	}
	convID := r.PathValue("id")
	var req updateConversationTitleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Title == "" {
		WriteError(w, http.StatusBadRequest, "INVALID_REQUEST", "title is required.")
		return
	}

	if err := h.svc.UpdateConversationTitle(r.Context(), claims.WorkspaceID, convID, req.Title); err != nil {
		notFoundOrInternal(w, err, "CONVERSATION_NOT_FOUND", "Conversation not found.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
