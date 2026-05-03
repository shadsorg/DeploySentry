package flags

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/shadsorg/deploysentry/internal/staging"
)

// CommitTuple is the registry payload for staged-change commit handlers in
// the flag domain. Mirrors RevertTuple.
type CommitTuple struct {
	ResourceType string
	Action       string
	Handler      staging.CommitHandler
}

// FlagCommitHandlers returns the commit handlers the staging registry should
// register for the flag family. Phase A shipped `toggle`; Phase C-1 added
// `update`, `archive`, and `restore`; Phase C-2 adds the targeting-rule
// (update / delete) and per-environment-state (rule + flag) actions.
//
// `create` actions (`flag.create`, `flag_rule.create`) are intentionally
// absent until provisional-id resolution lands — a staged create has no
// resource_id, only a provisional_id, and other staged rows in the same
// batch may reference that placeholder.
//
// Wire up in cmd/api/main.go: for each tuple, call registry.Register(...).
func FlagCommitHandlers(svc FlagService) []CommitTuple {
	return []CommitTuple{
		{ResourceType: "flag", Action: "toggle", Handler: commitFlagToggle(svc)},
		{ResourceType: "flag", Action: "update", Handler: commitFlagUpdate(svc)},
		{ResourceType: "flag", Action: "archive", Handler: commitFlagArchive(svc)},
		{ResourceType: "flag", Action: "restore", Handler: commitFlagRestore(svc)},
		{ResourceType: "flag_rule", Action: "update", Handler: commitFlagRuleUpdate(svc)},
		{ResourceType: "flag_rule", Action: "delete", Handler: commitFlagRuleDelete(svc)},
		{ResourceType: "flag_rule_env_state", Action: "update", Handler: commitFlagRuleEnvStateUpdate(svc)},
		{ResourceType: "flag_env_state", Action: "update", Handler: commitFlagEnvStateUpdate(svc)},
	}
}

// togglePayload is the JSON shape stored in StagedChange.NewValue for a
// staged flag.toggle. Matches the toggle endpoint's request body.
type togglePayload struct {
	Enabled bool `json:"enabled"`
}

// commitFlagUpdate applies a staged whole-row update by replacing the flag
// with the body in NewValue. The resource_id from the staged row overrides
// the body's id so a malformed/mismatched payload can't write to the wrong
// row.
func commitFlagUpdate(svc FlagService) staging.CommitHandler {
	return func(ctx context.Context, _ pgx.Tx, row *models.StagedChange) (string, error) {
		if row.ResourceID == nil {
			return "", fmt.Errorf("flag.update commit: resource_id required")
		}
		if len(row.NewValue) == 0 {
			return "", fmt.Errorf("flag.update commit: new_value required")
		}
		var flag models.FeatureFlag
		if err := json.Unmarshal(row.NewValue, &flag); err != nil {
			return "", fmt.Errorf("flag.update commit: parse new_value: %w", err)
		}
		flag.ID = *row.ResourceID
		if err := svc.UpdateFlag(ctx, &flag); err != nil {
			return "", fmt.Errorf("flag.update commit: %w", err)
		}
		return "flag.updated", nil
	}
}

// commitFlagArchive applies a staged archive. The action carries no
// payload; the handler only needs the resource id.
func commitFlagArchive(svc FlagService) staging.CommitHandler {
	return func(ctx context.Context, _ pgx.Tx, row *models.StagedChange) (string, error) {
		if row.ResourceID == nil {
			return "", fmt.Errorf("flag.archive commit: resource_id required")
		}
		if err := svc.ArchiveFlag(ctx, *row.ResourceID); err != nil {
			return "", fmt.Errorf("flag.archive commit: %w", err)
		}
		return "flag.archived", nil
	}
}

// commitFlagRestore reverses an archive (clears archived_at + delete_after).
// Restoring an already-active flag is a no-op at the service layer.
func commitFlagRestore(svc FlagService) staging.CommitHandler {
	return func(ctx context.Context, _ pgx.Tx, row *models.StagedChange) (string, error) {
		if row.ResourceID == nil {
			return "", fmt.Errorf("flag.restore commit: resource_id required")
		}
		if err := svc.RestoreFlag(ctx, *row.ResourceID); err != nil {
			return "", fmt.Errorf("flag.restore commit: %w", err)
		}
		return "flag.restored", nil
	}
}

