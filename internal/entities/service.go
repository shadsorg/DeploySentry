package entities

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
)

// FlagActivityChecker checks for recent flag evaluation activity.
type FlagActivityChecker interface {
	HasRecentFlagActivity(ctx context.Context, projectID uuid.UUID, since time.Time) ([]models.FlagActivitySummary, error)
}

const (
	recentActivityWindow    = 14 * 24 * time.Hour
	hardDeleteEligibleAfter = 7 * 24 * time.Hour
)

// EntityService defines the interface for entity management.
type EntityService interface {
	CreateOrg(ctx context.Context, org *models.Organization, creatorID uuid.UUID) error
	GetOrgBySlug(ctx context.Context, slug string) (*models.Organization, error)
	ListOrgsByUser(ctx context.Context, userID uuid.UUID) ([]*models.Organization, error)
	UpdateOrg(ctx context.Context, org *models.Organization) error

	CreateProject(ctx context.Context, project *models.Project) error
	GetProjectBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*models.Project, error)
	ListProjectsByOrg(ctx context.Context, orgID uuid.UUID, includeDeleted bool) ([]*models.Project, error)
	UpdateProject(ctx context.Context, project *models.Project) error
	SoftDeleteProject(ctx context.Context, orgID uuid.UUID, slug string) ([]models.FlagActivitySummary, error)
	HardDeleteProject(ctx context.Context, orgID uuid.UUID, slug string) (*time.Time, error)
	RestoreProject(ctx context.Context, orgID uuid.UUID, slug string) error

	CreateApp(ctx context.Context, app *models.Application) error
	GetAppBySlug(ctx context.Context, projectID uuid.UUID, slug string) (*models.Application, error)
	ListAppsByProject(ctx context.Context, projectID uuid.UUID) ([]*models.Application, error)
	UpdateApp(ctx context.Context, app *models.Application) error

	ListEnvironmentsByApp(ctx context.Context, appID uuid.UUID) ([]*models.Environment, error)

	ListEnvironments(ctx context.Context, orgID uuid.UUID) ([]OrgEnvironment, error)
	CreateEnvironment(ctx context.Context, env *OrgEnvironment) error
	GetEnvironmentBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*OrgEnvironment, error)
	UpdateEnvironment(ctx context.Context, env *OrgEnvironment) error
	DeleteEnvironment(ctx context.Context, id uuid.UUID) error
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

func (s *entityService) ListProjectsByOrg(ctx context.Context, orgID uuid.UUID, includeDeleted bool) ([]*models.Project, error) {
	return s.repo.ListProjectsByOrg(ctx, orgID, includeDeleted)
}

func (s *entityService) UpdateProject(ctx context.Context, project *models.Project) error {
	project.UpdatedAt = time.Now().UTC()
	return s.repo.UpdateProject(ctx, project)
}

func (s *entityService) SoftDeleteProject(ctx context.Context, orgID uuid.UUID, slug string) ([]models.FlagActivitySummary, error) {
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

	since := time.Now().UTC().Add(-recentActivityWindow)
	activeFlags, err := s.flagActivity.HasRecentFlagActivity(ctx, project.ID, since)
	if err != nil {
		return nil, fmt.Errorf("checking flag activity: %w", err)
	}
	if len(activeFlags) > 0 {
		return activeFlags, errors.New("project has flags with recent activity")
	}

	return nil, s.repo.SoftDeleteProject(ctx, project.ID)
}

func (s *entityService) HardDeleteProject(ctx context.Context, orgID uuid.UUID, slug string) (*time.Time, error) {
	project, err := s.repo.GetProjectBySlug(ctx, orgID, slug)
	if err != nil {
		return nil, err
	}
	if project == nil {
		return nil, errors.New("project not found")
	}
	if project.DeletedAt == nil {
		return nil, errors.New("project must be soft-deleted first")
	}

	eligibleAt := project.DeletedAt.Add(hardDeleteEligibleAfter)
	if time.Now().UTC().Before(eligibleAt) {
		return &eligibleAt, errors.New("project must be soft-deleted for at least 7 days")
	}

	return nil, s.repo.HardDeleteProject(ctx, project.ID)
}

func (s *entityService) RestoreProject(ctx context.Context, orgID uuid.UUID, slug string) error {
	project, err := s.repo.GetProjectBySlug(ctx, orgID, slug)
	if err != nil {
		return err
	}
	if project == nil {
		return errors.New("project not found")
	}
	if project.DeletedAt == nil {
		return errors.New("project is not deleted")
	}
	return s.repo.RestoreProject(ctx, project.ID)
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

func (s *entityService) ListAppsByProject(ctx context.Context, projectID uuid.UUID) ([]*models.Application, error) {
	return s.repo.ListAppsByProject(ctx, projectID)
}

func (s *entityService) UpdateApp(ctx context.Context, app *models.Application) error {
	app.UpdatedAt = time.Now().UTC()
	return s.repo.UpdateApp(ctx, app)
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
