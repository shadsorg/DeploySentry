package engine

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/deploysentry/deploysentry/internal/rollout/applicator"
	"github.com/google/uuid"
)

// --- stubs ---

type stubApplicator struct {
	mu      sync.Mutex
	signal  applicator.HealthScore
	applied []float64
	errApply, errRevert error
}

func (s *stubApplicator) setSignal(sc applicator.HealthScore) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.signal = sc
}
func (s *stubApplicator) Apply(_ context.Context, _ *models.Rollout, step models.Step) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.applied = append(s.applied, step.Percent)
	return s.errApply
}
func (s *stubApplicator) Revert(_ context.Context, _ *models.Rollout) error { return s.errRevert }
func (s *stubApplicator) CurrentSignal(_ context.Context, _ *models.Rollout, _ *models.SignalSource) (applicator.HealthScore, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.signal, nil
}

type stubRepo struct {
	mu       sync.Mutex
	rollouts map[uuid.UUID]*models.Rollout
	phases   []*models.RolloutPhase
	events   []*models.RolloutEvent
}

func newStubRepo() *stubRepo { return &stubRepo{rollouts: map[uuid.UUID]*models.Rollout{}} }

// Implement the RepoSet interface used by Engine (defined in engine.go).
func (r *stubRepo) GetRollout(_ context.Context, id uuid.UUID) (*models.Rollout, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.rollouts[id], nil
}
func (r *stubRepo) UpdateRolloutStatus(_ context.Context, id uuid.UUID, st models.RolloutStatus, reason *string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if ro, ok := r.rollouts[id]; ok {
		ro.Status = st
		if reason != nil {
			ro.RollbackReason = reason
		}
	}
	return nil
}
func (r *stubRepo) UpdateRolloutPhasePointer(_ context.Context, id uuid.UUID, idx int, startedAt, lastHealthy *time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if ro, ok := r.rollouts[id]; ok {
		ro.CurrentPhaseIndex = idx
		ro.CurrentPhaseStartedAt = startedAt
		ro.LastHealthySince = lastHealthy
	}
	return nil
}
func (r *stubRepo) MarkRolloutCompleted(_ context.Context, id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if ro, ok := r.rollouts[id]; ok {
		now := time.Now().UTC()
		ro.Status = models.RolloutSucceeded
		ro.CompletedAt = &now
	}
	return nil
}
func (r *stubRepo) BulkInsertPhases(_ context.Context, phases []*models.RolloutPhase) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, p := range phases {
		if p.ID == uuid.Nil {
			p.ID = uuid.New()
		}
		r.phases = append(r.phases, p)
	}
	return nil
}
func (r *stubRepo) ListPhases(_ context.Context, rid uuid.UUID) ([]*models.RolloutPhase, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*models.RolloutPhase
	for _, p := range r.phases {
		if p.RolloutID == rid {
			out = append(out, p)
		}
	}
	return out, nil
}
func (r *stubRepo) UpdatePhaseStatus(_ context.Context, id uuid.UUID, st models.PhaseStatus, ea, xa *time.Time, ap, hs *float64, notes string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, p := range r.phases {
		if p.ID == id {
			p.Status = st
			if ea != nil {
				p.EnteredAt = ea
			}
			if xa != nil {
				p.ExitedAt = xa
			}
			if ap != nil {
				p.AppliedPercent = ap
			}
			if hs != nil {
				p.HealthScoreAtExit = hs
			}
			if notes != "" {
				p.Notes = notes
			}
		}
	}
	return nil
}
func (r *stubRepo) InsertEvent(_ context.Context, e *models.RolloutEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	r.events = append(r.events, e)
	return nil
}

type stubPublisher struct {
	mu   sync.Mutex
	sent []string
}

func (p *stubPublisher) Publish(_ context.Context, subject string, _ []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.sent = append(p.sent, subject)
	return nil
}

// --- tests ---