// commitFlagRuleUpdate applies a staged whole-row update to a targeting
// rule. The staged new_value is the full TargetingRule JSON. Like
// commitFlagUpdate, the row's resource_id overrides any id in the body.
func commitFlagRuleUpdate(svc FlagService) staging.CommitHandler {
	return func(ctx context.Context, _ pgx.Tx, row *models.StagedChange) (string, error) {
		if row.ResourceID == nil {
			return "", fmt.Errorf("flag_rule.update commit: resource_id required")
		}
		if len(row.NewValue) == 0 {
			return "", fmt.Errorf("flag_rule.update commit: new_value required")
		}
		var rule models.TargetingRule
		if err := json.Unmarshal(row.NewValue, &rule); err != nil {
			return "", fmt.Errorf("flag_rule.update commit: parse new_value: %w", err)
		}
		rule.ID = *row.ResourceID
		if err := svc.UpdateRule(ctx, &rule); err != nil {
			return "", fmt.Errorf("flag_rule.update commit: %w", err)
		}
		return "flag.rule.updated", nil
	}
}

// commitFlagRuleDelete removes a targeting rule by id.
func commitFlagRuleDelete(svc FlagService) staging.CommitHandler {
	return func(ctx context.Context, _ pgx.Tx, row *models.StagedChange) (string, error) {
		if row.ResourceID == nil {
			return "", fmt.Errorf("flag_rule.delete commit: resource_id required")
		}
		if err := svc.DeleteRule(ctx, *row.ResourceID); err != nil {
			return "", fmt.Errorf("flag_rule.delete commit: %w", err)
		}
		return "flag.rule.deleted", nil
	}
}

// ruleEnvStatePayload is the JSON shape stored in NewValue for a staged
// per-rule per-env state update. The staged row's resource_id is the rule
// id; environment_id and enabled live in the payload.
type ruleEnvStatePayload struct {
	EnvironmentID uuid.UUID `json:"environment_id"`
	Enabled       bool      `json:"enabled"`
}

// commitFlagRuleEnvStateUpdate flips per-environment activation for a
// targeting rule. The staged row's resource_id is the rule id.
func commitFlagRuleEnvStateUpdate(svc FlagService) staging.CommitHandler {
	return func(ctx context.Context, _ pgx.Tx, row *models.StagedChange) (string, error) {
		if row.ResourceID == nil {
			return "", fmt.Errorf("flag_rule_env_state.update commit: resource_id (rule id) required")
		}
		if len(row.NewValue) == 0 {
			return "", fmt.Errorf("flag_rule_env_state.update commit: new_value required")
		}
		var payload ruleEnvStatePayload
		if err := json.Unmarshal(row.NewValue, &payload); err != nil {
			return "", fmt.Errorf("flag_rule_env_state.update commit: parse new_value: %w", err)
		}
		if payload.EnvironmentID == uuid.Nil {
			return "", fmt.Errorf("flag_rule_env_state.update commit: environment_id required")
		}
		if _, err := svc.SetRuleEnvironmentState(ctx, *row.ResourceID, payload.EnvironmentID, payload.Enabled); err != nil {
			return "", fmt.Errorf("flag_rule_env_state.update commit: %w", err)
		}
		return "flag.rule.env_state.updated", nil
	}
}

// commitFlagEnvStateUpdate persists a per-environment flag state — the
// flag's enabled/value for a single environment. The staged row's
// resource_id is the flag id; environment_id and the new state live in
// the payload (the FlagEnvironmentState shape).
func commitFlagEnvStateUpdate(svc FlagService) staging.CommitHandler {
	return func(ctx context.Context, _ pgx.Tx, row *models.StagedChange) (string, error) {
		if row.ResourceID == nil {
			return "", fmt.Errorf("flag_env_state.update commit: resource_id (flag id) required")
		}
		if len(row.NewValue) == 0 {
			return "", fmt.Errorf("flag_env_state.update commit: new_value required")
		}
		var state models.FlagEnvironmentState
		if err := json.Unmarshal(row.NewValue, &state); err != nil {
			return "", fmt.Errorf("flag_env_state.update commit: parse new_value: %w", err)
		}
		state.FlagID = *row.ResourceID
		if state.EnvironmentID == uuid.Nil {
			return "", fmt.Errorf("flag_env_state.update commit: environment_id required")
		}
		if err := svc.SetFlagEnvState(ctx, &state); err != nil {
			return "", fmt.Errorf("flag_env_state.update commit: %w", err)
		}
		return "flag.env_state.updated", nil
	}
}

