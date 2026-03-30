package ratings

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
)

// RatingService defines the interface for managing flag ratings and error stats.
type RatingService interface {
	UpsertRating(ctx context.Context, rating *models.FlagRating) error
	GetRating(ctx context.Context, flagID, userID uuid.UUID) (*models.FlagRating, error)
	ListRatings(ctx context.Context, flagID uuid.UUID, limit, offset int) ([]*models.FlagRating, error)
	DeleteRating(ctx context.Context, flagID, userID uuid.UUID) error
	GetRatingSummary(ctx context.Context, flagID uuid.UUID) (*models.RatingSummary, error)
	ReportErrors(ctx context.Context, projectID uuid.UUID, entries []ErrorReportEntry, envID, orgID uuid.UUID) error
	GetErrorSummary(ctx context.Context, flagID uuid.UUID, period time.Duration) (*models.ErrorSummary, error)
	GetErrorsByOrg(ctx context.Context, flagID uuid.UUID, period time.Duration) ([]*models.OrgErrorBreakdown, error)
	IsRatingsEnabled(ctx context.Context, orgID uuid.UUID) (bool, error)
}

// ErrorReportEntry is a single flag's error data from an SDK batch report.
type ErrorReportEntry struct {
	FlagKey     string `json:"flag_key"`
	Evaluations int64  `json:"evaluations"`
	Errors      int64  `json:"errors"`
}

type ratingService struct {
	repo RatingRepository
}

// NewRatingService creates a new RatingService backed by the given repository.
func NewRatingService(repo RatingRepository) RatingService {
	return &ratingService{repo: repo}
}

func (s *ratingService) UpsertRating(ctx context.Context, rating *models.FlagRating) error {
	if err := rating.Validate(); err != nil {
		return err
	}
	now := time.Now().UTC()
	rating.CreatedAt = now
	rating.UpdatedAt = now
	return s.repo.UpsertRating(ctx, rating)
}

func (s *ratingService) GetRating(ctx context.Context, flagID, userID uuid.UUID) (*models.FlagRating, error) {
	return s.repo.GetRating(ctx, flagID, userID)
}

func (s *ratingService) ListRatings(ctx context.Context, flagID uuid.UUID, limit, offset int) ([]*models.FlagRating, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	return s.repo.ListRatings(ctx, flagID, limit, offset)
}

func (s *ratingService) DeleteRating(ctx context.Context, flagID, userID uuid.UUID) error {
	return s.repo.DeleteRating(ctx, flagID, userID)
}

func (s *ratingService) GetRatingSummary(ctx context.Context, flagID uuid.UUID) (*models.RatingSummary, error) {
	return s.repo.GetRatingSummary(ctx, flagID)
}

func (s *ratingService) ReportErrors(ctx context.Context, projectID uuid.UUID, entries []ErrorReportEntry, envID, orgID uuid.UUID) error {
	now := time.Now().UTC().Truncate(time.Hour)
	for _, entry := range entries {
		flagID, err := s.repo.ResolveFlagID(ctx, projectID, entry.FlagKey)
		if err != nil {
			return fmt.Errorf("resolving flag key %q: %w", entry.FlagKey, err)
		}
		stat := &models.FlagErrorStat{
			FlagID:           flagID,
			EnvironmentID:    envID,
			OrgID:            orgID,
			PeriodStart:      now,
			TotalEvaluations: entry.Evaluations,
			ErrorCount:       entry.Errors,
		}
		if err := s.repo.UpsertErrorStats(ctx, stat); err != nil {
			return err
		}
	}
	return nil
}

func (s *ratingService) GetErrorSummary(ctx context.Context, flagID uuid.UUID, period time.Duration) (*models.ErrorSummary, error) {
	return s.repo.GetErrorSummary(ctx, flagID, period)
}

func (s *ratingService) GetErrorsByOrg(ctx context.Context, flagID uuid.UUID, period time.Duration) ([]*models.OrgErrorBreakdown, error) {
	return s.repo.GetErrorsByOrg(ctx, flagID, period)
}

func (s *ratingService) IsRatingsEnabled(ctx context.Context, orgID uuid.UUID) (bool, error) {
	setting, err := s.repo.GetSettingValue(ctx, orgID, "flag_ratings_enabled")
	if err != nil {
		return false, err
	}
	if setting == nil {
		return false, nil
	}
	type enabledValue struct {
		Enabled bool `json:"enabled"`
	}
	var v enabledValue
	if err := json.Unmarshal(setting.Value, &v); err != nil {
		return false, nil
	}
	return v.Enabled, nil
}
