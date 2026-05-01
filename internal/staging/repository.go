package staging

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/shadsorg/deploysentry/internal/models"
)

// Repository persists staged_changes rows. The Postgres implementation lives
// in internal/platform/database/postgres/staging.go.
type Repository interface {
	// Upsert stores row, replacing any prior staged change for the same
	// (user_id, org_id, resource_type, resource_id|provisional_id, field_path)
	// tuple — that's the spec's "latest-edit-wins" rule.
	Upsert(ctx context.Context, row *models.StagedChange) error

	// ListForUser returns the user's staged changes within an org, newest
	// first. Used by the review page and the header banner.
	ListForUser(ctx context.Context, userID, orgID uuid.UUID) ([]*models.StagedChange, error)

	// ListForResource returns staged rows visible to a single user that
	// target one resource type within an org. Used by the read-overlay
	// helpers.
	ListForResource(ctx context.Context, userID, orgID uuid.UUID, resourceType string) ([]*models.StagedChange, error)

	// GetByIDs returns the rows matching the supplied ids that belong to the
	// given (user_id, org_id). Used by the commit endpoint to refuse cross-
	// user / cross-org tampering before dispatch.
	GetByIDs(ctx context.Context, userID, orgID uuid.UUID, ids []uuid.UUID) ([]*models.StagedChange, error)

	// DeleteByIDsTx removes rows by id within an open transaction. The
	// commit endpoint calls this after each row's CommitHandler succeeds so
	// the staged + production writes ride the same transaction boundary.
	DeleteByIDsTx(ctx context.Context, tx pgx.Tx, userID, orgID uuid.UUID, ids []uuid.UUID) error

	// DeleteAllForUser removes every staged row owned by the user in the
	// given org. Backs the "Discard all" header action and the FK cascade
	// when a user is removed from the org.
	DeleteAllForUser(ctx context.Context, userID, orgID uuid.UUID) (int64, error)

	// DeleteOlderThan removes any staged rows whose created_at predates
	// cutoff. Returns the deleted count for sweeper logging.
	DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error)

	// CountForUser returns the total staged-row count for the (user, org)
	// pair. Used by the header banner to render "N pending".
	CountForUser(ctx context.Context, userID, orgID uuid.UUID) (int, error)
}
