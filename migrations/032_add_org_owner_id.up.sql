-- Add missing owner_id column to organizations table
ALTER TABLE organizations
    ADD COLUMN IF NOT EXISTS owner_id UUID REFERENCES users(id);
