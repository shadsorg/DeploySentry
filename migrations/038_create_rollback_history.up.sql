CREATE TABLE rollback_history (
  id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  deployment_id        UUID NOT NULL REFERENCES deployments(id),
  target_deployment_id UUID REFERENCES deployments(id),
  reason               TEXT NOT NULL DEFAULT '',
  health_score         DOUBLE PRECISION,
  automatic            BOOLEAN NOT NULL DEFAULT false,
  strategy             TEXT NOT NULL,
  started_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
  completed_at         TIMESTAMPTZ,
  created_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_rollback_history_deployment ON rollback_history(deployment_id);
