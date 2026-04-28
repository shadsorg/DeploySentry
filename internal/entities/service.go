package entities

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/google/uuid"
)

const (
	recentActivityWindow    = 14 * 24 * time.Hour
	hardDeleteEligibleAfter = 7 * 24 * time.Hour
)

// EntityService defines the interface for entity management.
type EntityService interface {
	CreateOrg(ctx context.Context, org *models.Organization, creatorID uuid.UUID) error
	GetOrgBySlug(ctx context.Context, slug string) (*models.Organization, error)
	GetOrgByID(ctx context.Context, id uuid.UUID) (*models.Organization, error)
	ListOrgsByUser(ctx context.Context, userID uuid.UUID) ([]*models.Organization, error)
	UpdateOrg(ctx context.Context, org *models.Organization) error

	CreateProject(ctx context.Context, project *models.Project) error
	GetProjectBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*models.Project, error)
	ListProjectsByOrg(ctx context.Context, orgID uuid.UUID, includeDeleted bool, userID uuid.UUID, orgRole string) ([]*models.Project, error)
	UpdateProject(ctx context.Context, project *models.Project) error
	DeleteProject(ctx context.Context, orgID uuid.UUID, slug string) (*models.DeleteResult, error)
	HardDeleteProject(ctx context.Context, orgID uuid.UUID, slug string) error
	RestoreProject(ctx context.Context, orgID uuid.UUID, slug string) (*models.Project, error)

	CreateApp(ctx context.Context, app *models.Application) error
	GetAppBySlug(ctx context.Context, projectID uuid.UUID, slug string) (*models.Application, error)
	ListAppsByProject(ctx context.Context, projectID uuid.UUID, includeDeleted bool, userID uuid.UUID, orgRole string) ([]*models.Application, error)
	UpdateApp(ctx context.Context, app *models.Application) error
	UpdateAppMonitoringLinks(ctx context.Context, appID uuid.UUID, links []models.MonitoringLink) ([]models.MonitoringLink, error)
	DeleteApp(ctx context.Context, projectID uuid.UUID, slug string) (*models.DeleteResult, error)
	HardDeleteApp(ctx context.Context, projectID uuid.UUID, slug string) error
	RestoreApp(ctx context.Context, projectID uuid.UUID, slug string) (*models.Application, error)

	ListEnvironmentsByApp(ctx context.Context, appID uuid.UUID) ([]*models.Environment, error)

	ListEnvironments(ctx context.Context, orgID uuid.UUID) ([]OrgEnvironment, error)
	CreateEnvironment(ctx context.Context, env *OrgEnvironment) error
	GetEnvironmentBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*OrgEnvironment, error)
	UpdateEnvironment(ctx context.Context, env *OrgEnvironment) error
	DeleteEnvironment(ctx context.Context, id uuid.UUID) error
}

// FlagActivityChecker checks whether an entity has active flag usage.
type FlagActivityChecker interface {
	HasActiveFlags(ctx context.Context, projectID uuid.UUID) (bool, error)
}

type entityService struct {
	repo         EntityRepository
	envRepo      *EnvironmentRepository
	flagActivity FlagActivityChecker
}

func NewEntityService(repo EntityRepository, envRepo *EnvironmentRepository, flagActivity FlagActivityChecker) EntityService {
	return &entityService{repo: repo, envRepo: envRepo, flagActivity: flagActivity}
}

func (s *entityService) CreateOrg(ctx context.Context, org *models.Organization, creatorID uuid.UUID) error {
	org.ID = uuid.New()
	org.OwnerID = creatorID
	if org.Plan == "" {
		org.Plan = "free"
	}
	now := time.Now().UTC()
	org.CreatedAt = now
	org.UpdatedAt = now
	if err := org.Validate(); err != nil {
		return err
	}
	if err := s.repo.CreateOrg(ctx, org); err != nil {
		return err
	}
	return s.repo.AddOrgMember(ctx, org.ID, creatorID, "owner")
}

func (s *entityService) GetOrgBySlug(ctx context.Context, slug string) (*models.Organization, error) {
	if slug == "" {
		return nil, errors.New("slug is required")
	}
	return s.repo.GetOrgBySlug(ctx, slug)
}

func (s *entityService) GetOrgByID(ctx context.Context, id uuid.UUID) (*models.Organization, error) {
	if id == uuid.Nil {
		return nil, errors.New("id is required")
	}
	return s.repo.GetOrgByID(ctx, id)
}

func (s *entityService) ListOrgsByUser(ctx context.Context, userID uuid.UUID) ([]*models.Organization, error) {
	return s.repo.ListOrgsByUser(ctx, userID)
}

