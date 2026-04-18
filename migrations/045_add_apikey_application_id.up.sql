ALTER TABLE api_keys ADD COLUMN application_id UUID REFERENCES applications(id) ON DELETE SET NULL;
CREATE INDEX idx_api_keys_application_id ON api_keys(application_id);
