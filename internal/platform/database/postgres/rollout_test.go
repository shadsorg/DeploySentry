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

func TestStrategyDefaultsRepo_Upsert(t *testing.T) {
	ctx := context.Background()
	db := testDB(t)
	srepo := NewStrategyRepo(db)
	drepo := NewStrategyDefaultsRepo(db)
	orgID := uuid.New()
	s := sampleStrategy(orgID)
	if err := srepo.Create(ctx, s); err != nil {
		t.Fatal(err)
	}
	env := "prod"
	tt := models.TargetTypeDeploy
	d := &models.StrategyDefault{ScopeType: models.ScopeOrg, ScopeID: orgID, Environment: &env, TargetType: &tt, StrategyID: s.ID}
	if err := drepo.Upsert(ctx, d); err != nil {
		t.Fatalf("insert: %v", err)
	}
	// Upsert again replaces
	s2 := sampleStrategy(orgID)
	if err := srepo.Create(ctx, s2); err != nil {
		t.Fatal(err)
	}
	d.StrategyID = s2.ID
	if err := drepo.Upsert(ctx, d); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	got, err := drepo.ListByScope(ctx, models.ScopeOrg, orgID)
	if err != nil || len(got) != 1 || got[0].StrategyID != s2.ID {
		t.Fatalf("list: err=%v, got=%+v", err, got)
	}
}

func TestStrategyDefaultsRepo_WildcardKeyUniqueness(t *testing.T) {
	ctx := context.Background()
	db := testDB(t)
	srepo := NewStrategyRepo(db)
	drepo := NewStrategyDefaultsRepo(db)
	orgID := uuid.New()
	s := sampleStrategy(orgID)
	if err := srepo.Create(ctx, s); err != nil {
		t.Fatal(err)
	}
	// Two rows: (nil env, deploy target) and (nil env, nil target) must coexist.
	tt := models.TargetTypeDeploy
	if err := drepo.Upsert(ctx, &models.StrategyDefault{ScopeType: models.ScopeOrg, ScopeID: orgID, TargetType: &tt, StrategyID: s.ID}); err != nil {
		t.Fatal(err)
	}
	if err := drepo.Upsert(ctx, &models.StrategyDefault{ScopeType: models.ScopeOrg, ScopeID: orgID, StrategyID: s.ID}); err != nil {
		t.Fatal(err)
	}
	rows, err := drepo.ListByScope(ctx, models.ScopeOrg, orgID)
	if err != nil || len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d (err=%v)", len(rows), err)
	}
}

func TestRolloutPolicyRepo_Upsert(t *testing.T) {
	ctx := context.Background()
	db := testDB(t)
	repo := NewRolloutPolicyRepo(db)
	orgID := uuid.New()
	p := &models.RolloutPolicy{ScopeType: models.ScopeOrg, ScopeID: orgID, Enabled: true, Policy: models.PolicyPrompt}
	if err := repo.Upsert(ctx, p); err != nil {
		t.Fatal(err)
	}
	p.Policy = models.PolicyMandate
	if err := repo.Upsert(ctx, p); err != nil {
		t.Fatal(err)
	}
	rows, err := repo.ListByScope(ctx, models.ScopeOrg, orgID)
	if err != nil || len(rows) != 1 || rows[0].Policy != models.PolicyMandate {
		t.Fatalf("list: err=%v rows=%+v", err, rows)
	}
}
