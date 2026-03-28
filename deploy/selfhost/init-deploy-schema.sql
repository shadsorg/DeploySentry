-- Pre-create the deploy schema so that connections with search_path=deploy succeed
-- before the migration tool runs migration 000.
CREATE SCHEMA IF NOT EXISTS deploy;
