// Package llm provides the Go HTTP client for the Python AI Service.
// It implements provider.LLMProvider, provider.EmbeddingProvider,
// provider.RerankerProvider, provider.GuardrailProvider, and provider.EvaluatorProvider.
package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"course-assistant/internal/domain/provider"
)

// Client wraps the AI Service HTTP API.
type Client struct {
	base string
	http *http.Client
}

// Compile-time interface assertions.
var (
	_ provider.LLMProvider       = (*Client)(nil)
	_ provider.EmbeddingProvider   = (*Client)(nil)
	_ provider.RerankerProvider    = (*Client)(nil)
	_ provider.GuardrailProvider   = (*Client)(nil)
	_ provider.EvaluatorProvider   = (*Client)(nil)
)

// NewClient creates a Client pointed at aiServiceURL (e.g. "http://localhost:8000").
func NewClient(aiServiceURL string) *Client {
	return &Client{
		base: strings.TrimRight(aiServiceURL, "/"),
		http: &http.Client{Timeout: 60 * time.Second},
	}
}

// ── 1. provider.EmbeddingProvider ─────────────────────────────────────────

type embeddingRequest struct {
	Texts []string `json:"texts"`
}

type embeddingResponse struct {
	Embeddings [][]float32 `json:"embeddings"`
	Model      string      `json:"model"`
}

func (c *Client) Embed(ctx context.Context, texts []string) ([]provider.Vector, error) {
	var resp embeddingResponse
	if err := c.postJSON(ctx, "/embeddings", embeddingRequest{Texts: texts}, &resp); err != nil {
		return nil, fmt.Errorf("llm: embed: %w", err)
	}
	if len(resp.Embeddings) != len(texts) {
		return nil, fmt.Errorf("llm: embed: expected %d vectors, got %d", len(texts), len(resp.Embeddings))
	}
	vecs := make([]provider.Vector, len(resp.Embeddings))
	for i, e := range resp.Embeddings {
		vecs[i] = provider.Vector(e)
	}
	return vecs, nil
}

// ── 2. provider.RerankerProvider ──────────────────────────────────────────

type rerankChunk struct {
	ChunkID string `json:"chunk_id"`
	Content string `json:"content"`
}

type rerankRequest struct {
	Query      string        `json:"query"`
	Candidates []rerankChunk `json:"candidates"`
	TopK       int           `json:"top_k"`
}

type rerankResponse struct {
	RankedChunks []rerankChunk `json:"ranked_chunks"`
}

func (c *Client) Rerank(ctx context.Context, query string, candidates []provider.RerankCandidate) ([]provider.RankedChunk, error) {
	chunks := make([]rerankChunk, len(candidates))
	for i, cand := range candidates {
		chunks[i] = rerankChunk{
			ChunkID: cand.ChunkID,
			Content: cand.Content,
		}
	}

	var resp rerankResponse
	if err := c.postJSON(ctx, "/rerank", rerankRequest{
		Query:      query,
		Candidates: chunks,
		TopK:       len(candidates),
	}, &resp); err != nil {
		return nil, fmt.Errorf("llm: rerank: %w", err)
	}

	ranked := make([]provider.RankedChunk, len(resp.RankedChunks))
	for i, rc := range resp.RankedChunks {
		ranked[i] = provider.RankedChunk{
			ChunkID: rc.ChunkID,
			Score:   float64(len(resp.RankedChunks) - i),
		}
	}
	return ranked, nil
}

// ── 3. provider.LLMProvider ───────────────────────────────────────────────

type generationRequest struct {
	Query         string `json:"query"`
	Context       string `json:"context"`
	PromptVersion string `json:"prompt_version"`
}

type generationResponse struct {
	Content string `json:"content"`
}

func (c *Client) Generate(ctx context.Context, prompt provider.Prompt) (provider.Response, error) {
	var query string
	for _, msg := range prompt.Messages {
		if msg.Role == "user" {
			query = msg.Content
		}
	}

	var resp generationResponse
	if err := c.postJSON(ctx, "/generate", generationRequest{
		Query:         query,
		Context:       prompt.System,
		PromptVersion: prompt.PromptVersion,
	}, &resp); err != nil {
		return provider.Response{}, fmt.Errorf("llm: generate: %w", err)
	}

	return provider.Response{
		Content:       resp.Content,
		PromptVersion: prompt.PromptVersion,
	}, nil
}

