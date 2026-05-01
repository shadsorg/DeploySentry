package rollout

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/shadsorg/deploysentry/internal/staging"
)

// CommitTuple is the registry payload for staged-change commit handlers in
// the rollout family. Mirrors flags.CommitTuple / settings.CommitTuple.
type CommitTuple struct {
	ResourceType string
	Action       string
	Handler      staging.CommitHandler
}

// RolloutCommitHandlers returns the commit handlers the staging registry
// should register for the rollout family: strategies, strategy defaults,
// and rollout policies.
//
// `strategy.create` is intentionally absent until provisional-id resolution
// lands — same deferral as `flag.create` and `flag_rule.create`.
//
// Wire up in cmd/api/main.go: for each tuple, call registry.Register(...).
func RolloutCommitHandlers(
	strategies *StrategyService,
	defaults *StrategyDefaultService,
	policies *RolloutPolicyService,
) []CommitTuple {
	return []CommitTuple{
		{ResourceType: "strategy", Action: "update", Handler: commitStrategyUpdate(strategies)},
		{ResourceType: "strategy", Action: "delete", Handler: commitStrategyDelete(strategies)},
		{ResourceType: "strategy_default", Action: "upsert", Handler: commitStrategyDefaultUpsert(defaults)},
		{ResourceType: "strategy_default", Action: "delete", Handler: commitStrategyDefaultDelete(defaults)},
		{ResourceType: "rollout_policy", Action: "upsert", Handler: commitRolloutPolicyUpsert(policies)},
		{ResourceType: "rollout_policy", Action: "delete", Handler: commitRolloutPolicyDelete(policies)},
	}
}

// strategyUpdatePayload bundles the new strategy with the version the user
// staged against. Optimistic concurrency: if production has moved on by
// commit time, StrategyService.Update returns a version-mismatch error
// and the staging Service.Commit surfaces it as a per-row partial failure.
// That's the staging layer's natural cross-user conflict mechanism.
type strategyUpdatePayload struct {
	ExpectedVersion int             `json:"expected_version"`
	Strategy        models.Strategy `json:"strategy"`
}

func commitStrategyUpdate(svc *StrategyService) staging.CommitHandler {
	return func(ctx context.Context, _ pgx.Tx, row *models.StagedChange) (string, error) {
		if row.ResourceID == nil {
			return "", fmt.Errorf("strategy.update commit: resource_id required")
		}
		if len(row.NewValue) == 0 {
			return "", fmt.Errorf("strategy.update commit: new_value required")
		}
		var payload strategyUpdatePayload
		if err := json.Unmarshal(row.NewValue, &payload); err != nil {
			return "", fmt.Errorf("strategy.update commit: parse new_value: %w", err)
		}
		payload.Strategy.ID = *row.ResourceID
		if err := svc.Update(ctx, &payload.Strategy, payload.ExpectedVersion); err != nil {
			return "", fmt.Errorf("strategy.update commit: %w", err)
		}
		return "strategy.updated", nil
	}
}

func commitStrategyDelete(svc *StrategyService) staging.CommitHandler {
	return func(ctx context.Context, _ pgx.Tx, row *models.StagedChange) (string, error) {
		if row.ResourceID == nil {
			return "", fmt.Errorf("strategy.delete commit: resource_id required")
		}
		if err := svc.Delete(ctx, *row.ResourceID); err != nil {
			return "", fmt.Errorf("strategy.delete commit: %w", err)
		}
		return "strategy.deleted", nil
	}
}

func commitStrategyDefaultUpsert(svc *StrategyDefaultService) staging.CommitHandler {
	return func(ctx context.Context, _ pgx.Tx, row *models.StagedChange) (string, error) {
		if row.ResourceID == nil {
			return "", fmt.Errorf("strategy_default.upsert commit: resource_id required")
		}
		if len(row.NewValue) == 0 {
			return "", fmt.Errorf("strategy_default.upsert commit: new_value required")
		}
		var def models.StrategyDefault
		if err := json.Unmarshal(row.NewValue, &def); err != nil {
			return "", fmt.Errorf("strategy_default.upsert commit: parse new_value: %w", err)
		}
		def.ID = *row.ResourceID
		if err := svc.Upsert(ctx, &def); err != nil {
			return "", fmt.Errorf("strategy_default.upsert commit: %w", err)
		}
		return "strategy_default.upserted", nil
	}
}

func commitStrategyDefaultDelete(svc *StrategyDefaultService) staging.CommitHandler {
	return func(ctx context.Context, _ pgx.Tx, row *models.StagedChange) (string, error) {
		if row.ResourceID == nil {
			return "", fmt.Errorf("strategy_default.delete commit: resource_id required")
		}
		if err := svc.Delete(ctx, *row.ResourceID); err != nil {
			return "", fmt.Errorf("strategy_default.delete commit: %w", err)
		}
		return "strategy_default.deleted", nil
	}
}

func commitRolloutPolicyUpsert(svc *RolloutPolicyService) staging.CommitHandler {
	return func(ctx context.Context, _ pgx.Tx, row *models.StagedChange) (string, error) {
		if row.ResourceID == nil {
			return "", fmt.Errorf("rollout_policy.upsert commit: resource_id required")
		}
		if len(row.NewValue) == 0 {
			return "", fmt.Errorf("rollout_policy.upsert commit: new_value required")
		}
		var policy models.RolloutPolicy
		if err := json.Unmarshal(row.NewValue, &policy); err != nil {
			return "", fmt.Errorf("rollout_policy.upsert commit: parse new_value: %w", err)
		}
		policy.ID = *row.ResourceID
		if err := svc.Upsert(ctx, &policy); err != nil {
			return "", fmt.Errorf("rollout_policy.upsert commit: %w", err)
		}
		return "rollout_policy.upserted", nil
	}
}

func commitRolloutPolicyDelete(svc *RolloutPolicyService) staging.CommitHandler {
	return func(ctx context.Context, _ pgx.Tx, row *models.StagedChange) (string, error) {
		if row.ResourceID == nil {
			return "", fmt.Errorf("rollout_policy.delete commit: resource_id required")
		}
		if err := svc.Delete(ctx, *row.ResourceID); err != nil {
			return "", fmt.Errorf("rollout_policy.delete commit: %w", err)
		}
		return "rollout_policy.deleted", nil
	}
}
