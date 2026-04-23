package models

// OrgDeploymentsResponse is the paged list returned by
// GET /orgs/:slug/deployments.
type OrgDeploymentsResponse struct {
	Deployments []OrgDeploymentRow `json:"deployments"`
	NextCursor  string             `json:"next_cursor,omitempty"`
}

// OrgDeploymentRow carries one deployment plus the joined slug/name data
// the UI uses to render without a second round-trip.
type OrgDeploymentRow struct {
	*Deployment
	Application ApplicationSummary `json:"application"`
	Environment EnvironmentSummary `json:"environment"`
	Project     ProjectSummary     `json:"project"`
}
