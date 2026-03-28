-- Create webhooks table
CREATE TABLE webhooks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL,
    project_id UUID,
    name VARCHAR(255) NOT NULL,
    url TEXT NOT NULL,
    secret VARCHAR(255) NOT NULL,
    events TEXT[] NOT NULL DEFAULT '{}', -- Array of event types to listen for
    is_active BOOLEAN NOT NULL DEFAULT true,
    retry_attempts INTEGER NOT NULL DEFAULT 3,
    timeout_seconds INTEGER NOT NULL DEFAULT 10,

    -- Metadata
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by UUID,
    updated_by UUID,

    CONSTRAINT fk_webhooks_org FOREIGN KEY (org_id) REFERENCES organizations(id) ON DELETE CASCADE,
    CONSTRAINT fk_webhooks_project FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
    CONSTRAINT fk_webhooks_created_by FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL,
    CONSTRAINT fk_webhooks_updated_by FOREIGN KEY (updated_by) REFERENCES users(id) ON DELETE SET NULL
);

-- Create webhook_deliveries table to track delivery attempts
CREATE TABLE webhook_deliveries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    webhook_id UUID NOT NULL,
    event_type VARCHAR(100) NOT NULL,
    payload JSONB NOT NULL,

    -- Delivery tracking
    status VARCHAR(20) NOT NULL DEFAULT 'pending', -- pending, sent, failed, cancelled
    http_status INTEGER,
    response_body TEXT,
    error_message TEXT,
    attempt_count INTEGER NOT NULL DEFAULT 0,

    -- Timestamps
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    sent_at TIMESTAMPTZ,
    next_retry_at TIMESTAMPTZ,

    CONSTRAINT fk_webhook_deliveries_webhook FOREIGN KEY (webhook_id) REFERENCES webhooks(id) ON DELETE CASCADE
);

-- Add indexes for performance
CREATE INDEX idx_webhooks_org_id ON webhooks(org_id);
CREATE INDEX idx_webhooks_project_id ON webhooks(project_id);
CREATE INDEX idx_webhooks_events ON webhooks USING GIN (events);
CREATE INDEX idx_webhooks_active ON webhooks(is_active) WHERE is_active = true;

CREATE INDEX idx_webhook_deliveries_webhook_id ON webhook_deliveries(webhook_id);
CREATE INDEX idx_webhook_deliveries_status ON webhook_deliveries(status);
CREATE INDEX idx_webhook_deliveries_next_retry ON webhook_deliveries(next_retry_at) WHERE status = 'failed' AND next_retry_at IS NOT NULL;
CREATE INDEX idx_webhook_deliveries_created_at ON webhook_deliveries(created_at);

-- Add updated_at trigger for webhooks
CREATE TRIGGER trigger_webhooks_updated_at
    BEFORE UPDATE ON webhooks
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- Add comments for documentation
COMMENT ON TABLE webhooks IS 'Webhook endpoints for receiving event notifications';
COMMENT ON COLUMN webhooks.events IS 'Array of event types this webhook listens for (e.g., flag.created, deployment.completed)';
COMMENT ON COLUMN webhooks.secret IS 'Secret used to sign webhook payloads for verification';
COMMENT ON COLUMN webhooks.retry_attempts IS 'Maximum number of retry attempts for failed deliveries';
COMMENT ON COLUMN webhooks.timeout_seconds IS 'HTTP timeout for webhook delivery attempts';

COMMENT ON TABLE webhook_deliveries IS 'Individual webhook delivery attempts and their results';
COMMENT ON COLUMN webhook_deliveries.status IS 'Delivery status: pending, sent, failed, cancelled';
COMMENT ON COLUMN webhook_deliveries.payload IS 'JSON payload sent to the webhook endpoint';
COMMENT ON COLUMN webhook_deliveries.next_retry_at IS 'When to attempt the next retry for failed deliveries';