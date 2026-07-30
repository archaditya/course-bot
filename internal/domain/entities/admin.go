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

// SystemStats represents aggregate platform telemetry for admins.
type SystemStats struct {
	TotalUsers         int `json:"total_users"`
	ActiveUsers        int `json:"active_users"`
	RestrictedUsers    int `json:"restricted_users"`
	TotalConversations int `json:"total_conversations"`
	TotalDocuments     int `json:"total_documents"`
	TotalMessages      int `json:"total_messages"`
	TotalChunks        int `json:"total_chunks"`
}
