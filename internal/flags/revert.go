package flags

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/shadsorg/deploysentry/internal/auth"
	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/google/uuid"
)

// RevertTuple is the data needed to register a revert handler.
type RevertTuple struct {
	EntityType string
	Action     string
	Handler    auth.RevertHandler
}

// FlagRevertHandlers returns the registry tuples for the flag family.
// Wire up in cmd/api/main.go: for each tuple, call registry.Register(t.EntityType, t.Action, t.Handler).
func FlagRevertHandlers(svc FlagService) []RevertTuple {
	return []RevertTuple{
		{"flag", "flag.archived", revertFlagArchived(svc)},
		{"flag", "flag.created", revertFlagCreated(svc)},
		{"flag", "flag.updated", revertFlagUpdated(svc)},
		{"flag", "flag.toggled", revertFlagToggled(svc)},
		{"flag", "flag.rule.created", revertFlagRuleCreated(svc)},
		{"flag", "flag.rule.deleted", revertFlagRuleDeleted(svc)},
		{"flag", "flag.rule.env_state.updated", revertFlagRuleEnvStateUpdated(svc)},
		{"flag", "flag.env_state.updated", revertFlagEnvStateUpdated(svc)},
	}
}

// revertFlagArchived undoes a flag.archived action by unarchiving the flag.
func revertFlagArchived(svc FlagService) auth.RevertHandler {
	return func(ctx context.Context, entry *models.AuditLogEntry, force bool) (string, error) {
		flag, err := svc.GetFlag(ctx, entry.EntityID)
		if err != nil {
			return "", fmt.Errorf("flag.archived revert: load flag: %w", err)
		}
		if !flag.Archived && !force {
			return "", auth.ErrRevertRace
		}
		if err := svc.UnarchiveFlag(ctx, entry.EntityID); err != nil {
			return "", fmt.Errorf("flag.archived revert: unarchive: %w", err)
		}
		return "flag.archived.reverted", nil
	}
}

// revertFlagCreated undoes a flag.created action by archiving the flag.
func revertFlagCreated(svc FlagService) auth.RevertHandler {
	return func(ctx context.Context, entry *models.AuditLogEntry, force bool) (string, error) {
		flag, err := svc.GetFlag(ctx, entry.EntityID)
		if err != nil {
			return "", fmt.Errorf("flag.created revert: load flag: %w", err)
		}
		if flag.Archived && !force {
			return "", auth.ErrRevertRace
		}
		if err := svc.ArchiveFlag(ctx, entry.EntityID); err != nil {
			return "", fmt.Errorf("flag.created revert: archive: %w", err)
		}
		return "flag.created.reverted", nil
	}
}

// revertFlagUpdated undoes a flag.updated action by restoring old field values.
// Race detection compares the new_value payload against the current flag state.
// If any field set in new_value has since changed, we detect a race.
func revertFlagUpdated(svc FlagService) auth.RevertHandler {
	return func(ctx context.Context, entry *models.AuditLogEntry, force bool) (string, error) {
		if entry.OldValue == "" || entry.NewValue == "" {
			return "", fmt.Errorf("flag.updated revert: malformed payload: old_value or new_value is empty")
		}

		var newPayload map[string]interface{}
		if err := json.Unmarshal([]byte(entry.NewValue), &newPayload); err != nil {
			return "", fmt.Errorf("flag.updated revert: malformed payload: %w", err)
		}

		current, err := svc.GetFlag(ctx, entry.EntityID)
		if err != nil {
			return "", fmt.Errorf("flag.updated revert: load flag: %w", err)
		}

		if !force {
			// Marshal current to a comparable map and check fields touched in new_value.
			currentJSON, err := json.Marshal(map[string]interface{}{
				"name":          current.Name,
				"description":   current.Description,
				"category":      string(current.Category),
				"purpose":       current.Purpose,
				"owners":        current.Owners,
				"is_permanent":  current.IsPermanent,
				"expires_at":    current.ExpiresAt,
				"default_value": current.DefaultValue,
				"tags":          current.Tags,
			})
			if err != nil {
				return "", fmt.Errorf("flag.updated revert: marshal current: %w", err)
			}
			var currentMap map[string]interface{}
			if err := json.Unmarshal(currentJSON, &currentMap); err != nil {
				return "", fmt.Errorf("flag.updated revert: marshal current: %w", err)
			}

			// Re-marshal and unmarshal newPayload through JSON to normalize values
			// (e.g. float64 numbers from JSON decode).
			for k, newV := range newPayload {
				curV, exists := currentMap[k]
				if !exists {
					continue
				}
				newNorm, _ := json.Marshal(newV)
				curNorm, _ := json.Marshal(curV)
				if string(newNorm) != string(curNorm) {
					return "", auth.ErrRevertRace
				}
			}
		}

		// Apply: re-marshal old payload through the FeatureFlag struct so typed fields
		// (notably expires_at *time.Time) round-trip correctly. We only restore fields
		// that handler.go's update path itself supports.
		var oldFlag models.FeatureFlag
		if err := json.Unmarshal([]byte(entry.OldValue), &oldFlag); err != nil {
			return "", fmt.Errorf("flag.updated revert: malformed old_value payload: %w", err)
		}
		// Preserve identifying fields from current (don't let revert change them).
		oldFlag.ID = current.ID
		oldFlag.ProjectID = current.ProjectID
		oldFlag.Key = current.Key
		oldFlag.CreatedAt = current.CreatedAt
		oldFlag.CreatedBy = current.CreatedBy

		if err := svc.UpdateFlag(ctx, &oldFlag); err != nil {
			return "", fmt.Errorf("flag.updated revert: update: %w", err)
		}
		return "flag.updated.reverted", nil
	}
}

