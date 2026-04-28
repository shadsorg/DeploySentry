package entities

import (
	"context"
	"time"

	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/google/uuid"
)

// EntityRepository defines persistence for orgs, projects, and applications.
type EntityRepository interface {
	CreateOrg(ctx context.Context, org *models.Organization) error
	GetOrgBySlug(ctx context.Context, slug string) (*models.Organization, error)
	GetOrgByID(ctx context.Context, id uuid.UUID) (*models.Organization, error)
	ListOrgsByUser(ctx context.Context, userID uuid.UUID) ([]*models.Organization, error)
	UpdateOrg(ctx context.Context, org *models.Organization) error

	CreateProject(ctx context.Context, project *models.Project) error
	GetProjectByID(ctx context.Context, id uuid.UUID) (*models.Project, error)
	GetProjectBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*models.Project, error)
	ListProjectsByOrg(ctx context.Context, orgID uuid.UUID, includeDeleted bool, userID uuid.UUID, orgRole string) ([]*models.Project, error)
	UpdateProject(ctx context.Context, project *models.Project) error
	CountFlagsByProject(ctx context.Context, projectID uuid.UUID) (int, error)
	SoftDeleteProject(ctx context.Context, id uuid.UUID) error
	HardDeleteProject(ctx context.Context, id uuid.UUID) error
	RestoreProject(ctx context.Context, id uuid.UUID) error

	CreateApp(ctx context.Context, app *models.Application) error
	GetAppByID(ctx context.Context, id uuid.UUID) (*models.Application, error)
	GetAppBySlug(ctx context.Context, projectID uuid.UUID, slug string) (*models.Application, error)
	ListAppsByProject(ctx context.Context, projectID uuid.UUID, includeDeleted bool, userID uuid.UUID, orgRole string) ([]*models.Application, error)
	UpdateApp(ctx context.Context, app *models.Application) error
	UpdateAppMonitoringLinks(ctx context.Context, appID uuid.UUID, links []models.MonitoringLink) error
	CountFlagsByApp(ctx context.Context, applicationID uuid.UUID) (int, error)
	SoftDeleteApp(ctx context.Context, id uuid.UUID) error
	HardDeleteApp(ctx context.Context, id uuid.UUID) error
	RestoreApp(ctx context.Context, id uuid.UUID) error

	HasRecentFlagActivity(ctx context.Context, projectID uuid.UUID, applicationID *uuid.UUID, since time.Time) ([]models.FlagActivitySummary, error)

	ListEnvironmentsByApp(ctx context.Context, appID uuid.UUID) ([]*models.Environment, error)

	AddOrgMember(ctx context.Context, orgID, userID uuid.UUID, role string) error
}
