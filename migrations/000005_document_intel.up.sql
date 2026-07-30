-- Adds AI-generated source intelligence columns to documents.
-- Populated by the Worker after indexing completes, via the AI Service
-- /generate-source-intel endpoint. These power the frontend's dynamic
-- suggestion questions, Source Guide modal, and welcome overview card.

ALTER TABLE documents ADD COLUMN ai_summary text NOT NULL DEFAULT '';
ALTER TABLE documents ADD COLUMN ai_questions jsonb NOT NULL DEFAULT '[]';
ALTER TABLE documents ADD COLUMN ai_overview text NOT NULL DEFAULT '';
