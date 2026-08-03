package http

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"archadilm/internal/domain/entities"
	"archadilm/internal/domain/provider"
	"archadilm/internal/domain/repository"
	"archadilm/internal/infrastructure/llm"
)

type LearningHandler struct {
	learningNodes repository.LearningNodeRepository
	conversations repository.ConversationRepository
	documents     repository.DocumentRepository
	chunks        repository.ChunkRepository
	aiClient      *llm.Client
	ids           provider.IDGenerator
}

func NewLearningHandler(
	learningNodes repository.LearningNodeRepository,
	conversations repository.ConversationRepository,
	documents repository.DocumentRepository,
	chunks repository.ChunkRepository,
	aiClient *llm.Client,
	ids provider.IDGenerator,
) *LearningHandler {
	return &LearningHandler{
		learningNodes: learningNodes,
		conversations: conversations,
		documents:     documents,
		chunks:        chunks,
		aiClient:      aiClient,
		ids:           ids,
	}
}

func (h *LearningHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /conversations/{id}/learning-nodes", h.createLearningNode)
	mux.HandleFunc("GET /conversations/{id}/learning-nodes", h.listLearningNodes)
	mux.HandleFunc("DELETE /conversations/{id}/learning-nodes/{nodeID}", h.deleteLearningNode)
}

func (h *LearningHandler) createLearningNode(w http.ResponseWriter, r *http.Request) {
	claims, ok := ClaimsFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Missing access token.")
		return
	}

	conversationID := r.PathValue("id")
	if _, err := h.conversations.GetByID(r.Context(), claims.WorkspaceID, conversationID); err != nil {
		notFoundOrInternal(w, err, "CONVERSATION_NOT_FOUND", "Conversation not found.")
		return
	}

	var req struct {
		ToolType entities.ToolType `json:"tool_type"`
		Title    string            `json:"title,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_BODY", "Invalid JSON body.")
		return
	}

	switch req.ToolType {
	case entities.ToolTypeSummary, entities.ToolTypeKeyTakeaways, entities.ToolTypeFlashcards,
		entities.ToolTypeQuiz, entities.ToolTypeMindMap, entities.ToolTypeAIReport:
		// Valid
	default:
		WriteError(w, http.StatusBadRequest, "INVALID_TOOL_TYPE", "Invalid tool_type.")
		return
	}

	nodeID := h.ids.New()
	title := strings.TrimSpace(req.Title)
	if title == "" {
		switch req.ToolType {
		case entities.ToolTypeSummary:
			title = "Source Summary"
		case entities.ToolTypeKeyTakeaways:
			title = "Key Takeaways"
		case entities.ToolTypeFlashcards:
			title = "Study Flashcards"
		case entities.ToolTypeQuiz:
			title = "Knowledge Quiz"
		case entities.ToolTypeMindMap:
			title = "Concept Mind Map"
		case entities.ToolTypeAIReport:
			title = "Deep-Dive AI Report"
		}
	}

	node := &entities.LearningNode{
		ID:             nodeID,
		ConversationID: conversationID,
		ToolType:       req.ToolType,
		Title:          title,
		Status:         entities.LearningNodeStatusGenerating,
	}

	if err := h.learningNodes.Create(r.Context(), node); err != nil {
		WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create learning node.")
		return
	}

	// Launch async generation in background
	go h.processGeneration(claims.WorkspaceID, conversationID, nodeID, req.ToolType, title)

	writeJSON(w, http.StatusAccepted, node)
}

func (h *LearningHandler) processGeneration(workspaceID, conversationID, nodeID string, toolType entities.ToolType, title string) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	// 1. Get indexed documents for this conversation
	docs, err := h.documents.ListByConversation(ctx, workspaceID, conversationID)
	if err != nil {
		log.Printf("learning_handler: list docs error: %v", err)
		_ = h.learningNodes.UpdateContent(ctx, nodeID, json.RawMessage(`{"error": "Failed to load sources"}`), string(entities.LearningNodeStatusFailed))
		return
	}

	// Filter indexed docs
	var indexedDocIDs []string
	for _, d := range docs {
		if d.Status == entities.DocumentStatusIndexed {
			indexedDocIDs = append(indexedDocIDs, d.ID)
		}
	}

	if len(indexedDocIDs) == 0 {
		_ = h.learningNodes.UpdateContent(ctx, nodeID, json.RawMessage(`{"error": "No indexed sources available to generate from."}`), string(entities.LearningNodeStatusFailed))
		return
	}

	// 2. Fetch chunks across indexed documents
	var combinedContent strings.Builder
	for _, docID := range indexedDocIDs {
		chunks, err := h.chunks.ListByDocument(ctx, docID)
		if err != nil {
			continue
		}
		for i, c := range chunks {
			if combinedContent.Len() > 15000 {
				break
			}
			if i > 0 {
				combinedContent.WriteString("\n\n")
			}
			combinedContent.WriteString(c.Content)
		}
		if combinedContent.Len() > 15000 {
			break
		}
	}

	if combinedContent.Len() == 0 {
		_ = h.learningNodes.UpdateContent(ctx, nodeID, json.RawMessage(`{"error": "Source content is empty."}`), string(entities.LearningNodeStatusFailed))
		return
	}

	// 3. Call AI Service to generate learning tool
	result, err := h.aiClient.GenerateLearningTool(ctx, string(toolType), combinedContent.String(), title)
	if err != nil {
		log.Printf("learning_handler: generate tool error: %v", err)
		errMsg := fmt.Sprintf(`{"error": %q}`, err.Error())
		_ = h.learningNodes.UpdateContent(ctx, nodeID, json.RawMessage(errMsg), string(entities.LearningNodeStatusFailed))
		return
	}

	// 4. Update node content and mark as ready
	if err := h.learningNodes.UpdateContent(ctx, nodeID, result, string(entities.LearningNodeStatusReady)); err != nil {
		log.Printf("learning_handler: update content error: %v", err)
	}
}

func (h *LearningHandler) listLearningNodes(w http.ResponseWriter, r *http.Request) {
	claims, ok := ClaimsFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Missing access token.")
		return
	}

	conversationID := r.PathValue("id")
	if _, err := h.conversations.GetByID(r.Context(), claims.WorkspaceID, conversationID); err != nil {
		notFoundOrInternal(w, err, "CONVERSATION_NOT_FOUND", "Conversation not found.")
		return
	}

	nodes, err := h.learningNodes.ListByConversation(r.Context(), conversationID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list learning nodes.")
		return
	}

	if nodes == nil {
		nodes = []*entities.LearningNode{}
	}

	writeJSON(w, http.StatusOK, map[string]any{"items": nodes})
}

func (h *LearningHandler) deleteLearningNode(w http.ResponseWriter, r *http.Request) {
	claims, ok := ClaimsFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Missing access token.")
		return
	}

	conversationID := r.PathValue("id")
	if _, err := h.conversations.GetByID(r.Context(), claims.WorkspaceID, conversationID); err != nil {
		notFoundOrInternal(w, err, "CONVERSATION_NOT_FOUND", "Conversation not found.")
		return
	}

	nodeID := r.PathValue("nodeID")
	if err := h.learningNodes.Delete(r.Context(), nodeID); err != nil {
		WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to delete learning node.")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
