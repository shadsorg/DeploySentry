# Flag Ratings & Error Tracking Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a marketplace-style ratings system (1-5 stars + comments) and SDK error tracking for shared feature flags, opt-in per org.

**Architecture:** Two new tables (`flag_ratings`, `flag_error_stats`) with a dedicated ratings package under `internal/ratings/`. Ratings have their own handler, service interface, and repository — following the same pattern as `internal/flags/`. Error stats share the same package since they're closely related. The existing `settings` table stores the org-level opt-in toggle. Routes register alongside existing handlers in `cmd/api/main.go`.

**Tech Stack:** Go 1.22+, PostgreSQL 16 (deploy schema), Gin HTTP framework, pgx v5, testify/assert

**Spec:** `docs/superpowers/specs/2026-03-29-flag-ratings-and-error-tracking-design.md`

---

## File Structure

| Action | Path | Responsibility |
|--------|------|----------------|
| Create | `internal/models/flag_rating.go` | FlagRating model + Validate() |
| Create | `internal/models/flag_error_stat.go` | FlagErrorStat model + RatingSummary/ErrorSummary response structs |
| Create | `internal/ratings/repository.go` | RatingRepository interface (includes flag key resolution) |
| Create | `internal/ratings/service.go` | RatingService interface + concrete implementation |
| Create | `internal/ratings/handler.go` | HTTP handler + RegisterRoutes |
| Create | `internal/ratings/handler_test.go` | Handler tests with mock service |
| Create | `internal/ratings/service_test.go` | Service tests with mock repository |
| Create | `internal/platform/database/postgres/ratings.go` | PostgreSQL repository implementation |
| Create | `migrations/030_create_flag_ratings.up.sql` | Create flag_ratings + flag_error_stats tables |
| Create | `migrations/030_create_flag_ratings.down.sql` | Drop tables |
| Modify | `cmd/api/main.go:174-269` | Wire up RatingRepository, RatingService, RatingHandler |
| Modify | `internal/flags/handler.go` | Add rating_summary and error_rate to flag responses |

---

### Task 1: Database Migration

**Files:**
- Create: `migrations/030_create_flag_ratings.up.sql`
- Create: `migrations/030_create_flag_ratings.down.sql`

- [ ] **Step 1: Write the up migration**

```sql
-- 030_create_flag_ratings.up.sql
-- Flag ratings and error tracking for the CrowdSoft marketplace.

CREATE TABLE flag_ratings (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    flag_id    UUID        NOT NULL REFERENCES feature_flags(id) ON DELETE CASCADE,
    user_id    UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    org_id     UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    rating     SMALLINT    NOT NULL CHECK (rating >= 1 AND rating <= 5),
    comment    TEXT        CHECK (length(comment) <= 2000),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (flag_id, user_id)
);

CREATE INDEX deploy_idx_flag_ratings_flag_id ON flag_ratings (flag_id);
CREATE INDEX deploy_idx_flag_ratings_org_id ON flag_ratings (org_id);

CREATE TABLE flag_error_stats (
    id                UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    flag_id           UUID        NOT NULL REFERENCES feature_flags(id) ON DELETE CASCADE,
    environment_id    UUID        NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    org_id            UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    period_start      TIMESTAMPTZ NOT NULL,
    total_evaluations BIGINT      NOT NULL DEFAULT 0,
    error_count       BIGINT      NOT NULL DEFAULT 0,
    UNIQUE (flag_id, environment_id, org_id, period_start)
);

CREATE INDEX deploy_idx_flag_error_stats_flag_id ON flag_error_stats (flag_id);
```

- [ ] **Step 2: Write the down migration**

```sql
-- 030_create_flag_ratings.down.sql
DROP TABLE IF EXISTS flag_error_stats;
DROP TABLE IF EXISTS flag_ratings;
```

- [ ] **Step 3: Run migration**

Run: `make migrate-up`
Expected: Migration 030 applied successfully

- [ ] **Step 4: Verify tables exist**

Run: `make psql` then `\dt flag_ratings` and `\dt flag_error_stats`
Expected: Both tables listed in the deploy schema

- [ ] **Step 5: Test down migration and re-apply**

Run: `make migrate-down` then `make migrate-up`
Expected: Clean round-trip, no errors

- [ ] **Step 6: Commit**

```bash
git add migrations/030_create_flag_ratings.up.sql migrations/030_create_flag_ratings.down.sql
git commit -m "feat: add migration 030 for flag_ratings and flag_error_stats tables"
```

---

### Task 2: Models

**Files:**
- Create: `internal/models/flag_rating.go`
- Create: `internal/models/flag_error_stat.go`