func (c *Client) Stream(ctx context.Context, prompt provider.Prompt) (<-chan provider.Token, error) {
	var query string
	for _, msg := range prompt.Messages {
		if msg.Role == "user" {
			query = msg.Content
		}
	}

	body, err := json.Marshal(generationRequest{
		Query:         query,
		Context:       prompt.System,
		PromptVersion: prompt.PromptVersion,
	})
	if err != nil {
		return nil, fmt.Errorf("llm: stream marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/generate", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("llm: stream request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/plain")

	streamClient := &http.Client{}
	resp, err := streamClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("llm: stream do: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("llm: stream status %d", resp.StatusCode)
	}

	ch := make(chan provider.Token, 64)
	go func() {
		defer close(ch)
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			text := scanner.Text()
			if text == "[DONE]" {
				ch <- provider.Token{Done: true}
				return
			}
			if strings.HasPrefix(text, "[ERROR:") {
				return
			}
			ch <- provider.Token{Text: text, Done: false}
		}
	}()

	return ch, nil
}

// ── 4. provider.EvaluatorProvider ─────────────────────────────────────────

type evaluationRequest struct {
	Query         string `json:"query"`
	Response      string `json:"response"`
	Context       string `json:"context"`
	PromptVersion string `json:"prompt_version"`
}

type evaluationResponse struct {
	Score           float64 `json:"score"`
	PassesThreshold bool    `json:"passes_threshold"`
}

func (c *Client) Evaluate(ctx context.Context, query string, response string, groundingChunks []string) (int, string, error) {
	context_ := strings.Join(groundingChunks, "\n---\n")
	var resp evaluationResponse
	if err := c.postJSON(ctx, "/evaluate", evaluationRequest{
		Query:         query,
		Response:      response,
		Context:       context_,
		PromptVersion: "1.0",
	}, &resp); err != nil {
		return 0, "", fmt.Errorf("llm: evaluate: %w", err)
	}

	reason := "evaluation passed"
	if !resp.PassesThreshold {
		reason = "evaluation failed threshold"
	}
	return int(resp.Score), reason, nil
}

// ── 5. provider.GuardrailProvider ─────────────────────────────────────────

type guardrailRequest struct {
	Text string `json:"text"`
}

type guardrailResponse struct {
	Passed bool   `json:"passed"`
	Reason string `json:"reason"`
}

func (c *Client) CheckInjection(ctx context.Context, text string) (provider.InjectionResult, error) {
	var resp guardrailResponse
	if err := c.postJSON(ctx, "/guardrails/check", guardrailRequest{Text: text}, &resp); err != nil {
		return provider.InjectionResult{}, fmt.Errorf("llm: check-injection: %w", err)
	}
	return provider.InjectionResult{
		IsInjection: !resp.Passed,
		Reason:      resp.Reason,
	}, nil
}

func (c *Client) CheckPII(ctx context.Context, text string) (provider.PIIResult, error) {
	var resp guardrailResponse
	if err := c.postJSON(ctx, "/guardrails/check", guardrailRequest{Text: text}, &resp); err != nil {
		return provider.PIIResult{}, fmt.Errorf("llm: check-pii: %w", err)
	}
	redacted := text
	if !resp.Passed {
		redacted = "[REDACTED]"
	}
	return provider.PIIResult{
		ContainsPII: !resp.Passed,
		Redacted:    redacted,
	}, nil
}

// ── 6. Extra Worker Helper Methods ────────────────────────────────────────

type titleRequest struct {
	Content       string `json:"content"`
	PromptVersion string `json:"prompt_version"`
}

type titleResponse struct {
	Title string `json:"title"`
}

func (c *Client) GenerateTitle(ctx context.Context, content, promptVersion string) (string, error) {
	var resp titleResponse
	if err := c.postJSON(ctx, "/generate-title", titleRequest{Content: content, PromptVersion: promptVersion}, &resp); err != nil {
		return "", fmt.Errorf("llm: generate-title: %w", err)
	}
	return resp.Title, nil
}

type summaryRequest struct {
	Content       string `json:"content"`
	PromptVersion string `json:"prompt_version"`
}

type summaryResponse struct {
	Summary string `json:"summary"`
}

func (c *Client) GenerateSummary(ctx context.Context, content, promptVersion string) (string, error) {
	var resp summaryResponse
	if err := c.postJSON(ctx, "/generate-summary", summaryRequest{Content: content, PromptVersion: promptVersion}, &resp); err != nil {
		return "", fmt.Errorf("llm: generate-summary: %w", err)
	}
	return resp.Summary, nil
}

// ── Internal Helpers ──────────────────────────────────────────────────────

func (c *Client) postJSON(ctx context.Context, path string, body, out any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+path, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("http post %s: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ai service %s: status %d: %s", path, resp.StatusCode, string(raw))
	}

	return json.NewDecoder(resp.Body).Decode(out)
}
