package entities

import (
	"encoding/json"
	"time"
)

// ToolType represents the kind of learning tool generated.
type ToolType string

const (
	ToolTypeSummary      ToolType = "summary"
	ToolTypeKeyTakeaways ToolType = "key_takeaways"
	ToolTypeFlashcards   ToolType = "flashcards"
	ToolTypeQuiz         ToolType = "quiz"
	ToolTypeMindMap      ToolType = "mind_map"
	ToolTypeAIReport     ToolType = "ai_report"
)

// LearningNodeStatus tracks the generation lifecycle.
type LearningNodeStatus string

const (
	LearningNodeStatusGenerating LearningNodeStatus = "generating"
	LearningNodeStatusReady      LearningNodeStatus = "ready"
	LearningNodeStatusFailed     LearningNodeStatus = "failed"
)

// LearningNode is a generated study artifact (flashcards, quiz, summary, etc.)
// tied to a conversation's indexed sources.
type LearningNode struct {
	ID             string             `json:"id"`
	ConversationID string             `json:"conversation_id"`
	ToolType       ToolType           `json:"tool_type"`
	Title          string             `json:"title"`
	Content        json.RawMessage    `json:"content"`
	Status         LearningNodeStatus `json:"status"`
	CreatedAt      time.Time          `json:"created_at"`
	UpdatedAt      time.Time          `json:"updated_at"`
}
