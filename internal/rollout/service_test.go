package rollout

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
)

// --- in-memory fake repos ---

type fakeStratRepo struct {
	rows map[uuid.UUID]*models.Strategy
}

func newFakeStratRepo() *fakeStratRepo { return &fakeStratRepo{rows: map[uuid.UUID]*models.Strategy{}} }

func (f *fakeStratRepo) Create(_ context.Context, s *models.Strategy) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	for _, existing := range f.rows {
		if existing.ScopeType == s.ScopeType && existing.ScopeID == s.ScopeID && existing.Name == s.Name {
			return errors.New("dup")
		}
	}
	s.Version = 1
	f.rows[s.ID] = s
	return nil
}
func (f *fakeStratRepo) Get(_ context.Context, id uuid.UUID) (*models.Strategy, error) {
	if r, ok := f.rows[id]; ok {
		return r, nil
	}
	return nil, errors.New("not found")
}
func (f *fakeStratRepo) GetByName(_ context.Context, st models.ScopeType, sid uuid.UUID, name string) (*models.Strategy, error) {
	for _, r := range f.rows {
		if r.ScopeType == st && r.ScopeID == sid && r.Name == name {
			return r, nil
		}
	}
	return nil, errors.New("not found")
}
func (f *fakeStratRepo) ListByScope(_ context.Context, st models.ScopeType, sid uuid.UUID) ([]*models.Strategy, error) {
	var out []*models.Strategy
	for _, r := range f.rows {
		if r.ScopeType == st && r.ScopeID == sid {
			out = append(out, r)
		}
	}
	return out, nil
}
func (f *fakeStratRepo) ListByAnyScope(_ context.Context, refs []ScopeRef) ([]*models.Strategy, error) {
	refset := map[ScopeRef]bool{}
	for _, r := range refs {
		refset[r] = true
	}
	var out []*models.Strategy
	for _, r := range f.rows {
		if refset[ScopeRef{r.ScopeType, r.ScopeID}] {
			out = append(out, r)
		}
	}
	return out, nil
}
func (f *fakeStratRepo) Update(_ context.Context, s *models.Strategy, expected int) error {
	cur, ok := f.rows[s.ID]
	if !ok {
		return errors.New("not found")
	}
	if cur.Version != expected {
		return errors.New("version conflict")
	}
	cur.Description, cur.Steps, cur.Version = s.Description, s.Steps, cur.Version+1
	return nil
}
func (f *fakeStratRepo) SoftDelete(_ context.Context, id uuid.UUID) error {
	delete(f.rows, id)
	return nil
}
func (f *fakeStratRepo) IsReferenced(_ context.Context, id uuid.UUID) (bool, error) {
	return false, nil
}

// --- tests ---

func TestStrategyService_CreateValidates(t *testing.T) {
	svc := NewStrategyService(newFakeStratRepo(), nil)
	bad := &models.Strategy{Name: "bad", TargetType: models.TargetTypeDeploy} // no steps
	if err := svc.Create(context.Background(), bad); err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestStrategyService_EffectiveList_Inheritance(t *testing.T) {
	repo := newFakeStratRepo()
	orgID, projID := uuid.New(), uuid.New()
	orgStrat := &models.Strategy{ScopeType: models.ScopeOrg, ScopeID: orgID, Name: "std",
		TargetType:             models.TargetTypeDeploy,
		Steps:                  []models.Step{{Percent: 100}},
		DefaultHealthThreshold: 0.95, DefaultRollbackOnFailure: true}
	if err := repo.Create(context.Background(), orgStrat); err != nil {
		t.Fatal(err)
	}
	projStrat := &models.Strategy{ScopeType: models.ScopeProject, ScopeID: projID, Name: "std", // shadows org
		TargetType: models.TargetTypeDeploy, Steps: []models.Step{{Percent: 100}},
		DefaultHealthThreshold: 0.95, DefaultRollbackOnFailure: true}
	if err := repo.Create(context.Background(), projStrat); err != nil {
		t.Fatal(err)
	}

	svc := NewStrategyService(repo, nil)
	eff, err := svc.EffectiveList(context.Background(), ScopeRef{models.ScopeProject, projID}, &projID, &orgID)
	if err != nil {
		t.Fatal(err)
	}
	if len(eff) != 1 || eff[0].Strategy.ID != projStrat.ID {
		t.Fatalf("expected project to shadow org: %+v", eff)
	}
	if eff[0].OriginScope.Type != models.ScopeProject {
		t.Fatalf("origin_scope mismatch: %+v", eff[0].OriginScope)
	}
}

func TestStrategyService_Delete_BlockedIfReferenced(t *testing.T) {
	r := newFakeStratRepo()
	// Force IsReferenced to return true.
	rw := &refsTrue{fakeStratRepo: r}
	svc := NewStrategyService(rw, nil)
	orgID := uuid.New()
	s := &models.Strategy{ScopeType: models.ScopeOrg, ScopeID: orgID, Name: "x", TargetType: models.TargetTypeDeploy,
		Steps: []models.Step{{Percent: 100}}, DefaultHealthThreshold: 0.95}
	if err := r.Create(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(context.Background(), s.ID); err == nil {
		t.Fatalf("expected reference-protection error")
	}
}

func TestStrategyService_Delete_BlockedIfSystem(t *testing.T) {
	r := newFakeStratRepo()
	svc := NewStrategyService(r, nil)
	orgID := uuid.New()
	s := &models.Strategy{ScopeType: models.ScopeOrg, ScopeID: orgID, Name: "x", TargetType: models.TargetTypeDeploy,
		Steps: []models.Step{{Percent: 100}}, DefaultHealthThreshold: 0.95, IsSystem: true}
	if err := r.Create(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(context.Background(), s.ID); err == nil {
		t.Fatalf("expected system-protection error")
	}
}

// refsTrue wraps fakeStratRepo to force IsReferenced=true.
type refsTrue struct{ *fakeStratRepo }

func (r *refsTrue) IsReferenced(_ context.Context, _ uuid.UUID) (bool, error) { return true, nil }

var _ = time.Now // silence unused
