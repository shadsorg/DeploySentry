-- Add missing columns to org_members
ALTER TABLE org_members
    ADD COLUMN IF NOT EXISTS id         UUID DEFAULT gen_random_uuid(),
    ADD COLUMN IF NOT EXISTS invited_by UUID REFERENCES users(id),
    ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

-- Expand role constraint to include 'viewer'
ALTER TABLE org_members DROP CONSTRAINT IF EXISTS org_members_role_check;
ALTER TABLE org_members ADD CONSTRAINT org_members_role_check
    CHECK (role IN ('owner', 'admin', 'member', 'viewer'));

-- Add missing columns to project_members
ALTER TABLE project_members
    ADD COLUMN IF NOT EXISTS id         UUID DEFAULT gen_random_uuid(),
    ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();
