-- 1. Recreate project_members with original schema
CREATE TABLE project_members (
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role       TEXT NOT NULL CHECK (role IN ('admin', 'editor', 'viewer', 'deployer')),
    id         UUID DEFAULT gen_random_uuid(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (project_id, user_id)
);

-- 2. Migrate project-level user grants back to project_members
INSERT INTO project_members (project_id, user_id, role, created_at)
SELECT rg.project_id, rg.user_id,
    CASE WHEN rg.permission = 'write' THEN 'admin' ELSE 'viewer' END,
    rg.created_at
FROM resource_grants rg
WHERE rg.project_id IS NOT NULL AND rg.user_id IS NOT NULL;

-- 3. Drop resource_grants, group_members, groups
DROP TABLE IF EXISTS resource_grants;
DROP TABLE IF EXISTS group_members;
DROP TABLE IF EXISTS groups;
