-- Reverse of 062: remove staged-changes-enabled settings seeded per org.
-- Conservative: only deletes rows where org_id is set (not project/app/env).
DELETE FROM settings
WHERE key = 'staged-changes-enabled'
  AND org_id IS NOT NULL;
