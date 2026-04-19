package rollout

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
)

// Compile-time assertions: fakes must satisfy their interfaces.
var _ RolloutRepository = (*fakeRolloutRepo)(nil)
var _ RolloutEventRepository = (*fakeEventRepo)(nil)
var _ RolloutPhaseRepository = (*fakePhaseRepo)(nil)
var _ Publisher = (*fakePublisher)(nil)

// Fakes for the rollout repos, reusing stub patterns.

type fakeRolloutRepo struct{ rows map[uuid.UUID]*models.Rollout }

func newFakeRolloutRepo() *fakeRolloutRepo { return &fakeRolloutRepo{rows: map[uuid.UUID]*models.Rollout{}} }

func (f *fakeRolloutRepo) Create(_ context.Context, r *models.Rollout) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	f.rows[r.ID] = r
	return nil
}
func (f *fakeRolloutRepo) Get(_ context.Context, id uuid.UUID) (*models.Rollout, error) {
	if r, ok := f.rows[id]; ok {
		return r, nil
	}
	return nil, errors.New("not found")
}
func (f *fakeRolloutRepo) GetActiveByDeployment(_ context.Context, dep uuid.UUID) (*models.Rollout, error) {
	for _, r := range f.rows {
		if r.TargetRef.DeploymentID != nil && *r.TargetRef.DeploymentID == dep.String() && !r.IsTerminal() {
			return r, nil
		}
	}
	return nil, errors.New("not found")
}
func (f *fakeRolloutRepo) GetActiveByConfig(_ context.Context, _, _ string) (*models.Rollout, error) {
	return nil, errors.New("not found")
}
func (f *fakeRolloutRepo) GetActiveByRule(_ context.Context, ruleID uuid.UUID) (*models.Rollout, error) {
	for _, r := range f.rows {
		if r.TargetRef.RuleID != nil && *r.TargetRef.RuleID == ruleID.String() && !r.IsTerminal() {
			return r, nil
		}
	}
	return nil, errors.New("not found")
}
func (f *fakeRolloutRepo) List(_ context.Context, _ RolloutListOptions) ([]*models.Rollout, error) {
	return nil, nil
}
func (f *fakeRolloutRepo) UpdateStatus(_ context.Context, id uuid.UUID, st models.RolloutStatus, reason *string) error {
	if r, ok := f.rows[id]; ok {
		r.Status = st
		if reason != nil {
			r.RollbackReason = reason
		}
	}
	return nil
}
func (f *fakeRolloutRepo) UpdatePhasePointer(_ context.Context, _ uuid.UUID, _ int, _, _ *time.Time) error {
	return nil
}
func (f *fakeRolloutRepo) MarkCompleted(_ context.Context, _ uuid.UUID) error { return nil }
func (f *fakeRolloutRepo) ListByRelease(_ context.Context, relID uuid.UUID) ([]*models.Rollout, error) {
	var out []*models.Rollout
	for _, r := range f.rows {
		if r.ReleaseID != nil && *r.ReleaseID == relID {
			out = append(out, r)
		}
	}
	return out, nil
}
func (f *fakeRolloutRepo) SetReleaseID(_ context.Context, id uuid.UUID, relID *uuid.UUID) error {
	if r, ok := f.rows[id]; ok {
		r.ReleaseID = relID
		return nil
	}
	return errors.New("not found")
}

type fakeEventRepo struct{ events []*models.RolloutEvent }

func (f *fakeEventRepo) Insert(_ context.Context, e *models.RolloutEvent) error {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	f.events = append(f.events, e)
	return nil
}
func (f *fakeEventRepo) ListByRollout(_ context.Context, _ uuid.UUID, _ int) ([]*models.RolloutEvent, error) {
	return nil, nil
}

type fakePhaseRepo struct{}

func (f *fakePhaseRepo) BulkInsert(_ context.Context, _ []*models.RolloutPhase) error { return nil }
func (f *fakePhaseRepo) ListByRollout(_ context.Context, _ uuid.UUID) ([]*models.RolloutPhase, error) {
	return nil, nil
}
func (f *fakePhaseRepo) UpdateStatus(_ context.Context, _ uuid.UUID, _ models.PhaseStatus, _, _ *time.Time, _, _ *float64, _ string) error {
	return nil
}

type fakePublisher struct{ subjects []string }

func (p *fakePublisher) Publish(_ context.Context, subject string, _ []byte) error {
	p.subjects = append(p.subjects, subject)
	return nil
}

func newTestService() *RolloutService {
	return NewRolloutService(
		newFakeRolloutRepo(),
		&fakePhaseRepo{},
		&fakeEventRepo{},
		&fakePublisher{},
	)
}

// --- tests ---

