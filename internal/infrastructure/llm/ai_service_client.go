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
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"

	"archadilm/internal/domain/provider"
	"archadilm/internal/infrastructure/resilience"
)

// Client wraps the AI Service HTTP API.
type Client struct {
    base         string
    http         *http.Client
    embedCB      *resilience.CircuitBreaker
	extractCB    *resilience.CircuitBreaker
    generateCB   *resilience.CircuitBreaker
    rerankCB     *resilience.CircuitBreaker
    evaluateCB   *resilience.CircuitBreaker
    guardrailCB  *resilience.CircuitBreaker
}

// Compile-time interface assertions.
var (
	_ provider.LLMProvider       = (*Client)(nil)
	_ provider.EmbeddingProvider = (*Client)(nil)
	_ provider.RerankerProvider  = (*Client)(nil)
	_ provider.GuardrailProvider = (*Client)(nil)
	_ provider.EvaluatorProvider = (*Client)(nil)
)

// NewClient creates a Client pointed at aiServiceURL (e.g. "http://localhost:8000").
func NewClient(aiServiceURL string) *Client {
	return &Client{
		base: strings.TrimRight(aiServiceURL, "/"),
		http: &http.Client{Timeout: 60 * time.Second},
		// Circuit breakers: open after 5 failures, reset after 30 seconds
        embedCB:     resilience.NewCircuitBreaker(5, 30*time.Second),
        extractCB:   resilience.NewCircuitBreaker(5, 30*time.Second),
        generateCB:  resilience.NewCircuitBreaker(5, 30*time.Second),
        rerankCB:    resilience.NewCircuitBreaker(5, 30*time.Second),
        evaluateCB:  resilience.NewCircuitBreaker(5, 30*time.Second),
        guardrailCB: resilience.NewCircuitBreaker(5, 30*time.Second),
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
    err := c.embedCB.Execute(ctx, func() error {
        return c.postJSON(ctx, "/embeddings", embeddingRequest{Texts: texts}, &resp)
    })
    
    if err != nil {
        if err == resilience.ErrCircuitOpen {
            return nil, fmt.Errorf("llm: embed: circuit breaker open, AI Service unavailable")
        }
        return nil, fmt.Errorf("llm: embed: %w", err)
    }
    
    vecs := make([]provider.Vector, len(resp.Embeddings))
    for i, e := range resp.Embeddings {
        vecs[i] = provider.Vector(e)
    }
    return vecs, nil
}

// ── 2. provider.RerankerProvider ──────────────────────────────────────────

type rerankChunk struct {
	ChunkID    string `json:"chunk_id"`
	DocumentID string `json:"document_id,omitempty"`
	Content    string `json:"content"`
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
            ChunkID:    cand.ChunkID,
            DocumentID: cand.DocumentID,
            Content:    cand.Content,
        }
    }
 
    var resp rerankResponse
    err := c.rerankCB.Execute(ctx, func() error {
        return c.postJSON(ctx, "/rerank", rerankRequest{
            Query:      query,
            Candidates: chunks,
            TopK:       len(candidates),
        }, &resp)
    })
    
    if err != nil {
        if err == resilience.ErrCircuitOpen {
            return nil, fmt.Errorf("llm: rerank: circuit breaker open, AI Service unavailable")
        }
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
    err := c.generateCB.Execute(ctx, func() error {
        return c.postJSON(ctx, "/generate", generationRequest{
            Query:         query,
            Context:       prompt.System,
            PromptVersion: prompt.PromptVersion,
        }, &resp)
    })
    
    if err != nil {
        if err == resilience.ErrCircuitOpen {
            return provider.Response{}, fmt.Errorf("llm: generate: circuit breaker open, AI Service unavailable")
        }
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
    err := c.evaluateCB.Execute(ctx, func() error {
        return c.postJSON(ctx, "/evaluate", evaluationRequest{
            Query:         query,
            Response:      response,
            Context:       context_,
            PromptVersion: "1.0",
        }, &resp)
    })
    
    if err != nil {
        if err == resilience.ErrCircuitOpen {
            return 0, "", fmt.Errorf("llm: evaluate: circuit breaker open, AI Service unavailable")
        }
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
    err := c.guardrailCB.Execute(ctx, func() error {
        return c.postJSON(ctx, "/guardrails/check", guardrailRequest{Text: text}, &resp)
    })
    
    if err != nil {
        if err == resilience.ErrCircuitOpen {
            return provider.InjectionResult{}, fmt.Errorf("llm: check-injection: circuit breaker open, AI Service unavailable")
        }
        return provider.InjectionResult{}, fmt.Errorf("llm: check-injection: %w", err)
    }
    return provider.InjectionResult{
        IsInjection: !resp.Passed,
        Reason:      resp.Reason,
    }, nil
}

func (c *Client) CheckPII(ctx context.Context, text string) (provider.PIIResult, error) {
    var resp guardrailResponse
    err := c.guardrailCB.Execute(ctx, func() error {
        return c.postJSON(ctx, "/guardrails/check", guardrailRequest{Text: text}, &resp)
    })
    
    if err != nil {
        if err == resilience.ErrCircuitOpen {
            return provider.PIIResult{}, fmt.Errorf("llm: check-pii: circuit breaker open, AI Service unavailable")
        }
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

// ── 7. Advanced RAG Pipeline Methods ──────────────────────────────────────

// QueryEnhancement holds the result of query understanding.
type QueryEnhancement struct {
	StepBack   string   `json:"step_back"`
	Rewritten  string   `json:"rewritten"`
	SubQueries []string `json:"sub_queries"`
}

type enhanceQueryRequest struct {
	Query         string `json:"query"`
	PromptVersion string `json:"prompt_version"`
}

// EnhanceQuery calls the AI Service to get step-back, rewritten, and
// sub-query variants of the user's question.
func (c *Client) EnhanceQuery(ctx context.Context, query string) (*QueryEnhancement, error) {
	var resp QueryEnhancement
	if err := c.postJSON(ctx, "/enhance-query", enhanceQueryRequest{
		Query:         query,
		PromptVersion: "1.0",
	}, &resp); err != nil {
		return nil, fmt.Errorf("llm: enhance-query: %w", err)
	}
	return &resp, nil
}

type hydeDocumentRequest struct {
	Query         string `json:"query"`
	PromptVersion string `json:"prompt_version"`
}

type hydeDocumentResponse struct {
	Document string `json:"document"`
}

// HydeDocument generates a hypothetical document for HyDE embedding.
func (c *Client) HydeDocument(ctx context.Context, query string) (string, error) {
	var resp hydeDocumentResponse
	if err := c.postJSON(ctx, "/hyde-document", hydeDocumentRequest{
		Query:         query,
		PromptVersion: "1.0",
	}, &resp); err != nil {
		return "", fmt.Errorf("llm: hyde-document: %w", err)
	}
	return resp.Document, nil
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

const (
    maxPDFSize = 50 * 1024 * 1024  // 50MB limit
)

// ExtractPDF sends raw PDF bytes to the AI Service and returns extracted pages.
// Uses multipart/form-data with circuit breaker, context support, and size validation.
func (c *Client) ExtractPDF(ctx context.Context, pdfData []byte) ([]PDFPage, error) {
    // Security: Validate PDF size before sending
    if len(pdfData) == 0 {
        return nil, fmt.Errorf("llm: extract-pdf: empty pdf data")
    }
    if len(pdfData) > maxPDFSize {
        return nil, fmt.Errorf("llm: extract-pdf: pdf size %d exceeds limit %d", len(pdfData), maxPDFSize)
    }
 
    var result struct {
        Pages []PDFPage `json:"pages"`
    }
 
    err := c.extractCB.Execute(ctx, func() error {
        var buf bytes.Buffer
        writer := multipart.NewWriter(&buf)
        
        part, err := writer.CreateFormFile("file", "document.pdf")
        if err != nil {
            return fmt.Errorf("create form file: %w", err)
        }
        if _, err := part.Write(pdfData); err != nil {
            return fmt.Errorf("write pdf data: %w", err)
        }
        if err := writer.Close(); err != nil {
            return fmt.Errorf("close multipart writer: %w", err)
        }
 
        req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/extract-pdf", &buf)
        if err != nil {
            return fmt.Errorf("create request: %w", err)
        }
        req.Header.Set("Content-Type", writer.FormDataContentType())
 
        resp, err := c.http.Do(req)
        if err != nil {
            return fmt.Errorf("http request: %w", err)
        }
        defer resp.Body.Close()
 
        if resp.StatusCode != http.StatusOK {
            raw, _ := io.ReadAll(resp.Body)
            return fmt.Errorf("ai service returned status %d: %s", resp.StatusCode, string(raw))
        }
 
        return json.NewDecoder(resp.Body).Decode(&result)
    })
 
    if err != nil {
        if err == resilience.ErrCircuitOpen {
            return nil, fmt.Errorf("llm: extract-pdf: circuit breaker open, AI Service unavailable")
        }
        return nil, fmt.Errorf("llm: extract-pdf: %w", err)
    }
 
    if len(result.Pages) == 0 {
        return nil, fmt.Errorf("llm: extract-pdf: no pages extracted")
    }
 
    return result.Pages, nil
}

// PDFPage represents one page of extracted PDF text.
type PDFPage struct {
	PageNumber int    `json:"page_number"`
	Text       string `json:"text"`
}

// URLSection is one section of extracted web page content.
type URLSection struct {
	Text    string `json:"text"`
	Heading string `json:"heading,omitempty"`
}

// URLExtraction is the result of web URL extraction.
type URLExtraction struct {
	Title    string       `json:"title"`
	Sections []URLSection `json:"sections"`
}

type extractURLRequest struct {
	URL string `json:"url"`
}

// ExtractURL fetches and extracts readable text from a web URL.
// Validates URL for SSRF prevention, uses circuit breaker and context.
func (c *Client) ExtractURL(ctx context.Context, targetURL string, allowedDomains []string) (*URLExtraction, error) {
    // Security: Validate URL to prevent SSRF attacks
    if targetURL == "" {
        return nil, fmt.Errorf("llm: extract-url: empty url")
    }
 
    parsedURL, err := url.Parse(targetURL)
    if err != nil {
        return nil, fmt.Errorf("llm: extract-url: invalid url: %w", err)
    }
 
    // Security: Only allow http/https schemes
    if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
        return nil, fmt.Errorf("llm: extract-url: invalid scheme %q, only http/https allowed", parsedURL.Scheme)
    }
 
    // Security: Block private/internal IPs (SSRF prevention)
    host := parsedURL.Hostname()
    if isPrivateIP(host) {
        return nil, fmt.Errorf("llm: extract-url: access to private IP %q blocked", host)
    }

	// Security: Check domain whitelist if configured
    if len(allowedDomains) > 0 {
        domain := parsedURL.Hostname()
        allowed := false
        for _, allowedDomain := range allowedDomains {
            if domain == allowedDomain || strings.HasSuffix(domain, "."+allowedDomain) {
                allowed = true
                break
            }
        }
        if !allowed {
            return nil, fmt.Errorf("llm: extract-url: domain %q not in whitelist", domain)
        }
    }
 
    var resp URLExtraction
    err = c.extractCB.Execute(ctx, func() error {
        return c.postJSON(ctx, "/extract-url", extractURLRequest{URL: targetURL}, &resp)
    })
 
    if err != nil {
        if err == resilience.ErrCircuitOpen {
            return nil, fmt.Errorf("llm: extract-url: circuit breaker open, AI Service unavailable")
        }
        return nil, fmt.Errorf("llm: extract-url: %w", err)
    }
 
    if len(resp.Sections) == 0 {
        return nil, fmt.Errorf("llm: extract-url: no content extracted from url")
    }
 
    return &resp, nil
}
 
// isPrivateIP checks if a hostname is a private/internal IP address.
// This prevents SSRF attacks against internal services.
func isPrivateIP(host string) bool {
    // Check for localhost variants
    if host == "localhost" || host == "127.0.0.1" || host == "::1" {
        return true
    }
 
    // Check for private IP ranges (basic check)
    // In production, use a proper IP parsing library
    privateRanges := []string{
        "10.", "172.16.", "172.17.", "172.18.", "172.19.", "172.20.",
        "172.21.", "172.22.", "172.23.", "172.24.", "172.25.", "172.26.",
        "172.27.", "172.28.", "172.29.", "172.30.", "172.31.", "192.168.",
    }
    for _, prefix := range privateRanges {
        if len(host) > len(prefix) && host[:len(prefix)] == prefix {
            return true
        }
    }
 
    return false
}
