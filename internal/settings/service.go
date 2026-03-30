package settings

import (
	"context"
	"fmt"
	"time"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
)

// SettingService defines the business-logic layer for hierarchical settings.
type SettingService interface {
	List(ctx context.Context, scope string, targetID uuid.UUID) ([]*models.Setting, error)
	Resolve(ctx context.Context, key string, orgID, projectID, appID, envID *uuid.UUID) (*models.Setting, error)
	Set(ctx context.Context, setting *models.Setting) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type settingService struct {
	repo SettingRepository
}

// NewSettingService creates a new SettingService backed by the given repository.
func NewSettingService(repo SettingRepository) SettingService {
	return &settingService{repo: repo}
}

func (s *settingService) List(ctx context.Context, scope string, targetID uuid.UUID) ([]*models.Setting, error) {
	settings, err := s.repo.ListByScope(ctx, scope, targetID)
	if err != nil {
		return nil, fmt.Errorf("listing settings: %w", err)
	}
	return settings, nil
}

func (s *settingService) Resolve(ctx context.Context, key string, orgID, projectID, appID, envID *uuid.UUID) (*models.Setting, error) {
	setting, err := s.repo.Resolve(ctx, key, orgID, projectID, appID, envID)
	if err != nil {
		return nil, fmt.Errorf("resolving setting: %w", err)
	}
	return setting, nil
}

func (s *settingService) Set(ctx context.Context, setting *models.Setting) error {
	setting.UpdatedAt = time.Now().UTC()
	if err := setting.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}
	if err := s.repo.Upsert(ctx, setting); err != nil {
		return fmt.Errorf("setting value: %w", err)
	}
	return nil
}

func (s *settingService) Delete(ctx context.Context, id uuid.UUID) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("deleting setting: %w", err)
	}
	return nil
}
