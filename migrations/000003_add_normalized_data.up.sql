ALTER TABLE documents ADD COLUMN IF NOT EXISTS normalized_data JSONB;
ALTER TABLE documents ADD COLUMN IF NOT EXISTS normalization_version VARCHAR(50);
