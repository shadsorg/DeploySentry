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
// register for the flag family. Phase A shipped `toggle`; Phase C-1 adds
// `update`, `archive`, and `restore`. `create` is intentionally absent
// until provisional-id resolution lands (a staged create has no
// resource_id, only a provisional_id, and other staged rows in the same
// batch may reference that placeholder).
//
// Wire up in cmd/api/main.go: for each tuple, call registry.Register(...).
func FlagCommitHandlers(svc FlagService) []CommitTuple {
	return []CommitTuple{
		{ResourceType: "flag", Action: "toggle", Handler: commitFlagToggle(svc)},
		{ResourceType: "flag", Action: "update", Handler: commitFlagUpdate(svc)},
		{ResourceType: "flag", Action: "archive", Handler: commitFlagArchive(svc)},
		{ResourceType: "flag", Action: "restore", Handler: commitFlagRestore(svc)},
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

