package rollout

import (
	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
)

// AncestorScopes returns scope refs from most-specific to least-specific for
// a given leaf scope. Parents passed as nil are skipped; pass nil for both to
// walk an org-only leaf.
func AncestorScopes(leaf ScopeRef, projectID, orgID *uuid.UUID) []ScopeRef {
	switch leaf.Type {
	case models.ScopeApp:
		out := []ScopeRef{leaf}
		if projectID != nil {
			out = append(out, ScopeRef{models.ScopeProject, *projectID})
		}
		if orgID != nil {
			out = append(out, ScopeRef{models.ScopeOrg, *orgID})
		}
		return out
	case models.ScopeProject:
		out := []ScopeRef{leaf}
		if orgID != nil {
			out = append(out, ScopeRef{models.ScopeOrg, *orgID})
		}
		return out
	case models.ScopeOrg:
		return []ScopeRef{leaf}
	}
	return nil
}

// ResolveDefault walks ancestors most-specific → least-specific, within each
// scope trying key patterns in priority order (env+target, env+any, any+target, any+any).
// First match wins.
func ResolveDefault(rows []*models.StrategyDefault, ancestors []ScopeRef, env *string, target *models.TargetType) *models.StrategyDefault {
	bucket := map[ScopeRef][]*models.StrategyDefault{}
	for _, r := range rows {
		ref := ScopeRef{r.ScopeType, r.ScopeID}
		bucket[ref] = append(bucket[ref], r)
	}
	patterns := []struct {
		matchEnv    bool
		matchTarget bool
	}{{true, true}, {true, false}, {false, true}, {false, false}}
	for _, anc := range ancestors {
		for _, pat := range patterns {
			for _, row := range bucket[anc] {
				if pat.matchEnv {
					if row.Environment == nil || env == nil || *row.Environment != *env {
						continue
					}
				} else if row.Environment != nil {
					continue
				}
				if pat.matchTarget {
					if row.TargetType == nil || target == nil || *row.TargetType != *target {
						continue
					}
				} else if row.TargetType != nil {
					continue
				}
				return row
			}
		}
	}
	return nil
}

// ResolvePolicy walks ancestors most-specific → least-specific and returns the
// first matching policy. Same key-pattern priority as ResolveDefault.
func ResolvePolicy(rows []*models.RolloutPolicy, ancestors []ScopeRef, env *string, target *models.TargetType) *models.RolloutPolicy {
	bucket := map[ScopeRef][]*models.RolloutPolicy{}
	for _, r := range rows {
		ref := ScopeRef{r.ScopeType, r.ScopeID}
		bucket[ref] = append(bucket[ref], r)
	}
	patterns := []struct {
		matchEnv    bool
		matchTarget bool
	}{{true, true}, {true, false}, {false, true}, {false, false}}
	for _, anc := range ancestors {
		for _, pat := range patterns {
			for _, row := range bucket[anc] {
				if pat.matchEnv {
					if row.Environment == nil || env == nil || *row.Environment != *env {
						continue
					}
				} else if row.Environment != nil {
					continue
				}
				if pat.matchTarget {
					if row.TargetType == nil || target == nil || *row.TargetType != *target {
						continue
					}
				} else if row.TargetType != nil {
					continue
				}
				return row
			}
		}
	}
	return nil
}
