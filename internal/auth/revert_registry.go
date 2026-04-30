package auth

import (
	"context"
	"errors"

	"github.com/shadsorg/deploysentry/internal/models"
)

// ErrRevertRace is returned when the current resource state differs from the
// audit entry's NewValue, meaning a revert would clobber a newer change.
var ErrRevertRace = errors.New("resource modified since audit entry; revert would overwrite newer change")

// ErrNotRevertible is returned for actions with no registered handler.
var ErrNotRevertible = errors.New("action is not revertible")

// RevertHandler undoes the action recorded in entry. force=true bypasses race
// detection. On success it returns the action name to write into the new
// audit row (e.g. "flag.archived.reverted"). Handlers are registered per
// (entity_type, action) pair.
type RevertHandler func(ctx context.Context, entry *models.AuditLogEntry, force bool) (newAction string, err error)

// RevertRegistry maps (entity_type, action) → RevertHandler.
type RevertRegistry struct {
	handlers map[string]RevertHandler
}

func NewRevertRegistry() *RevertRegistry {
	return &RevertRegistry{handlers: map[string]RevertHandler{}}
}

func (r *RevertRegistry) Register(entityType, action string, h RevertHandler) {
	r.handlers[key(entityType, action)] = h
}

func (r *RevertRegistry) IsRevertible(entityType, action string) bool {
	_, ok := r.handlers[key(entityType, action)]
	return ok
}

// Revert dispatches the entry's revert handler. Returns the action name to
// record in the new audit row.
func (r *RevertRegistry) Revert(ctx context.Context, entry *models.AuditLogEntry, force bool) (string, error) {
	h, ok := r.handlers[key(entry.EntityType, entry.Action)]
	if !ok {
		return "", ErrNotRevertible
	}
	return h(ctx, entry, force)
}

func key(entityType, action string) string { return entityType + ":" + action }
