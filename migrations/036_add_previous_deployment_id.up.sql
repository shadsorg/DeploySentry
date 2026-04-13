ALTER TABLE deployments
  ADD COLUMN previous_deployment_id UUID REFERENCES deployments(id);

CREATE INDEX idx_deployments_previous ON deployments(previous_deployment_id)
  WHERE previous_deployment_id IS NOT NULL;