func TestRolloutService_AttachDeploy_CreatesRollout(t *testing.T) {
	svc := newTestService()
	snap := &models.Strategy{Name: "t", TargetType: models.TargetTypeDeploy, Steps: []models.Step{{Percent: 100}}, DefaultHealthThreshold: 0.95}
	depID := uuid.New()
	ro, err := svc.AttachDeploy(context.Background(), depID, snap, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if ro.TargetType != models.TargetTypeDeploy || ro.TargetRef.DeploymentID == nil || *ro.TargetRef.DeploymentID != depID.String() {
		t.Fatalf("wrong rollout: %+v", ro)
	}
	if ro.Status != models.RolloutPending {
		t.Fatalf("expected pending, got %s", ro.Status)
	}
}

func TestRolloutService_Pause_Active(t *testing.T) {
	svc := newTestService()
	snap := &models.Strategy{Name: "t", TargetType: models.TargetTypeDeploy, Steps: []models.Step{{Percent: 100}}, DefaultHealthThreshold: 0.95}
	ro, _ := svc.AttachDeploy(context.Background(), uuid.New(), snap, nil, nil)
	ro.Status = models.RolloutActive
	if err := svc.Pause(context.Background(), ro.ID, uuid.New(), "manual"); err != nil {
		t.Fatalf("pause: %v", err)
	}
	if ro.Status != models.RolloutPaused {
		t.Fatalf("got %s", ro.Status)
	}
}

func TestRolloutService_ForcePromote_RequiresReason(t *testing.T) {
	svc := newTestService()
	snap := &models.Strategy{Name: "t", TargetType: models.TargetTypeDeploy, Steps: []models.Step{{Percent: 100}}, DefaultHealthThreshold: 0.95}
	ro, _ := svc.AttachDeploy(context.Background(), uuid.New(), snap, nil, nil)
	ro.Status = models.RolloutActive
	if err := svc.ForcePromote(context.Background(), ro.ID, uuid.New(), ""); err == nil {
		t.Fatalf("expected error on empty reason")
	}
	if err := svc.ForcePromote(context.Background(), ro.ID, uuid.New(), "operator override"); err != nil {
		t.Fatalf("force-promote: %v", err)
	}
}

func TestRolloutService_Rollback_RequiresReason(t *testing.T) {
	svc := newTestService()
	snap := &models.Strategy{Name: "t", TargetType: models.TargetTypeDeploy, Steps: []models.Step{{Percent: 100}}, DefaultHealthThreshold: 0.95}
	ro, _ := svc.AttachDeploy(context.Background(), uuid.New(), snap, nil, nil)
	ro.Status = models.RolloutActive
	if err := svc.Rollback(context.Background(), ro.ID, uuid.New(), ""); err == nil {
		t.Fatalf("expected error on empty reason")
	}
	if err := svc.Rollback(context.Background(), ro.ID, uuid.New(), "broken"); err != nil {
		t.Fatal(err)
	}
	if ro.Status != models.RolloutRolledBack {
		t.Fatalf("expected RolledBack, got %s", ro.Status)
	}
}

func TestRolloutService_StateMachine_PauseNonActive(t *testing.T) {
	svc := newTestService()
	snap := &models.Strategy{Name: "t", TargetType: models.TargetTypeDeploy, Steps: []models.Step{{Percent: 100}}, DefaultHealthThreshold: 0.95}
	ro, _ := svc.AttachDeploy(context.Background(), uuid.New(), snap, nil, nil)
	// Pending rollout should not be pausable.
	if err := svc.Pause(context.Background(), ro.ID, uuid.New(), ""); err == nil {
		t.Fatalf("expected state-machine error")
	}
}

func TestRolloutService_AttachConfig_CreatesRollout(t *testing.T) {
	svc := newTestService()
	snap := &models.Strategy{Name: "c", TargetType: models.TargetTypeConfig, Steps: []models.Step{{Percent: 100}}, DefaultHealthThreshold: 0.95}
	ruleID := uuid.New()
	ro, err := svc.AttachConfig(context.Background(), ruleID, 15, snap, nil, nil)
	if err != nil {
		t.Fatalf("attach: %v", err)
	}
	if ro.TargetType != models.TargetTypeConfig || ro.TargetRef.RuleID == nil || *ro.TargetRef.RuleID != ruleID.String() {
		t.Fatalf("wrong rollout: %+v", ro)
	}
	if ro.TargetRef.PreviousPercentage == nil || *ro.TargetRef.PreviousPercentage != 15 {
		t.Fatalf("expected previous_percentage=15, got %+v", ro.TargetRef.PreviousPercentage)
	}
	if ro.Status != models.RolloutPending {
		t.Fatalf("expected pending, got %s", ro.Status)
	}
}

func TestRolloutService_AttachConfig_RejectsIfRuleActive(t *testing.T) {
	svc := newTestService()
	snap := &models.Strategy{Name: "c", TargetType: models.TargetTypeConfig, Steps: []models.Step{{Percent: 100}}, DefaultHealthThreshold: 0.95}
	ruleID := uuid.New()
	if _, err := svc.AttachConfig(context.Background(), ruleID, 0, snap, nil, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AttachConfig(context.Background(), ruleID, 0, snap, nil, nil); err == nil {
		t.Fatalf("expected ErrAlreadyActiveOnTarget on second attach")
	}
}
