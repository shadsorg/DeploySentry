-- Revert schema security changes

-- Remove default privileges
ALTER DEFAULT PRIVILEGES IN SCHEMA deploy
    REVOKE ALL PRIVILEGES ON TABLES FROM deploysentry_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA deploy
    REVOKE ALL PRIVILEGES ON SEQUENCES FROM deploysentry_app;

-- Remove explicit permissions
REVOKE ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA deploy FROM deploysentry_app;
REVOKE ALL PRIVILEGES ON ALL TABLES IN SCHEMA deploy FROM deploysentry_app;
REVOKE CREATE ON SCHEMA deploy FROM deploysentry_app;
REVOKE USAGE ON SCHEMA deploy FROM deploysentry_app;