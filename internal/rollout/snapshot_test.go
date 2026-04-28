package rollout

import (
	"testing"
	"time"

	"github.com/shadsorg/deploysentry/internal/models"
)

func baseTemplate() *models.Strategy {
	return &models.Strategy{
		Name:       "std",
		TargetType: models.TargetTypeDeploy,
		Steps: []models.Step{
			{Percent: 1, MinDuration: 5 * time.Minute, MaxDuration: 30 * time.Minute, BakeTimeHealthy: 5 * time.Minute},
			{Percent: 100},
		},
		DefaultHealthThreshold:   0.95,
		DefaultRollbackOnFailure: true,
	}
}

func TestBuildSnapshot_NoOverrides_ReturnsCopy(t *testing.T) {
	tmpl := baseTemplate()
	snap := BuildSnapshot(tmpl, nil)
	if snap.Name != tmpl.Name {
		t.Fatal("name not copied")
	}
	snap.Steps[0].Percent = 99
	if tmpl.Steps[0].Percent == 99 {
		t.Fatal("snapshot aliased template steps — needs deep copy")
	}
}

func TestBuildSnapshot_OverrideStepField(t *testing.T) {
	tmpl := baseTemplate()
	overrides := &StrategyOverrides{
		Steps: map[int]StepOverride{
			0: {MinDurationMs: ptrInt64(int64(10 * time.Minute / time.Millisecond))},
		},
	}
	snap := BuildSnapshot(tmpl, overrides)
	if snap.Steps[0].MinDuration != 10*time.Minute {
		t.Fatalf("expected override to 10m, got %s", snap.Steps[0].MinDuration)
	}
	if snap.Steps[0].MaxDuration != tmpl.Steps[0].MaxDuration {
		t.Fatal("non-overridden field lost")
	}
}

func TestBuildSnapshot_OverrideDefault(t *testing.T) {
	tmpl := baseTemplate()
	th := 0.99
	overrides := &StrategyOverrides{DefaultHealthThreshold: &th}
	snap := BuildSnapshot(tmpl, overrides)
	if snap.DefaultHealthThreshold != 0.99 {
		t.Fatalf("got %g", snap.DefaultHealthThreshold)
	}
}

func ptrInt64(v int64) *int64 { return &v }
