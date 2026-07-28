-- Reverses 000004_flatten_to_conversation_scoped_model.up.sql. Recreates
-- the Project -> Course -> Lesson hierarchy and backfills one Project +
-- one Course per workspace so existing rows have somewhere to attach to.

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

CREATE TABLE lessons (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    course_id  uuid NOT NULL REFERENCES courses(id) ON DELETE CASCADE,
    module_id  uuid,
    title      text NOT NULL,
    position   int NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_lessons_course_id ON lessons(course_id);

-- One Project + one Course per workspace, to give existing rows a home.
INSERT INTO projects (id, workspace_id, name)
    SELECT gen_random_uuid(), w.id, 'Default Project' FROM workspaces w;
INSERT INTO courses (id, project_id, title, status)
    SELECT gen_random_uuid(), p.id, 'Default Course', 'INDEXED' FROM projects p;

ALTER TABLE conversations ADD COLUMN project_id uuid REFERENCES projects(id) ON DELETE CASCADE;
UPDATE conversations c SET project_id = p.id FROM projects p WHERE p.workspace_id = c.workspace_id;
ALTER TABLE conversations ALTER COLUMN project_id SET NOT NULL;

ALTER TABLE documents ADD COLUMN lesson_id uuid REFERENCES lessons(id) ON DELETE CASCADE;
ALTER TABLE documents ADD COLUMN course_id uuid REFERENCES courses(id) ON DELETE CASCADE;
UPDATE documents d SET course_id = c.id
    FROM courses c JOIN projects p ON c.project_id = p.id
    JOIN conversations conv ON conv.workspace_id = p.workspace_id
    WHERE conv.id = d.conversation_id;
INSERT INTO lessons (id, course_id, title)
    SELECT gen_random_uuid(), d.course_id, d.original_filename FROM documents d;
UPDATE documents d SET lesson_id = l.id FROM lessons l WHERE l.course_id = d.course_id AND l.title = d.original_filename;
ALTER TABLE documents ALTER COLUMN lesson_id SET NOT NULL;
ALTER TABLE documents ALTER COLUMN course_id SET NOT NULL;
ALTER TABLE documents DROP COLUMN status;
ALTER TABLE documents DROP COLUMN conversation_id;
CREATE INDEX idx_documents_lesson_id ON documents(lesson_id);
CREATE INDEX idx_documents_course_id ON documents(course_id);

ALTER TABLE chunks ADD COLUMN course_id uuid REFERENCES courses(id) ON DELETE CASCADE;
UPDATE chunks ch SET course_id = d.course_id FROM documents d WHERE ch.document_id = d.id;
ALTER TABLE chunks ALTER COLUMN course_id SET NOT NULL;
ALTER TABLE chunks DROP COLUMN conversation_id;
CREATE INDEX idx_chunks_course_id ON chunks(course_id);

ALTER TABLE jobs ADD COLUMN course_id uuid REFERENCES courses(id) ON DELETE CASCADE;
UPDATE jobs j SET course_id = d.course_id FROM documents d WHERE j.document_id = d.id;
ALTER TABLE jobs ALTER COLUMN course_id SET NOT NULL;
ALTER TABLE jobs ALTER COLUMN document_id DROP NOT NULL;
ALTER TABLE jobs DROP COLUMN conversation_id;
DROP INDEX IF EXISTS idx_jobs_document_id;
CREATE INDEX idx_jobs_course_id ON jobs(course_id);

-- Only drop workspace_id now that every table that needed to join through
-- it (documents, via conversations) has already been backfilled.
ALTER TABLE conversations DROP COLUMN workspace_id;
