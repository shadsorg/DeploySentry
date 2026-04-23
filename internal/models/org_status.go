package models

import (
	"time"

	"github.com/google/uuid"
)

// OrgStatusResponse is the fan-in returned by GET /orgs/:slug/status. It is
// grouped by project → application → environment so the dashboard can render
// a compact heatmap without further processing.
type OrgStatusResponse struct {
	Org         OrgSummary             `json:"org"`
	GeneratedAt time.Time              `json:"generated_at"`
	Projects    []OrgStatusProjectNode `json:"projects"`
}

// OrgSummary trims the Organization to the identity fields the UI needs.
type OrgSummary struct {
	ID   uuid.UUID `json:"id"`
	Slug string    `json:"slug"`
	Name string    `json:"name"`
}

// OrgStatusProjectNode wraps a project and its applications, with an
// aggregated health roll-up computed from the child cells.
type OrgStatusProjectNode struct {
	Project          ProjectSummary            `json:"project"`
	AggregateHealth  HealthState               `json:"aggregate_health"`
	Applications    []OrgStatusApplicationNode `json:"applications"`
}

// ProjectSummary mirrors OrgSummary for projects.
type ProjectSummary struct {
	ID   uuid.UUID `json:"id"`
	Slug string    `json:"slug"`
	Name string    `json:"name"`
}

// OrgStatusApplicationNode carries one row of the status table.
type OrgStatusApplicationNode struct {
	Application ApplicationSummary        `json:"application"`
	Envs        []OrgStatusEnvironmentCell `json:"environments"`
}

// ApplicationSummary is the slim view of an Application embedded in org-wide
// responses. Keeps monitoring_links alongside so the UI can render icons.
type ApplicationSummary struct {
	ID              uuid.UUID        `json:"id"`
	Slug            string           `json:"slug"`
	Name            string           `json:"name"`
	MonitoringLinks []MonitoringLink `json:"monitoring_links"`
}

// OrgStatusEnvironmentCell is one chip in the heatmap.
type OrgStatusEnvironmentCell struct {
	Environment       EnvironmentSummary       `json:"environment"`
	CurrentDeployment *OrgStatusDeploymentMini `json:"current_deployment,omitempty"`
	Health            HealthBlock              `json:"health"`
	NeverDeployed     bool                     `json:"never_deployed"`
}

// OrgStatusDeploymentMini is the subset of Deployment the status page needs.
type OrgStatusDeploymentMini struct {
	ID          uuid.UUID    `json:"id"`
	Version     string       `json:"version"`
	CommitSHA   string       `json:"commit_sha,omitempty"`
	Status      DeployStatus `json:"status"`
	Mode        DeployMode   `json:"mode"`
	Source      *string      `json:"source,omitempty"`
	CompletedAt *time.Time   `json:"completed_at,omitempty"`
}
