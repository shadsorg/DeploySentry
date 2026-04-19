package rollout

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Compile-time interface assertions.
var _ StrategyDefaultRepository = (*fakeDefaultsRepo)(nil)
var _ RolloutPolicyRepository = (*fakePolicyRepo)(nil)

// newTestHandler wires a handler with fake repos for unit tests.
func newTestHandler(t *testing.T) (*Handler, *fakeStratRepo, uuid.UUID) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	stratRepo := newFakeStratRepo()
	defaultsRepo := newFakeDefaultsRepo()
	policyRepo := newFakePolicyRepo()
	resolver := &fakeOrgResolver{orgID: uuid.New()}
	h := NewHandler(
		NewStrategyService(stratRepo, nil),
		NewStrategyDefaultService(defaultsRepo),
		NewRolloutPolicyService(policyRepo),
		resolver,
	)
	return h, stratRepo, resolver.orgID
}

type fakeOrgResolver struct{ orgID uuid.UUID }

func (f *fakeOrgResolver) ResolveOrg(c *gin.Context) (uuid.UUID, error) { return f.orgID, nil }
func (f *fakeOrgResolver) ResolveProject(c *gin.Context) (uuid.UUID, uuid.UUID, error) {
	return f.orgID, uuid.New(), nil
}
func (f *fakeOrgResolver) ResolveApp(c *gin.Context) (uuid.UUID, uuid.UUID, uuid.UUID, error) {
	return f.orgID, uuid.New(), uuid.New(), nil
}

func TestHandler_CreateStrategy_OK(t *testing.T) {
	h, _, _ := newTestHandler(t)
	r := gin.New()
	h.RegisterRoutes(r.Group("/api/v1"))
	body, _ := json.Marshal(map[string]any{
		"name": "std", "target_type": "deploy",
		"default_health_threshold": 0.95, "default_rollback_on_failure": true,
		"steps": []map[string]any{
			{"percent": 1, "min_duration": int64(time.Minute), "max_duration": int64(time.Minute), "bake_time_healthy": 0},
			{"percent": 100, "min_duration": 0, "max_duration": 0, "bake_time_healthy": 0},
		},
	})
	req := httptest.NewRequest("POST", "/api/v1/orgs/acme/strategies", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 201 {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
}

func TestHandler_CreateStrategy_ValidationError(t *testing.T) {
	h, _, _ := newTestHandler(t)
	r := gin.New()
	h.RegisterRoutes(r.Group("/api/v1"))
	body, _ := json.Marshal(map[string]any{"name": "", "target_type": "deploy"})
	req := httptest.NewRequest("POST", "/api/v1/orgs/acme/strategies", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Fatalf("expected 400 got %d", w.Code)
	}
}

func TestHandler_List_ReturnsEffective(t *testing.T) {
	h, repo, orgID := newTestHandler(t)
	_ = repo.Create(nil, &models.Strategy{
		ScopeType: models.ScopeOrg, ScopeID: orgID, Name: "std", TargetType: models.TargetTypeDeploy,
		Steps: []models.Step{{Percent: 100}}, DefaultHealthThreshold: 0.95, DefaultRollbackOnFailure: true,
	})
	r := gin.New()
	h.RegisterRoutes(r.Group("/api/v1"))
	req := httptest.NewRequest("GET", "/api/v1/orgs/acme/strategies", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Items []struct {
			Strategy    models.Strategy `json:"strategy"`
			OriginScope ScopeRef        `json:"origin_scope"`
		} `json:"items"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Items) != 1 || resp.Items[0].Strategy.Name != "std" {
		t.Fatalf("unexpected list: %s", w.Body.String())
	}
}

// fakes for defaults + policies (unused in strategy tests but needed by NewHandler)

type fakeDefaultsRepo struct{ rows []*models.StrategyDefault }

func newFakeDefaultsRepo() *fakeDefaultsRepo { return &fakeDefaultsRepo{} }

func (f *fakeDefaultsRepo) Upsert(_ context.Context, d *models.StrategyDefault) error {
	for _, r := range f.rows {
		if r.ScopeType == d.ScopeType && r.ScopeID == d.ScopeID &&
			ptrEq(r.Environment, d.Environment) && ttPtrEq(r.TargetType, d.TargetType) {
			r.StrategyID = d.StrategyID
			return nil
		}
	}
	d.ID = uuid.New()
	f.rows = append(f.rows, d)
	return nil
}

func (f *fakeDefaultsRepo) ListByScope(_ context.Context, st models.ScopeType, sid uuid.UUID) ([]*models.StrategyDefault, error) {
	var out []*models.StrategyDefault
	for _, r := range f.rows {
		if r.ScopeType == st && r.ScopeID == sid {
			out = append(out, r)
		}
	}
	return out, nil
}

func (f *fakeDefaultsRepo) Delete(_ context.Context, id uuid.UUID) error {
	for i, r := range f.rows {
		if r.ID == id {
			f.rows = append(f.rows[:i], f.rows[i+1:]...)
			return nil
		}
	}
	return nil
}

func (f *fakeDefaultsRepo) DeleteByKey(_ context.Context, st models.ScopeType, sid uuid.UUID, env *string, tt *models.TargetType) error {
	for i, r := range f.rows {
		if r.ScopeType == st && r.ScopeID == sid && ptrEq(r.Environment, env) && ttPtrEq(r.TargetType, tt) {
			f.rows = append(f.rows[:i], f.rows[i+1:]...)
			return nil
		}
	}
	return nil
}

type fakePolicyRepo struct{ rows []*models.RolloutPolicy }

func newFakePolicyRepo() *fakePolicyRepo { return &fakePolicyRepo{} }

func (f *fakePolicyRepo) Upsert(_ context.Context, p *models.RolloutPolicy) error {
	for _, r := range f.rows {
		if r.ScopeType == p.ScopeType && r.ScopeID == p.ScopeID &&
			ptrEq(r.Environment, p.Environment) && ttPtrEq(r.TargetType, p.TargetType) {
			r.Enabled, r.Policy = p.Enabled, p.Policy
			return nil
		}
	}
	p.ID = uuid.New()
	f.rows = append(f.rows, p)
	return nil
}

func (f *fakePolicyRepo) ListByScope(_ context.Context, st models.ScopeType, sid uuid.UUID) ([]*models.RolloutPolicy, error) {
	var out []*models.RolloutPolicy
	for _, r := range f.rows {
		if r.ScopeType == st && r.ScopeID == sid {
			out = append(out, r)
		}
	}
	return out, nil
}

func (f *fakePolicyRepo) Delete(_ context.Context, id uuid.UUID) error {
	for i, r := range f.rows {
		if r.ID == id {
			f.rows = append(f.rows[:i], f.rows[i+1:]...)
			return nil
		}
	}
	return nil
}

func ptrEq(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func ttPtrEq(a, b *models.TargetType) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}