// CreateTuple is the registry payload for staged-create commit handlers in
// the flag domain. Mirrors CommitTuple. Mid-batch errors during commit roll
// back the whole tx (provisional resolution requires real atomicity).
type CreateTuple struct {
	ResourceType string
	Action       string
	Handler      staging.CreateHandler
}

// FlagCreateHandlers returns the staging create handlers for the flag
// family. Wired in cmd/api/main.go alongside FlagCommitHandlers.
func FlagCreateHandlers(svc FlagService) []CreateTuple {
	return []CreateTuple{
		{ResourceType: "flag", Action: "create", Handler: commitFlagCreate(svc)},
		{ResourceType: "flag_rule", Action: "create", Handler: commitFlagRuleCreate(svc)},
	}
}

// commitFlagCreate persists a staged flag.create through the supplied tx.
// The provisional id stays out of production; the real id is returned to
// Service.Commit via the staging.CreateHandler contract for cross-row
// resolution. Cache invalidation + event publish are deferred to the
// post-commit hook so they only fire if tx.Commit succeeds.
func commitFlagCreate(svc FlagService) staging.CreateHandler {
	return func(ctx context.Context, tx pgx.Tx, row *models.StagedChange) (uuid.UUID, string, func(context.Context), error) {
		if len(row.NewValue) == 0 {
			return uuid.Nil, "", nil, fmt.Errorf("flag.create commit: new_value required")
		}
		var flag models.FeatureFlag
		if err := json.Unmarshal(row.NewValue, &flag); err != nil {
			return uuid.Nil, "", nil, fmt.Errorf("flag.create commit: parse new_value: %w", err)
		}
		flag.ID = uuid.Nil // force fresh id; staging owns provisional, never the real
		realID, err := svc.CreateFlagTx(ctx, tx, &flag)
		if err != nil {
			return uuid.Nil, "", nil, fmt.Errorf("flag.create commit: %w", err)
		}
		// Snapshot for the hook so the closure doesn't reference any state
		// that could be rewritten by a later resolver pass on this row.
		snapshot := flag
		hook := func(hookCtx context.Context) {
			svc.PublishCreated(hookCtx, &snapshot)
		}
		return realID, "flag.created", hook, nil
	}
}

// commitFlagRuleCreate is filled in by Task 8 — leave a stub so
// FlagCreateHandlers compiles without forward-referencing it.
// Do not remove: the stub keeps both tuples registrable now without making
// Task 8 a forced prerequisite.
func commitFlagRuleCreate(svc FlagService) staging.CreateHandler {
	return func(ctx context.Context, tx pgx.Tx, row *models.StagedChange) (uuid.UUID, string, func(context.Context), error) {
		return uuid.Nil, "", nil, fmt.Errorf("flag_rule.create commit: not implemented (Task 8)")
	}
}

func commitFlagToggle(svc FlagService) staging.CommitHandler {
	return func(ctx context.Context, _ pgx.Tx, row *models.StagedChange) (string, error) {
		// Phase A note: flag.toggle commit dispatches through the existing
		// FlagService method, which manages cache invalidation, NATS
		// broadcast, and audit semantics independently of the supplied tx.
		// Phase B/C will introduce tx-aware variants when commit-time
		// atomicity becomes load-bearing (e.g., when a single Deploy
		// touches both a flag toggle and its rule activation).
		if row.ResourceID == nil {
			return "", fmt.Errorf("flag.toggle commit: resource_id required")
		}
		var payload togglePayload
		if len(row.NewValue) == 0 {
			return "", fmt.Errorf("flag.toggle commit: new_value required")
		}
		if err := json.Unmarshal(row.NewValue, &payload); err != nil {
			return "", fmt.Errorf("flag.toggle commit: parse new_value: %w", err)
		}
		if err := svc.ToggleFlag(ctx, *row.ResourceID, payload.Enabled); err != nil {
			return "", fmt.Errorf("flag.toggle commit: %w", err)
		}
		return "flag.toggled", nil
	}
}