- [ ] **Step 1: Create FlagRating model**

```go
// internal/models/flag_rating.go
package models

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// FlagRating represents a user's star rating and optional comment on a feature flag.
type FlagRating struct {
	ID        uuid.UUID `json:"id" db:"id"`
	FlagID    uuid.UUID `json:"flag_id" db:"flag_id"`
	UserID    uuid.UUID `json:"user_id" db:"user_id"`
	OrgID     uuid.UUID `json:"org_id" db:"org_id"`
	Rating    int16     `json:"rating" db:"rating"`
	Comment   string    `json:"comment,omitempty" db:"comment"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// Validate checks that the FlagRating has all required fields and valid values.
func (r *FlagRating) Validate() error {
	if r.FlagID == uuid.Nil {
		return errors.New("flag_id is required")
	}
	if r.UserID == uuid.Nil {
		return errors.New("user_id is required")
	}
	if r.OrgID == uuid.Nil {
		return errors.New("org_id is required")
	}
	if r.Rating < 1 || r.Rating > 5 {
		return errors.New("rating must be between 1 and 5")
	}
	if len(r.Comment) > 2000 {
		return errors.New("comment must be 2000 characters or fewer")
	}
	return nil
}

// RatingSummary is the aggregate rating data returned in API responses.
type RatingSummary struct {
	Average      float64        `json:"average"`
	Count        int            `json:"count"`
	Distribution map[int16]int  `json:"distribution"`
}
```

- [ ] **Step 2: Create FlagErrorStat model**

```go
// internal/models/flag_error_stat.go
package models

import (
	"time"

	"github.com/google/uuid"
)

// FlagErrorStat represents aggregated error counts for a flag in a given
// environment, org, and hourly time bucket.
type FlagErrorStat struct {
	ID               uuid.UUID `json:"id" db:"id"`
	FlagID           uuid.UUID `json:"flag_id" db:"flag_id"`
	EnvironmentID    uuid.UUID `json:"environment_id" db:"environment_id"`
	OrgID            uuid.UUID `json:"org_id" db:"org_id"`
	PeriodStart      time.Time `json:"period_start" db:"period_start"`
	TotalEvaluations int64     `json:"total_evaluations" db:"total_evaluations"`
	ErrorCount       int64     `json:"error_count" db:"error_count"`
}

// ErrorSummary is the aggregate error data returned in API responses.
type ErrorSummary struct {
	Percentage float64 `json:"percentage"`
	Period     string  `json:"period"`
}

// OrgErrorBreakdown is the per-org error data returned for admin endpoints.
type OrgErrorBreakdown struct {
	OrgID            uuid.UUID `json:"org_id"`
	TotalEvaluations int64     `json:"total_evaluations"`
	ErrorCount       int64     `json:"error_count"`
	Percentage       float64   `json:"percentage"`
}
```

- [ ] **Step 3: Commit**

```bash
git add internal/models/flag_rating.go internal/models/flag_error_stat.go
git commit -m "feat: add FlagRating and FlagErrorStat models"
```

---

### Task 3: Repository Interface

**Files:**
- Create: `internal/ratings/repository.go`

- [ ] **Step 1: Define repository interface**

```go
// internal/ratings/repository.go
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
```

- [ ] **Step 2: Commit**

```bash
git add internal/ratings/repository.go
git commit -m "feat: add RatingRepository interface"
```

---

### Task 4: Service Layer

**Files:**
- Create: `internal/ratings/service.go`
- Test: `internal/ratings/service_test.go`

- [ ] **Step 1: Write failing test for UpsertRating**

```go
// internal/ratings/service_test.go
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
	upsertRatingFn    func(ctx context.Context, rating *models.FlagRating) error
	getRatingFn       func(ctx context.Context, flagID, userID uuid.UUID) (*models.FlagRating, error)
	listRatingsFn     func(ctx context.Context, flagID uuid.UUID, limit, offset int) ([]*models.FlagRating, error)
	deleteRatingFn    func(ctx context.Context, flagID, userID uuid.UUID) error
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/sgamel/git/DeploySentry && go test ./internal/ratings/ -v -run TestUpsertRating`
Expected: FAIL — `NewRatingService` not defined

- [ ] **Step 3: Write the service implementation**

```go
// internal/ratings/service.go
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
	// setting.Value is json.RawMessage like {"enabled": true}
	type enabledValue struct {
		Enabled bool `json:"enabled"`
	}
	var v enabledValue
	if err := json.Unmarshal(setting.Value, &v); err != nil {
		return false, nil
	}
	return v.Enabled, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/sgamel/git/DeploySentry && go test ./internal/ratings/ -v -run TestUpsertRating`
Expected: All 3 tests PASS

- [ ] **Step 5: Write additional service tests**

Add to `internal/ratings/service_test.go`:

```go
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
```

- [ ] **Step 6: Run all service tests**

Run: `cd /Users/sgamel/git/DeploySentry && go test ./internal/ratings/ -v`
Expected: All tests PASS

- [ ] **Step 7: Commit**

```bash
git add internal/ratings/service.go internal/ratings/service_test.go
git commit -m "feat: add RatingService with validation, error reporting, and settings check"
```

---

### Task 5: HTTP Handler

**Files:**
- Create: `internal/ratings/handler.go`
- Test: `internal/ratings/handler_test.go`

- [ ] **Step 1: Write failing handler tests**

```go
// internal/ratings/handler_test.go
package ratings

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/deploysentry/deploysentry/internal/auth"
	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// Mock service for handler tests
