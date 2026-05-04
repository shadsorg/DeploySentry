package staging

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/shadsorg/deploysentry/internal/models"
)

// --- fake settingResolver ---

type fakeSettingResolver struct {
	setting *models.Setting
	err     error
}

func (f *fakeSettingResolver) Resolve(_ context.Context, _ string, _, _, _, _ *uuid.UUID) (*models.Setting, error) {
	return f.setting, f.err
}

// --- helpers ---

func makeStagingEnabledSetting(enabled bool) *models.Setting {
	v, _ := json.Marshal(enabled)
	return &models.Setting{
		ID:    uuid.New(),
		Value: json.RawMessage(v),
	}
}

func getEnabledRequest(t *testing.T, orgID uuid.UUID, resolver settingResolver) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewHandlerWithSettings(nil, resolver)
	r.GET("/orgs/:orgSlug/staging", func(c *gin.Context) {
		// Simulate what ResolveOrgRole middleware sets.
		c.Set("org_id", orgID.String())
		h.getEnabled(c)
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/orgs/acme/staging", nil)
	r.ServeHTTP(w, req)
	return w
}

// --- tests ---

func TestGetEnabled_ReturnsTrueWhenSettingIsTrue(t *testing.T) {
	orgID := uuid.New()
	resolver := &fakeSettingResolver{setting: makeStagingEnabledSetting(true)}
	w := getEnabledRequest(t, orgID, resolver)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body map[string]bool
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !body["enabled"] {
		t.Errorf("expected enabled=true, got false")
	}
}

func TestGetEnabled_ReturnsFalseWhenSettingIsFalse(t *testing.T) {
	orgID := uuid.New()
	resolver := &fakeSettingResolver{setting: makeStagingEnabledSetting(false)}
	w := getEnabledRequest(t, orgID, resolver)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body map[string]bool
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body["enabled"] {
		t.Errorf("expected enabled=false, got true")
	}
}

func TestGetEnabled_ReturnsFalseWhenSettingMissing(t *testing.T) {
	orgID := uuid.New()
	resolver := &fakeSettingResolver{setting: nil, err: errors.New("not found")}
	w := getEnabledRequest(t, orgID, resolver)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body map[string]bool
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body["enabled"] {
		t.Errorf("expected enabled=false when setting is missing, got true")
	}
}

func TestGetEnabled_ReturnsFalseWhenNoOrgContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	resolver := &fakeSettingResolver{setting: makeStagingEnabledSetting(true)}
	h := NewHandlerWithSettings(nil, resolver)
	r.GET("/orgs/:orgSlug/staging", h.getEnabled)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/orgs/acme/staging", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body map[string]bool
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body["enabled"] {
		t.Errorf("expected enabled=false when org context missing, got true")
	}
}
