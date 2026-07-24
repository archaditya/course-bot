package observability

import (
	"sync/atomic"
	"time"
)

type Metrics struct {
	// Processing times
	ParserProcessingTime    atomic.Int64
	ChunkerProcessingTime   atomic.Int64
	MetadataProcessingTime  atomic.Int64
	EmbeddingProcessingTime atomic.Int64
	
	// Queue depths
	UploadQueueDepth    atomic.Int64
	ManifestQueueDepth  atomic.Int64
	ParseQueueDepth     atomic.Int64
	ChunkQueueDepth     atomic.Int64
	MetadataQueueDepth  atomic.Int64
	
	// Error counts
	ParserErrors    atomic.Int64
	ChunkerErrors   atomic.Int64
	MetadataErrors  atomic.Int64
	EmbeddingErrors atomic.Int64
	
	// AI Service metrics
	AIServiceLatency  atomic.Int64
	AIServiceErrors   atomic.Int64
	AIServiceCalls    atomic.Int64
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