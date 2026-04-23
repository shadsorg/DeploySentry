package orgstate

import (
	"context"
	"fmt"
	"time"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/deploysentry/deploysentry/internal/platform/database/postgres"
	"github.com/google/uuid"
)

// DeploymentsService assembles /orgs/:slug/deployments responses.
type DeploymentsService struct {
	entities EntityLookup
	batch    OrgStatusBatcher
}

func NewDeploymentsService(e EntityLookup, batch OrgStatusBatcher) *DeploymentsService {
	return &DeploymentsService{entities: e, batch: batch}
}

// ListParams is the external shape of the query-string filters.
type ListParams struct {
	ProjectID     *uuid.UUID
	ApplicationID *uuid.UUID
	EnvironmentID *uuid.UUID
	Status        *models.DeployStatus
	Mode          *models.DeployMode
	From          *time.Time
	To            *time.Time
	Cursor        string
	Limit         int
}

// List returns one page of org-wide deploy history for the caller.
//
// Visibility: Phase 1 defers fine-grained grant filtering. For non-owner
// callers this reads deployments only for projects they can list (resolved
// via the entity service); owners see everything.
func (s *DeploymentsService) List(ctx context.Context, orgSlug string, userID uuid.UUID, orgRole string, p ListParams) (*models.OrgDeploymentsResponse, error) {
	org, err := s.entities.GetOrgBySlug(ctx, orgSlug)
	if err != nil {
		return nil, fmt.Errorf("get org: %w", err)
	}
	if org == nil {
		return nil, ErrOrgNotFound
	}

	visibleProjects, err := s.entities.ListProjectsByOrg(ctx, org.ID, false, userID, orgRole)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	visible := make(map[uuid.UUID]bool, len(visibleProjects))
	for _, proj := range visibleProjects {
		visible[proj.ID] = true
	}

	page, err := s.batch.ListDeploymentsByOrg(ctx, org.ID, postgres.DeploymentsByOrgFilters{
		ProjectID:     p.ProjectID,
		ApplicationID: p.ApplicationID,
		EnvironmentID: p.EnvironmentID,
		Status:        p.Status,
		Mode:          p.Mode,
		From:          p.From,
		To:            p.To,
	}, p.Cursor, p.Limit)
	if err != nil {
		return nil, fmt.Errorf("list deployments: %w", err)
	}

	resp := &models.OrgDeploymentsResponse{
		Deployments: make([]models.OrgDeploymentRow, 0, len(page.Rows)),
		NextCursor:  page.NextCursor,
	}
	for _, row := range page.Rows {
		if !visible[row.ProjectID] {
			continue
		}
		resp.Deployments = append(resp.Deployments, models.OrgDeploymentRow{
			Deployment: row.Deployment,
			Project: models.ProjectSummary{
				ID:   row.ProjectID,
				Slug: row.ProjectSlug,
				Name: row.ProjectName,
			},
			Application: models.ApplicationSummary{
				ID:   row.Deployment.ApplicationID,
				Slug: row.ApplicationSlug,
				Name: row.ApplicationName,
			},
			Environment: models.EnvironmentSummary{
				ID:   row.Deployment.EnvironmentID,
				Slug: row.EnvironmentSlug,
				Name: row.EnvironmentName,
			},
		})
	}
	return resp, nil
}
