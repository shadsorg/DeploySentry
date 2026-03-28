-- Secure the deploy schema for shared database environments
-- These statements are wrapped in a DO block so they can gracefully skip
-- when the expected roles do not exist (e.g. single-user self-hosted setups).

DO $$
BEGIN
  -- Ensure proper schema ownership and permissions
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'deploysentry_owner') THEN
    ALTER SCHEMA deploy OWNER TO deploysentry_owner;
  END IF;

  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'deploysentry_app') THEN
    -- Grant schema usage to application user
    GRANT USAGE ON SCHEMA deploy TO deploysentry_app;
    GRANT CREATE ON SCHEMA deploy TO deploysentry_app;

    -- Grant table permissions for all existing and future tables
    GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA deploy TO deploysentry_app;
    GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA deploy TO deploysentry_app;

    -- Set default privileges for future objects
    ALTER DEFAULT PRIVILEGES IN SCHEMA deploy
        GRANT ALL PRIVILEGES ON TABLES TO deploysentry_app;
    ALTER DEFAULT PRIVILEGES IN SCHEMA deploy
        GRANT ALL PRIVILEGES ON SEQUENCES TO deploysentry_app;

    -- Prevent unauthorized access to other schemas
    REVOKE ALL ON SCHEMA public FROM deploysentry_app;

    -- Revoke CREATE on the current database
    EXECUTE format('REVOKE CREATE ON DATABASE %I FROM deploysentry_app', current_database());
  END IF;
END
$$;
