// Package chat implements the query pipeline use case:
// pre-guardrails → retrieve → rerank → generate (streaming) → evaluate
// → post-guardrails → persist Message + Citations.
//
// See docs/05-query-pipeline.md for the full pipeline with retry loop.
package chat

import (
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"strings"
	"time"

	"course-assistant/internal/domain/entities"
	"course-assistant/internal/domain/provider"
	"course-assistant/internal/domain/repository"
	"course-assistant/internal/infrastructure/llm"
)

// Service owns the chat (query pipeline) use case.
type Service struct {
	conversations repository.ConversationRepository
	messages      repository.MessageRepository
	citations     repository.CitationRepository
	projects      repository.ProjectRepository
	courses       repository.CourseRepository // workspace-isolation check on course_id
	chunks        repository.ChunkRepository  // fetch real content for grounding + citations
	aiClient      *llm.Client
	guardrails    provider.GuardrailProvider
	evaluator     provider.EvaluatorProvider
	vectors       provider.VectorStore
	embedder      provider.EmbeddingProvider
	maxRetries    int
	ids 		  provider.IDGenerator
}

// NewService wires the chat service.
func NewService(
	conversations repository.ConversationRepository,
	messages repository.MessageRepository,
	citations repository.CitationRepository,
	projects repository.ProjectRepository,
	courses repository.CourseRepository,
	chunks repository.ChunkRepository,
	aiClient *llm.Client,
	vectors provider.VectorStore,
	embedder provider.EmbeddingProvider,
	maxRetries int,
	ids provider.IDGenerator,
) *Service {
	return &Service{
		conversations: conversations,
		messages:      messages,
		citations:     citations,
		projects:      projects,
		courses:       courses,
		chunks:        chunks,
		aiClient:      aiClient,
		guardrails:    aiClient, // llm.Client implements GuardrailProvider
		evaluator:     aiClient, // llm.Client implements EvaluatorProvider
		vectors:       vectors,
		embedder:      embedder,
		maxRetries:    maxRetries,
		ids:		   ids,
	}
}

// MessageResult is the final assembled response after the pipeline completes.
type MessageResult struct {
	MessageID  string
	Content    string
	Citations  []CitationResult
	Confidence string // "normal" | "low_confidence"
}

// CitationResult is the client-facing citation shape from
// docs/10-api-contracts.md#chat.
type CitationResult struct {
	ChunkID        string `json:"chunk_id"`
	DocumentID     string `json:"document_id"`
	StartTimestamp *int   `json:"start_timestamp,omitempty"`
	Title          string `json:"title,omitempty"`
}

// StreamToken is one token emitted during streaming.
type StreamToken struct {
	Text    string
	Done    bool
	Error   string
}

// CreateConversation creates a new Conversation in the given project.
func (s *Service) CreateConversation(ctx context.Context, ws repository.WorkspaceID, projectID string) (*entities.Conversation, error) {
	conv := &entities.Conversation{
		ID:        chatNewID(),
		ProjectID: projectID,
		Title:     "New Chat",
	}
	if err := s.conversations.Create(ctx, conv); err != nil {
		return nil, fmt.Errorf("chat: create conversation: %w", err)
	}
	return conv, nil
}

