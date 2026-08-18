package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"archadilm/internal/domain/entities"
	"archadilm/internal/domain/provider"
	"archadilm/internal/domain/repository"
	"archadilm/internal/infrastructure/llm"
	rediscache "archadilm/internal/infrastructure/redis"
)

const MaxUserMessageLength = 4000

// Service owns the chat (query pipeline) use case. Retrieval is always
// scoped to one conversation's own documents — there is no broader scope to
// accidentally search across.
type Service struct {
	conversations repository.ConversationRepository
	documents     repository.DocumentRepository
	messages      repository.MessageRepository
	citations     repository.CitationRepository
	chunks        repository.ChunkRepository
	aiClient      *llm.Client
	guardrails    provider.GuardrailProvider
	evaluator     provider.EvaluatorProvider
	vectors       provider.VectorStore
	embedder      provider.EmbeddingProvider
	cache         *rediscache.Cache
	maxRetries    int
	ids           provider.IDGenerator
}

func NewService(
	conversations repository.ConversationRepository,
	documents repository.DocumentRepository,
	messages repository.MessageRepository,
	citations repository.CitationRepository,
	chunks repository.ChunkRepository,
	aiClient *llm.Client,
	vectors provider.VectorStore,
	embedder provider.EmbeddingProvider,
	maxRetries int,
	ids provider.IDGenerator,
	cache *rediscache.Cache,
) *Service {
	return &Service{
		conversations: conversations,
		documents:     documents,
		messages:      messages,
		citations:     citations,
		chunks:        chunks,
		aiClient:      aiClient,
		guardrails:    aiClient,
		evaluator:     aiClient,
		vectors:       vectors,
		embedder:      embedder,
		cache:         cache,
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

func (s *Service) CreateConversation(ctx context.Context, ws repository.WorkspaceID) (*entities.Conversation, error) {
	conv := &entities.Conversation{
		ID:          s.ids.New(),
		WorkspaceID: ws,
		Title:       "New Chat",
	}
	if err := s.conversations.Create(ctx, conv); err != nil {
		return nil, fmt.Errorf("chat: create conversation: %w", err)
	}
	return conv, nil
}

// Send runs the query pipeline for one user turn. If the query is a simple
// greeting or smalltalk, it takes the fast-path route. For knowledge queries,
// it executes the full 10-step RAG retrieval & grounding pipeline.
func (s *Service) Send(
	ctx context.Context,
	ws repository.WorkspaceID,
	conversationID string,
	userContent string,
	tokenCh chan<- StreamToken,
) (*MessageResult, error) {
	startTime := time.Now()

	if strings.TrimSpace(userContent) == "" {
		return nil, fmt.Errorf("chat: user content cannot be empty")
	}
	if utf8.RuneCountInString(userContent) > MaxUserMessageLength {
		return nil, fmt.Errorf("chat: user content exceeds limit of %d characters", MaxUserMessageLength)
	}

	// Verify conversation exists and belongs to workspace
	if _, err := s.conversations.GetByID(ctx, ws, conversationID); err != nil {
		return nil, fmt.Errorf("chat: conversation access denied: %w", err)
	}

	// Persist User Message to Postgres
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

	// ── Step 0: Fast Intent Classification & Route Decision ─────────────────
	// Check for simple greetings or casual smalltalk before running heavy guardrails/search
	cleanInput := strings.TrimSpace(strings.ToLower(userContent))
	isFastRoute := strings.HasPrefix(cleanInput, "hi") ||
		strings.HasPrefix(cleanInput, "hello") ||
		strings.HasPrefix(cleanInput, "hey") ||
		strings.HasPrefix(cleanInput, "thanks") ||
		cleanInput == "how are you"

	var context_ string
	var rankedChunks []*entities.Chunk
	var ltmMemories []string
	usedVector := false

	// ── Summary Fast-Path: Use pre-generated AI summary from indexing ────
	// Detects broad summary/overview requests and serves the pre-computed
	// Document.AISummary + Document.AIOverview instantly, skipping the heavy
	// RAG pipeline (guardrails, HyDE, vector search, etc.)
	isSummaryRoute := strings.Contains(cleanInput, "summary") ||
		strings.Contains(cleanInput, "summarize") ||
		strings.Contains(cleanInput, "summarise") ||
		strings.Contains(cleanInput, "overview") ||
		strings.Contains(cleanInput, "what is this about") ||
		strings.Contains(cleanInput, "what is this video") ||
		strings.Contains(cleanInput, "what is this document")

	if isSummaryRoute && s.documents != nil {
		docs, err := s.documents.ListByConversation(ctx, string(ws), conversationID)
		if err == nil && len(docs) > 0 {
			var summaryBuilder strings.Builder
			for _, d := range docs {
				if d.AISummary != "" || d.AIOverview != "" {
					fmt.Fprintf(&summaryBuilder, "## Source: %s\n\n", d.OriginalFilename)
					if d.AIOverview != "" {
						fmt.Fprintf(&summaryBuilder, "%s\n\n", d.AIOverview)
					}
					if d.AISummary != "" {
						fmt.Fprintf(&summaryBuilder, "### Detailed Summary\n%s\n\n", d.AISummary)
					}
				}
			}
			if summaryBuilder.Len() > 0 {
				context_ = summaryBuilder.String()
				log.Printf("chat: SUMMARY FAST-PATH for conversation=%s (%d docs)", conversationID, len(docs))
			}
		}
	}

	if isFastRoute {
		// FAST-PATH ROUTE: Skip Guardrails, HyDE & Qdrant Search (<200ms TTFT)
		context_ = "User is engaging in casual greeting or smalltalk. Respond warmly and concisely."
	} else if context_ != "" {
		// Summary fast-path already populated context_ above — skip full RAG
	} else {
		// ── Cache Hit Path: serve cached response instantly ────────────────
		if s.cache != nil {
			if cached, ok := s.cache.Get(ctx, conversationID, userContent); ok {
				var cachedResult MessageResult
				if err := json.Unmarshal([]byte(cached), &cachedResult); err == nil {
					log.Printf("chat: CACHE HIT for conversation=%s", conversationID)

					// Stream cached content in word-sized chunks for natural UX
					words := strings.Fields(cachedResult.Content)
					for i, w := range words {
						if i > 0 {
							tokenCh <- StreamToken{Text: " "}
						}
						tokenCh <- StreamToken{Text: w}
					}
					tokenCh <- StreamToken{Done: true}

					// Persist assistant message for history
					assistantMsg := &entities.Message{
						ID:             s.ids.New(),
						ConversationID: conversationID,
						Role:           entities.MessageRoleAssistant,
						Content:        cachedResult.Content,
						Status:         entities.MessageStatusCompleted,
					}
					_ = s.messages.Create(ctx, assistantMsg)

					return &MessageResult{
						MessageID:  assistantMsg.ID,
						Content:    cachedResult.Content,
						Citations:  cachedResult.Citations,
						Confidence: cachedResult.Confidence,
					}, nil
				}
			}
		}

		// KNOWLEDGE / MEMORY ROUTE: Execute full RAG pipeline

		// ── Step 1: Guardrails (concurrent) ─────────────────────────────────
		var (
			guardWg             sync.WaitGroup
			injResult           provider.InjectionResult
			piiResult           provider.PIIResult
			errInj, errPII      error
		)

		guardWg.Add(2)
		go func() {
			defer guardWg.Done()
			injResult, errInj = s.guardrails.CheckInjection(ctx, userContent)
		}()
		go func() {
			defer guardWg.Done()
			piiResult, errPII = s.guardrails.CheckPII(ctx, userContent)
		}()
		guardWg.Wait()

		if errInj != nil {
			log.Printf("chat: injection check: %v", errInj)
		} else if injResult.IsInjection {
			return nil, fmt.Errorf("chat: query rejected: prompt injection detected")
		}

		if errPII != nil {
			log.Printf("chat: pii check: %v", errPII)
		} else if piiResult.ContainsPII {
			return nil, fmt.Errorf("chat: query rejected: PII detected in input")
		}

		// ── Step 2 & 3: Concurrent Query Enhancement + HyDE Document ─────────
		usedVector = true
		var (
			wg                  sync.WaitGroup
			enhanced            *llm.QueryEnhancement
			hydeDoc             string
			errEnhance, errHyde error
		)

		wg.Add(3)
		go func() {
			defer wg.Done()
			enhanced, errEnhance = s.aiClient.EnhanceQuery(ctx, userContent)
		}()
		go func() {
			defer wg.Done()
			hydeDoc, errHyde = s.aiClient.HydeDocument(ctx, userContent)
		}()
		go func() {
			defer wg.Done()
			ltmMemories, _ = s.aiClient.SearchMemory(ctx, string(ws), userContent)
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
				results, err := s.vectors.Search(ctx, conversationID, v, 20)
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
			// Relevance thresholding: take matching chunks (fallback to raw top results if score filter is too strict)
			var relevant []provider.VectorSearchResult
			for _, r := range sr.results {
				if r.Score >= 0.25 {
					relevant = append(relevant, r)
				}
			}
			if len(relevant) == 0 && len(sr.results) > 0 {
				// Fallback to top-3 chunks for broad queries like "summarize video"
				limit := 3
				if len(sr.results) < limit {
					limit = len(sr.results)
				}
				relevant = append(relevant, sr.results[:limit]...)
			}
			if len(relevant) > 0 {
				allResultSets = append(allResultSets, relevant)
			}
		}

		// ── Step 6: Reciprocal Rank Fusion ────────────────────────────────────
		mergedResults := reciprocalRankFusion(allResultSets, 20)

		if len(mergedResults) == 0 {
			log.Printf("chat: Qdrant returned 0 relevant chunks for conversation=%s", conversationID)
			context_ = "No relevant material was found in the uploaded sources for this question. The user may need to upload additional documents or ask a more specific question."
		}

		// ── Step 7: Fetch Chunk Content from Postgres ─────────────────────────
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

		// ── Step 8: Qdrant RRF Ranking (Fast In-Memory Order) ────────────────
		ranked := make([]provider.RankedChunk, len(mergedResults))
		for i, r := range mergedResults {
			ranked[i] = provider.RankedChunk{ChunkID: r.ChunkID, Score: r.Score}
		}

		topK := 5
		if len(ranked) < topK {
			topK = len(ranked)
		}
		ranked = ranked[:topK]

		seenContent := make(map[string]bool)
		for _, rc := range ranked {
			if c, ok := chunkByID[rc.ChunkID]; ok {
				key := fmt.Sprintf("%s:%s", c.DocumentID, strings.TrimSpace(c.Content))
				if !seenContent[key] {
					seenContent[key] = true
					rankedChunks = append(rankedChunks, c)
				}
			}
		}

		var contextBuilder strings.Builder
		for i, c := range rankedChunks {
			if contextBuilder.Len() > 8000 {
				break
			}
			fmt.Fprintf(&contextBuilder, "--- Excerpt %d ---\n%s\n\n", i+1, c.Content)
		}
		context_ = contextBuilder.String()
		if context_ == "" {
			context_ = "No relevant material was found for this question."
		}
	}

	// ── Step 9: Conversational History (STM) & Mem0 Long-Term Memory (LTM) ─────
	if len(ltmMemories) > 0 {
		ltmBlock := fmt.Sprintf("User Long-Term Memory Facts:\n- %s\n\n", strings.Join(ltmMemories, "\n- "))
		context_ = ltmBlock + context_
	}

	var conversationHistory []provider.PromptMessage
	pastMsgs, _, errList := s.messages.ListByConversation(ctx, conversationID, "", 10)
	if errList == nil && len(pastMsgs) > 1 {
		for _, pm := range pastMsgs {
			if pm.ID == userMsg.ID {
				continue
			}
			role := "user"
			if pm.Role == entities.MessageRoleAssistant {
				role = "assistant"
			}
			conversationHistory = append(conversationHistory, provider.PromptMessage{
				Role:    role,
				Content: pm.Content,
			})
		}
	}
	conversationHistory = append(conversationHistory, provider.PromptMessage{
		Role:    "user",
		Content: fmt.Sprintf("Reference material:\n%s\n\nUser question: %s", context_, userContent),
	})

	var bestContent string
	confidence := "normal"

	// Always signal stream completion, even if all retries fail.
	streamCompleted := false
	defer func() {
		if !streamCompleted {
			tokenCh <- StreamToken{Done: true}
		}
	}()

	for attempt := 1; attempt <= s.maxRetries; attempt++ {
		prompt := provider.Prompt{
			System: `You are a knowledgeable study partner having a one-on-one conversation. Answer naturally and directly. Always remember details the user shares about themselves (like their name, background, or preferences) from their Long-Term Memory facts and past conversation messages.

FORMATTING RULES:
- Use ## Headings for major sections.
- Use - Bullet lists for features, key concepts, and points.
- Always put EVERY heading and EVERY bullet point on its own new line with a blank line before it.
- Never run multiple list items or headings together on the same line.`,
			Messages:      conversationHistory,
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

		bestContent = fullContent
		break
	}

	// If all retries failed, send a friendly message instead of leaving the stream stuck
	if bestContent == "" {
		bestContent = "Hmm, I'm having trouble generating a response right now. Could you try rephrasing your question or asking something different? 🔄"
		tokenCh <- StreamToken{Text: bestContent}
	}

	tokenCh <- StreamToken{Done: true}
	streamCompleted = true

	// Async Mem0 LTM Fact Extraction
	go func() {
		addCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := s.aiClient.AddMemory(addCtx, string(ws), userContent, bestContent); err != nil {
			log.Printf("chat: mem0 add memory notice: %v", err)
		}
	}()

	// ── Step 10: Persist Assistant Message, Citations & Log Metrics ───────
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
		displayTitle := c.DocumentName
		if displayTitle == "" {
			displayTitle = c.Title
		}
		citResults[i] = CitationResult{
			ChunkID:        c.ID,
			DocumentID:     c.DocumentID,
			StartTimestamp: c.StartTimestamp,
			Title:          displayTitle,
		}
	}
	if len(cits) > 0 {
		_ = s.citations.CreateBatch(ctx, cits)
	}

	// Log clear structured telemetry
	previewText := strings.ReplaceAll(bestContent, "\n", " ")
	if len(previewText) > 100 {
		previewText = previewText[:100] + "..."
	}

	log.Printf("\n"+
		"=================== [CHAT TELEMETRY METRICS] ===================\n"+
		"| FastRoute      : %t\n"+
		"| Vector Search  : %t (Citations: %d)\n"+
		"| Latency        : %v\n"+
		"| Output Preview : \"%s\"\n"+
		"================================================================\n",
		isFastRoute, usedVector, len(citResults), time.Since(startTime), previewText)


	result := &MessageResult{
		MessageID:  assistantMsg.ID,
		Content:    bestContent,
		Citations:  citResults,
		Confidence: confidence,
	}

	// Cache the result for knowledge-route queries (15 min TTL)
	if s.cache != nil && usedVector {
		if cacheJSON, err := json.Marshal(result); err == nil {
			s.cache.Set(ctx, conversationID, userContent, string(cacheJSON), 15*time.Minute)
		}
	}

	return result, nil
}

func (s *Service) ListConversations(ctx context.Context, ws repository.WorkspaceID) ([]*entities.Conversation, string, error) {
	return s.conversations.ListByWorkspace(ctx, ws, "", 50)
}

func (s *Service) GetChunk(ctx context.Context, id string) (*entities.Chunk, error) {
	return s.chunks.GetByID(ctx, id)
}

func (s *Service) GetConversationMessages(ctx context.Context, conversationID string) ([]*entities.Message, string, error) {
	msgs, next, err := s.messages.ListByConversation(ctx, conversationID, "", 100)
	if err != nil {
		return nil, "", err
	}
	for _, m := range msgs {
		if m.Role == entities.MessageRoleAssistant {
			cits, err := s.citations.ListByMessage(ctx, m.ID)
			if err == nil && len(cits) > 0 {
				m.Citations = cits
			}
		}
	}
	return msgs, next, nil
}

func (s *Service) DeleteConversation(ctx context.Context, ws repository.WorkspaceID, id string) error {
	return s.conversations.Delete(ctx, ws, id)
}

func (s *Service) UpdateConversationTitle(ctx context.Context, ws repository.WorkspaceID, id string, title string) error {
	return s.conversations.UpdateTitle(ctx, ws, id, title)
}
