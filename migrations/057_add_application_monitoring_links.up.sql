ALTER TABLE applications
    ADD COLUMN monitoring_links JSONB NOT NULL DEFAULT '[]'::jsonb;
