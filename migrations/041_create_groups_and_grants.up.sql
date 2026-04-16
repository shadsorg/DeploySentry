-- 1. Create groups table
CREATE TABLE groups (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id      UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    slug        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_by  UUID REFERENCES users(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(org_id, slug)
);

CREATE INDEX idx_groups_org_id ON groups(org_id);

-- 2. Create group_members table
CREATE TABLE group_members (
    group_id   UUID NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (group_id, user_id)
);

CREATE INDEX idx_group_members_user_id ON group_members(user_id);

-- 3. Create resource_grants table
CREATE TABLE resource_grants (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id         UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    project_id     UUID REFERENCES projects(id) ON DELETE CASCADE,
    application_id UUID REFERENCES applications(id) ON DELETE CASCADE,
    user_id        UUID REFERENCES users(id) ON DELETE CASCADE,
    group_id       UUID REFERENCES groups(id) ON DELETE CASCADE,
    permission     TEXT NOT NULL CHECK (permission IN ('read', 'write')),
    granted_by     UUID REFERENCES users(id),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- Exactly one of project_id or application_id must be set
    CONSTRAINT chk_resource_target CHECK (
        (project_id IS NOT NULL AND application_id IS NULL) OR
        (project_id IS NULL AND application_id IS NOT NULL)
    ),

    -- Exactly one of user_id or group_id must be set
    CONSTRAINT chk_grantee CHECK (
        (user_id IS NOT NULL AND group_id IS NULL) OR
        (user_id IS NULL AND group_id IS NOT NULL)
    )
);

-- Unique partial indexes for all 4 combinations
CREATE UNIQUE INDEX idx_resource_grants_project_user
    ON resource_grants(project_id, user_id)
    WHERE project_id IS NOT NULL AND user_id IS NOT NULL;

CREATE UNIQUE INDEX idx_resource_grants_project_group
    ON resource_grants(project_id, group_id)
    WHERE project_id IS NOT NULL AND group_id IS NOT NULL;

CREATE UNIQUE INDEX idx_resource_grants_app_user
    ON resource_grants(application_id, user_id)
    WHERE application_id IS NOT NULL AND user_id IS NOT NULL;

CREATE UNIQUE INDEX idx_resource_grants_app_group
    ON resource_grants(application_id, group_id)
    WHERE application_id IS NOT NULL AND group_id IS NOT NULL;

-- Partial indexes for lookups
CREATE INDEX idx_resource_grants_project_id
    ON resource_grants(project_id)
    WHERE project_id IS NOT NULL;

CREATE INDEX idx_resource_grants_application_id
    ON resource_grants(application_id)
    WHERE application_id IS NOT NULL;

CREATE INDEX idx_resource_grants_user_id
    ON resource_grants(user_id)
    WHERE user_id IS NOT NULL;

CREATE INDEX idx_resource_grants_group_id
    ON resource_grants(group_id)
    WHERE group_id IS NOT NULL;

-- 4. Migrate existing project_members data into resource_grants
INSERT INTO resource_grants (org_id, project_id, user_id, permission, created_at)
SELECT p.org_id, pm.project_id, pm.user_id,
    CASE WHEN pm.role IN ('admin', 'developer') THEN 'write' ELSE 'read' END,
    pm.created_at
FROM project_members pm
JOIN projects p ON p.id = pm.project_id;

-- 5. Drop project_members table
DROP TABLE project_members;
