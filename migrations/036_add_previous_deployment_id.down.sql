DROP INDEX IF EXISTS idx_deployments_previous;
ALTER TABLE deployments DROP COLUMN IF EXISTS previous_deployment_id;
