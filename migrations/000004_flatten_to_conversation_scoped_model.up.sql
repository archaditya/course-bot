-- Simplifies the model to: User -> Conversations -> (own) Documents.
--
-- Removes the Project -> Course -> Lesson -> Document hierarchy. A
-- Conversation now belongs directly to a Workspace (workspace is already
-- 1:1 with a user — internal auth plumbing, not a product concept), and
-- every Document/Chunk/Job belongs directly to the Conversation whose
-- knowledge base it's part of. Each conversation is its own self-contained
-- notebook: its own sources, its own indexing status per source, and
-- retrieval that only ever searches within that conversation's own
-- documents.
--
-- Why: the old hierarchy required a courseID to scope a chat query, but
-- Conversations only ever carried a projectID (never a courseID), and the
-- frontend never had one to send. Retrieval silently fell back to an
-- unfiltered Qdrant search — any conversation could surface citations from
-- *any* other conversation's (or workspace's) documents. Scoping retrieval
-- to conversation_id, with no unfiltered fallback, removes that path
-- entirely rather than papering over one call site.

-- ── conversations: project_id -> workspace_id (unchanged ownership, just
--    one hop shorter — a conversation belonged to a project which belonged
--    to a workspace; now it belongs to the workspace directly) ────────────
ALTER TABLE conversations ADD COLUMN workspace_id uuid REFERENCES workspaces(id) ON DELETE CASCADE;
UPDATE conversations c SET workspace_id = p.workspace_id FROM projects p WHERE c.project_id = p.id;
ALTER TABLE conversations ALTER COLUMN workspace_id SET NOT NULL;
ALTER TABLE conversations DROP CONSTRAINT conversations_project_id_fkey;
ALTER TABLE conversations DROP COLUMN project_id;
CREATE INDEX idx_conversations_workspace_id ON conversations(workspace_id);

-- ── documents: each source now belongs to the Conversation it was added
--    to, not to a lesson/course. Absorbs Course's status/lifecycle, since
--    there's no course-level grouping status anymore — each source tracks
--    its own indexing progress independently. ─────────────────────────────
ALTER TABLE documents ADD COLUMN conversation_id uuid REFERENCES conversations(id) ON DELETE CASCADE;
-- Best-effort backfill for any pre-existing rows: attach to the first
-- conversation in the same workspace as the document's old course/project,
-- since there is no exact prior mapping from course to conversation.
UPDATE documents d SET conversation_id = (
    SELECT conv.id FROM conversations conv
    JOIN courses c ON d.course_id = c.id
    JOIN projects p ON c.project_id = p.id
    WHERE conv.workspace_id = p.workspace_id
    ORDER BY conv.created_at ASC LIMIT 1
);
DELETE FROM documents WHERE conversation_id IS NULL;
ALTER TABLE documents ALTER COLUMN conversation_id SET NOT NULL;

ALTER TABLE documents ADD COLUMN status text NOT NULL DEFAULT 'UPLOADING' CHECK (status IN (
    'UPLOADING', 'UPLOADED', 'PARSING', 'NORMALIZING',
    'CHUNKING', 'EMBEDDING', 'INDEXED', 'FAILED'
));
-- courses.status defaults to 'CREATED' (a pre-upload placeholder used
-- before any file existed for that course). Document has no equivalent
-- state — a Document only ever starts at 'UPLOADING' — so map it across
-- rather than widening the check constraint to include a status that
-- should never occur for a Document going forward.
UPDATE documents d SET status = CASE c.status WHEN 'CREATED' THEN 'UPLOADING' ELSE c.status END
    FROM courses c WHERE d.course_id = c.id;

ALTER TABLE documents DROP CONSTRAINT documents_lesson_id_fkey;
ALTER TABLE documents DROP CONSTRAINT documents_course_id_fkey;
ALTER TABLE documents DROP COLUMN lesson_id;
ALTER TABLE documents DROP COLUMN course_id;
CREATE INDEX idx_documents_conversation_id ON documents(conversation_id);
CREATE INDEX idx_documents_conversation_created_at ON documents(conversation_id, created_at DESC);
DROP INDEX IF EXISTS idx_documents_course_created_at;

-- ── chunks: course_id -> conversation_id (this is the Qdrant filter key —
--    retrieval for a conversation only ever searches its own chunks) ──────
ALTER TABLE chunks ADD COLUMN conversation_id uuid REFERENCES conversations(id) ON DELETE CASCADE;
UPDATE chunks ch SET conversation_id = d.conversation_id FROM documents d WHERE ch.document_id = d.id;
DELETE FROM chunks WHERE conversation_id IS NULL;
ALTER TABLE chunks ALTER COLUMN conversation_id SET NOT NULL;
ALTER TABLE chunks DROP CONSTRAINT chunks_course_id_fkey;
ALTER TABLE chunks DROP COLUMN course_id;
CREATE INDEX idx_chunks_conversation_id ON chunks(conversation_id);

-- ── jobs: course_id -> conversation_id; document_id becomes required ─────
-- (every job now tracks exactly one document — there is no more
-- manifest-level job that fans out across an entire collection)
ALTER TABLE jobs ADD COLUMN conversation_id uuid REFERENCES conversations(id) ON DELETE CASCADE;
UPDATE jobs j SET conversation_id = d.conversation_id FROM documents d WHERE j.document_id = d.id;
-- Stale manifest-level jobs (document_id was NULL) predate per-document
-- jobs entirely and carry no retrievable state worth keeping.
DELETE FROM jobs WHERE conversation_id IS NULL;
ALTER TABLE jobs ALTER COLUMN conversation_id SET NOT NULL;
ALTER TABLE jobs ALTER COLUMN document_id SET NOT NULL;
ALTER TABLE jobs DROP CONSTRAINT jobs_course_id_fkey;
ALTER TABLE jobs DROP COLUMN course_id;
CREATE INDEX idx_jobs_conversation_id ON jobs(conversation_id);
CREATE INDEX idx_jobs_document_id ON jobs(document_id);

-- ── drop the now-unused hierarchy tables ───────────────────────────────────
DROP TABLE IF EXISTS lessons;
DROP TABLE IF EXISTS modules;
DROP TABLE IF EXISTS courses;
DROP TABLE IF EXISTS projects;
