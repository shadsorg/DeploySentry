package entities

import (
	"context"
	"errors"
	"time"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
)

// EntityService defines the interface for entity management.
type EntityService interface {
	CreateOrg(ctx context.Context, org *models.Organization, creatorID uuid.UUID) error
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
}

type entityService struct {
	repo EntityRepository
}

func NewEntityService(repo EntityRepository) EntityService {
	return &entityService{repo: repo}
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

func (s *entityService) ListProjectsByOrg(ctx context.Context, orgID uuid.UUID) ([]*models.Project, error) {
	return s.repo.ListProjectsByOrg(ctx, orgID)
}

func (s *entityService) UpdateProject(ctx context.Context, project *models.Project) error {
	project.UpdatedAt = time.Now().UTC()
	return s.repo.UpdateProject(ctx, project)
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
