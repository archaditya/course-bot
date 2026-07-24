ALTER TABLE documents ADD COLUMN normalized_data JSONB;
ALTER TABLE documents ADD COLUMN normalization_version VARCHAR(50);