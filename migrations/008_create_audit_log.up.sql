CREATE TABLE audit_log (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id        UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    project_id    UUID        REFERENCES projects(id) ON DELETE SET NULL,
    user_id       UUID        REFERENCES users(id) ON DELETE SET NULL,
    action        TEXT        NOT NULL,
    resource_type TEXT        NOT NULL,
    resource_id   UUID,
    old_value     JSONB,
    new_value     JSONB,
    ip_address    INET,
    user_agent    TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_audit_log_created_at ON audit_log (created_at);
CREATE INDEX idx_audit_log_org_created_at ON audit_log (org_id, created_at);
