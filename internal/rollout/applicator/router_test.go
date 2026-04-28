package applicator

import (
	"context"
	"errors"
	"testing"

	"github.com/shadsorg/deploysentry/internal/models"
)

type stubInner struct {
	name   string
	calls  int
	err    error
	signal HealthScore
}

func (s *stubInner) Apply(_ context.Context, _ *models.Rollout, _ models.Step) error {
	s.calls++
	return s.err
}
func (s *stubInner) Revert(_ context.Context, _ *models.Rollout) error { s.calls++; return s.err }
func (s *stubInner) CurrentSignal(_ context.Context, _ *models.Rollout, _ *models.SignalSource) (HealthScore, error) {
	return s.signal, s.err
}

func TestRouter_Apply_DispatchesByTargetType(t *testing.T) {
	deployInner := &stubInner{name: "deploy"}
	configInner := &stubInner{name: "config"}
	r := NewRouter(deployInner, configInner)

	dep := &models.Rollout{TargetType: models.TargetTypeDeploy}
	cfg := &models.Rollout{TargetType: models.TargetTypeConfig}
	if err := r.Apply(context.Background(), dep, models.Step{Percent: 50}); err != nil {
		t.Fatal(err)
	}
	if err := r.Apply(context.Background(), cfg, models.Step{Percent: 75}); err != nil {
		t.Fatal(err)
	}
	if deployInner.calls != 1 {
		t.Fatalf("deploy called %d", deployInner.calls)
	}
	if configInner.calls != 1 {
		t.Fatalf("config called %d", configInner.calls)
	}
}

func TestRouter_UnknownTargetType_Errors(t *testing.T) {
	r := NewRouter(&stubInner{}, &stubInner{})
	ro := &models.Rollout{TargetType: models.TargetType("mystery")}
	if err := r.Apply(context.Background(), ro, models.Step{}); err == nil {
		t.Fatalf("expected error for unknown target_type")
	}
}

func TestRouter_NilInner_Errors(t *testing.T) {
	r := NewRouter(nil, &stubInner{})
	ro := &models.Rollout{TargetType: models.TargetTypeDeploy}
	if err := r.Apply(context.Background(), ro, models.Step{}); err == nil {
		t.Fatalf("expected error when deploy applicator not provided")
	}
}

func TestRouter_CurrentSignal_Dispatches(t *testing.T) {
	deployInner := &stubInner{signal: HealthScore{Score: 0.8}}
	configInner := &stubInner{signal: HealthScore{Score: 0.9}}
	r := NewRouter(deployInner, configInner)

	dep := &models.Rollout{TargetType: models.TargetTypeDeploy}
	cfg := &models.Rollout{TargetType: models.TargetTypeConfig}
	ds, _ := r.CurrentSignal(context.Background(), dep, nil)
	cs, _ := r.CurrentSignal(context.Background(), cfg, nil)
	if ds.Score != 0.8 || cs.Score != 0.9 {
		t.Fatalf("mis-dispatched signals: deploy=%g config=%g", ds.Score, cs.Score)
	}
}

func TestRouter_Errors_FromInner(t *testing.T) {
	deployInner := &stubInner{err: errors.New("x")}
	r := NewRouter(deployInner, &stubInner{})
	if err := r.Apply(context.Background(), &models.Rollout{TargetType: models.TargetTypeDeploy}, models.Step{}); err == nil {
		t.Fatalf("expected inner error to propagate")
	}
}