// ---------------------------------------------------------------------------

type mockRatingService struct {
	upsertRatingFn    func(ctx context.Context, rating *models.FlagRating) error
	getRatingFn       func(ctx context.Context, flagID, userID uuid.UUID) (*models.FlagRating, error)
	listRatingsFn     func(ctx context.Context, flagID uuid.UUID, limit, offset int) ([]*models.FlagRating, error)
	deleteRatingFn    func(ctx context.Context, flagID, userID uuid.UUID) error
	getRatingSummaryFn func(ctx context.Context, flagID uuid.UUID) (*models.RatingSummary, error)
	reportErrorsFn    func(ctx context.Context, projectID uuid.UUID, entries []ErrorReportEntry, envID, orgID uuid.UUID) error
	getErrorSummaryFn func(ctx context.Context, flagID uuid.UUID, period time.Duration) (*models.ErrorSummary, error)
	getErrorsByOrgFn  func(ctx context.Context, flagID uuid.UUID, period time.Duration) ([]*models.OrgErrorBreakdown, error)
	isRatingsEnabledFn func(ctx context.Context, orgID uuid.UUID) (bool, error)
}

func (m *mockRatingService) UpsertRating(ctx context.Context, r *models.FlagRating) error {
	if m.upsertRatingFn != nil { return m.upsertRatingFn(ctx, r) }
	return nil
}
func (m *mockRatingService) GetRating(ctx context.Context, fid, uid uuid.UUID) (*models.FlagRating, error) {
	if m.getRatingFn != nil { return m.getRatingFn(ctx, fid, uid) }
	return &models.FlagRating{}, nil
}
func (m *mockRatingService) ListRatings(ctx context.Context, fid uuid.UUID, l, o int) ([]*models.FlagRating, error) {
	if m.listRatingsFn != nil { return m.listRatingsFn(ctx, fid, l, o) }
	return []*models.FlagRating{}, nil
}
func (m *mockRatingService) DeleteRating(ctx context.Context, fid, uid uuid.UUID) error {
	if m.deleteRatingFn != nil { return m.deleteRatingFn(ctx, fid, uid) }
	return nil
}
func (m *mockRatingService) GetRatingSummary(ctx context.Context, fid uuid.UUID) (*models.RatingSummary, error) {
	if m.getRatingSummaryFn != nil { return m.getRatingSummaryFn(ctx, fid) }
	return &models.RatingSummary{}, nil
}
func (m *mockRatingService) ReportErrors(ctx context.Context, projectID uuid.UUID, entries []ErrorReportEntry, envID, orgID uuid.UUID) error {
	if m.reportErrorsFn != nil { return m.reportErrorsFn(ctx, projectID, entries, envID, orgID) }
	return nil
}
func (m *mockRatingService) GetErrorSummary(ctx context.Context, fid uuid.UUID, p time.Duration) (*models.ErrorSummary, error) {
	if m.getErrorSummaryFn != nil { return m.getErrorSummaryFn(ctx, fid, p) }
	return &models.ErrorSummary{}, nil
}
func (m *mockRatingService) GetErrorsByOrg(ctx context.Context, fid uuid.UUID, p time.Duration) ([]*models.OrgErrorBreakdown, error) {
	if m.getErrorsByOrgFn != nil { return m.getErrorsByOrgFn(ctx, fid, p) }
	return []*models.OrgErrorBreakdown{}, nil
}
func (m *mockRatingService) IsRatingsEnabled(ctx context.Context, oid uuid.UUID) (bool, error) {
	if m.isRatingsEnabledFn != nil { return m.isRatingsEnabledFn(ctx, oid) }
	return true, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func setupRatingRouter(svc RatingService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", uuid.New())          // JWT middleware sets uuid.UUID
		c.Set("org_id", uuid.New().String())   // org_id is stored as string
		c.Set("role", auth.RoleOwner)
		c.Next()
	})
	rbac := auth.NewRBACChecker()
	handler := NewHandler(svc, rbac)
	handler.RegisterRoutes(router.Group("/api"))
	return router
}

