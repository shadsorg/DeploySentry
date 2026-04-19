package rollout

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
)

// Attacher composes strategy resolution + policy enforcement + rollout creation
// for a deploy. Called by the deploy handler's AttachFromDeployRequest method.
type Attacher struct {
	strategies *StrategyService
	defaults   *StrategyDefaultService
	policies   *RolloutPolicyService
	rollouts   *RolloutService
}

// NewAttacher builds an Attacher.
func NewAttacher(s *StrategyService, d *StrategyDefaultService, p *RolloutPolicyService, r *RolloutService) *Attacher {
	return &Attacher{strategies: s, defaults: d, policies: p, rollouts: r}
}

// AttachIntent carries the caller-supplied rollout info + scope context needed
// to resolve references and policy.
type AttachIntent struct {
	StrategyID   *uuid.UUID
	StrategyName string
	Overrides    json.RawMessage
	ReleaseID    *uuid.UUID
	Leaf         ScopeRef
	ProjectID    *uuid.UUID
	OrgID        *uuid.UUID
	Environment  *string
}

// ErrMandateWithoutStrategy is returned when policy=mandate is on and no strategy
// resolves (explicit or via defaults).
var ErrMandateWithoutStrategy = errors.New("rollout strategy required by scope policy, but none provided or resolved")

// AttachDeploy attaches a rollout to the given deployment.
func (a *Attacher) AttachDeploy(ctx context.Context, d *models.Deployment, intent *AttachIntent, actor uuid.UUID) error {
	target := models.TargetTypeDeploy

	// Resolve explicit strategy first.
	var tmpl *models.Strategy
	if intent.StrategyID != nil {
		got, err := a.strategies.Get(ctx, *intent.StrategyID)
		if err != nil {
			return fmt.Errorf("strategy not found by id: %w", err)
		}
		tmpl = got
	} else if intent.StrategyName != "" {
		// Walk ancestors to find the strategy by name.
		ancestors := AncestorScopes(intent.Leaf, intent.ProjectID, intent.OrgID)
		for _, anc := range ancestors {
			got, err := a.strategies.GetByName(ctx, anc.Type, anc.ID, intent.StrategyName)
			if err == nil && got != nil {
				tmpl = got
				break
			}
		}
		if tmpl == nil {
			return fmt.Errorf("strategy %q not found in scope ancestry", intent.StrategyName)
		}
	}

	// If no explicit, try default resolution.
	if tmpl == nil {
		def, err := a.defaults.Resolve(ctx, intent.Leaf, intent.ProjectID, intent.OrgID, intent.Environment, &target)
		if err == nil && def != nil {
			got, err := a.strategies.Get(ctx, def.StrategyID)
			if err == nil {
				tmpl = got
			}
		}
	}

	// Enforce policy.
	policy, _ := a.policies.Resolve(ctx, intent.Leaf, intent.ProjectID, intent.OrgID, intent.Environment, &target)
	if policy != nil && policy.Enabled && policy.Policy == models.PolicyMandate && tmpl == nil {
		return ErrMandateWithoutStrategy
	}
	if tmpl == nil {
		// No strategy and no mandate → no rollout; deploy proceeds via legacy path.
		return nil
	}

	// Build snapshot.
	var overrides *StrategyOverrides
	if len(intent.Overrides) > 0 {
		var o StrategyOverrides
		if err := json.Unmarshal(intent.Overrides, &o); err != nil {
			return fmt.Errorf("overrides invalid: %w", err)
		}
		overrides = &o
	}
	snap := BuildSnapshot(tmpl, overrides)

	// Create the rollout.
	_, err := a.rollouts.AttachDeploy(ctx, d.ID, snap, intent.ReleaseID, &actor)
	if err != nil {
		return err
	}
	return nil
}

// AttachConfig attaches a rollout to a targeting rule. previousPercentage is
// captured so Revert can restore the pre-rollout value.
func (a *Attacher) AttachConfig(ctx context.Context, ruleID uuid.UUID, previousPercentage int, intent *AttachIntent, actor uuid.UUID) error {
	target := models.TargetTypeConfig

	// Resolve explicit strategy first.
	var tmpl *models.Strategy
	if intent.StrategyID != nil {
		got, err := a.strategies.Get(ctx, *intent.StrategyID)
		if err != nil {
			return fmt.Errorf("strategy not found by id: %w", err)
		}
		tmpl = got
	} else if intent.StrategyName != "" {
		ancestors := AncestorScopes(intent.Leaf, intent.ProjectID, intent.OrgID)
		for _, anc := range ancestors {
			got, err := a.strategies.GetByName(ctx, anc.Type, anc.ID, intent.StrategyName)
			if err == nil && got != nil {
				tmpl = got
				break
			}
		}
		if tmpl == nil {
			return fmt.Errorf("strategy %q not found in scope ancestry", intent.StrategyName)
		}
	}

	if tmpl == nil {
		def, err := a.defaults.Resolve(ctx, intent.Leaf, intent.ProjectID, intent.OrgID, intent.Environment, &target)
		if err == nil && def != nil {
			got, err := a.strategies.Get(ctx, def.StrategyID)
			if err == nil {
				tmpl = got
			}
		}
	}

	policy, _ := a.policies.Resolve(ctx, intent.Leaf, intent.ProjectID, intent.OrgID, intent.Environment, &target)
	if policy != nil && policy.Enabled && policy.Policy == models.PolicyMandate && tmpl == nil {
		return ErrMandateWithoutStrategy
	}
	if tmpl == nil {
		return nil
	}

	var overrides *StrategyOverrides
	if len(intent.Overrides) > 0 {
		var o StrategyOverrides
		if err := json.Unmarshal(intent.Overrides, &o); err != nil {
			return fmt.Errorf("overrides invalid: %w", err)
		}
		overrides = &o
	}
	snap := BuildSnapshot(tmpl, overrides)

	_, err := a.rollouts.AttachConfig(ctx, ruleID, previousPercentage, snap, intent.ReleaseID, &actor)
	return err
}