// revertFlagToggled undoes a flag.toggled action by restoring the previous enabled state.
func revertFlagToggled(svc FlagService) auth.RevertHandler {
	return func(ctx context.Context, entry *models.AuditLogEntry, force bool) (string, error) {
		var newPayload struct {
			Enabled bool `json:"enabled"`
		}
		if err := json.Unmarshal([]byte(entry.NewValue), &newPayload); err != nil {
			return "", fmt.Errorf("flag.toggled revert: malformed payload: %w", err)
		}
		var oldPayload struct {
			Enabled bool `json:"enabled"`
		}
		if err := json.Unmarshal([]byte(entry.OldValue), &oldPayload); err != nil {
			return "", fmt.Errorf("flag.toggled revert: malformed payload: %w", err)
		}

		current, err := svc.GetFlag(ctx, entry.EntityID)
		if err != nil {
			return "", fmt.Errorf("flag.toggled revert: load flag: %w", err)
		}
		if current.Enabled != newPayload.Enabled && !force {
			return "", auth.ErrRevertRace
		}
		if err := svc.ToggleFlag(ctx, entry.EntityID, oldPayload.Enabled); err != nil {
			return "", fmt.Errorf("flag.toggled revert: toggle: %w", err)
		}
		return "flag.toggled.reverted", nil
	}
}

// revertFlagRuleCreated undoes a flag.rule.created action by deleting the rule.
// If the rule is already gone (not found), this is an idempotent success.
// Other errors (e.g., transient DB errors) propagate.
func revertFlagRuleCreated(svc FlagService) auth.RevertHandler {
	return func(ctx context.Context, entry *models.AuditLogEntry, force bool) (string, error) {
		var payload struct {
			RuleID uuid.UUID `json:"rule_id"`
		}
		if err := json.Unmarshal([]byte(entry.NewValue), &payload); err != nil {
			return "", fmt.Errorf("flag.rule.created revert: malformed payload: %w", err)
		}

		if _, err := svc.GetRule(ctx, payload.RuleID); err != nil {
			// Only treat "not found" as idempotent success; propagate other errors.
			// The postgres layer returns its ErrNotFound sentinel unwrapped,
			// so it survives the service-layer wrapping and we check for it here.
			if strings.Contains(err.Error(), "not found") {
				return "flag.rule.created.reverted", nil
			}
			return "", fmt.Errorf("flag.rule.created revert: load rule: %w", err)
		}
		// Rule still exists; delete it
		if err := svc.DeleteRule(ctx, payload.RuleID); err != nil {
			return "", fmt.Errorf("flag.rule.created revert: delete rule: %w", err)
		}
		return "flag.rule.created.reverted", nil
	}
}

// revertFlagRuleDeleted undoes a flag.rule.deleted action by re-creating the rule.
// Depends on Fix 1 in handler.go: old_value must be the full rule JSON.
func revertFlagRuleDeleted(svc FlagService) auth.RevertHandler {
	return func(ctx context.Context, entry *models.AuditLogEntry, force bool) (string, error) {
		var rule models.TargetingRule
		if err := json.Unmarshal([]byte(entry.OldValue), &rule); err != nil {
			return "", fmt.Errorf("flag.rule.deleted revert: malformed payload (audit predates fix): %w", err)
		}
		if rule.ID == uuid.Nil {
			return "", fmt.Errorf("flag.rule.deleted revert: malformed payload (audit predates fix): rule_id missing")
		}

		existing, err := svc.GetRule(ctx, rule.ID)
		if err != nil && !strings.Contains(err.Error(), "not found") {
			return "", fmt.Errorf("flag.rule.deleted revert: load rule: %w", err)
		}
		if existing != nil && !force {
			// Rule already exists — someone re-created it since the delete.
			return "", auth.ErrRevertRace
		}
		if err := svc.AddRule(ctx, &rule); err != nil {
			return "", fmt.Errorf("flag.rule.deleted revert: add rule: %w", err)
		}
		return "flag.rule.deleted.reverted", nil
	}
}