func (s *entityService) UpdateOrg(ctx context.Context, org *models.Organization) error {
	org.UpdatedAt = time.Now().UTC()
	return s.repo.UpdateOrg(ctx, org)
}

func (s *entityService) CreateProject(ctx context.Context, project *models.Project) error {
	project.ID = uuid.New()
	now := time.Now().UTC()
	project.CreatedAt = now
	project.UpdatedAt = now
	if err := project.Validate(); err != nil {
		return err
	}
	return s.repo.CreateProject(ctx, project)
}

func (s *entityService) GetProjectBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*models.Project, error) {
	return s.repo.GetProjectBySlug(ctx, orgID, slug)
}

func (s *entityService) ListProjectsByOrg(ctx context.Context, orgID uuid.UUID, includeDeleted bool, userID uuid.UUID, orgRole string) ([]*models.Project, error) {
	return s.repo.ListProjectsByOrg(ctx, orgID, includeDeleted, userID, orgRole)
}

func (s *entityService) UpdateProject(ctx context.Context, project *models.Project) error {
	project.UpdatedAt = time.Now().UTC()
	return s.repo.UpdateProject(ctx, project)
}

func (s *entityService) DeleteProject(ctx context.Context, orgID uuid.UUID, slug string) (*models.DeleteResult, error) {
	project, err := s.repo.GetProjectBySlug(ctx, orgID, slug)
	if err != nil {
		return nil, err
	}
	if project == nil {
		return nil, errors.New("project not found")
	}
	if project.DeletedAt != nil {
		return nil, errors.New("project is already deleted")
	}

	count, err := s.repo.CountFlagsByProject(ctx, project.ID)
	if err != nil {
		return nil, err
	}
	if count == 0 {
		if err := s.repo.HardDeleteProject(ctx, project.ID); err != nil {
			return nil, err
		}
		return &models.DeleteResult{Deleted: "permanent"}, nil
	}

	activeFlags, err := s.repo.HasRecentFlagActivity(ctx, project.ID, nil, time.Now().UTC().Add(-recentActivityWindow))
	if err != nil {
		return nil, err
	}
	if len(activeFlags) > 0 {
		return &models.DeleteResult{ActiveFlags: activeFlags}, nil
	}

	if err := s.repo.SoftDeleteProject(ctx, project.ID); err != nil {
		return nil, err
	}
	eligible := time.Now().UTC().Add(hardDeleteEligibleAfter)
	return &models.DeleteResult{Deleted: "soft", EligibleForHardDelete: &eligible}, nil
}

func (s *entityService) HardDeleteProject(ctx context.Context, orgID uuid.UUID, slug string) error {
	project, err := s.repo.GetProjectBySlug(ctx, orgID, slug)
	if err != nil {
		return err
	}
	if project == nil {
		return errors.New("project not found")
	}
	if project.DeletedAt == nil {
		return errors.New("project must be soft-deleted before hard delete")
	}
	eligible := project.DeletedAt.Add(hardDeleteEligibleAfter)
	if time.Now().UTC().Before(eligible) {
		return fmt.Errorf("project cannot be hard-deleted until %s", eligible.Format(time.RFC3339))
	}
	return s.repo.HardDeleteProject(ctx, project.ID)
}

func (s *entityService) RestoreProject(ctx context.Context, orgID uuid.UUID, slug string) (*models.Project, error) {
	project, err := s.repo.GetProjectBySlug(ctx, orgID, slug)
	if err != nil {
		return nil, err
	}
	if project == nil {
		return nil, errors.New("project not found")
	}
	if project.DeletedAt == nil {
		return nil, errors.New("project is not deleted")
	}
	if err := s.repo.RestoreProject(ctx, project.ID); err != nil {
		return nil, err
	}
	project.DeletedAt = nil
	return project, nil
}

func (s *entityService) CreateApp(ctx context.Context, app *models.Application) error {
	app.ID = uuid.New()
	now := time.Now().UTC()
	app.CreatedAt = now
	app.UpdatedAt = now
	if err := app.Validate(); err != nil {
		return err
	}
	return s.repo.CreateApp(ctx, app)
}

func (s *entityService) GetAppBySlug(ctx context.Context, projectID uuid.UUID, slug string) (*models.Application, error) {
	return s.repo.GetAppBySlug(ctx, projectID, slug)
}

