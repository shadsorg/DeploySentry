package rollout

import (
	"context"
	"testing"

	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/google/uuid"
)

type capturedAction struct {
	siblingID uuid.UUID
	action    string // "pause" or "rollback"
}

type fakeSiblingActor struct {
	calls []capturedAction
}

func (f *fakeSiblingActor) Pause(_ context.Context, id uuid.UUID, _ uuid.UUID, _ string) error {
	f.calls = append(f.calls, capturedAction{siblingID: id, action: "pause"})
	return nil
}
func (f *fakeSiblingActor) Rollback(_ context.Context, id uuid.UUID, _ uuid.UUID, _ string) error {
	f.calls = append(f.calls, capturedAction{siblingID: id, action: "rollback"})
	return nil
}

func TestCoordinator_Independent_NoOp(t *testing.T) {
	grpRepo := newFakeGroupRepo()
	roRepo := newFakeRolloutRepo()
	gsvc := NewRolloutGroupService(grpRepo, roRepo)

	gID := uuid.New()
	_ = grpRepo.Create(context.Background(), &models.RolloutGroup{ID: gID, Name: "r", ScopeType: models.ScopeOrg, ScopeID: uuid.New(), CoordinationPolicy: models.CoordinationIndependent})

	originID := uuid.New()
	origin := &models.Rollout{ID: originID, Status: models.RolloutRolledBack, ReleaseID: &gID}
	sibling := &models.Rollout{ID: uuid.New(), Status: models.RolloutActive, ReleaseID: &gID}
	roRepo.rows[origin.ID] = origin
	roRepo.rows[sibling.ID] = sibling

	actor := &fakeSiblingActor{}
	coord := NewCoordinator(gsvc, actor)
	if err := coord.OnRollback(context.Background(), originID); err != nil {
		t.Fatal(err)
	}
	if len(actor.calls) != 0 {
		t.Fatalf("independent policy should not act, got %v", actor.calls)
	}
}

func TestCoordinator_PauseOnSiblingAbort(t *testing.T) {
	grpRepo := newFakeGroupRepo()
	roRepo := newFakeRolloutRepo()
	gsvc := NewRolloutGroupService(grpRepo, roRepo)

	gID := uuid.New()
	_ = grpRepo.Create(context.Background(), &models.RolloutGroup{ID: gID, ScopeType: models.ScopeOrg, ScopeID: uuid.New(), Name: "r", CoordinationPolicy: models.CoordinationPauseOnSiblingAbort})

	originID := uuid.New()
	s1 := &models.Rollout{ID: uuid.New(), Status: models.RolloutActive, ReleaseID: &gID}
	s2 := &models.Rollout{ID: uuid.New(), Status: models.RolloutPaused, ReleaseID: &gID}
	sDone := &models.Rollout{ID: uuid.New(), Status: models.RolloutSucceeded, ReleaseID: &gID}
	origin := &models.Rollout{ID: originID, Status: models.RolloutRolledBack, ReleaseID: &gID}
	for _, r := range []*models.Rollout{origin, s1, s2, sDone} {
		roRepo.rows[r.ID] = r
	}

	actor := &fakeSiblingActor{}
	coord := NewCoordinator(gsvc, actor)
	if err := coord.OnRollback(context.Background(), originID); err != nil {
		t.Fatal(err)
	}
	// Only active siblings get paused (state-machine rule: Pause requires Active).
	if len(actor.calls) != 1 || actor.calls[0].siblingID != s1.ID || actor.calls[0].action != "pause" {
		t.Fatalf("expected [pause s1], got %+v", actor.calls)
	}
}

func TestCoordinator_CascadeAbort(t *testing.T) {
	grpRepo := newFakeGroupRepo()
	roRepo := newFakeRolloutRepo()
	gsvc := NewRolloutGroupService(grpRepo, roRepo)

	gID := uuid.New()
	_ = grpRepo.Create(context.Background(), &models.RolloutGroup{ID: gID, ScopeType: models.ScopeOrg, ScopeID: uuid.New(), Name: "r", CoordinationPolicy: models.CoordinationCascadeAbort})

	originID := uuid.New()
	s1 := &models.Rollout{ID: uuid.New(), Status: models.RolloutActive, ReleaseID: &gID}
	s2 := &models.Rollout{ID: uuid.New(), Status: models.RolloutPaused, ReleaseID: &gID}
	origin := &models.Rollout{ID: originID, Status: models.RolloutRolledBack, ReleaseID: &gID}
	for _, r := range []*models.Rollout{origin, s1, s2} {
		roRepo.rows[r.ID] = r
	}

	actor := &fakeSiblingActor{}
	coord := NewCoordinator(gsvc, actor)
	if err := coord.OnRollback(context.Background(), originID); err != nil {
		t.Fatal(err)
	}
	gotIDs := map[uuid.UUID]string{}
	for _, c := range actor.calls {
		gotIDs[c.siblingID] = c.action
	}
	if gotIDs[s1.ID] != "rollback" || gotIDs[s2.ID] != "rollback" {
		t.Fatalf("expected rollback on both; got %+v", actor.calls)
	}
}

func TestCoordinator_NoGroup_NoOp(t *testing.T) {
	grpRepo := newFakeGroupRepo()
	roRepo := newFakeRolloutRepo()
	gsvc := NewRolloutGroupService(grpRepo, roRepo)

	originID := uuid.New()
	origin := &models.Rollout{ID: originID, Status: models.RolloutRolledBack} // no ReleaseID
	roRepo.rows[origin.ID] = origin

	actor := &fakeSiblingActor{}
	coord := NewCoordinator(gsvc, actor)
	if err := coord.OnRollback(context.Background(), originID); err != nil {
		t.Fatal(err)
	}
	if len(actor.calls) != 0 {
		t.Fatalf("no group → no coordination; got %+v", actor.calls)
	}
}
