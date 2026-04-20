// Package config implements the config-target Applicator: bridges rollout phase
// progression to flag targeting-rule percentage updates.
package config

import (
	"context"
	"errors"
	"math"

	"github.com/deploysentry/deploysentry/internal/health"
	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/deploysentry/deploysentry/internal/rollout/applicator"
	"github.com/google/uuid"
)

// RuleUpdater is the subset of the flag service the applicator depends on.
// Implemented by a small adapter in cmd/api wiring that calls
// flags.FlagService.UpdateRule with the new percentage.
type RuleUpdater interface {
	UpdateRulePercentage(ctx context.Context, ruleID uuid.UUID, percentage int) error
}

// RuleFlagResolver resolves a rule → its flag (for app_id + env_id lookup).
type RuleFlagResolver interface {
	GetRule(ctx context.Context, ruleID uuid.UUID) (*models.TargetingRule, error)
	GetFlag(ctx context.Context, flagID uuid.UUID) (*models.FeatureFlag, error)
}

// HealthLookup reads health for a deployment. Optional.
type HealthLookup interface {
	GetHealth(deploymentID uuid.UUID) (*health.DeploymentHealth, error)
}

// DeploymentFinder locates the current (most recent active/succeeded)
// deployment for an (application, environment). Optional.
type DeploymentFinder interface {
	CurrentDeploymentFor(ctx context.Context, appID, envID uuid.UUID) (*models.Deployment, error)
}

// Applicator implements applicator.Applicator for config targets.
type Applicator struct {
	updater RuleUpdater
	rules   RuleFlagResolver
	health  HealthLookup
	finder  DeploymentFinder
}

// NewApplicator builds a config applicator. The rf, health, and finder
// arguments are optional (nil is accepted); when any is nil the applicator
// falls back to a constant healthy score for CurrentSignal.
func NewApplicator(u RuleUpdater, rf RuleFlagResolver, health HealthLookup, finder DeploymentFinder) *Applicator {
	return &Applicator{updater: u, rules: rf, health: health, finder: finder}
}

var _ applicator.Applicator = (*Applicator)(nil)

// ErrMissingRuleID is returned when a config rollout lacks a rule_id.
var ErrMissingRuleID = errors.New("config rollout missing rule_id in target_ref")

func (a *Applicator) ruleID(ro *models.Rollout) (uuid.UUID, error) {
	if ro.TargetRef.RuleID == nil {
		return uuid.Nil, ErrMissingRuleID
	}
	return uuid.Parse(*ro.TargetRef.RuleID)
}

// Apply sets the targeting rule's percentage to step.Percent (rounded to int 0-100).
func (a *Applicator) Apply(ctx context.Context, ro *models.Rollout, step models.Step) error {
	ruleID, err := a.ruleID(ro)
	if err != nil {
		return err
	}
	pct := int(math.Round(step.Percent))
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	return a.updater.UpdateRulePercentage(ctx, ruleID, pct)
}

// Revert restores the rule's previous percentage (from target_ref.PreviousPercentage)
// or sets to 0 if not captured.
func (a *Applicator) Revert(ctx context.Context, ro *models.Rollout) error {
	ruleID, err := a.ruleID(ro)
	if err != nil {
		return err
	}
	pct := 0
	if ro.TargetRef.PreviousPercentage != nil {
		pct = *ro.TargetRef.PreviousPercentage
	}
	return a.updater.UpdateRulePercentage(ctx, ruleID, pct)
}

// CurrentSignal returns a real health signal when the rule's flag can be
// resolved to an (app, env) pair with an active deployment being monitored.
// Falls back to a constant healthy score (1.0) on any lookup failure, which
// preserves forward progress when health infrastructure is unavailable.
func (a *Applicator) CurrentSignal(ctx context.Context, ro *models.Rollout, _ *models.SignalSource) (applicator.HealthScore, error) {
	// Fallback if any dependency is missing.
	if a.rules == nil || a.finder == nil || a.health == nil {
		return applicator.HealthScore{Score: 1.0}, nil
	}
	ruleID, err := a.ruleID(ro)
	if err != nil {
		return applicator.HealthScore{Score: 1.0}, nil
	}
	rule, err := a.rules.GetRule(ctx, ruleID)
	if err != nil {
		return applicator.HealthScore{Score: 1.0}, nil
	}
	flag, err := a.rules.GetFlag(ctx, rule.FlagID)
	if err != nil || flag.ApplicationID == nil || flag.EnvironmentID == nil {
		return applicator.HealthScore{Score: 1.0}, nil
	}
	dep, err := a.finder.CurrentDeploymentFor(ctx, *flag.ApplicationID, *flag.EnvironmentID)
	if err != nil || dep == nil {
		return applicator.HealthScore{Score: 1.0}, nil
	}
	h, err := a.health.GetHealth(dep.ID)
	if err != nil || h == nil {
		return applicator.HealthScore{Score: 1.0}, nil
	}
	score := applicator.HealthScore{Score: h.Overall}
	if h.Metrics != nil {
		score.ErrorRate = h.Metrics["error_rate"]
		score.LatencyP99Ms = h.Metrics["latency_p99_ms"]
		score.LatencyP50Ms = h.Metrics["latency_p50_ms"]
		score.RequestRate = h.Metrics["request_rate"]
	}
	return score, nil
}