// revertFlagRuleEnvStateUpdated undoes a flag.rule.env_state.updated action.
// Depends on Fix 2 in handler.go: old_value must contain {rule_id, environment_id, enabled}.
func revertFlagRuleEnvStateUpdated(svc FlagService) auth.RevertHandler {
	return func(ctx context.Context, entry *models.AuditLogEntry, force bool) (string, error) {
		var newPayload struct {
			RuleID        uuid.UUID `json:"rule_id"`
			EnvironmentID uuid.UUID `json:"environment_id"`
			Enabled       bool      `json:"enabled"`
		}
		if err := json.Unmarshal([]byte(entry.NewValue), &newPayload); err != nil {
			return "", fmt.Errorf("flag.rule.env_state.updated revert: malformed new_value: %w", err)
		}
		var oldPayload struct {
			RuleID        uuid.UUID `json:"rule_id"`
			EnvironmentID uuid.UUID `json:"environment_id"`
			Enabled       bool      `json:"enabled"`
		}
		if err := json.Unmarshal([]byte(entry.OldValue), &oldPayload); err != nil {
			return "", fmt.Errorf("flag.rule.env_state.updated revert: malformed old_value (audit predates fix): %w", err)
		}

		// entry.EntityID is flagID (per handler.go line 1126).
		states, err := svc.ListRuleEnvironmentStates(ctx, entry.EntityID)
		if err != nil {
			return "", fmt.Errorf("flag.rule.env_state.updated revert: list states: %w", err)
		}

		if !force {
			var found *models.RuleEnvironmentState
			for _, s := range states {
				if s.RuleID == newPayload.RuleID && s.EnvironmentID == newPayload.EnvironmentID {
					found = s
					break
				}
			}
			if found != nil && found.Enabled != newPayload.Enabled {
				return "", auth.ErrRevertRace
			}
		}

		if _, err := svc.SetRuleEnvironmentState(ctx, newPayload.RuleID, newPayload.EnvironmentID, oldPayload.Enabled); err != nil {
			return "", fmt.Errorf("flag.rule.env_state.updated revert: set state: %w", err)
		}
		return "flag.rule.env_state.updated.reverted", nil
	}
}

// revertFlagEnvStateUpdated undoes a flag.env_state.updated action.
// Depends on Fix 3 in handler.go: old_value must contain {environment_id, enabled, value}.
func revertFlagEnvStateUpdated(svc FlagService) auth.RevertHandler {
	return func(ctx context.Context, entry *models.AuditLogEntry, force bool) (string, error) {
		var newPayload struct {
			EnvironmentID uuid.UUID        `json:"environment_id"`
			Enabled       bool             `json:"enabled"`
			Value         *json.RawMessage `json:"value"`
		}
		if err := json.Unmarshal([]byte(entry.NewValue), &newPayload); err != nil {
			return "", fmt.Errorf("flag.env_state.updated revert: malformed new_value: %w", err)
		}
		var oldPayload struct {
			EnvironmentID uuid.UUID        `json:"environment_id"`
			Enabled       bool             `json:"enabled"`
			Value         *json.RawMessage `json:"value"`
		}
		if err := json.Unmarshal([]byte(entry.OldValue), &oldPayload); err != nil {
			return "", fmt.Errorf("flag.env_state.updated revert: malformed old_value (audit predates fix): %w", err)
		}

		if !force {
			states, err := svc.ListFlagEnvStates(ctx, entry.EntityID)
			if err != nil {
				return "", fmt.Errorf("flag.env_state.updated revert: list states: %w", err)
			}
			for _, s := range states {
				if s.EnvironmentID == newPayload.EnvironmentID {
					currentVal, _ := json.Marshal(s.Value)
					newVal, _ := json.Marshal(newPayload.Value)
					if s.Enabled != newPayload.Enabled || string(currentVal) != string(newVal) {
						return "", auth.ErrRevertRace
					}
					break
				}
			}
		}

		state := &models.FlagEnvironmentState{
			FlagID:        entry.EntityID,
			EnvironmentID: oldPayload.EnvironmentID,
			Enabled:       oldPayload.Enabled,
			Value:         oldPayload.Value,
		}
		if err := svc.SetFlagEnvState(ctx, state); err != nil {
			return "", fmt.Errorf("flag.env_state.updated revert: set state: %w", err)
		}
		return "flag.env_state.updated.reverted", nil
	}
}
