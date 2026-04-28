package deploy

import (
	"context"
	"errors"
	"testing"

	"github.com/shadsorg/deploysentry/internal/health"
	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/google/uuid"
)

type stubTrafficSetter struct {
	called    bool
	lastPct   int
	returnErr error
}

func (s *stubTrafficSetter) SetTrafficPercent(ctx context.Context, deploymentID uuid.UUID, pct int) error {
	s.called = true
	s.lastPct = pct
	return s.returnErr
}

type stubHealth struct {
	score *health.DeploymentHealth
	err   error
}

func (s *stubHealth) GetHealth(id uuid.UUID) (*health.DeploymentHealth, error) {
	return s.score, s.err
}

func TestDeployApplicator_Apply_CallsSetTraffic(t *testing.T) {
	ts := &stubTrafficSetter{}
	app := NewApplicator(ts, &stubHealth{})
	depID := uuid.New()
	did := depID.String()
	ro := &models.Rollout{
		TargetType: models.TargetTypeDeploy,
		TargetRef:  models.RolloutTargetRef{DeploymentID: &did},
	}
	step := models.Step{Percent: 25}
	if err := app.Apply(context.Background(), ro, step); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !ts.called || ts.lastPct != 25 {
		t.Fatalf("expected SetTrafficPercent(25), got called=%v pct=%d", ts.called, ts.lastPct)
	}
}

func TestDeployApplicator_Apply_MissingDeploymentID(t *testing.T) {
	app := NewApplicator(&stubTrafficSetter{}, &stubHealth{})
	ro := &models.Rollout{TargetType: models.TargetTypeDeploy}
	if err := app.Apply(context.Background(), ro, models.Step{Percent: 10}); err == nil {
		t.Fatalf("expected error on missing deployment_id")
	}
}

func TestDeployApplicator_Revert_SetsZero(t *testing.T) {
	ts := &stubTrafficSetter{}
	app := NewApplicator(ts, &stubHealth{})
	did := uuid.NewString()
	ro := &models.Rollout{TargetType: models.TargetTypeDeploy, TargetRef: models.RolloutTargetRef{DeploymentID: &did}}
	if err := app.Revert(context.Background(), ro); err != nil {
		t.Fatal(err)
	}
	if ts.lastPct != 0 {
		t.Fatalf("expected revert to 0, got %d", ts.lastPct)
	}
}

func TestDeployApplicator_CurrentSignal_MapsHealth(t *testing.T) {
	h := &stubHealth{score: &health.DeploymentHealth{
		Overall: 0.98,
		Healthy: true,
	}}
	app := NewApplicator(&stubTrafficSetter{}, h)
	did := uuid.NewString()
	ro := &models.Rollout{TargetType: models.TargetTypeDeploy, TargetRef: models.RolloutTargetRef{DeploymentID: &did}}
	s, err := app.CurrentSignal(context.Background(), ro, nil)
	if err != nil {
		t.Fatal(err)
	}
	if s.Score != 0.98 || s.ErrorRate != 0 || s.LatencyP99Ms != 0 {
		t.Fatalf("mapping wrong: %+v", s)
	}
}

func TestDeployApplicator_CurrentSignal_MapsMetrics(t *testing.T) {
	h := &stubHealth{score: &health.DeploymentHealth{
		Overall: 0.98,
		Metrics: map[string]float64{
			"error_rate":     0.012,
			"latency_p99_ms": 145,
		},
	}}
	app := NewApplicator(&stubTrafficSetter{}, h)
	did := uuid.NewString()
	ro := &models.Rollout{TargetType: models.TargetTypeDeploy, TargetRef: models.RolloutTargetRef{DeploymentID: &did}}
	s, err := app.CurrentSignal(context.Background(), ro, nil)
	if err != nil {
		t.Fatal(err)
	}
	if s.Score != 0.98 || s.ErrorRate != 0.012 || s.LatencyP99Ms != 145 {
		t.Fatalf("wrong mapping: %+v", s)
	}
}

func TestDeployApplicator_CurrentSignal_ErrPropagates(t *testing.T) {
	h := &stubHealth{err: errors.New("x")}
	app := NewApplicator(&stubTrafficSetter{}, h)
	did := uuid.NewString()
	ro := &models.Rollout{TargetType: models.TargetTypeDeploy, TargetRef: models.RolloutTargetRef{DeploymentID: &did}}
	if _, err := app.CurrentSignal(context.Background(), ro, nil); err == nil {
		t.Fatalf("expected error")
	}
}
