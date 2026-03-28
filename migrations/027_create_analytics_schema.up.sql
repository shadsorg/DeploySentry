-- Create analytics schema for developer dashboard and insights

-- Flag evaluation events for usage analytics
CREATE TABLE flag_evaluation_events (
    id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id     UUID        NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    environment_id UUID        NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    flag_key       TEXT        NOT NULL,
    user_id        TEXT,        -- User who triggered evaluation (can be external user)
    sdk_version    TEXT,        -- SDK version that performed evaluation
    result_value   TEXT        NOT NULL,  -- Evaluated result
    rule_id        UUID,        -- Which targeting rule matched (if any)
    latency_ms     INTEGER     NOT NULL,  -- Evaluation latency
    cache_hit      BOOLEAN     NOT NULL DEFAULT false,
    error_message  TEXT,        -- If evaluation failed
    ip_address     INET,        -- Client IP for geographic analytics
    user_agent     TEXT,        -- Client user agent
    context_attrs  JSONB       NOT NULL DEFAULT '{}',  -- Evaluation context
    evaluated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Deployment events for deployment analytics
CREATE TABLE deployment_events (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    deployment_id UUID        NOT NULL REFERENCES deployments(id) ON DELETE CASCADE,
    event_type    TEXT        NOT NULL CHECK (event_type IN (
        'created', 'started', 'phase_completed', 'promoted',
        'paused', 'resumed', 'completed', 'failed', 'rolled_back', 'cancelled'
    )),
    phase_name    TEXT,        -- For phase_completed events
    traffic_pct   INTEGER,     -- Traffic percentage at time of event
    health_score  DECIMAL(3,2), -- Health score (0.00-1.00)
    error_message TEXT,        -- For failed events
    triggered_by  UUID        REFERENCES users(id) ON DELETE SET NULL,
    metadata      JSONB       NOT NULL DEFAULT '{}',
    occurred_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- API request metrics for system health analytics
CREATE TABLE api_request_metrics (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    request_id    TEXT        NOT NULL,   -- X-Request-ID header
    method        TEXT        NOT NULL,
    path          TEXT        NOT NULL,
    status_code   INTEGER     NOT NULL,
    latency_ms    INTEGER     NOT NULL,
    user_id       UUID        REFERENCES users(id) ON DELETE SET NULL,
    api_key_id    UUID        REFERENCES api_keys(id) ON DELETE SET NULL,
    ip_address    INET,
    user_agent    TEXT,
    error_message TEXT,        -- For 4xx/5xx responses
    request_size  BIGINT,      -- Request body size in bytes
    response_size BIGINT,      -- Response body size in bytes
    recorded_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Aggregated metrics for efficient dashboard queries
CREATE TABLE daily_flag_stats (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id       UUID        NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    environment_id   UUID        NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    flag_key         TEXT        NOT NULL,
    stat_date        DATE        NOT NULL,
    evaluation_count BIGINT      NOT NULL DEFAULT 0,
    unique_users     BIGINT      NOT NULL DEFAULT 0,
    cache_hit_rate   DECIMAL(5,2) NOT NULL DEFAULT 0.00,  -- Percentage
    avg_latency_ms   DECIMAL(8,2) NOT NULL DEFAULT 0.00,
    error_count      BIGINT      NOT NULL DEFAULT 0,
    true_results     BIGINT      NOT NULL DEFAULT 0,  -- For boolean flags
    false_results    BIGINT      NOT NULL DEFAULT 0, -- For boolean flags
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(project_id, environment_id, flag_key, stat_date)
);

-- Performance indexes for analytics queries
CREATE INDEX idx_flag_eval_events_project_time ON flag_evaluation_events (project_id, evaluated_at);
CREATE INDEX idx_flag_eval_events_flag_time ON flag_evaluation_events (project_id, flag_key, evaluated_at);
CREATE INDEX idx_flag_eval_events_user ON flag_evaluation_events (project_id, user_id, evaluated_at);

CREATE INDEX idx_deployment_events_deployment_time ON deployment_events (deployment_id, occurred_at);
CREATE INDEX idx_deployment_events_type_time ON deployment_events (event_type, occurred_at);

CREATE INDEX idx_api_metrics_path_time ON api_request_metrics (path, recorded_at);
CREATE INDEX idx_api_metrics_status_time ON api_request_metrics (status_code, recorded_at);
CREATE INDEX idx_api_metrics_user_time ON api_request_metrics (user_id, recorded_at) WHERE user_id IS NOT NULL;

CREATE INDEX idx_daily_flag_stats_project_date ON daily_flag_stats (project_id, stat_date);
CREATE INDEX idx_daily_flag_stats_flag_date ON daily_flag_stats (project_id, flag_key, stat_date);

-- Partitioning for large event tables (optional, for high-volume deployments)
-- This can be enabled later if needed:
-- SELECT create_hypertable('flag_evaluation_events', 'evaluated_at', if_not_exists => TRUE);
-- SELECT create_hypertable('deployment_events', 'occurred_at', if_not_exists => TRUE);
-- SELECT create_hypertable('api_request_metrics', 'recorded_at', if_not_exists => TRUE);