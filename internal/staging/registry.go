package staging

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/shadsorg/deploysentry/internal/models"
)

// CommitHandler applies one staged row to its production table inside an open
// pgx transaction. It returns the audit-log action string to record on
// success (e.g. "flag.toggled"). Handlers MUST be transactional — they may
// only touch state through tx so the deploy commit can roll the whole batch
// back atomically.
type CommitHandler func(ctx context.Context, tx pgx.Tx, row *models.StagedChange) (auditAction string, err error)

// ErrNoCommitHandler is returned by Dispatch when no handler is registered
// for the (resource_type, action) pair on a row. Phase A intentionally
// registers only a small slice of resources; the rest fall through.
var ErrNoCommitHandler = errors.New("staging: no commit handler registered")

// CommitRegistry maps (resource_type, action) → CommitHandler. Mirrors
// internal/auth.RevertRegistry so reviewers can apply prior intuition here.
type CommitRegistry struct {
	handlers map[string]CommitHandler
}

// NewCommitRegistry builds an empty registry.
func NewCommitRegistry() *CommitRegistry {
	return &CommitRegistry{handlers: map[string]CommitHandler{}}
}

// Register installs h for the given (resource_type, action) pair. A second
// Register call for the same key overwrites the first — useful in tests.
func (r *CommitRegistry) Register(resourceType, action string, h CommitHandler) {
	r.handlers[commitKey(resourceType, action)] = h
}

// IsCommittable reports whether a handler is registered for the pair.
func (r *CommitRegistry) IsCommittable(resourceType, action string) bool {
	_, ok := r.handlers[commitKey(resourceType, action)]
	return ok
}

// Dispatch runs the handler for row. Returns ErrNoCommitHandler if no handler
// is registered. Caller is responsible for opening / committing tx.
func (r *CommitRegistry) Dispatch(ctx context.Context, tx pgx.Tx, row *models.StagedChange) (string, error) {
	h, ok := r.handlers[commitKey(row.ResourceType, row.Action)]
	if !ok {
		return "", fmt.Errorf("%w for %s.%s", ErrNoCommitHandler, row.ResourceType, row.Action)
	}
	return h(ctx, tx, row)
}

func commitKey(resourceType, action string) string { return resourceType + ":" + action }

// CreateHandler applies a staged CREATE row to its production table inside
// an open pgx transaction. It returns the real UUID minted for the new
// resource (so the staging service can bind provisional → real in the
// resolver), the audit-log action string, an optional post-commit hook for
// cache invalidation / event publish (run only after tx.Commit succeeds),
// and an error.
type CreateHandler func(ctx context.Context, tx pgx.Tx, row *models.StagedChange) (realID uuid.UUID, auditAction string, postCommit func(context.Context), err error)

// ErrNoCreateHandler is returned by CreateRegistry.Dispatch when no create
// handler is registered for the (resource_type, action) pair.
var ErrNoCreateHandler = errors.New("staging: no create handler registered")

// CreateRegistry maps (resource_type, action) → CreateHandler. Sits beside
// the existing CommitRegistry; Service.Commit dispatches to whichever fits
// based on row.ProvisionalID being non-nil.
type CreateRegistry struct {
	handlers map[string]CreateHandler
}

// NewCreateRegistry builds an empty registry.
func NewCreateRegistry() *CreateRegistry { return &CreateRegistry{handlers: map[string]CreateHandler{}} }

// Register installs h for the given (resource_type, action) pair.
func (r *CreateRegistry) Register(resourceType, action string, h CreateHandler) {
	r.handlers[commitKey(resourceType, action)] = h
}

// IsCreatable reports whether a create handler is registered for the pair.
func (r *CreateRegistry) IsCreatable(resourceType, action string) bool {
	_, ok := r.handlers[commitKey(resourceType, action)]
	return ok
}

// Dispatch runs the create handler for row.
func (r *CreateRegistry) Dispatch(ctx context.Context, tx pgx.Tx, row *models.StagedChange) (uuid.UUID, string, func(context.Context), error) {
	h, ok := r.handlers[commitKey(row.ResourceType, row.Action)]
	if !ok {
		return uuid.Nil, "", nil, fmt.Errorf("%w for %s.%s", ErrNoCreateHandler, row.ResourceType, row.Action)
	}
	return h(ctx, tx, row)
}
