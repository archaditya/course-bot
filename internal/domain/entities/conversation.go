package entities

import "time"

// MessageRole is "user" or "assistant".
type MessageRole string

const (
	MessageRoleUser      MessageRole = "user"
	MessageRoleAssistant MessageRole = "assistant"
)

// MessageStatus mirrors the Conversation/Message Lifecycle in
// docs/03-domain-model.md#conversation--message-lifecycle.
type MessageStatus string

const (
	MessageStatusDrafted       MessageStatus = "DRAFTED"
	MessageStatusSent          MessageStatus = "SENT"
	MessageStatusStreaming     MessageStatus = "STREAMING"
	MessageStatusCompleted     MessageStatus = "COMPLETED"
	MessageStatusLowConfidence MessageStatus = "LOW_CONFIDENCE"
)

// Conversation is a chat thread, scoped directly to a Workspace. Every
// conversation can be grounded in every Document the workspace owns.
type Conversation struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspace_id"`
	Title       string    `json:"title"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Message is one turn inside a Conversation.
type Message struct {
	ID             string        `json:"id"`
	ConversationID string        `json:"conversation_id"`
	Role           MessageRole   `json:"role"`
	Content        string        `json:"content"`
	Status         MessageStatus `json:"status"`
	CreatedAt      time.Time     `json:"created_at"`
	Citations      []*Citation   `json:"citations,omitempty"`
}

// Citation links a Message back to the Chunk(s) it was grounded in.
type Citation struct {
	ID             string `json:"id"`
	MessageID      string `json:"message_id"`
	ChunkID        string `json:"chunk_id"`
	StartTimestamp *int   `json:"start_timestamp,omitempty"`
	PageNumber     *int   `json:"page_number,omitempty"`
	DocumentID     string `json:"document_id,omitempty"`
	Title          string `json:"title,omitempty"`
}
