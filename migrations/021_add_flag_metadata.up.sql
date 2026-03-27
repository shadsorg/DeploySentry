-- Add metadata columns to feature_flags for flag classification and lifecycle management.
ALTER TABLE feature_flags
    ADD COLUMN category VARCHAR(20) NOT NULL DEFAULT 'feature'
        CHECK (category IN ('release', 'feature', 'experiment', 'ops', 'permission')),
    ADD COLUMN purpose TEXT NOT NULL DEFAULT '',
    ADD COLUMN owners TEXT[] NOT NULL DEFAULT '{}',
    ADD COLUMN is_permanent BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN expires_at TIMESTAMPTZ;

-- Index for finding flags by category (common filter in dashboards).
CREATE INDEX idx_feature_flags_category ON feature_flags (project_id, category);

-- Index for finding expired or soon-to-expire flags.
CREATE INDEX idx_feature_flags_expires_at ON feature_flags (expires_at)
    WHERE expires_at IS NOT NULL AND archived_at IS NULL;