func makeRollout(steps []models.Step) *models.Rollout {
	return &models.Rollout{
		ID:         uuid.New(),
		TargetType: models.TargetTypeDeploy,
		TargetRef:  models.RolloutTargetRef{DeploymentID: ptrStr(uuid.NewString())},
		Status:     models.RolloutPending,
		StrategySnapshot: models.Strategy{
			Name:                     "t",
			TargetType:               models.TargetTypeDeploy,
			Steps:                    steps,
			DefaultHealthThreshold:   0.95,
			DefaultRollbackOnFailure: true,
		},
		SignalSource: models.SignalSource{Kind: "app_env"},
	}
}

func ptrStr(s string) *string { return &s }

// TestEngine_Drive_CompletesOnHealthy drives a 2-step healthy rollout and asserts
// both percents were applied and status is Succeeded.
func TestEngine_Drive_CompletesOnHealthy(t *testing.T) {
	repo := newStubRepo()
	app := &stubApplicator{signal: applicator.HealthScore{Score: 0.99}}
	eng := New(repo, app, &stubPublisher{}, EngineOptions{PollInterval: 10 * time.Millisecond, Now: func() time.Time { return time.Now() }})

	ro := makeRollout([]models.Step{
		{Percent: 50, MinDuration: 20 * time.Millisecond, MaxDuration: 200 * time.Millisecond, BakeTimeHealthy: 10 * time.Millisecond},
		{Percent: 100, MinDuration: 0, MaxDuration: 0, BakeTimeHealthy: 0},
	})
	repo.rollouts[ro.ID] = ro

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := eng.DriveRollout(ctx, ro.ID); err != nil {
		t.Fatalf("drive: %v", err)
	}
	if ro.Status != models.RolloutSucceeded {
		t.Fatalf("expected Succeeded, got %s", ro.Status)
	}
	if len(app.applied) != 2 || app.applied[0] != 50 || app.applied[1] != 100 {
		t.Fatalf("expected [50,100], got %v", app.applied)
	}
}

// TestEngine_Drive_RollsBackOnUnhealthyMax drives a rollout where health never
// reaches threshold, so max_duration expires and rollback fires.
func TestEngine_Drive_RollsBackOnUnhealthyMax(t *testing.T) {
	repo := newStubRepo()
	app := &stubApplicator{signal: applicator.HealthScore{Score: 0.5}}
	eng := New(repo, app, &stubPublisher{}, EngineOptions{PollInterval: 5 * time.Millisecond, Now: func() time.Time { return time.Now() }})

	ro := makeRollout([]models.Step{
		{Percent: 25, MinDuration: 10 * time.Millisecond, MaxDuration: 50 * time.Millisecond, BakeTimeHealthy: 20 * time.Millisecond},
		{Percent: 100},
	})
	repo.rollouts[ro.ID] = ro

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := eng.DriveRollout(ctx, ro.ID); err == nil {
		t.Fatalf("expected error indicating rollback")
	}
	if ro.Status != models.RolloutRolledBack {
		t.Fatalf("expected RolledBack, got %s", ro.Status)
	}
}

// TestEngine_Drive_AbortConditionTrips immediately rolls back when an abort
// condition is tripped, without waiting for max_duration.
func TestEngine_Drive_AbortConditionTrips(t *testing.T) {
	repo := newStubRepo()
	app := &stubApplicator{signal: applicator.HealthScore{Score: 0.99, ErrorRate: 0.10}}
	eng := New(repo, app, &stubPublisher{}, EngineOptions{PollInterval: 5 * time.Millisecond, Now: func() time.Time { return time.Now() }})

	ro := makeRollout([]models.Step{
		{
			Percent: 5, MinDuration: 5 * time.Millisecond, MaxDuration: 5 * time.Second, BakeTimeHealthy: 2 * time.Second,
			AbortConditions: []models.StepAbortCondition{{Metric: "error_rate", Operator: ">", Threshold: 0.02, Window: time.Second}},
		},
		{Percent: 100},
	})
	repo.rollouts[ro.ID] = ro

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	_ = eng.DriveRollout(ctx, ro.ID)
	if ro.Status != models.RolloutRolledBack {
		t.Fatalf("expected RolledBack, got %s", ro.Status)
	}
}
