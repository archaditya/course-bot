-- Initial schema. Mirrors docs/03-domain-model.md#database-design exactly,
-- plus refresh_tokens (docs/08-security.md#jwt-rotation) and audit_logs
-- (docs/09-deployment.md#observability), which the ER diagram in that doc
-- doesn't draw but the surrounding text requires.
--
-- Workspace isolation (docs/08-security.md#workspace-isolation): every
-- query-bearing table is scoped by workspace_id, directly or transitively
-- via project_id -> workspace_id. Nothing here supports a query path that
-- skips that scope.

CREATE EXTENSION IF NOT EXISTS pgcrypto; -- for gen_random_uuid()

CREATE TABLE users (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email         text NOT NULL UNIQUE,
    password_hash text,               -- null when auth_provider = 'google'
    auth_provider text NOT NULL CHECK (auth_provider IN ('google', 'password')),
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE refresh_tokens (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash text NOT NULL UNIQUE,
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_refresh_tokens_user_id ON refresh_tokens(user_id);

-- MVP: 1:1 with users. unique(user_id) enforces that until the multi-team
-- phase drops it — see docs/03-domain-model.md#entities (Workspace).
CREATE TABLE workspaces (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    name       text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE projects (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name         text NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_projects_workspace_id ON projects(workspace_id);

CREATE TABLE courses (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    title      text NOT NULL,
    status     text NOT NULL DEFAULT 'CREATED' CHECK (status IN (
        'CREATED', 'UPLOADING', 'UPLOADED', 'PARSING', 'NORMALIZING',
        'CHUNKING', 'EMBEDDING', 'INDEXED', 'FAILED'
    )),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_courses_project_id ON courses(project_id);

-- Reserved for future use per docs/03-domain-model.md — not required for MVP,
-- but modeled now so lessons.module_id doesn't need a later migration.
CREATE TABLE modules (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    course_id  uuid NOT NULL REFERENCES courses(id) ON DELETE CASCADE,
    title      text NOT NULL,
    position   int NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_modules_course_id ON modules(course_id);

CREATE TABLE lessons (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    course_id  uuid NOT NULL REFERENCES courses(id) ON DELETE CASCADE,
    module_id  uuid REFERENCES modules(id) ON DELETE SET NULL,
    title      text NOT NULL,
    position   int NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_lessons_course_id ON lessons(course_id);

CREATE TABLE documents (
    id                    uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    lesson_id             uuid NOT NULL REFERENCES lessons(id) ON DELETE CASCADE,
    course_id             uuid NOT NULL REFERENCES courses(id) ON DELETE CASCADE, -- denormalized, see entities.Document
    source_type           text NOT NULL CHECK (source_type IN ('srt', 'vtt', 'video', 'pdf', 'docx', 'github', 'url', 'text')),
    storage_path          text NOT NULL DEFAULT '',   -- R2 raw/ pointer, immutable
    source_url            text,            -- optional, populated when source_type == 'url' || 'video'
    original_filename     text NOT NULL,
    checksum              text NOT NULL,   -- sha256 hex string
    created_at            timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_documents_lesson_id ON documents(lesson_id);
CREATE INDEX idx_documents_course_id ON documents(course_id);
CREATE INDEX idx_documents_checksum ON documents(checksum);

CREATE TABLE chunks (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    document_id       uuid NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    course_id         uuid NOT NULL REFERENCES courses(id) ON DELETE CASCADE, -- denormalized: Qdrant payload filter key
    start_timestamp   int,
    end_timestamp     int,
    page_number       int,
    title             text NOT NULL DEFAULT '',
    summary           text NOT NULL DEFAULT '',
    content           text NOT NULL,
    token_count       int NOT NULL DEFAULT 0,
    embedding_version text NOT NULL,
    vector_ref        text NOT NULL, -- pointer into Qdrant, not the vector itself
    created_at        timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_chunks_document_id ON chunks(document_id);
CREATE INDEX idx_chunks_course_id ON chunks(course_id);

CREATE TABLE conversations (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    title      text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_conversations_project_id ON conversations(project_id);

CREATE TABLE messages (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id uuid NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    role            text NOT NULL CHECK (role IN ('user', 'assistant')),
    content         text NOT NULL,
    status          text NOT NULL DEFAULT 'DRAFTED' CHECK (status IN (
        'DRAFTED', 'SENT', 'STREAMING', 'COMPLETED', 'LOW_CONFIDENCE'
    )),
    created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_messages_conversation_id ON messages(conversation_id);

CREATE TABLE citations (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    message_id      uuid NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    chunk_id        uuid NOT NULL REFERENCES chunks(id) ON DELETE CASCADE,
    start_timestamp int,
    page_number     int
);
CREATE INDEX idx_citations_message_id ON citations(message_id);
CREATE INDEX idx_citations_chunk_id ON citations(chunk_id);

CREATE TABLE jobs (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    course_id        uuid NOT NULL REFERENCES courses(id) ON DELETE CASCADE,
    document_id      uuid REFERENCES documents(id) ON DELETE CASCADE,
    stage            text NOT NULL CHECK (stage IN (
        'manifest', 'parsing', 'normalizing', 'chunking', 'metadata', 'embedding', 'indexing'
    )),
    status           text NOT NULL DEFAULT 'QUEUED' CHECK (status IN (
        'QUEUED', 'RUNNING', 'SUCCEEDED', 'RETRYING', 'DEAD_LETTERED'
    )),
    attempts         int NOT NULL DEFAULT 0,
    max_attempts     int NOT NULL DEFAULT 3, -- see docs/09-deployment.md#non-functional-requirements
    pipeline_version text NOT NULL,
    last_error       text,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_jobs_course_id ON jobs(course_id);
CREATE INDEX idx_jobs_status ON jobs(status);

-- Append-only; see docs/09-deployment.md#observability. No update/delete
-- path is ever expected against this table.
CREATE TABLE audit_logs (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    action     text NOT NULL,
    resource   text NOT NULL,
    metadata   jsonb NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_audit_logs_user_id ON audit_logs(user_id);
