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

// Conversation is a chat thread within a Project.
type Conversation struct {
	ID        string
	ProjectID string
	Title     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Message is one turn inside a Conversation.
type Message struct {
	ID             string
	ConversationID string
	Role           MessageRole
	Content        string
	Status         MessageStatus
	CreatedAt      time.Time
}

// Citation links a Message back to the Chunk(s) it was grounded in.
type Citation struct {
	ID             string
	MessageID      string
	ChunkID        string
	StartTimestamp *int
	PageNumber     *int
}
