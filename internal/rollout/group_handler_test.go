package rollout

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type fakeGroupScopeResolver struct{ orgID uuid.UUID }

func (f *fakeGroupScopeResolver) ResolveOrg(_ *gin.Context) (uuid.UUID, error) {
	return f.orgID, nil
}

func newTestGroupHandler(t *testing.T) (*RolloutGroupHandler, *fakeGroupRepo, *fakeRolloutRepo, uuid.UUID) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	grpRepo := newFakeGroupRepo()
	roRepo := newFakeRolloutRepo()
	gsvc := NewRolloutGroupService(grpRepo, roRepo)
	orgID := uuid.New()
	resolver := &fakeGroupScopeResolver{orgID: orgID}
	h := NewRolloutGroupHandler(gsvc, resolver)
	return h, grpRepo, roRepo, orgID
}

func TestRolloutGroupHandler_Create_Defaults(t *testing.T) {
	h, _, _, _ := newTestGroupHandler(t)
	r := gin.New()
	h.RegisterRoutes(r.Group("/api/v1"))

	body, _ := json.Marshal(map[string]any{"name": "v1.1"})
	req := httptest.NewRequest("POST", "/api/v1/orgs/acme/rollout-groups", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 201 {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	var resp models.RolloutGroup
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.CoordinationPolicy != models.CoordinationIndependent {
		t.Fatalf("expected independent default, got %s", resp.CoordinationPolicy)
	}
}

func TestRolloutGroupHandler_List_ReturnsShape(t *testing.T) {
	h, grpRepo, _, orgID := newTestGroupHandler(t)
	_ = grpRepo.Create(nil, &models.RolloutGroup{ScopeType: models.ScopeOrg, ScopeID: orgID, Name: "v1", CoordinationPolicy: models.CoordinationIndependent})
	r := gin.New()
	h.RegisterRoutes(r.Group("/api/v1"))

	req := httptest.NewRequest("GET", "/api/v1/orgs/acme/rollout-groups", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("code=%d", w.Code)
	}
	var resp struct{ Items []models.RolloutGroup `json:"items"` }
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Items) != 1 || resp.Items[0].Name != "v1" {
		t.Fatalf("unexpected: %s", w.Body.String())
	}
}

func TestRolloutGroupHandler_Attach_SetsRolloutReleaseID(t *testing.T) {
	h, grpRepo, roRepo, orgID := newTestGroupHandler(t)
	g := &models.RolloutGroup{ID: uuid.New(), ScopeType: models.ScopeOrg, ScopeID: orgID, Name: "v1", CoordinationPolicy: models.CoordinationIndependent}
	_ = grpRepo.Create(nil, g)
	ro := &models.Rollout{ID: uuid.New(), TargetType: models.TargetTypeDeploy, Status: models.RolloutActive}
	roRepo.rows[ro.ID] = ro

	r := gin.New()
	h.RegisterRoutes(r.Group("/api/v1"))
	body, _ := json.Marshal(map[string]any{"rollout_id": ro.ID.String()})
	req := httptest.NewRequest("POST", "/api/v1/orgs/acme/rollout-groups/"+g.ID.String()+"/attach", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	if ro.ReleaseID == nil || *ro.ReleaseID != g.ID {
		t.Fatalf("not attached: %+v", ro.ReleaseID)
	}
}

func TestRolloutGroupHandler_UpdatePolicy(t *testing.T) {
	h, grpRepo, _, orgID := newTestGroupHandler(t)
	g := &models.RolloutGroup{ID: uuid.New(), ScopeType: models.ScopeOrg, ScopeID: orgID, Name: "v1", CoordinationPolicy: models.CoordinationIndependent}
	_ = grpRepo.Create(nil, g)

	r := gin.New()
	h.RegisterRoutes(r.Group("/api/v1"))
	body, _ := json.Marshal(map[string]any{"coordination_policy": "cascade_abort", "name": "v1", "description": ""})
	req := httptest.NewRequest("PUT", "/api/v1/orgs/acme/rollout-groups/"+g.ID.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	got, _ := grpRepo.Get(nil, g.ID)
	if got.CoordinationPolicy != models.CoordinationCascadeAbort {
		t.Fatalf("policy not updated: %s", got.CoordinationPolicy)
	}
}
