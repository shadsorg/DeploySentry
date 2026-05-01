package members

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/shadsorg/deploysentry/internal/staging"
)

// CommitTuple mirrors flags.CommitTuple so cmd/api/main.go can iterate
// uniformly when registering staging commit handlers.
type CommitTuple struct {
	ResourceType string
	Action       string
	Handler      staging.CommitHandler
}

// MemberCommitHandlers returns the commit handlers the staging registry
// should register for the members family. Per spec only role changes are
// stageable — invites and removals stay immediate.
func MemberCommitHandlers(svc Service) []CommitTuple {
	return []CommitTuple{
		{ResourceType: "member", Action: "role_changed", Handler: commitMemberRoleChanged(svc)},
	}
}

// rolePayload is the JSON shape stored in NewValue for a staged role
// change. The org_id is read from row.OrgID (every staged row carries
// it), the user_id from row.ResourceID. Only the new role lives here.
type rolePayload struct {
	Role string `json:"role"`
}

// commitMemberRoleChanged dispatches a staged member-role update. Spec:
// member.added invites and member.removed are out of scope (invites send
// email immediately; removals are destructive enough to bypass staging).
func commitMemberRoleChanged(svc Service) staging.CommitHandler {
	return func(ctx context.Context, _ pgx.Tx, row *models.StagedChange) (string, error) {
		if row.ResourceID == nil {
			return "", fmt.Errorf("member.role_changed commit: resource_id (user id) required")
		}
		if len(row.NewValue) == 0 {
			return "", fmt.Errorf("member.role_changed commit: new_value required")
		}
		var payload rolePayload
		if err := json.Unmarshal(row.NewValue, &payload); err != nil {
			return "", fmt.Errorf("member.role_changed commit: parse new_value: %w", err)
		}
		if payload.Role == "" {
			return "", fmt.Errorf("member.role_changed commit: role required")
		}
		if err := svc.UpdateOrgMemberRole(ctx, row.OrgID, *row.ResourceID, models.OrgRole(payload.Role)); err != nil {
			return "", fmt.Errorf("member.role_changed commit: %w", err)
		}
		return "member.role_changed", nil
	}
}
