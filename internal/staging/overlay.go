package staging

import (
	"time"

	"github.com/google/uuid"
	"github.com/shadsorg/deploysentry/internal/models"
)

// ApplyFunc applies a single staged row's update/delete/create to a typed
// production value. Implementations:
//
//   - For action='delete': return (zero, true) to signal removal from a list.
//   - For action='update'/'toggle' on an existing row: return (patched, false).
//   - For action='create' on a synthetic row built from row.NewValue: return
//     (synthesized, false). Callers handle creates separately via SyntheticFunc.
//
// Returning the zero value with drop=false leaves the entry unchanged in the
// list (e.g. when the staged row's field_path doesn't apply to this entry).
type ApplyFunc[T any] func(production T, staged *models.StagedChange) (patched T, drop bool)

// SyntheticFunc materialises a staged CREATE row into a typed production
// shape. Used by OverlayList to append staged creates to the result.
type SyntheticFunc[T any] func(staged *models.StagedChange) (T, bool)

// ResourceID extracts the production resource id from a typed value so the
// overlay knows which staged rows apply to which production row.
type ResourceID[T any] func(T) uuid.UUID

// OverlayList layers the user's staged_changes for one resource type over a
// freshly-loaded slice of production rows. Returns the patched slice with:
//   - production entries patched in place by matching staged updates
//   - production entries dropped when a staged delete targets them
//   - synthetic entries appended for staged creates
//
// staged is the slice of rows for this user/org/resource_type.
//
// Order of production rows is preserved; synthetic rows are appended at the
// end in the staged-row order.
func OverlayList[T any](
	production []T,
	staged []*models.StagedChange,
	id ResourceID[T],
	apply ApplyFunc[T],
	synth SyntheticFunc[T],
) []T {
	if len(staged) == 0 {
		return production
	}

	// Bucket staged rows by resource id so the per-production-row loop is O(1)
	// per row instead of O(N) re-scan of staged.
	updatesByID := make(map[uuid.UUID][]*models.StagedChange, len(staged))
	creates := make([]*models.StagedChange, 0)
	for _, s := range staged {
		switch s.Action {
		case "create":
			creates = append(creates, s)
		default:
			if s.ResourceID != nil {
				updatesByID[*s.ResourceID] = append(updatesByID[*s.ResourceID], s)
			}
		}
	}

	out := make([]T, 0, len(production)+len(creates))
	for _, p := range production {
		pid := id(p)
		patched := p
		dropped := false
		for _, s := range updatesByID[pid] {
			next, drop := apply(patched, s)
			if drop {
				dropped = true
				break
			}
			patched = next
		}
		if !dropped {
			out = append(out, patched)
		}
	}

	for _, s := range creates {
		if v, ok := synth(s); ok {
			out = append(out, v)
		}
	}

	return out
}

// OverlayDetail layers staged rows over a single production value. Used by
// detail handlers (e.g. GET /flags/:id). If a staged delete targets the
// resource, returns (zero, true) so the handler can return 404. Returns
// (patched, false) otherwise, even when no staged rows apply.
func OverlayDetail[T any](
	production T,
	staged []*models.StagedChange,
	id ResourceID[T],
	apply ApplyFunc[T],
) (T, bool) {
	pid := id(production)
	patched := production
	for _, s := range staged {
		if s.ResourceID == nil || *s.ResourceID != pid {
			continue
		}
		next, drop := apply(patched, s)
		if drop {
			var zero T
			return zero, true
		}
		patched = next
	}
	return patched, false
}

// Marker is the wire-format envelope attached to overlay-emitted resources
// that came from a staged change. Dashboard renders <StagedBadge> when
// present; SDK clients don't request the overlay so they never see it.
type Marker struct {
	ProvisionalID *uuid.UUID `json:"provisional_id,omitempty"`
	Action        string     `json:"action"`
	StagedAt      time.Time  `json:"staged_at"`
}

// SetMarkerFunc lets a typed DTO carry the envelope. Each handler defines
// how to splice Marker into its response struct (typically by adding a
// `Staged *Marker `json:"_staged,omitempty"`` field).
type SetMarkerFunc[T any] func(target *T, m Marker)

// OverlayListMarked behaves like OverlayList but additionally tags every
// emitted row that came from a staged change with a Marker via setMarker.
// Production rows that no staged change applies to pass through with no
// marker.
func OverlayListMarked[T any](
	production []T,
	staged []*models.StagedChange,
	id ResourceID[T],
	apply ApplyFunc[T],
	synth SyntheticFunc[T],
	setMarker SetMarkerFunc[T],
) []T {
	if len(staged) == 0 {
		return production
	}

	updatesByID := make(map[uuid.UUID][]*models.StagedChange, len(staged))
	creates := make([]*models.StagedChange, 0)
	for _, s := range staged {
		switch s.Action {
		case "create":
			creates = append(creates, s)
		default:
			if s.ResourceID != nil {
				updatesByID[*s.ResourceID] = append(updatesByID[*s.ResourceID], s)
			}
		}
	}

	out := make([]T, 0, len(production)+len(creates))
	for _, p := range production {
		pid := id(p)
		patched := p
		dropped := false
		var lastApplied *models.StagedChange
		for _, s := range updatesByID[pid] {
			next, drop := apply(patched, s)
			if drop {
				dropped = true
				break
			}
			patched = next
			lastApplied = s
		}
		if !dropped {
			if lastApplied != nil && setMarker != nil {
				setMarker(&patched, Marker{Action: lastApplied.Action, StagedAt: lastApplied.CreatedAt})
			}
			out = append(out, patched)
		}
	}

	for _, s := range creates {
		v, ok := synth(s)
		if !ok {
			continue
		}
		if setMarker != nil {
			setMarker(&v, Marker{ProvisionalID: s.ProvisionalID, Action: "create", StagedAt: s.CreatedAt})
		}
		out = append(out, v)
	}

	return out
}
