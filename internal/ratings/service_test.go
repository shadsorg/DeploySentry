package ratings

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// Mock
// ---------------------------------------------------------------------------

type mockRatingRepo struct {
	upsertRatingFn     func(ctx context.Context, rating *models.FlagRating) error
	getRatingFn        func(ctx context.Context, flagID, userID uuid.UUID) (*models.FlagRating, error)
	listRatingsFn      func(ctx context.Context, flagID uuid.UUID, limit, offset int) ([]*models.FlagRating, error)
	deleteRatingFn     func(ctx context.Context, flagID, userID uuid.UUID) error
	getRatingSummaryFn func(ctx context.Context, flagID uuid.UUID) (*models.RatingSummary, error)
	upsertErrorStatsFn func(ctx context.Context, stat *models.FlagErrorStat) error
	getErrorSummaryFn  func(ctx context.Context, flagID uuid.UUID, period time.Duration) (*models.ErrorSummary, error)
	getErrorsByOrgFn   func(ctx context.Context, flagID uuid.UUID, period time.Duration) ([]*models.OrgErrorBreakdown, error)
	resolveFlagIDFn    func(ctx context.Context, projectID uuid.UUID, flagKey string) (uuid.UUID, error)
	getSettingValueFn  func(ctx context.Context, orgID uuid.UUID, key string) (*models.Setting, error)
}

func (m *mockRatingRepo) UpsertRating(ctx context.Context, rating *models.FlagRating) error {
	if m.upsertRatingFn != nil {
		return m.upsertRatingFn(ctx, rating)
	}
	return nil
}

func (m *mockRatingRepo) GetRating(ctx context.Context, flagID, userID uuid.UUID) (*models.FlagRating, error) {
	if m.getRatingFn != nil {
		return m.getRatingFn(ctx, flagID, userID)
	}
	return nil, nil
}

func (m *mockRatingRepo) ListRatings(ctx context.Context, flagID uuid.UUID, limit, offset int) ([]*models.FlagRating, error) {
	if m.listRatingsFn != nil {
		return m.listRatingsFn(ctx, flagID, limit, offset)
	}
	return []*models.FlagRating{}, nil
}

func (m *mockRatingRepo) DeleteRating(ctx context.Context, flagID, userID uuid.UUID) error {
	if m.deleteRatingFn != nil {
		return m.deleteRatingFn(ctx, flagID, userID)
	}
	return nil
}

func (m *mockRatingRepo) GetRatingSummary(ctx context.Context, flagID uuid.UUID) (*models.RatingSummary, error) {
	if m.getRatingSummaryFn != nil {
		return m.getRatingSummaryFn(ctx, flagID)
	}
	return &models.RatingSummary{}, nil
}

func (m *mockRatingRepo) UpsertErrorStats(ctx context.Context, stat *models.FlagErrorStat) error {
	if m.upsertErrorStatsFn != nil {
		return m.upsertErrorStatsFn(ctx, stat)
	}
	return nil
}

func (m *mockRatingRepo) GetErrorSummary(ctx context.Context, flagID uuid.UUID, period time.Duration) (*models.ErrorSummary, error) {
	if m.getErrorSummaryFn != nil {
		return m.getErrorSummaryFn(ctx, flagID, period)
	}
	return &models.ErrorSummary{}, nil
}

func (m *mockRatingRepo) GetErrorsByOrg(ctx context.Context, flagID uuid.UUID, period time.Duration) ([]*models.OrgErrorBreakdown, error) {
	if m.getErrorsByOrgFn != nil {
		return m.getErrorsByOrgFn(ctx, flagID, period)
	}
	return []*models.OrgErrorBreakdown{}, nil
}

func (m *mockRatingRepo) ResolveFlagID(ctx context.Context, projectID uuid.UUID, flagKey string) (uuid.UUID, error) {
	if m.resolveFlagIDFn != nil {
		return m.resolveFlagIDFn(ctx, projectID, flagKey)
	}
	return uuid.New(), nil
}

