package rollout

import (
	"context"
	"errors"
	"testing"

	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/google/uuid"
)

type fakeGroupRepo struct {
	rows map[uuid.UUID]*models.RolloutGroup
}

func newFakeGroupRepo() *fakeGroupRepo { return &fakeGroupRepo{rows: map[uuid.UUID]*models.RolloutGroup{}} }

func (f *fakeGroupRepo) Create(_ context.Context, g *models.RolloutGroup) error {
	if g.ID == uuid.Nil {
		g.ID = uuid.New()
	}
	f.rows[g.ID] = g
	return nil
}
func (f *fakeGroupRepo) Get(_ context.Context, id uuid.UUID) (*models.RolloutGroup, error) {
	if g, ok := f.rows[id]; ok {
		return g, nil
	}
	return nil, errors.New("not found")
}
func (f *fakeGroupRepo) ListByScope(_ context.Context, st models.ScopeType, sid uuid.UUID) ([]*models.RolloutGroup, error) {
	var out []*models.RolloutGroup
	for _, g := range f.rows {
		if g.ScopeType == st && g.ScopeID == sid {
			out = append(out, g)
		}
	}
	return out, nil
}
func (f *fakeGroupRepo) Update(_ context.Context, g *models.RolloutGroup) error {
	if _, ok := f.rows[g.ID]; !ok {
		return errors.New("not found")
	}
	f.rows[g.ID] = g
	return nil
}
func (f *fakeGroupRepo) Delete(_ context.Context, id uuid.UUID) error {
	delete(f.rows, id)
	return nil
}

var _ RolloutGroupRepository = (*fakeGroupRepo)(nil)

func TestRolloutGroupService_Create_DefaultsIndependent(t *testing.T) {
	svc := NewRolloutGroupService(newFakeGroupRepo(), newFakeRolloutRepo())
	g := &models.RolloutGroup{ScopeType: models.ScopeOrg, ScopeID: uuid.New(), Name: "v1.1"}
	if err := svc.Create(context.Background(), g); err != nil {
		t.Fatal(err)
	}
	if g.CoordinationPolicy != models.CoordinationIndependent {
		t.Fatalf("expected default independent, got %s", g.CoordinationPolicy)
	}
}

func TestRolloutGroupService_Attach_SetsRolloutReleaseID(t *testing.T) {
	grpRepo := newFakeGroupRepo()
	roRepo := newFakeRolloutRepo()
	svc := NewRolloutGroupService(grpRepo, roRepo)

	g := &models.RolloutGroup{ID: uuid.New(), ScopeType: models.ScopeOrg, ScopeID: uuid.New(), Name: "v1", CoordinationPolicy: models.CoordinationIndependent}
	_ = grpRepo.Create(context.Background(), g)
	ro := &models.Rollout{ID: uuid.New(), TargetType: models.TargetTypeDeploy, Status: models.RolloutActive}
	roRepo.rows[ro.ID] = ro

	if err := svc.Attach(context.Background(), g.ID, ro.ID); err != nil {
		t.Fatalf("attach: %v", err)
	}
	if ro.ReleaseID == nil || *ro.ReleaseID != g.ID {
		t.Fatalf("release_id not attached: %+v", ro.ReleaseID)
	}
}

func TestRolloutGroupService_Siblings_ExcludesOriginator(t *testing.T) {
	grpRepo := newFakeGroupRepo()
	roRepo := newFakeRolloutRepo()
	svc := NewRolloutGroupService(grpRepo, roRepo)

	gID := uuid.New()
	r1 := &models.Rollout{ID: uuid.New(), TargetType: models.TargetTypeDeploy, Status: models.RolloutActive, ReleaseID: &gID}
	r2 := &models.Rollout{ID: uuid.New(), TargetType: models.TargetTypeDeploy, Status: models.RolloutActive, ReleaseID: &gID}
	rDone := &models.Rollout{ID: uuid.New(), TargetType: models.TargetTypeDeploy, Status: models.RolloutSucceeded, ReleaseID: &gID}
	roRepo.rows[r1.ID] = r1
	roRepo.rows[r2.ID] = r2
	roRepo.rows[rDone.ID] = rDone

	siblings, err := svc.ActiveSiblings(context.Background(), gID, r1.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(siblings) != 1 || siblings[0].ID != r2.ID {
		t.Fatalf("expected [r2], got %+v", siblings)
	}
}

func TestRolloutGroupService_Policy_Lookup(t *testing.T) {
	grpRepo := newFakeGroupRepo()
	roRepo := newFakeRolloutRepo()
	svc := NewRolloutGroupService(grpRepo, roRepo)
	g := &models.RolloutGroup{ID: uuid.New(), ScopeType: models.ScopeOrg, ScopeID: uuid.New(), Name: "x", CoordinationPolicy: models.CoordinationCascadeAbort}
	_ = grpRepo.Create(context.Background(), g)
	pol, err := svc.GetPolicy(context.Background(), g.ID)
	if err != nil || pol != models.CoordinationCascadeAbort {
		t.Fatalf("got pol=%s err=%v", pol, err)
	}
}
