// Package provider defines the vendor-agnostic interfaces from
// docs/02-system-architecture.md#provider-abstraction. Vendor choice
// (which LLM, which reranker, which guardrail classifier) is a config-time
// implementation detail behind these interfaces, never a type the rest of
// the codebase names directly. Concrete implementations live in
// internal/infrastructure/llm and talk to the Python AI Service described in
// docs/06-ai-service.md over gRPC/HTTP.
package provider

import "context"

// PromptVersion tags every Generate/Stream/Evaluate call so a prompt change
// is A/B-able and traceable without a redeploy — see
// docs/06-ai-service.md#prompt-versioning.
type PromptVersion = string

type Prompt struct {
	System        string
	Messages      []PromptMessage
	PromptVersion PromptVersion
}

type PromptMessage struct {
	Role    string // "user" | "assistant"
	Content string
}

type Response struct {
	Content       string
	PromptVersion PromptVersion
}

type Token struct {
	Text string
	Done bool
}

// LLMProvider generates or streams a completion for a prompt. The AI Service
// is stateless: it computes and returns, never persists — see
// docs/06-ai-service.md#core-principle-stateless-compute.
type LLMProvider interface {
	Generate(ctx context.Context, prompt Prompt) (Response, error)
	Stream(ctx context.Context, prompt Prompt) (<-chan Token, error)
}

// Vector is a dense embedding. Kept as []float32 rather than a richer type
// since nothing in Go code needs to inspect individual dimensions.
type Vector []float32

type EmbeddingProvider interface {
	Embed(ctx context.Context, texts []string) ([]Vector, error)
}

// RankedChunk pairs a candidate chunk ID with a reranker score. Kept as an ID
// reference (not the full entities.Chunk) so this package never imports
// internal/domain/entities and creates a dependency cycle risk; callers in
// application/ resolve IDs back to entities themselves.
type RankedChunk struct {
	ChunkID string
	Score   float64
}

type RerankCandidate struct {
	ChunkID string
	Content string
}

type RerankerProvider interface {
	Rerank(ctx context.Context, query string, candidates []RerankCandidate) ([]RankedChunk, error)
}

type PIIResult struct {
	ContainsPII bool
	Redacted    string // safe-to-log version, if ContainsPII
}

type InjectionResult struct {
	IsInjection bool
	Reason      string
}

// GuardrailProvider implements the pre/post-guardrail checks from
// docs/05-query-pipeline.md#pipeline-detail.
type GuardrailProvider interface {
	CheckPII(ctx context.Context, text string) (PIIResult, error)
	CheckInjection(ctx context.Context, text string) (InjectionResult, error)
}

// EvaluatorProvider scores a generated response 1-10 per
// docs/05-query-pipeline.md#pipeline-detail. Modeled separately from
// LLMProvider even though MVP backs both with the same mini-model, because
// the query pipeline's retry loop reasons about "the evaluator" as its own
// role, independent of which model implements it.
type EvaluatorProvider interface {
	Evaluate(ctx context.Context, query string, response string, groundingChunks []string) (score int, reason string, err error)
}
