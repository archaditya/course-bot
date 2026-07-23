-- Bring the persisted model in line with the application model. This is
-- additive so existing installations and queued documents survive.
ALTER TABLE users ADD COLUMN IF NOT EXISTS full_name text;
UPDATE users SET full_name = split_part(email, '@', 1)
WHERE full_name IS NULL OR btrim(full_name) = '';
ALTER TABLE users ALTER COLUMN full_name SET NOT NULL;

ALTER TABLE documents ADD COLUMN IF NOT EXISTS source_url text;
ALTER TABLE documents ADD COLUMN IF NOT EXISTS normalized_ref text;
ALTER TABLE documents ADD COLUMN IF NOT EXISTS normalization_version text;
ALTER TABLE documents ADD COLUMN IF NOT EXISTS updated_at timestamptz NOT NULL DEFAULT now();
CREATE INDEX IF NOT EXISTS idx_documents_course_created_at ON documents(course_id, created_at DESC);
