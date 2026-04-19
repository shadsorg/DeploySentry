// Package config implements the config-target Applicator: bridges rollout phase
// progression to flag targeting-rule percentage updates.
package config

import (
	"context"
	"errors"
	"math"

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

// Applicator implements applicator.Applicator for config targets.
type Applicator struct {
	updater RuleUpdater
}

// NewApplicator builds a config applicator.
func NewApplicator(u RuleUpdater) *Applicator { return &Applicator{updater: u} }

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

// CurrentSignal returns a constant healthy signal — config rollouts advance on
// time alone in Plan 3. Future work may wire an app+env health reader here.
func (a *Applicator) CurrentSignal(_ context.Context, _ *models.Rollout, _ *models.SignalSource) (applicator.HealthScore, error) {
	return applicator.HealthScore{Score: 1.0}, nil
}
