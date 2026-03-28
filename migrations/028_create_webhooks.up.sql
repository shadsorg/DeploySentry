-- Create webhooks table (enhanced version of webhook_endpoints)
-- Uses IF NOT EXISTS to be idempotent with earlier migrations (017/018)
CREATE TABLE IF NOT EXISTS webhooks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL,
    project_id UUID,
    name VARCHAR(255) NOT NULL,
    url TEXT NOT NULL,
    secret VARCHAR(255) NOT NULL,
    events TEXT[] NOT NULL DEFAULT '{}',
    is_active BOOLEAN NOT NULL DEFAULT true,
    retry_attempts INTEGER NOT NULL DEFAULT 3,
    timeout_seconds INTEGER NOT NULL DEFAULT 10,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by UUID,
    updated_by UUID,

    CONSTRAINT fk_webhooks_org FOREIGN KEY (org_id) REFERENCES organizations(id) ON DELETE CASCADE,
    CONSTRAINT fk_webhooks_project FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
    CONSTRAINT fk_webhooks_created_by FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL,
    CONSTRAINT fk_webhooks_updated_by FOREIGN KEY (updated_by) REFERENCES users(id) ON DELETE SET NULL
);

-- Enhance existing webhook_deliveries if it exists, or create it
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_tables WHERE tablename = 'webhook_deliveries' AND schemaname = current_schema()) THEN
    CREATE TABLE webhook_deliveries (
        id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
        webhook_id UUID NOT NULL,
        event_type VARCHAR(100) NOT NULL,
        payload JSONB NOT NULL,
        status VARCHAR(20) NOT NULL DEFAULT 'pending',
        http_status INTEGER,
        response_body TEXT,
        error_message TEXT,
        attempt_count INTEGER NOT NULL DEFAULT 0,
        created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
        sent_at TIMESTAMPTZ,
        next_retry_at TIMESTAMPTZ,
        CONSTRAINT fk_webhook_deliveries_webhook FOREIGN KEY (webhook_id) REFERENCES webhooks(id) ON DELETE CASCADE
    );
  ELSE
    -- Add columns that may be missing from the earlier migration (018)
    ALTER TABLE webhook_deliveries ADD COLUMN IF NOT EXISTS webhook_id UUID;
    ALTER TABLE webhook_deliveries ADD COLUMN IF NOT EXISTS http_status INTEGER;
    ALTER TABLE webhook_deliveries ADD COLUMN IF NOT EXISTS error_message TEXT;
    ALTER TABLE webhook_deliveries ADD COLUMN IF NOT EXISTS attempt_count INTEGER NOT NULL DEFAULT 0;
    ALTER TABLE webhook_deliveries ADD COLUMN IF NOT EXISTS sent_at TIMESTAMPTZ;
  END IF;
END
$$;

-- Add indexes for performance (IF NOT EXISTS)
CREATE INDEX IF NOT EXISTS idx_webhooks_org_id ON webhooks(org_id);
CREATE INDEX IF NOT EXISTS idx_webhooks_project_id ON webhooks(project_id);
CREATE INDEX IF NOT EXISTS idx_webhooks_events ON webhooks USING GIN (events);
CREATE INDEX IF NOT EXISTS idx_webhooks_active ON webhooks(is_active) WHERE is_active = true;

CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_webhook_id ON webhook_deliveries(webhook_id);
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_status ON webhook_deliveries(status);
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_next_retry ON webhook_deliveries(next_retry_at) WHERE status = 'failed' AND next_retry_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_created_at ON webhook_deliveries(created_at);

-- Create the shared updated_at trigger function if it doesn't exist
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Add updated_at trigger for webhooks (skip if already exists)
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trigger_webhooks_updated_at') THEN
    CREATE TRIGGER trigger_webhooks_updated_at
        BEFORE UPDATE ON webhooks
        FOR EACH ROW
        EXECUTE FUNCTION update_updated_at_column();
  END IF;
END
$$;

-- Add comments for documentation
COMMENT ON TABLE webhooks IS 'Webhook endpoints for receiving event notifications';
COMMENT ON TABLE webhook_deliveries IS 'Individual webhook delivery attempts and their results';
