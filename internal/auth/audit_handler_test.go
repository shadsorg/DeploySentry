package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubAuditRepo implements AuditLogRepository for testing.
type stubAuditRepo struct {
	entries []*models.AuditLogEntry
	total   int
}

func (s *stubAuditRepo) QueryAuditLogs(_ context.Context, _ AuditLogFilter) ([]*models.AuditLogEntry, int, error) {
	return s.entries, s.total, nil
}

func TestQueryAuditLog_PopulatesRevertible(t *testing.T) {
	orgID := uuid.New()

	// Registry has a handler for ("flag", "flag.archived") only.
	reg := NewRevertRegistry()
	reg.Register("flag", "flag.archived", func(_ context.Context, _ *models.AuditLogEntry, _ bool) (string, error) {
		return "flag.archived.reverted", nil
	})

	revertibleEntry := &models.AuditLogEntry{
		ID:         uuid.New(),
		OrgID:      orgID,
		ActorID:    uuid.New(),
		Action:     "flag.archived",
		EntityType: "flag",
		EntityID:   uuid.New(),
		CreatedAt:  time.Now(),
	}
	nonRevertibleEntry := &models.AuditLogEntry{
		ID:         uuid.New(),
		OrgID:      orgID,
		ActorID:    uuid.New(),
		Action:     "flag.unrevertible",
		EntityType: "flag",
		EntityID:   uuid.New(),
		CreatedAt:  time.Now(),
	}

	repo := &stubAuditRepo{
		entries: []*models.AuditLogEntry{revertibleEntry, nonRevertibleEntry},
		total:   2,
	}

	h := NewAuditHandler(repo, reg)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("org_id", orgID.String())
	c.Request, _ = http.NewRequest(http.MethodGet, "/audit-log", nil)

	h.queryAuditLog(c)

	require.Equal(t, http.StatusOK, w.Code)

	var body struct {
		Entries []struct {
			Action     string `json:"action"`
			Revertible bool   `json:"revertible"`
		} `json:"entries"`
		Total int `json:"total"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	require.Len(t, body.Entries, 2)

	// Find by action so order doesn't matter.
	byAction := map[string]bool{}
	for _, e := range body.Entries {
		byAction[e.Action] = e.Revertible
	}

	assert.True(t, byAction["flag.archived"], "flag.archived should be revertible")
	assert.False(t, byAction["flag.unrevertible"], "flag.unrevertible should not be revertible")
}

func TestQueryAuditLog_NilRegistrySkipsPopulation(t *testing.T) {
	orgID := uuid.New()

	entry := &models.AuditLogEntry{
		ID:         uuid.New(),
		OrgID:      orgID,
		ActorID:    uuid.New(),
		Action:     "flag.archived",
		EntityType: "flag",
		EntityID:   uuid.New(),
		CreatedAt:  time.Now(),
	}

	repo := &stubAuditRepo{entries: []*models.AuditLogEntry{entry}, total: 1}

	// Pass nil registry — should not panic, Revertible stays false.
	h := NewAuditHandler(repo, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("org_id", orgID.String())
	c.Request, _ = http.NewRequest(http.MethodGet, "/audit-log", nil)

	h.queryAuditLog(c)

	require.Equal(t, http.StatusOK, w.Code)

	var body struct {
		Entries []struct {
			Revertible bool `json:"revertible"`
		} `json:"entries"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	require.Len(t, body.Entries, 1)
	assert.False(t, body.Entries[0].Revertible)
}
