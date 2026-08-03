CREATE TABLE IF NOT EXISTS learning_nodes (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    tool_type       TEXT NOT NULL,
    title           TEXT NOT NULL DEFAULT '',
    content         JSONB NOT NULL DEFAULT '{}',
    status          TEXT NOT NULL DEFAULT 'generating',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_learning_nodes_conversation ON learning_nodes(conversation_id);