func toJSON(t *testing.T, v interface{}) *bytes.Buffer {
	t.Helper()
	b, err := json.Marshal(v)
	assert.NoError(t, err)
	return bytes.NewBuffer(b)
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestCreateRating_Valid(t *testing.T) {
	flagID := uuid.New()
	svc := &mockRatingService{
		upsertRatingFn: func(_ context.Context, r *models.FlagRating) error {
			assert.Equal(t, flagID, r.FlagID)
			assert.Equal(t, int16(4), r.Rating)
			return nil
		},
	}
	router := setupRatingRouter(svc)

	body := map[string]interface{}{"rating": 4, "comment": "Great feature"}
	req := httptest.NewRequest(http.MethodPost, "/api/flags/"+flagID.String()+"/ratings", toJSON(t, body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCreateRating_RatingsDisabled(t *testing.T) {
	svc := &mockRatingService{
		isRatingsEnabledFn: func(_ context.Context, _ uuid.UUID) (bool, error) {
			return false, nil
		},
	}
	router := setupRatingRouter(svc)

	body := map[string]interface{}{"rating": 4}
	req := httptest.NewRequest(http.MethodPost, "/api/flags/"+uuid.New().String()+"/ratings", toJSON(t, body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestGetRatingSummary(t *testing.T) {
	flagID := uuid.New()
	svc := &mockRatingService{
		getRatingSummaryFn: func(_ context.Context, id uuid.UUID) (*models.RatingSummary, error) {
			assert.Equal(t, flagID, id)
			return &models.RatingSummary{Average: 4.2, Count: 10, Distribution: map[int16]int{4: 5, 5: 5}}, nil
		},
	}
	router := setupRatingRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/flags/"+flagID.String()+"/ratings/summary", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp models.RatingSummary
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, 4.2, resp.Average)
	assert.Equal(t, 10, resp.Count)
}

func TestDeleteRating(t *testing.T) {
	svc := &mockRatingService{}
	router := setupRatingRouter(svc)

	req := httptest.NewRequest(http.MethodDelete, "/api/flags/"+uuid.New().String()+"/ratings", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestReportErrors_Handler(t *testing.T) {
	var capturedEntries []ErrorReportEntry
	svc := &mockRatingService{
		reportErrorsFn: func(_ context.Context, _ uuid.UUID, entries []ErrorReportEntry, _, _ uuid.UUID) error {
			capturedEntries = entries
			return nil
		},
	}
	router := setupRatingRouter(svc)

	body := map[string]interface{}{
		"project_id":     uuid.New().String(),
		"environment_id": uuid.New().String(),
		"org_id":         uuid.New().String(),
		"stats": []map[string]interface{}{
			{"flag_key": "test-flag", "evaluations": 100, "errors": 2},
		},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/flags/errors/report", toJSON(t, body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
}

func TestGetErrorsByOrg_RequiresAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", uuid.New().String())
		c.Set("org_id", uuid.New().String())
		c.Set("role", auth.RoleMember) // not admin/owner
		c.Next()
	})
	rbac := auth.NewRBACChecker()
	handler := NewHandler(&mockRatingService{}, rbac)
	handler.RegisterRoutes(router.Group("/api"))

	req := httptest.NewRequest(http.MethodGet, "/api/flags/"+uuid.New().String()+"/errors/by-org", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/sgamel/git/DeploySentry && go test ./internal/ratings/ -v -run "TestCreateRating|TestGetRating|TestDeleteRating|TestReportErrors|TestGetErrors"`
Expected: FAIL — `NewHandler` not defined

- [ ] **Step 3: Write the handler implementation**

```go
// internal/ratings/handler.go
package ratings

import (
	"net/http"
	"strconv"
	"time"

	"github.com/deploysentry/deploysentry/internal/auth"
	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Handler provides HTTP endpoints for flag ratings and error tracking.
type Handler struct {
	service RatingService
	rbac    *auth.RBACChecker
}

// NewHandler creates a new ratings HTTP handler.
func NewHandler(service RatingService, rbac *auth.RBACChecker) *Handler {
	return &Handler{service: service, rbac: rbac}
}

// RegisterRoutes mounts all rating and error tracking routes on the given router group.
// NOTE: These routes nest under /flags alongside the existing flags handler routes.
// Gin resolves these correctly because each route has distinct path segments
// beyond /:id (e.g., /:id/ratings vs /:id alone). The /errors/report path uses
// a fixed "errors" segment, not a parameterized one, so no conflict with /:id.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	flags := rg.Group("/flags")
	{
		// Error reporting (not gated by ratings toggle)
		flags.POST("/errors/report", auth.RequirePermission(h.rbac, auth.PermFlagUpdate), h.reportErrors)

		// Per-flag routes
		flag := flags.Group("/:id")
		{
			// Ratings (gated by ratings toggle)
			flag.POST("/ratings", auth.RequirePermission(h.rbac, auth.PermFlagUpdate), h.upsertRating)
			flag.GET("/ratings", auth.RequirePermission(h.rbac, auth.PermFlagRead), h.listRatings)
			flag.GET("/ratings/summary", auth.RequirePermission(h.rbac, auth.PermFlagRead), h.getRatingSummary)
			flag.DELETE("/ratings", auth.RequirePermission(h.rbac, auth.PermFlagUpdate), h.deleteRating)

			// Error stats (not gated by ratings toggle)
			flag.GET("/errors/summary", auth.RequirePermission(h.rbac, auth.PermFlagRead), h.getErrorSummary)
			flag.GET("/errors/by-org", auth.RequirePermission(h.rbac, auth.PermOrgManage), h.getErrorsByOrg)
		}
	}
}

// ---------------------------------------------------------------------------
// Request types
// ---------------------------------------------------------------------------

type upsertRatingRequest struct {
	Rating  int16  `json:"rating" binding:"required"`
	Comment string `json:"comment"`
}

type reportErrorsRequest struct {
	ProjectID     uuid.UUID          `json:"project_id" binding:"required"`
	EnvironmentID uuid.UUID          `json:"environment_id" binding:"required"`
	OrgID         uuid.UUID          `json:"org_id" binding:"required"`
	Stats         []errorStatEntry   `json:"stats" binding:"required"`
}

type errorStatEntry struct {
	FlagKey     string `json:"flag_key" binding:"required"`
	Evaluations int64  `json:"evaluations" binding:"required"`
	Errors      int64  `json:"errors"`
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

// getContextIDs extracts user_id and org_id from the gin context.
// user_id is set as uuid.UUID by JWT middleware; org_id is set as string.
func getContextIDs(c *gin.Context) (userID uuid.UUID, orgID uuid.UUID, ok bool) {
	uid, exists := c.Get("user_id")
	if !exists {
		return uuid.Nil, uuid.Nil, false
	}
	userID, _ = uid.(uuid.UUID)

	oid, err := uuid.Parse(c.GetString("org_id"))
	if err != nil {
		return uuid.Nil, uuid.Nil, false
	}
	return userID, oid, true
}

func (h *Handler) upsertRating(c *gin.Context) {
	flagID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid flag ID"})
		return
	}

	userID, orgID, ok := getContextIDs(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing user/org context"})
		return
	}

	enabled, err := h.service.IsRatingsEnabled(c.Request.Context(), orgID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check ratings setting"})
		return
	}
	if !enabled {
		c.JSON(http.StatusForbidden, gin.H{"error": "ratings are not enabled for this organization"})
		return
	}

	var req upsertRatingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	rating := &models.FlagRating{
		FlagID:  flagID,
		UserID:  userID,
		OrgID:   orgID,
		Rating:  req.Rating,
		Comment: req.Comment,
	}

	if err := h.service.UpsertRating(c.Request.Context(), rating); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rating)
}

// requireRatingsEnabled is a helper that checks the org's ratings toggle.
// Returns orgID and whether the check passed. Writes HTTP error if not.
func (h *Handler) requireRatingsEnabled(c *gin.Context) (uuid.UUID, bool) {
	_, orgID, ok := getContextIDs(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing user/org context"})
		return uuid.Nil, false
	}
	enabled, err := h.service.IsRatingsEnabled(c.Request.Context(), orgID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check ratings setting"})
		return uuid.Nil, false
	}
	if !enabled {
		c.JSON(http.StatusForbidden, gin.H{"error": "ratings are not enabled for this organization"})
		return uuid.Nil, false
	}
	return orgID, true
}

func (h *Handler) listRatings(c *gin.Context) {
	flagID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid flag ID"})
		return
	}
	if _, ok := h.requireRatingsEnabled(c); !ok {
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	ratings, err := h.service.ListRatings(c.Request.Context(), flagID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, ratings)
}

func (h *Handler) getRatingSummary(c *gin.Context) {
	flagID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid flag ID"})
		return
	}
	if _, ok := h.requireRatingsEnabled(c); !ok {
		return
	}

	summary, err := h.service.GetRatingSummary(c.Request.Context(), flagID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, summary)
}

func (h *Handler) deleteRating(c *gin.Context) {
	flagID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid flag ID"})
		return
	}
	if _, ok := h.requireRatingsEnabled(c); !ok {
		return
	}

	userID, _, ok := getContextIDs(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing user context"})
		return
	}

	if err := h.service.DeleteRating(c.Request.Context(), flagID, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) reportErrors(c *gin.Context) {
	var req reportErrorsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	entries := make([]ErrorReportEntry, len(req.Stats))
	for i, s := range req.Stats {
		entries[i] = ErrorReportEntry{FlagKey: s.FlagKey, Evaluations: s.Evaluations, Errors: s.Errors}
	}

	if err := h.service.ReportErrors(c.Request.Context(), req.ProjectID, entries, req.EnvironmentID, req.OrgID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"status": "accepted"})
}

func (h *Handler) getErrorSummary(c *gin.Context) {
	flagID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid flag ID"})
		return
	}

	periodStr := c.DefaultQuery("period", "7d")
	period := parsePeriod(periodStr)

	summary, err := h.service.GetErrorSummary(c.Request.Context(), flagID, period)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, summary)
}

func (h *Handler) getErrorsByOrg(c *gin.Context) {
	flagID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid flag ID"})
		return
	}

	periodStr := c.DefaultQuery("period", "7d")
	period := parsePeriod(periodStr)

	breakdown, err := h.service.GetErrorsByOrg(c.Request.Context(), flagID, period)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, breakdown)
}

func parsePeriod(s string) time.Duration {
	switch s {
	case "24h":
		return 24 * time.Hour
	case "7d":
		return 7 * 24 * time.Hour
	case "30d":
		return 30 * 24 * time.Hour
	default:
		return 7 * 24 * time.Hour
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/sgamel/git/DeploySentry && go test ./internal/ratings/ -v`
Expected: All tests PASS

- [ ] **Step 5: Commit**

```bash
git add internal/ratings/handler.go internal/ratings/handler_test.go
git commit -m "feat: add ratings and error tracking HTTP handler with tests"
```

---

### Task 6: PostgreSQL Repository Implementation

**Files:**
- Create: `internal/platform/database/postgres/ratings.go`

- [ ] **Step 1: Implement the PostgreSQL repository**

```go
// internal/platform/database/postgres/ratings.go
package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RatingRepository implements ratings.RatingRepository using PostgreSQL.
type RatingRepository struct {
	pool *pgxpool.Pool
}

// NewRatingRepository creates a new RatingRepository backed by the given pool.
func NewRatingRepository(pool *pgxpool.Pool) *RatingRepository {
	return &RatingRepository{pool: pool}
}

func (r *RatingRepository) UpsertRating(ctx context.Context, rating *models.FlagRating) error {
	query := `
		INSERT INTO flag_ratings (id, flag_id, user_id, org_id, rating, comment, created_at, updated_at)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (flag_id, user_id) DO UPDATE SET
			rating = EXCLUDED.rating,
			comment = EXCLUDED.comment,
			updated_at = EXCLUDED.updated_at
		RETURNING id, created_at, updated_at`
	return r.pool.QueryRow(ctx, query,
		rating.FlagID, rating.UserID, rating.OrgID,
		rating.Rating, rating.Comment,
		rating.CreatedAt, rating.UpdatedAt,
	).Scan(&rating.ID, &rating.CreatedAt, &rating.UpdatedAt)
}

func (r *RatingRepository) GetRating(ctx context.Context, flagID, userID uuid.UUID) (*models.FlagRating, error) {
	query := `SELECT id, flag_id, user_id, org_id, rating, comment, created_at, updated_at
		FROM flag_ratings WHERE flag_id = $1 AND user_id = $2`
	var fr models.FlagRating
	err := r.pool.QueryRow(ctx, query, flagID, userID).Scan(
		&fr.ID, &fr.FlagID, &fr.UserID, &fr.OrgID,
		&fr.Rating, &fr.Comment, &fr.CreatedAt, &fr.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &fr, nil
}

func (r *RatingRepository) ListRatings(ctx context.Context, flagID uuid.UUID, limit, offset int) ([]*models.FlagRating, error) {
	query := `SELECT id, flag_id, user_id, org_id, rating, comment, created_at, updated_at
		FROM flag_ratings WHERE flag_id = $1
		ORDER BY created_at DESC LIMIT $2 OFFSET $3`
	rows, err := r.pool.Query(ctx, query, flagID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*models.FlagRating
	for rows.Next() {
		var fr models.FlagRating
		if err := rows.Scan(&fr.ID, &fr.FlagID, &fr.UserID, &fr.OrgID,
			&fr.Rating, &fr.Comment, &fr.CreatedAt, &fr.UpdatedAt); err != nil {
			return nil, err
		}
		results = append(results, &fr)
	}
	return results, rows.Err()
}

func (r *RatingRepository) DeleteRating(ctx context.Context, flagID, userID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM flag_ratings WHERE flag_id = $1 AND user_id = $2`, flagID, userID)
	return err
}

func (r *RatingRepository) GetRatingSummary(ctx context.Context, flagID uuid.UUID) (*models.RatingSummary, error) {
	query := `SELECT COALESCE(AVG(rating), 0), COUNT(*) FROM flag_ratings WHERE flag_id = $1`
	summary := &models.RatingSummary{Distribution: make(map[int16]int)}
	err := r.pool.QueryRow(ctx, query, flagID).Scan(&summary.Average, &summary.Count)
	if err != nil {
		return nil, err
	}
	if summary.Count == 0 {
		return summary, nil
	}

	distQuery := `SELECT rating, COUNT(*) FROM flag_ratings WHERE flag_id = $1 GROUP BY rating`
	rows, err := r.pool.Query(ctx, distQuery, flagID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var star int16
		var count int
		if err := rows.Scan(&star, &count); err != nil {
			return nil, err
		}
		summary.Distribution[star] = count
	}
	return summary, rows.Err()
}

func (r *RatingRepository) UpsertErrorStats(ctx context.Context, stat *models.FlagErrorStat) error {
	query := `
		INSERT INTO flag_error_stats (id, flag_id, environment_id, org_id, period_start, total_evaluations, error_count)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6)
		ON CONFLICT (flag_id, environment_id, org_id, period_start) DO UPDATE SET
			total_evaluations = flag_error_stats.total_evaluations + EXCLUDED.total_evaluations,
			error_count = flag_error_stats.error_count + EXCLUDED.error_count`
	_, err := r.pool.Exec(ctx, query,
		stat.FlagID, stat.EnvironmentID, stat.OrgID,
		stat.PeriodStart, stat.TotalEvaluations, stat.ErrorCount,
	)
	return err
}

func (r *RatingRepository) GetErrorSummary(ctx context.Context, flagID uuid.UUID, period time.Duration) (*models.ErrorSummary, error) {
	query := `SELECT COALESCE(SUM(total_evaluations), 0), COALESCE(SUM(error_count), 0)
		FROM flag_error_stats
		WHERE flag_id = $1 AND period_start >= $2`
	since := time.Now().UTC().Add(-period)
	var totalEvals, errorCount int64
	err := r.pool.QueryRow(ctx, query, flagID, since).Scan(&totalEvals, &errorCount)
	if err != nil {
		return nil, err
	}
	pct := 0.0
	if totalEvals > 0 {
		pct = float64(errorCount) / float64(totalEvals) * 100
	}
	periodStr := "7d"
	switch {
	case period <= 24*time.Hour:
		periodStr = "24h"
	case period <= 7*24*time.Hour:
		periodStr = "7d"
	default:
		periodStr = "30d"
	}
	return &models.ErrorSummary{Percentage: pct, Period: periodStr}, nil
}

func (r *RatingRepository) GetErrorsByOrg(ctx context.Context, flagID uuid.UUID, period time.Duration) ([]*models.OrgErrorBreakdown, error) {
	query := `SELECT org_id, SUM(total_evaluations), SUM(error_count)
		FROM flag_error_stats
		WHERE flag_id = $1 AND period_start >= $2
		GROUP BY org_id
		ORDER BY SUM(error_count) DESC`
	since := time.Now().UTC().Add(-period)
	rows, err := r.pool.Query(ctx, query, flagID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*models.OrgErrorBreakdown
	for rows.Next() {
		var b models.OrgErrorBreakdown
		if err := rows.Scan(&b.OrgID, &b.TotalEvaluations, &b.ErrorCount); err != nil {
			return nil, err
		}
		if b.TotalEvaluations > 0 {
			b.Percentage = float64(b.ErrorCount) / float64(b.TotalEvaluations) * 100
		}
		results = append(results, &b)
	}
	return results, rows.Err()
}

func (r *RatingRepository) ResolveFlagID(ctx context.Context, projectID uuid.UUID, flagKey string) (uuid.UUID, error) {
	query := `SELECT id FROM feature_flags WHERE project_id = $1 AND key = $2`
	var flagID uuid.UUID
	err := r.pool.QueryRow(ctx, query, projectID, flagKey).Scan(&flagID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("flag not found: project=%s key=%s", projectID, flagKey)
	}
	return flagID, nil
}

func (r *RatingRepository) GetSettingValue(ctx context.Context, orgID uuid.UUID, key string) (*models.Setting, error) {
	query := `SELECT id, org_id, key, value, updated_by, updated_at
		FROM settings WHERE org_id = $1 AND key = $2`
	var s models.Setting
	err := r.pool.QueryRow(ctx, query, orgID, key).Scan(
		&s.ID, &s.OrgID, &s.Key, &s.Value, &s.UpdatedBy, &s.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/sgamel/git/DeploySentry && go build ./internal/platform/database/postgres/`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add internal/platform/database/postgres/ratings.go
git commit -m "feat: add PostgreSQL rating repository implementation"
```

---

### Task 7: Wire Up in main.go

**Files:**
- Modify: `cmd/api/main.go:174-269`

- [ ] **Step 1: Add imports and wiring**

In the Repositories section (after line 180), add:
```go
ratingRepo := postgres.NewRatingRepository(db.Pool)
```

In the Services section (after line 192), add:
```go
ratingService := ratings.NewRatingService(ratingRepo)
```

In the Routes section (after line 262, after the flags handler registration), add:
```go
ratings.NewHandler(ratingService, rbacChecker).RegisterRoutes(api)
```

Add to imports:
```go
"github.com/deploysentry/deploysentry/internal/ratings"
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/sgamel/git/DeploySentry && go build ./cmd/api/`
Expected: No errors

- [ ] **Step 3: Run all tests**

Run: `cd /Users/sgamel/git/DeploySentry && go test ./...`
Expected: All tests PASS

- [ ] **Step 4: Commit**

```bash
git add cmd/api/main.go
git commit -m "feat: wire up ratings handler, service, and repository in API server"
```

---

### Task 8: Augmented Flag Responses

**Files:**
- Modify: `internal/flags/handler.go`
- Modify: `internal/flags/handler_test.go`

Per the spec, `GET /flags` and `GET /flags/:id` should include `rating_summary` and `error_rate` fields when data exists. This requires the flags handler to call the ratings service.

- [ ] **Step 1: Add RatingService dependency to flags handler**

In `internal/flags/handler.go`, add a `ratingSvc` field to the `Handler` struct:

```go
type Handler struct {
	service      FlagService
	rbac         *auth.RBACChecker
	sse          *SSEBroker
	webhookSvc   *webhooks.Service
	analyticsSvc *analytics.Service
	ratingSvc    ratings.RatingService // optional, may be nil
}
```

Update `NewHandler` to accept an optional `ratings.RatingService` parameter. When wiring in `main.go`, pass the `ratingService`.

- [ ] **Step 2: Create response wrapper struct**

```go
type flagResponse struct {
	*models.FeatureFlag
	RatingSummary *models.RatingSummary `json:"rating_summary,omitempty"`
	ErrorRate     *models.ErrorSummary  `json:"error_rate,omitempty"`
}
```

- [ ] **Step 3: Augment getFlag handler**

After fetching the flag, if `h.ratingSvc != nil`:
1. Check if ratings enabled for the requesting org
2. If enabled, call `GetRatingSummary(flagID)` and attach to response
3. Always call `GetErrorSummary(flagID, 7*24*time.Hour)` and attach to response

- [ ] **Step 4: Augment listFlags handler**

Same approach, but batch the summary lookups across all returned flags.

- [ ] **Step 5: Write test for augmented getFlag**

Add a test to `internal/flags/handler_test.go` that verifies `rating_summary` and `error_rate` appear in the `GET /flags/:id` response when the rating service returns data.

- [ ] **Step 6: Run tests**

Run: `cd /Users/sgamel/git/DeploySentry && go test ./internal/flags/ -v`
Expected: All tests PASS

- [ ] **Step 7: Commit**

```bash
git add internal/flags/handler.go internal/flags/handler_test.go
git commit -m "feat: augment flag responses with rating_summary and error_rate"
```

---

### Task 9: Update Plan Documentation

**Files:**
- Modify: `docs/Current_Initiatives.md`

- [ ] **Step 1: Create the plan doc entry**

Add a row to `docs/Current_Initiatives.md`:

```markdown
| Flag Ratings & Error Tracking | Implementation | [Link](./superpowers/specs/2026-03-29-flag-ratings-and-error-tracking-design.md) |
```

- [ ] **Step 2: Commit**

```bash
git add docs/Current_Initiatives.md
git commit -m "docs: add flag ratings initiative to current initiatives"
```
