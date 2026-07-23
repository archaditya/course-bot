DROP INDEX IF EXISTS idx_documents_course_created_at;
ALTER TABLE documents DROP COLUMN IF EXISTS updated_at;
ALTER TABLE documents DROP COLUMN IF EXISTS normalization_version;
ALTER TABLE documents DROP COLUMN IF EXISTS normalized_ref;
ALTER TABLE documents DROP COLUMN IF EXISTS source_url;
ALTER TABLE users DROP COLUMN IF EXISTS full_name;
