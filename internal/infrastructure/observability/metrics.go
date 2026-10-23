package observability

import (
	"context"
	"sync/atomic"
	"time"
)

const TraceIDHeader = "X-Trace-ID"

type traceIDContextKey struct{}

// WithTraceID injects a distributed trace ID into context.
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceIDContextKey{}, traceID)
}

// TraceIDFromContext retrieves the trace ID from context.
func TraceIDFromContext(ctx context.Context) string {
	if traceID, ok := ctx.Value(traceIDContextKey{}).(string); ok {
		return traceID
	}
	return ""
}

type Metrics struct {
	// Processing times
	ParserProcessingTime    atomic.Int64
	ChunkerProcessingTime   atomic.Int64
	MetadataProcessingTime  atomic.Int64
	EmbeddingProcessingTime atomic.Int64

	// Queue depths
	UploadQueueDepth   atomic.Int64
	ManifestQueueDepth atomic.Int64
	ParseQueueDepth    atomic.Int64
	ChunkQueueDepth    atomic.Int64
	MetadataQueueDepth atomic.Int64

	// Error counts
	ParserErrors    atomic.Int64
	ChunkerErrors   atomic.Int64
	MetadataErrors  atomic.Int64
	EmbeddingErrors atomic.Int64

	// AI Service metrics
	AIServiceLatency atomic.Int64
	AIServiceErrors  atomic.Int64
	AIServiceCalls   atomic.Int64

	// GenAI Chat pipeline metrics (Tier 2 Observability)
	ChatRequestsTotal           atomic.Int64
	ChatFastRouteHits           atomic.Int64
	ChatKnowledgeRouteHits      atomic.Int64
	ChatGuardrailDurationNs     atomic.Int64
	ChatQueryEnhanceDurationNs  atomic.Int64
	ChatMemorySearchDurationNs  atomic.Int64
	ChatRetrievalDurationNs     atomic.Int64
	ChatGenerationDurationNs    atomic.Int64
	ChatTotalLatencyNs          atomic.Int64
}

var GlobalMetrics = &Metrics{}

func RecordProcessingTime(stage string, duration time.Duration) {
	nanos := duration.Nanoseconds()
	switch stage {
	case "parser":
		GlobalMetrics.ParserProcessingTime.Add(nanos)
	case "chunker":
		GlobalMetrics.ChunkerProcessingTime.Add(nanos)
	case "metadata":
		GlobalMetrics.MetadataProcessingTime.Add(nanos)
	case "embedding":
		GlobalMetrics.EmbeddingProcessingTime.Add(nanos)
	}
}

func RecordError(stage string) {
	switch stage {
	case "parser":
		GlobalMetrics.ParserErrors.Add(1)
	case "chunker":
		GlobalMetrics.ChunkerErrors.Add(1)
	case "metadata":
		GlobalMetrics.MetadataErrors.Add(1)
	case "embedding":
		GlobalMetrics.EmbeddingErrors.Add(1)
	}
}

func RecordAIServiceCall(latency time.Duration, err error) {
	GlobalMetrics.AIServiceCalls.Add(1)
	GlobalMetrics.AIServiceLatency.Add(latency.Nanoseconds())
	if err != nil {
		GlobalMetrics.AIServiceErrors.Add(1)
	}
}

type ChatSpanTimings struct {
	IsFastRoute          bool
	IsKnowledgeRoute     bool
	GuardrailDuration    time.Duration
	QueryEnhanceDuration time.Duration
	MemorySearchDuration time.Duration
	RetrievalDuration    time.Duration
	GenerationDuration   time.Duration
	TotalLatency         time.Duration
}

func RecordChatTelemetry(t ChatSpanTimings) {
	GlobalMetrics.ChatRequestsTotal.Add(1)
	if t.IsFastRoute {
		GlobalMetrics.ChatFastRouteHits.Add(1)
	}
	if t.IsKnowledgeRoute {
		GlobalMetrics.ChatKnowledgeRouteHits.Add(1)
	}
	GlobalMetrics.ChatGuardrailDurationNs.Add(t.GuardrailDuration.Nanoseconds())
	GlobalMetrics.ChatQueryEnhanceDurationNs.Add(t.QueryEnhanceDuration.Nanoseconds())
	GlobalMetrics.ChatMemorySearchDurationNs.Add(t.MemorySearchDuration.Nanoseconds())
	GlobalMetrics.ChatRetrievalDurationNs.Add(t.RetrievalDuration.Nanoseconds())
	GlobalMetrics.ChatGenerationDurationNs.Add(t.GenerationDuration.Nanoseconds())
	GlobalMetrics.ChatTotalLatencyNs.Add(t.TotalLatency.Nanoseconds())
}
