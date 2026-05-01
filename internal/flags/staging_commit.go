package flags

import (
	"context"
	"encoding/json"
	"fmt"

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
// register for the flag family. Phase A only ships flag.toggle; Phase C will
// extend this list to cover create/update/archive/restore/rule mutations.
//
// Wire up in cmd/api/main.go: for each tuple, call registry.Register(...).
func FlagCommitHandlers(svc FlagService) []CommitTuple {
	return []CommitTuple{
		{ResourceType: "flag", Action: "toggle", Handler: commitFlagToggle(svc)},
	}
}

// togglePayload is the JSON shape stored in StagedChange.NewValue for a
// staged flag.toggle. Matches the toggle endpoint's request body.
type togglePayload struct {
	Enabled bool `json:"enabled"`
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

