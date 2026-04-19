package rollout

import (
	"testing"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
)

func TestAncestorScopes_App(t *testing.T) {
	orgID, projID, appID := uuid.New(), uuid.New(), uuid.New()
	got := AncestorScopes(ScopeRef{Type: models.ScopeApp, ID: appID}, &projID, &orgID)
	want := []ScopeRef{{models.ScopeApp, appID}, {models.ScopeProject, projID}, {models.ScopeOrg, orgID}}
	if len(got) != len(want) {
		t.Fatalf("len mismatch: %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("pos %d: got %v want %v", i, got[i], want[i])
		}
	}
}

func TestAncestorScopes_Org(t *testing.T) {
	orgID := uuid.New()
	got := AncestorScopes(ScopeRef{Type: models.ScopeOrg, ID: orgID}, nil, nil)
	if len(got) != 1 || got[0].Type != models.ScopeOrg {
		t.Fatalf("got %v", got)
	}
}

func TestResolveDefault_MatchesMostSpecificKey(t *testing.T) {
	orgID, projID := uuid.New(), uuid.New()
	env := "prod"
	tt := models.TargetTypeDeploy
	rows := []*models.StrategyDefault{
		{ScopeType: models.ScopeOrg, ScopeID: orgID, StrategyID: uuid.New()},                                // (org, any, any)
		{ScopeType: models.ScopeOrg, ScopeID: orgID, TargetType: &tt, StrategyID: uuid.New()},               // (org, any, deploy)
		{ScopeType: models.ScopeProject, ScopeID: projID, Environment: &env, TargetType: &tt, StrategyID: uuid.New()}, // (project, prod, deploy)
	}
	ancestors := []ScopeRef{{models.ScopeProject, projID}, {models.ScopeOrg, orgID}}
	got := ResolveDefault(rows, ancestors, &env, &tt)
	if got == nil || got.ScopeType != models.ScopeProject {
		t.Fatalf("expected project match, got %+v", got)
	}
}

func TestResolveDefault_FallsThroughToWildcard(t *testing.T) {
	orgID := uuid.New()
	tt := models.TargetTypeDeploy
	rows := []*models.StrategyDefault{
		{ScopeType: models.ScopeOrg, ScopeID: orgID, StrategyID: uuid.New()}, // (org, any, any)
	}
	env := "prod"
	ancestors := []ScopeRef{{models.ScopeOrg, orgID}}
	got := ResolveDefault(rows, ancestors, &env, &tt)
	if got == nil {
		t.Fatalf("expected wildcard fallback")
	}
}

func TestResolveDefault_NoMatch(t *testing.T) {
	ancestors := []ScopeRef{{models.ScopeOrg, uuid.New()}}
	env := "prod"
	tt := models.TargetTypeDeploy
	got := ResolveDefault(nil, ancestors, &env, &tt)
	if got != nil {
		t.Fatalf("expected nil, got %+v", got)
	}
}