func (m *mockRatingRepo) GetSettingValue(ctx context.Context, orgID uuid.UUID, key string) (*models.Setting, error) {
	if m.getSettingValueFn != nil {
		return m.getSettingValueFn(ctx, orgID, key)
	}
	return nil, nil
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestUpsertRating_Valid(t *testing.T) {
	flagID := uuid.New()
	userID := uuid.New()
	orgID := uuid.New()

	var captured *models.FlagRating
	repo := &mockRatingRepo{
		upsertRatingFn: func(_ context.Context, r *models.FlagRating) error {
			captured = r
			return nil
		},
	}
	svc := NewRatingService(repo)

	rating := &models.FlagRating{
		FlagID: flagID,
		UserID: userID,
		OrgID:  orgID,
		Rating: 4,
	}
	err := svc.UpsertRating(context.Background(), rating)
	assert.NoError(t, err)
	assert.NotNil(t, captured)
	assert.Equal(t, int16(4), captured.Rating)
	assert.False(t, captured.CreatedAt.IsZero())
	assert.False(t, captured.UpdatedAt.IsZero())
}

func TestUpsertRating_InvalidRating(t *testing.T) {
	svc := NewRatingService(&mockRatingRepo{})

	rating := &models.FlagRating{
		FlagID: uuid.New(),
		UserID: uuid.New(),
		OrgID:  uuid.New(),
		Rating: 6,
	}
	err := svc.UpsertRating(context.Background(), rating)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "rating must be between 1 and 5")
}

func TestUpsertRating_RepoError(t *testing.T) {
	repo := &mockRatingRepo{
		upsertRatingFn: func(_ context.Context, _ *models.FlagRating) error {
			return errors.New("db error")
		},
	}
	svc := NewRatingService(repo)

	rating := &models.FlagRating{
		FlagID: uuid.New(),
		UserID: uuid.New(),
		OrgID:  uuid.New(),
		Rating: 3,
	}
	err := svc.UpsertRating(context.Background(), rating)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "db error")
}

func TestIsRatingsEnabled_Enabled(t *testing.T) {
	orgID := uuid.New()
	repo := &mockRatingRepo{
		getSettingValueFn: func(_ context.Context, id uuid.UUID, key string) (*models.Setting, error) {
			assert.Equal(t, orgID, id)
			assert.Equal(t, "flag_ratings_enabled", key)
			return &models.Setting{
				OrgID: &orgID,
				Key:   key,
				Value: []byte(`{"enabled": true}`),
			}, nil
		},
	}
	svc := NewRatingService(repo)
	enabled, err := svc.IsRatingsEnabled(context.Background(), orgID)
	assert.NoError(t, err)
	assert.True(t, enabled)
}

func TestIsRatingsEnabled_Disabled(t *testing.T) {
	repo := &mockRatingRepo{
		getSettingValueFn: func(_ context.Context, _ uuid.UUID, _ string) (*models.Setting, error) {
			return nil, nil
		},
	}
	svc := NewRatingService(repo)
	enabled, err := svc.IsRatingsEnabled(context.Background(), uuid.New())
	assert.NoError(t, err)
	assert.False(t, enabled)
}

func TestReportErrors(t *testing.T) {
	resolvedID := uuid.New()
	var captured []*models.FlagErrorStat
	repo := &mockRatingRepo{
		resolveFlagIDFn: func(_ context.Context, _ uuid.UUID, _ string) (uuid.UUID, error) {
			return resolvedID, nil
		},
		upsertErrorStatsFn: func(_ context.Context, stat *models.FlagErrorStat) error {
			captured = append(captured, stat)
			return nil
		},
	}
	svc := NewRatingService(repo)

	entries := []ErrorReportEntry{
		{FlagKey: "flag-a", Evaluations: 100, Errors: 2},
		{FlagKey: "flag-b", Evaluations: 50, Errors: 0},
	}
	envID := uuid.New()
	orgID := uuid.New()
	err := svc.ReportErrors(context.Background(), uuid.New(), entries, envID, orgID)
	assert.NoError(t, err)
	assert.Len(t, captured, 2)
	assert.Equal(t, resolvedID, captured[0].FlagID)
	// Verify period_start was truncated to the hour
	assert.Equal(t, 0, captured[0].PeriodStart.Minute())
	assert.Equal(t, 0, captured[0].PeriodStart.Second())
}

func TestListRatings_DefaultLimit(t *testing.T) {
	var capturedLimit int
	repo := &mockRatingRepo{
		listRatingsFn: func(_ context.Context, _ uuid.UUID, limit, _ int) ([]*models.FlagRating, error) {
			capturedLimit = limit
			return []*models.FlagRating{}, nil
		},
	}
	svc := NewRatingService(repo)
	_, err := svc.ListRatings(context.Background(), uuid.New(), 0, 0)
	assert.NoError(t, err)
	assert.Equal(t, 20, capturedLimit)
}