// Send runs the full query pipeline for one user message. It persists the
// user message, streams tokens via the tokenCh channel, then persists the
// assistant message + citations. The caller reads from tokenCh and forwards
// tokens over SSE/WebSocket.
//
// The function blocks until the pipeline completes (after the last token is
// sent). Callers should run it in a goroutine if they need to stay responsive.
func (s *Service) Send(
	ctx context.Context,
	ws repository.WorkspaceID,
	conversationID string,
	courseID string,
	userContent string,
	tokenCh chan<- StreamToken,
) (*MessageResult, error) {
	// Verify conversation belongs to workspace
	conv, err := s.conversations.GetByID(ctx, ws, conversationID)
	if err != nil {
		return nil, fmt.Errorf("chat: get conversation: %w", err)
	}
	_ = conv

	// Verify the course being queried also belongs to this workspace — the
	// conversation check above does NOT cover this: course_id arrives as a
	// caller-supplied field on every request, so without this check a user
	// could query vectors from a course outside their workspace.
	// See docs/08-security.md#workspace-isolation.
	if _, err := s.courses.GetByID(ctx, ws, courseID); err != nil {
		return nil, fmt.Errorf("chat: course access denied: %w", err)
	}

	// Persist user message
	userMsg := &entities.Message{
		ID:             chatNewID(),
		ConversationID: conversationID,
		Role:           entities.MessageRoleUser,
		Content:        userContent,
		Status:         entities.MessageStatusSent,
	}
	if err := s.messages.Create(ctx, userMsg); err != nil {
		return nil, fmt.Errorf("chat: save user message: %w", err)
	}

	// ── Pre-guardrails (PII + injection) ──────────────────────────────────
	injResult, err := s.guardrails.CheckInjection(ctx, userContent)
	if err != nil {
		log.Printf("chat: injection check: %v", err)
	} else if injResult.IsInjection {
		return nil, fmt.Errorf("chat: query rejected: prompt injection detected")
	}

	piiResult, err := s.guardrails.CheckPII(ctx, userContent)
	if err != nil {
		log.Printf("chat: pii check: %v", err)
	} else if piiResult.ContainsPII {
		return nil, fmt.Errorf("chat: query rejected: PII detected in input")
	}

	// ── Embed query ────────────────────────────────────────────────────────
	vecs, err := s.embedder.Embed(ctx, []string{userContent})
	if err != nil {
		return nil, fmt.Errorf("chat: embed query: %w", err)
	}

	// ── Vector search (Qdrant) ─────────────────────────────────────────────
	searchResults, err := s.vectors.Search(ctx, courseID, vecs[0], 20)
	if err != nil {
		return nil, fmt.Errorf("chat: vector search: %w", err)
	}

	if len(searchResults) == 0 {
		noContent := "I couldn't find anything relevant to that question in this course material."
		tokenCh <- StreamToken{Text: noContent, Done: true}
		return &MessageResult{
			MessageID:  chatNewID(),
			Content:    noContent,
			Confidence: "normal",
		}, nil
	}

	// ── Fetch real chunk content from Postgres ─────────────────────────────
	// Qdrant only ever stores chunk_id + minimal filter payload (docs/07-storage.md);
	// the actual content, title, and timestamps live in Postgres. Without this
	// fetch, both reranking and generation have no real course material to work
	// with — this is the join step the pipeline depends on.
	chunkIDs := make([]string, len(searchResults))
	for i, r := range searchResults {
		chunkIDs[i] = r.ChunkID
	}
	fetchedChunks, err := s.chunks.GetByIDs(ctx, chunkIDs)
	if err != nil {
		return nil, fmt.Errorf("chat: fetch chunk content: %w", err)
	}
	chunkByID := make(map[string]*entities.Chunk, len(fetchedChunks))
	for _, c := range fetchedChunks {
		chunkByID[c.ID] = c
	}

	// ── Rerank ────────────────────────────────────────────────────────────
	candidates := make([]provider.RerankCandidate, 0, len(searchResults))
	for _, r := range searchResults {
		if c, ok := chunkByID[r.ChunkID]; ok {
			candidates = append(candidates, provider.RerankCandidate{ChunkID: c.ID, Content: c.Content})
		}
	}
	ranked, err := s.aiClient.Rerank(ctx, userContent, candidates)
	if err != nil {
		log.Printf("chat: rerank: %v (continuing with unranked)", err)
		// non-fatal; use original vector-search order
		ranked = make([]provider.RankedChunk, len(searchResults))
		for i, r := range searchResults {
			ranked[i] = provider.RankedChunk{ChunkID: r.ChunkID, Score: r.Score}
		}
	}

	// Take top-5 after reranking
	topK := 5
	if len(ranked) < topK {
		topK = len(ranked)
	}
	ranked = ranked[:topK]

	// Resolve the final ranked chunk_ids back to their full Chunk rows, in
	// ranked order — this order drives both the generation context and the
	// citations returned to the client.
	rankedChunks := make([]*entities.Chunk, 0, len(ranked))
	for _, rc := range ranked {
		if c, ok := chunkByID[rc.ChunkID]; ok {
			rankedChunks = append(rankedChunks, c)
		}
	}

	// Build context string for generation from real course content.
	var contextBuilder strings.Builder
	for i, c := range rankedChunks {
		fmt.Fprintf(&contextBuilder, "--- Excerpt %d ---\n%s\n\n", i+1, c.Content)
	}
	context_ := contextBuilder.String()
	if context_ == "" {
		context_ = "No relevant course material was found for this question."
	}

	// ── Generate with bounded retry loop ──────────────────────────────────
	// docs/05-query-pipeline.md: retry up to maxRetries if evaluator score < threshold
	var bestContent string
	var bestScore int
	confidence := "normal"

	for attempt := 1; attempt <= s.maxRetries; attempt++ {
		// Create streaming prompt
		prompt := provider.Prompt{
			System: "You are a helpful study assistant. Answer questions based ONLY on the provided course material. Cite your sources.",
			Messages: []provider.PromptMessage{
				{Role: "user", Content: fmt.Sprintf("Context:\n%s\n\nQuestion: %s", context_, userContent)},
			},
			PromptVersion: "1.0",
		}

		tokenStream, err := s.aiClient.Stream(ctx, prompt)
		if err != nil {
			log.Printf("chat: stream attempt %d: %v", attempt, err)
			continue
		}

		var fullContent string
		for token := range tokenStream {
			// if token.Error != "" {
			// 	break
			// }
			fullContent += token.Text
			if !token.Done {
				tokenCh <- StreamToken{Text: token.Text}
			}
		}

		// Evaluate
		score, _, err := s.evaluator.Evaluate(ctx, userContent, fullContent, []string{context_})
		if err != nil {
			log.Printf("chat: evaluate: %v", err)
			score = 8 // assume pass on evaluator failure
		}

		if score > bestScore {
			bestScore = score
			bestContent = fullContent
		}

		if score >= 7 {
			break
		}

		if attempt < s.maxRetries {
			log.Printf("chat: score %d < 7, retrying (attempt %d/%d)", score, attempt, s.maxRetries)
			time.Sleep(500 * time.Millisecond)
		} else {
			confidence = "low_confidence"
			log.Printf("chat: max retries reached, returning best attempt (score %d)", bestScore)
		}
	}

	tokenCh <- StreamToken{Done: true}

	// ── Persist assistant message ──────────────────────────────────────────
	msgStatus := entities.MessageStatusCompleted
	if confidence == "low_confidence" {
		msgStatus = entities.MessageStatusLowConfidence
	}
	assistantMsg := &entities.Message{
		ID:             chatNewID(),
		ConversationID: conversationID,
		Role:           entities.MessageRoleAssistant,
		Content:        bestContent,
		Status:         msgStatus,
	}
	if err := s.messages.Create(ctx, assistantMsg); err != nil {
		log.Printf("chat: save assistant message: %v", err)
	}

	// ── Persist citations ──────────────────────────────────────────────────
	// Built from rankedChunks (real Chunk rows), not bare chunk IDs, so both
	// the DB row and the client-facing result carry the real timestamp/page
	// and document/title info — this is what powers the clickable
	// timestamp-citation UI element.
	cits := make([]*entities.Citation, len(rankedChunks))
	citResults := make([]CitationResult, len(rankedChunks))
	for i, c := range rankedChunks {
		cits[i] = &entities.Citation{
			ID:             chatNewID(),
			MessageID:      assistantMsg.ID,
			ChunkID:        c.ID,
			StartTimestamp: c.StartTimestamp,
			PageNumber:     c.PageNumber,
		}
		citResults[i] = CitationResult{
			ChunkID:        c.ID,
			DocumentID:     c.DocumentID,
			StartTimestamp: c.StartTimestamp,
			Title:          c.Title,
		}
	}
	if len(cits) > 0 {
		if err := s.citations.CreateBatch(ctx, cits); err != nil {
			log.Printf("chat: save citations: %v", err)
		}
	}

	return &MessageResult{
		MessageID:  assistantMsg.ID,
		Content:    bestContent,
		Citations:  citResults,
		Confidence: confidence,
	}, nil
}

func chatNewID() string {
	b := make([]byte, 16)
	rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}
