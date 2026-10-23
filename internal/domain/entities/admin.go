package entities

import "time"

// UserUsageStat represents per-user usage telemetry for admin dashboards.
type UserUsageStat struct {
	ID                string    `json:"id"`
	Email             string    `json:"email"`
	FullName          string    `json:"full_name"`
	Role              UserRole  `json:"role"`
	IsDisabled        bool      `json:"is_disabled"`
	CreatedAt         time.Time `json:"created_at"`
	ConversationCount int       `json:"conversation_count"`
	DocumentCount     int       `json:"document_count"`
	MessageCount      int       `json:"message_count"`
	ChunkCount        int       `json:"chunk_count"`
}

// AISystemTelemetry represents live GenAI and RAG pipeline metrics for the admin UI.
type AISystemTelemetry struct {
	TotalChatRequests      int64   `json:"total_chat_requests"`
	FastRouteHits          int64   `json:"fast_route_hits"`
	KnowledgeRouteHits     int64   `json:"knowledge_route_hits"`
	AvgGuardrailLatencyMs  float64 `json:"avg_guardrail_latency_ms"`
	AvgEnhanceLatencyMs    float64 `json:"avg_enhance_latency_ms"`
	AvgMemoryLatencyMs     float64 `json:"avg_memory_latency_ms"`
	AvgRetrievalLatencyMs  float64 `json:"avg_retrieval_latency_ms"`
	AvgGenerationLatencyMs float64 `json:"avg_generation_latency_ms"`
	AvgTotalLatencyMs      float64 `json:"avg_total_latency_ms"`
	AIServiceErrors        int64   `json:"ai_service_errors"`
	AIServiceHealthy       bool    `json:"ai_service_healthy"`
}

// SystemStats represents aggregate platform telemetry for admins.
type SystemStats struct {
	TotalUsers         int                `json:"total_users"`
	ActiveUsers        int                `json:"active_users"`
	RestrictedUsers    int                `json:"restricted_users"`
	TotalConversations int                `json:"total_conversations"`
	TotalDocuments     int                `json:"total_documents"`
	TotalMessages      int                `json:"total_messages"`
	TotalChunks        int                `json:"total_chunks"`
	AITelemetry        *AISystemTelemetry `json:"ai_telemetry,omitempty"`
}

