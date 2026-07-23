// Package chat implements the advanced RAG query pipeline:
// pre-guardrails → query enhancement (step-back, rewrite, sub-queries) →
// HyDE document → multi-vector search → reciprocal rank fusion →
// rerank → generate (streaming) → evaluate → persist.
//
// See docs/05-query-pipeline.md for the pipeline design.
package chat

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"archadilm/internal/domain/entities"
	"archadilm/internal/domain/provider"
	"archadilm/internal/domain/repository"
	"archadilm/internal/infrastructure/llm"
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
	ids           provider.IDGenerator
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
		ids:           ids,
	}
}

// MessageResult is the final assembled response after the pipeline completes.
type MessageResult struct {
	MessageID  string           `json:"id"`
	Content    string           `json:"content"`
	Citations  []CitationResult `json:"citations"`
	Confidence string           `json:"confidence"` // "normal" | "low_confidence"
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
	Text  string
	Done  bool
	Error string
}

// CreateConversation creates a new Conversation in the given project.
func (s *Service) CreateConversation(ctx context.Context, ws repository.WorkspaceID, projectID string) (*entities.Conversation, error) {
	conv := &entities.Conversation{
		ID:        s.ids.New(),
		ProjectID: projectID,
		Title:     "New Chat",
	}
	if err := s.conversations.Create(ctx, conv); err != nil {
		return nil, fmt.Errorf("chat: create conversation: %w", err)
	}
	return conv, nil
}

