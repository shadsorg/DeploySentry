package ratings

import (
	"context"
	"time"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
)

// RatingRepository defines the persistence interface for flag ratings and error stats.
type RatingRepository interface {
	// UpsertRating creates or updates a user's rating for a flag.
	UpsertRating(ctx context.Context, rating *models.FlagRating) error

	// GetRating retrieves a specific user's rating for a flag.
	GetRating(ctx context.Context, flagID, userID uuid.UUID) (*models.FlagRating, error)

	// ListRatings returns paginated ratings for a flag.
	ListRatings(ctx context.Context, flagID uuid.UUID, limit, offset int) ([]*models.FlagRating, error)

	// DeleteRating removes a user's rating for a flag.
	DeleteRating(ctx context.Context, flagID, userID uuid.UUID) error

	// GetRatingSummary returns aggregate rating data for a flag.
	GetRatingSummary(ctx context.Context, flagID uuid.UUID) (*models.RatingSummary, error)

	// UpsertErrorStats inserts or increments error stats for a flag/env/org/hour bucket.
	UpsertErrorStats(ctx context.Context, stat *models.FlagErrorStat) error

	// GetErrorSummary returns the error percentage for a flag over the given duration.
	GetErrorSummary(ctx context.Context, flagID uuid.UUID, period time.Duration) (*models.ErrorSummary, error)

	// GetErrorsByOrg returns per-org error breakdown for a flag (admin only).
	GetErrorsByOrg(ctx context.Context, flagID uuid.UUID, period time.Duration) ([]*models.OrgErrorBreakdown, error)

	// ResolveFlagID looks up a flag's UUID from its project_id and key.
	ResolveFlagID(ctx context.Context, projectID uuid.UUID, flagKey string) (uuid.UUID, error)

	// GetSettingValue retrieves a setting value by org_id and key.
	GetSettingValue(ctx context.Context, orgID uuid.UUID, key string) (*models.Setting, error)
}
