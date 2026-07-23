package chat

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"

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
	courses       repository.CourseRepository
	chunks        repository.ChunkRepository
	aiClient      *llm.Client
	guardrails    provider.GuardrailProvider
	evaluator     provider.EvaluatorProvider
	vectors       provider.VectorStore
	embedder      provider.EmbeddingProvider
	maxRetries    int
	ids           provider.IDGenerator
}

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
		guardrails:    aiClient,
		evaluator:     aiClient,
		vectors:       vectors,
		embedder:      embedder,
		maxRetries:    maxRetries,
		ids:           ids,
	}
}

type MessageResult struct {
	MessageID  string           `json:"id"`
	Content    string           `json:"content"`
	Citations  []CitationResult `json:"citations"`
	Confidence string           `json:"confidence"`
}

type CitationResult struct {
	ChunkID        string `json:"chunk_id"`
	DocumentID     string `json:"document_id"`
	StartTimestamp *int   `json:"start_timestamp,omitempty"`
	Title          string `json:"title,omitempty"`
}

type StreamToken struct {
	Text  string
	Done  bool
	Error string
}

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

func (s *Service) Send(
	ctx context.Context,
	ws repository.WorkspaceID,
	conversationID string,
	courseID string,
	userContent string,
	tokenCh chan<- StreamToken,
) (*MessageResult, error) {
	conv, err := s.conversations.GetByID(ctx, ws, conversationID)
	if err != nil {
		return nil, fmt.Errorf("chat: get conversation: %w", err)
	}
	_ = conv

	if _, err := s.courses.GetByID(ctx, ws, courseID); err != nil {
		return nil, fmt.Errorf("chat: course access denied: %w", err)
	}

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

	// ── Step 1: Guardrails ───────────────────────────────────────────────
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

	// ── Step 2 & 3: Concurrent Query Enhancement + HyDE Document ─────────
	var (
		wg                  sync.WaitGroup
		enhanced            *llm.QueryEnhancement
		hydeDoc             string
		errEnhance, errHyde error
	)

	wg.Add(2)
	go func() {
		defer wg.Done()
		enhanced, errEnhance = s.aiClient.EnhanceQuery(ctx, userContent)
	}()
	go func() {
		defer wg.Done()
		hydeDoc, errHyde = s.aiClient.HydeDocument(ctx, userContent)
	}()
	wg.Wait()

	queryVariants := []string{userContent}
	if errEnhance != nil {
		log.Printf("chat: query enhancement failed (using original): %v", errEnhance)
	} else if enhanced != nil {
		if enhanced.StepBack != "" {
			queryVariants = append(queryVariants, enhanced.StepBack)
		}
		if enhanced.Rewritten != "" {
			queryVariants = append(queryVariants, enhanced.Rewritten)
		}
		queryVariants = append(queryVariants, enhanced.SubQueries...)
	}
	if errHyde != nil {
		log.Printf("chat: hyde failed: %v", errHyde)
	} else if hydeDoc != "" {
		queryVariants = append(queryVariants, hydeDoc)
	}

	// ── Step 4: Batch Vector Embeddings ──────────────────────────────────
	allVecs, err := s.embedder.Embed(ctx, queryVariants)
	if err != nil {
		return nil, fmt.Errorf("chat: embed queries: %w", err)
	}

	// ── Step 5: Parallel Vector Search in Qdrant ─────────────────────────
	type searchResult struct {
		results []provider.VectorSearchResult
		err     error
	}
	searchCh := make(chan searchResult, len(allVecs))
	var searchWg sync.WaitGroup

	for _, vec := range allVecs {
		searchWg.Add(1)
		go func(v provider.Vector) {
			defer searchWg.Done()
			results, err := s.vectors.Search(ctx, courseID, v, 20)
			searchCh <- searchResult{results: results, err: err}
		}(vec)
	}

	go func() {
		searchWg.Wait()
		close(searchCh)
	}()

	var allResultSets [][]provider.VectorSearchResult
	for sr := range searchCh {
		if sr.err != nil {
			log.Printf("chat: vector search error: %v", sr.err)
			continue
		}
		if len(sr.results) > 0 {
			allResultSets = append(allResultSets, sr.results)
		}
	}

	// ── Step 6: Reciprocal Rank Fusion ────────────────────────────────────
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

	// ── Step 7: Fetch Chunk Content ───────────────────────────────────────
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

	// ── Step 8: Rerank ────────────────────────────────────────────────────
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
		log.Printf("chat: rerank failed (using RRF order): %v", err)
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

	rankedChunks := make([]*entities.Chunk, 0, len(ranked))
	for _, rc := range ranked {
		if c, ok := chunkByID[rc.ChunkID]; ok {
			rankedChunks = append(rankedChunks, c)
		}
	}

	var contextBuilder strings.Builder
	for i, c := range rankedChunks {
		fmt.Fprintf(&contextBuilder, "--- Excerpt %d ---\n%s\n\n", i+1, c.Content)
	}
	context_ := contextBuilder.String()
	if context_ == "" {
		context_ = "No relevant course material was found for this question."
	}

	// ── Step 9: Stream Response Generation ───────────────────────────────
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

		score, _, err := s.evaluator.Evaluate(ctx, userContent, fullContent, []string{context_})
		if err != nil {
			log.Printf("chat: evaluate: %v", err)
			score = 8
		}

		if score > bestScore {
			bestScore = score
			bestContent = fullContent
		}

		if score >= 7 {
			break
		}
	}

	tokenCh <- StreamToken{Done: true}

	// ── Step 10: Persist Assistant Message + Citations ───────────────────
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
	_ = s.messages.Create(ctx, assistantMsg)

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
		_ = s.citations.CreateBatch(ctx, cits)
	}

	return &MessageResult{
		MessageID:  assistantMsg.ID,
		Content:    bestContent,
		Citations:  citResults,
		Confidence: confidence,
	}, nil
}

func (s *Service) ListConversations(ctx context.Context, ws repository.WorkspaceID, projectID string) ([]*entities.Conversation, string, error) {
	return s.conversations.ListByProject(ctx, ws, projectID, "", 50)
}

func (s *Service) GetChunk(ctx context.Context, id string) (*entities.Chunk, error) {
	return s.chunks.GetByID(ctx, id)
}