// Send runs the full advanced RAG pipeline for one user message:
//
//  1. Pre-guardrails (PII + injection check)
//  2. Query Enhancement (step-back, rewrite, sub-query decomposition)
//  3. HyDE Document Generation
//  4. Multi-vector embedding (original + step-back + HyDE + 3 sub-queries)
//  5. Parallel vector search (6 searches)
//  6. Reciprocal Rank Fusion → top-20
//  7. Fetch chunk content from Postgres
//  8. Rerank → top-5
//  9. Generate with retry loop
//  10. Persist message + citations
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

	// Verify the course being queried also belongs to this workspace.
	// See docs/08-security.md#workspace-isolation.
	if _, err := s.courses.GetByID(ctx, ws, courseID); err != nil {
		return nil, fmt.Errorf("chat: course access denied: %w", err)
	}

	// Persist user message
	userMsg := &entities.Message{
		ID:             s.ids.New(),
		ConversationID: conversationID,
		Role:           entities.MessageRoleUser,
		Content:        userContent,
		Status:         entities.MessageStatusSent,
	}
	if err := s.messages.Create(ctx, userMsg); err != nil {
		return nil, fmt.Errorf("chat: save user message: %w", err)
	}

	// ── Step 1: Pre-guardrails (PII + injection) ──────────────────────────
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

	// ── Step 2: Query Enhancement ─────────────────────────────────────────
	// Call AI Service for step-back prompting, query rewriting, and
	// sub-query decomposition. If it fails, fall back to the original query.
	queryVariants := []string{userContent}
	enhancement, err := s.aiClient.EnhanceQuery(ctx, userContent)
	if err != nil {
		log.Printf("chat: query enhancement failed (using original): %v", err)
	} else {
		queryVariants = append(queryVariants, enhancement.StepBack, enhancement.Rewritten)
		queryVariants = append(queryVariants, enhancement.SubQueries...)
	}

	// ── Step 3: HyDE Document Generation ──────────────────────────────────
	hydeDoc, err := s.aiClient.HydeDocument(ctx, userContent)
	if err != nil {
		log.Printf("chat: hyde generation failed (skipping): %v", err)
	} else if hydeDoc != "" {
		queryVariants = append(queryVariants, hydeDoc)
	}

	// ── Step 4: Multi-vector embedding ────────────────────────────────────
	// Embed all query variants in a single batch call for efficiency.
	allVecs, err := s.embedder.Embed(ctx, queryVariants)
	if err != nil {
		return nil, fmt.Errorf("chat: embed query variants: %w", err)
	}

	// ── Step 5: Parallel vector search ────────────────────────────────────
	// Run one search per query variant in parallel.
	type searchResult struct {
		results []provider.VectorSearchResult
		err     error
	}
	searchCh := make(chan searchResult, len(allVecs))
	var wg sync.WaitGroup

	for _, vec := range allVecs {
		wg.Add(1)
		go func(v provider.Vector) {
			defer wg.Done()
			results, err := s.vectors.Search(ctx, courseID, v, 20)
			searchCh <- searchResult{results: results, err: err}
		}(vec)
	}

	go func() {
		wg.Wait()
		close(searchCh)
	}()

	var allResultSets [][]provider.VectorSearchResult
	for sr := range searchCh {
		if sr.err != nil {
			log.Printf("chat: one vector search failed: %v", sr.err)
			continue
		}
		if len(sr.results) > 0 {
			allResultSets = append(allResultSets, sr.results)
		}
	}

	// ── Step 6: Reciprocal Rank Fusion ────────────────────────────────────
	// Merge all result sets and take top-20 for the reranking stage.
	mergedResults := reciprocalRankFusion(allResultSets, 20)

	if len(mergedResults) == 0 {
		noContent := "I couldn't find anything relevant to that question in this course material."
		tokenCh <- StreamToken{Text: noContent, Done: true}
		return &MessageResult{
			MessageID:  s.ids.New(),
			Content:    noContent,
			Confidence: "normal",
		}, nil
	}

	// ── Step 7: Fetch real chunk content from Postgres ─────────────────────
	chunkIDs := make([]string, len(mergedResults))
	for i, r := range mergedResults {
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

	// ── Step 8: Rerank → top-5 ────────────────────────────────────────────
	candidates := make([]provider.RerankCandidate, 0, len(mergedResults))
	for _, r := range mergedResults {
		if c, ok := chunkByID[r.ChunkID]; ok {
			candidates = append(candidates, provider.RerankCandidate{
				ChunkID:    c.ID,
				DocumentID: c.DocumentID,
				Content:    c.Content,
			})
		}
	}
	ranked, err := s.aiClient.Rerank(ctx, userContent, candidates)
	if err != nil {
		log.Printf("chat: rerank: %v (continuing with RRF order)", err)
		// non-fatal; use RRF order
		ranked = make([]provider.RankedChunk, len(mergedResults))
		for i, r := range mergedResults {
			ranked[i] = provider.RankedChunk{ChunkID: r.ChunkID, Score: r.Score}
		}
	}

	topK := 5
	if len(ranked) < topK {
		topK = len(ranked)
	}
	ranked = ranked[:topK]

	// Resolve ranked chunk_ids back to full Chunk rows
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

	// ── Step 9: Generate with bounded retry loop ──────────────────────────
	var bestContent string
	var bestScore int
	confidence := "normal"

	for attempt := 1; attempt <= s.maxRetries; attempt++ {
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

	// ── Step 10: Persist assistant message + citations ─────────────────────
	msgStatus := entities.MessageStatusCompleted
	if confidence == "low_confidence" {
		msgStatus = entities.MessageStatusLowConfidence
	}
	assistantMsg := &entities.Message{
		ID:             s.ids.New(),
		ConversationID: conversationID,
		Role:           entities.MessageRoleAssistant,
		Content:        bestContent,
		Status:         msgStatus,
	}
	if err := s.messages.Create(ctx, assistantMsg); err != nil {
		log.Printf("chat: save assistant message: %v", err)
	}

	cits := make([]*entities.Citation, len(rankedChunks))
	citResults := make([]CitationResult, len(rankedChunks))
	for i, c := range rankedChunks {
		cits[i] = &entities.Citation{
			ID:             s.ids.New(),
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
