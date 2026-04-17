CREATE TABLE agents (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    app_id          UUID NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    environment_id  UUID NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    status          TEXT NOT NULL DEFAULT 'connected' CHECK (status IN ('connected', 'stale', 'disconnected')),
    version         TEXT NOT NULL DEFAULT '',
    upstream_config JSONB NOT NULL DEFAULT '{}',
    last_seen_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    registered_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_agents_app_id ON agents(app_id);
CREATE INDEX idx_agents_status ON agents(status);

CREATE TABLE agent_heartbeats (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id        UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    deployment_id   UUID REFERENCES deployments(id) ON DELETE SET NULL,
    payload         JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_agent_heartbeats_agent_id ON agent_heartbeats(agent_id);
CREATE INDEX idx_agent_heartbeats_agent_deployment ON agent_heartbeats(agent_id, deployment_id);
