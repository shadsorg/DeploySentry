package staging

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/shadsorg/deploysentry/internal/models"
)

// Resolver maps provisional UUIDs minted during staging to the real UUIDs
// produced by the create handlers at commit time. Used inside Service.Commit
// to rewrite ResourceID + new_value/old_value JSON references on every
// dependent row before its handler runs.
type Resolver struct {
	m map[uuid.UUID]uuid.UUID
}

func NewResolver() *Resolver { return &Resolver{m: map[uuid.UUID]uuid.UUID{}} }

// Bind records that a provisional id resolves to a real id. Both invariants
// are checked: source must be provisional; destination must not be.
func (r *Resolver) Bind(provisional, real uuid.UUID) {
	if !IsProvisional(provisional) {
		panic(fmt.Sprintf("staging.Resolver.Bind: source %s is not provisional", provisional))
	}
	if IsProvisional(real) {
		panic(fmt.Sprintf("staging.Resolver.Bind: destination %s is provisional", real))
	}
	r.m[provisional] = real
}

// Lookup returns the real id bound to a provisional id, plus an ok flag.
func (r *Resolver) Lookup(id uuid.UUID) (uuid.UUID, bool) {
	real, ok := r.m[id]
	return real, ok
}

// RewriteUUIDsInJSON walks raw JSON and substitutes any string value that
// parses as a UUID and is bound in the resolver with the real id's string.
// Non-UUID strings, numbers, nulls, bools, and structural tokens pass
// through. Returns the rewritten JSON; input is not mutated.
func (r *Resolver) RewriteUUIDsInJSON(raw []byte) ([]byte, error) {
	if len(raw) == 0 {
		return raw, nil
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, fmt.Errorf("RewriteUUIDsInJSON: parse: %w", err)
	}
	walked := r.walk(v)
	out, err := json.Marshal(walked)
	if err != nil {
		return nil, fmt.Errorf("RewriteUUIDsInJSON: marshal: %w", err)
	}
	return out, nil
}

func (r *Resolver) walk(v any) any {
	switch t := v.(type) {
	case string:
		if u, err := uuid.Parse(t); err == nil {
			if real, ok := r.m[u]; ok {
				return real.String()
			}
		}
		return t
	case []any:
		out := make([]any, len(t))
		for i, x := range t {
			out[i] = r.walk(x)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, x := range t {
			out[k] = r.walk(x)
		}
		return out
	default:
		return v
	}
}

// RewriteRow rewrites every resolvable provisional reference on a staged row:
// its ResourceID column (when provisional + bound) and its NewValue + OldValue
// JSON. Mutates row in place; returns error if JSON parse fails.
func (r *Resolver) RewriteRow(row *models.StagedChange) error {
	if row.ResourceID != nil && IsProvisional(*row.ResourceID) {
		if real, ok := r.m[*row.ResourceID]; ok {
			row.ResourceID = &real
		}
	}
	if len(row.NewValue) > 0 {
		out, err := r.RewriteUUIDsInJSON(row.NewValue)
		if err != nil {
			return fmt.Errorf("RewriteRow new_value: %w", err)
		}
		row.NewValue = out
	}
	if len(row.OldValue) > 0 {
		out, err := r.RewriteUUIDsInJSON(row.OldValue)
		if err != nil {
			return fmt.Errorf("RewriteRow old_value: %w", err)
		}
		row.OldValue = out
	}
	return nil
}
