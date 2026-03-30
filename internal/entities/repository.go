package entities

import (
	"context"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
)

// EntityRepository defines persistence for orgs, projects, and applications.
type EntityRepository interface {
	CreateOrg(ctx context.Context, org *models.Organization) error
	GetOrgBySlug(ctx context.Context, slug string) (*models.Organization, error)
	ListOrgsByUser(ctx context.Context, userID uuid.UUID) ([]*models.Organization, error)
	UpdateOrg(ctx context.Context, org *models.Organization) error

	CreateProject(ctx context.Context, project *models.Project) error
	GetProjectBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*models.Project, error)
	ListProjectsByOrg(ctx context.Context, orgID uuid.UUID) ([]*models.Project, error)
	UpdateProject(ctx context.Context, project *models.Project) error

	CreateApp(ctx context.Context, app *models.Application) error
	GetAppBySlug(ctx context.Context, projectID uuid.UUID, slug string) (*models.Application, error)
	ListAppsByProject(ctx context.Context, projectID uuid.UUID) ([]*models.Application, error)
	UpdateApp(ctx context.Context, app *models.Application) error

	ListEnvironmentsByApp(ctx context.Context, appID uuid.UUID) ([]*models.Environment, error)

	AddOrgMember(ctx context.Context, orgID, userID uuid.UUID, role string) error
}
