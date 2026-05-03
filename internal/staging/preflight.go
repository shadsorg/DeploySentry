package staging

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/shadsorg/deploysentry/internal/models"
)

// batchPlan is the output of planBatch: rows in topo order (creates first,
// then mutations in dependency order, with the original input order preserved
// among rows that have no provisional dependency), plus the set of every
// provisional id minted by this batch.
type batchPlan struct {
	ordered    []*models.StagedChange
	knownProvs map[uuid.UUID]struct{}
}

// ErrUnresolvedProvisional is returned by planBatch when a row references a
// provisional UUID that is not minted by any row in the same batch. The
// commit endpoint surfaces it as CommitResult.FailedReason.
type ErrUnresolvedProvisional struct {
	RowID    uuid.UUID
	ProvUUID uuid.UUID
}

func (e *ErrUnresolvedProvisional) Error() string {
	return fmt.Sprintf(
		"row %s references provisional %s which is not in this deploy batch",
		e.RowID, e.ProvUUID,
	)
}

// planBatch partitions rows into creates + mutations, validates that every
// provisional reference is satisfied by a create in the same batch, and
// returns rows in dependency order. Cycle is impossible by construction
// because provisional ids only flow create → consumer.
func planBatch(rows []*models.StagedChange) (*batchPlan, error) {
	known := make(map[uuid.UUID]struct{})
	creates := make([]*models.StagedChange, 0)
	mutations := make([]*models.StagedChange, 0)
	for _, r := range rows {
		if r.ProvisionalID != nil {
			known[*r.ProvisionalID] = struct{}{}
			creates = append(creates, r)
		} else {
			mutations = append(mutations, r)
		}
	}

	for _, r := range rows {
		refs, err := collectProvisionals(r)
		if err != nil {
			return nil, fmt.Errorf("planBatch: scan row %s: %w", r.ID, err)
		}
		for ref := range refs {
			// A create row's own ProvisionalID is "known" — skip it.
			if r.ProvisionalID != nil && *r.ProvisionalID == ref {
				continue
			}
			if _, ok := known[ref]; !ok {
				return nil, &ErrUnresolvedProvisional{RowID: r.ID, ProvUUID: ref}
			}
		}
	}

	ordered := make([]*models.StagedChange, 0, len(rows))
	ordered = append(ordered, creates...)
	ordered = append(ordered, mutations...)
	return &batchPlan{ordered: ordered, knownProvs: known}, nil
}

// collectProvisionals walks ResourceID + new_value + old_value of a row and
// returns every UUID that has the provisional variant byte. Other UUIDs and
// non-UUID strings are ignored.
func collectProvisionals(row *models.StagedChange) (map[uuid.UUID]struct{}, error) {
	out := make(map[uuid.UUID]struct{})
	if row.ResourceID != nil && IsProvisional(*row.ResourceID) {
		out[*row.ResourceID] = struct{}{}
	}
	for _, raw := range [][]byte{row.NewValue, row.OldValue} {
		if len(raw) == 0 {
			continue
		}
		var v any
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, fmt.Errorf("parse JSON: %w", err)
		}
		walkProvisionals(v, out)
	}
	return out, nil
}

func walkProvisionals(v any, out map[uuid.UUID]struct{}) {
	switch t := v.(type) {
	case string:
		if u, err := uuid.Parse(t); err == nil && IsProvisional(u) {
			out[u] = struct{}{}
		}
	case []any:
		for _, x := range t {
			walkProvisionals(x, out)
		}
	case map[string]any:
		for _, x := range t {
			walkProvisionals(x, out)
		}
	}
}
