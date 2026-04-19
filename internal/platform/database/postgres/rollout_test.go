package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
)

// newStrategyRepoForTest returns a StrategyRepo backed by the project's
// standard test database (same helper every other postgres_test file uses).
// See helpers.go for testDB().
func newStrategyRepoForTest(t *testing.T) *StrategyRepo {
	t.Helper()
	return NewStrategyRepo(testDB(t))
}

func sampleStrategy(scopeID uuid.UUID) *models.Strategy {
	return &models.Strategy{
		ScopeType:                models.ScopeOrg,
		ScopeID:                  scopeID,
		Name:                     "canary-" + uuid.NewString()[:8],
		Description:              "t",
		TargetType:               models.TargetTypeDeploy,
		Steps:                    []models.Step{{Percent: 1, MinDuration: time.Minute, MaxDuration: time.Minute, BakeTimeHealthy: 0}, {Percent: 100}},
		DefaultHealthThreshold:   0.95,
		DefaultRollbackOnFailure: true,
	}
}

func TestStrategyRepo_CreateGet(t *testing.T) {
	ctx := context.Background()
	repo := newStrategyRepoForTest(t)
	orgID := uuid.New()
	s := sampleStrategy(orgID)

	if err := repo.Create(ctx, s); err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := repo.GetByName(ctx, models.ScopeOrg, orgID, s.Name)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ID != s.ID || got.Name != s.Name || len(got.Steps) != 2 {
		t.Fatalf("mismatch: %+v", got)
	}
}

func TestStrategyRepo_UniqueNamePerScope(t *testing.T) {
	ctx := context.Background()
	repo := newStrategyRepoForTest(t)
	orgID := uuid.New()
	s1 := sampleStrategy(orgID)
	if err := repo.Create(ctx, s1); err != nil {
		t.Fatal(err)
	}
	s2 := sampleStrategy(orgID)
	s2.Name = s1.Name
	if err := repo.Create(ctx, s2); err == nil {
		t.Fatalf("expected unique violation")
	}
}

func TestStrategyRepo_UpdateOptimisticVersion(t *testing.T) {
	ctx := context.Background()
	repo := newStrategyRepoForTest(t)
	orgID := uuid.New()
	s := sampleStrategy(orgID)
	if err := repo.Create(ctx, s); err != nil {
		t.Fatal(err)
	}
	s.Description = "updated"
	if err := repo.Update(ctx, s, 1); err != nil {
		t.Fatalf("update v1: %v", err)
	}
	if err := repo.Update(ctx, s, 1); err == nil {
		t.Fatalf("expected version conflict")
	}
}

func TestStrategyRepo_SoftDelete(t *testing.T) {
	ctx := context.Background()
	repo := newStrategyRepoForTest(t)
	orgID := uuid.New()
	s := sampleStrategy(orgID)
	if err := repo.Create(ctx, s); err != nil {
		t.Fatal(err)
	}
	if err := repo.SoftDelete(ctx, s.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GetByName(ctx, models.ScopeOrg, orgID, s.Name); err == nil {
		t.Fatalf("expected not-found after soft delete")
	}
}
