package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
)

func sampleRollout(t *testing.T) *models.Rollout {
	t.Helper()
	depID := uuid.NewString()
	strat := models.Strategy{
		ScopeType:  models.ScopeOrg,
		ScopeID:    uuid.New(),
		Name:       "test",
		TargetType: models.TargetTypeDeploy,
		Steps: []models.Step{
			{Percent: 1, MinDuration: time.Minute, MaxDuration: time.Minute, BakeTimeHealthy: 0},
			{Percent: 100, MinDuration: 0, MaxDuration: 0, BakeTimeHealthy: 0},
		},
		DefaultHealthThreshold:   0.95,
		DefaultRollbackOnFailure: true,
	}
	return &models.Rollout{
		ID:               uuid.New(),
		TargetType:       models.TargetTypeDeploy,
		TargetRef:        models.RolloutTargetRef{DeploymentID: &depID},
		StrategySnapshot: strat,
		SignalSource:     models.SignalSource{Kind: "app_env"},
		Status:           models.RolloutPending,
	}
}

func TestRolloutRepo_CreateGet(t *testing.T) {
	ctx := context.Background()
	repo := NewRolloutRepo(testDB(t))
	r := sampleRollout(t)
	if err := repo.Create(ctx, r); err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := repo.Get(ctx, r.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != models.RolloutPending || got.TargetType != models.TargetTypeDeploy {
		t.Fatalf("unexpected: %+v", got)
	}
	if got.StrategySnapshot.Name != "test" || len(got.StrategySnapshot.Steps) != 2 {
		t.Fatalf("snapshot mismatch: %+v", got.StrategySnapshot)
	}
}

func TestRolloutRepo_GetActiveByDeployment(t *testing.T) {
	ctx := context.Background()
	repo := NewRolloutRepo(testDB(t))
	r := sampleRollout(t)
	r.Status = models.RolloutActive
	if err := repo.Create(ctx, r); err != nil {
		t.Fatal(err)
	}
	depID, _ := uuid.Parse(*r.TargetRef.DeploymentID)
	got, err := repo.GetActiveByDeployment(ctx, depID)
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if got.ID != r.ID {
		t.Fatalf("expected %v got %v", r.ID, got.ID)
	}
}

func TestRolloutRepo_UpdateStatus(t *testing.T) {
	ctx := context.Background()
	repo := NewRolloutRepo(testDB(t))
	r := sampleRollout(t)
	if err := repo.Create(ctx, r); err != nil {
		t.Fatal(err)
	}
	reason := "manual"
	if err := repo.UpdateStatus(ctx, r.ID, models.RolloutRolledBack, &reason); err != nil {
		t.Fatal(err)
	}
	got, _ := repo.Get(ctx, r.ID)
	if got.Status != models.RolloutRolledBack || got.RollbackReason == nil || *got.RollbackReason != "manual" {
		t.Fatalf("status not updated: %+v", got)
	}
}

func TestRolloutPhaseRepo_BulkInsertList(t *testing.T) {
	ctx := context.Background()
	db := testDB(t)
	rrepo := NewRolloutRepo(db)
	prepo := NewRolloutPhaseRepo(db)
	r := sampleRollout(t)
	if err := rrepo.Create(ctx, r); err != nil {
		t.Fatal(err)
	}
	phases := []*models.RolloutPhase{
		{RolloutID: r.ID, PhaseIndex: 0, StepSnapshot: r.StrategySnapshot.Steps[0], Status: models.PhasePending},
		{RolloutID: r.ID, PhaseIndex: 1, StepSnapshot: r.StrategySnapshot.Steps[1], Status: models.PhasePending},
	}
	if err := prepo.BulkInsert(ctx, phases); err != nil {
		t.Fatal(err)
	}
	got, err := prepo.ListByRollout(ctx, r.ID)
	if err != nil || len(got) != 2 {
		t.Fatalf("list: err=%v got=%+v", err, got)
	}
}

func TestRolloutEventRepo_InsertList(t *testing.T) {
	ctx := context.Background()
	db := testDB(t)
	rrepo := NewRolloutRepo(db)
	erepo := NewRolloutEventRepo(db)
	r := sampleRollout(t)
	if err := rrepo.Create(ctx, r); err != nil {
		t.Fatal(err)
	}
	e := &models.RolloutEvent{
		RolloutID: r.ID,
		EventType: models.EventAttached,
		ActorType: "system",
		Payload:   map[string]interface{}{"source": "test"},
	}
	if err := erepo.Insert(ctx, e); err != nil {
		t.Fatal(err)
	}
	list, err := erepo.ListByRollout(ctx, r.ID, 10)
	if err != nil || len(list) != 1 || list[0].EventType != models.EventAttached {
		t.Fatalf("list: err=%v got=%+v", err, list)
	}
}
