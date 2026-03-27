-- Secure the deploy schema for shared database environments

-- Ensure proper schema ownership and permissions
ALTER SCHEMA deploy OWNER TO deploysentry_owner;

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
REVOKE CREATE ON DATABASE CURRENT_DATABASE() FROM deploysentry_app;