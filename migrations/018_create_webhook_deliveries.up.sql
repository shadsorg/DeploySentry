CREATE TABLE webhook_deliveries (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    endpoint_id     UUID        NOT NULL REFERENCES webhook_endpoints(id) ON DELETE CASCADE,
    event_type      TEXT        NOT NULL,
    payload         JSONB       NOT NULL,
    response_status INT,
    response_body   TEXT,
    delivered_at    TIMESTAMPTZ,
    attempts        INT         NOT NULL DEFAULT 0,
    next_retry_at   TIMESTAMPTZ,
    status          TEXT        CHECK (status IN ('pending', 'delivered', 'failed')),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