func (s *entityService) ListAppsByProject(ctx context.Context, projectID uuid.UUID, includeDeleted bool, userID uuid.UUID, orgRole string) ([]*models.Application, error) {
	return s.repo.ListAppsByProject(ctx, projectID, includeDeleted, userID, orgRole)
}

func (s *entityService) UpdateApp(ctx context.Context, app *models.Application) error {
	app.UpdatedAt = time.Now().UTC()
	return s.repo.UpdateApp(ctx, app)
}

// UpdateAppMonitoringLinks validates + persists the monitoring_links set
// for an application. Returns the normalized slice that was stored.
func (s *entityService) UpdateAppMonitoringLinks(ctx context.Context, appID uuid.UUID, links []models.MonitoringLink) ([]models.MonitoringLink, error) {
	cleaned, err := models.ValidateMonitoringLinks(links)
	if err != nil {
		return nil, err
	}
	if err := s.repo.UpdateAppMonitoringLinks(ctx, appID, cleaned); err != nil {
		return nil, err
	}
	return cleaned, nil
}

func (s *entityService) DeleteApp(ctx context.Context, projectID uuid.UUID, slug string) (*models.DeleteResult, error) {
	app, err := s.repo.GetAppBySlug(ctx, projectID, slug)
	if err != nil {
		return nil, err
	}
	if app == nil {
		return nil, errors.New("application not found")
	}
	if app.DeletedAt != nil {
		return nil, errors.New("application is already deleted")
	}

	count, err := s.repo.CountFlagsByApp(ctx, app.ID)
	if err != nil {
		return nil, err
	}
	if count == 0 {
		if err := s.repo.HardDeleteApp(ctx, app.ID); err != nil {
			return nil, err
		}
		return &models.DeleteResult{Deleted: "permanent"}, nil
	}

	activeFlags, err := s.repo.HasRecentFlagActivity(ctx, projectID, &app.ID, time.Now().UTC().Add(-recentActivityWindow))
	if err != nil {
		return nil, err
	}
	if len(activeFlags) > 0 {
		return &models.DeleteResult{ActiveFlags: activeFlags}, nil
	}

	if err := s.repo.SoftDeleteApp(ctx, app.ID); err != nil {
		return nil, err
	}
	eligible := time.Now().UTC().Add(hardDeleteEligibleAfter)
	return &models.DeleteResult{Deleted: "soft", EligibleForHardDelete: &eligible}, nil
}

func (s *entityService) HardDeleteApp(ctx context.Context, projectID uuid.UUID, slug string) error {
	app, err := s.repo.GetAppBySlug(ctx, projectID, slug)
	if err != nil {
		return err
	}
	if app == nil {
		return errors.New("application not found")
	}
	if app.DeletedAt == nil {
		return errors.New("application must be soft-deleted before hard delete")
	}
	eligible := app.DeletedAt.Add(hardDeleteEligibleAfter)
	if time.Now().UTC().Before(eligible) {
		return fmt.Errorf("application cannot be hard-deleted until %s", eligible.Format(time.RFC3339))
	}
	return s.repo.HardDeleteApp(ctx, app.ID)
}

func (s *entityService) RestoreApp(ctx context.Context, projectID uuid.UUID, slug string) (*models.Application, error) {
	app, err := s.repo.GetAppBySlug(ctx, projectID, slug)
	if err != nil {
		return nil, err
	}
	if app == nil {
		return nil, errors.New("application not found")
	}
	if app.DeletedAt == nil {
		return nil, errors.New("application is not deleted")
	}
	if err := s.repo.RestoreApp(ctx, app.ID); err != nil {
		return nil, err
	}
	app.DeletedAt = nil
	return app, nil
}

func (s *entityService) ListEnvironmentsByApp(ctx context.Context, appID uuid.UUID) ([]*models.Environment, error) {
	return s.repo.ListEnvironmentsByApp(ctx, appID)
}

func (s *entityService) ListEnvironments(ctx context.Context, orgID uuid.UUID) ([]OrgEnvironment, error) {
	return s.envRepo.ListByOrg(ctx, orgID)
}

func (s *entityService) CreateEnvironment(ctx context.Context, env *OrgEnvironment) error {
	return s.envRepo.Create(ctx, env)
}

func (s *entityService) GetEnvironmentBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*OrgEnvironment, error) {
	return s.envRepo.GetBySlug(ctx, orgID, slug)
}

func (s *entityService) UpdateEnvironment(ctx context.Context, env *OrgEnvironment) error {
	return s.envRepo.Update(ctx, env)
}

func (s *entityService) DeleteEnvironment(ctx context.Context, id uuid.UUID) error {
	return s.envRepo.Delete(ctx, id)
}
