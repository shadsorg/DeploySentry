package settings

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/shadsorg/deploysentry/internal/staging"
)

// CommitTuple is the registry payload for staged-change commit handlers in
// the settings domain. Mirrors flags.CommitTuple to keep cmd/api/main.go's
// registration loop uniform across packages.
type CommitTuple struct {
	ResourceType string
	Action       string
	Handler      staging.CommitHandler
}

// SettingCommitHandlers returns the commit handlers the staging registry
// should register for the settings family.
//
// Wire up in cmd/api/main.go: for each tuple, call registry.Register(...).
func SettingCommitHandlers(svc SettingService) []CommitTuple {
	return []CommitTuple{
		{ResourceType: "setting", Action: "update", Handler: commitSettingUpdate(svc)},
		{ResourceType: "setting", Action: "delete", Handler: commitSettingDelete(svc)},
	}
}

// commitSettingUpdate persists the staged setting via SettingService.Set.
// The staged row's resource_id overrides the body's id so a malformed
// payload can't write to the wrong row.
func commitSettingUpdate(svc SettingService) staging.CommitHandler {
	return func(ctx context.Context, _ pgx.Tx, row *models.StagedChange) (string, error) {
		if row.ResourceID == nil {
			return "", fmt.Errorf("setting.update commit: resource_id required")
		}
		if len(row.NewValue) == 0 {
			return "", fmt.Errorf("setting.update commit: new_value required")
		}
		var setting models.Setting
		if err := json.Unmarshal(row.NewValue, &setting); err != nil {
			return "", fmt.Errorf("setting.update commit: parse new_value: %w", err)
		}
		setting.ID = *row.ResourceID
		if err := svc.Set(ctx, &setting); err != nil {
			return "", fmt.Errorf("setting.update commit: %w", err)
		}
		return "setting.updated", nil
	}
}

// commitSettingDelete removes the setting by id.
func commitSettingDelete(svc SettingService) staging.CommitHandler {
	return func(ctx context.Context, _ pgx.Tx, row *models.StagedChange) (string, error) {
		if row.ResourceID == nil {
			return "", fmt.Errorf("setting.delete commit: resource_id required")
		}
		if err := svc.Delete(ctx, *row.ResourceID); err != nil {
			return "", fmt.Errorf("setting.delete commit: %w", err)
		}
		return "setting.deleted", nil
	}
}
