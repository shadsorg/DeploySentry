package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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

// ---------------------------------------------------------------------------
// Stubs
// ---------------------------------------------------------------------------

type stubRevertWriter struct {
	entry        *models.AuditLogEntry
	getErr       error
	writeErr     error
	writeCalled  bool
	writtenEntry *models.AuditLogEntry
}

func (s *stubRevertWriter) GetAuditLogEntry(_ context.Context, _ uuid.UUID) (*models.AuditLogEntry, error) {
	return s.entry, s.getErr
}

func (s *stubRevertWriter) WriteAuditLog(_ context.Context, entry *models.AuditLogEntry) error {
	s.writeCalled = true
	s.writtenEntry = entry
	return s.writeErr
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// serveRevert wires a gin router with the revert handler and serves a single
// request, returning the recorder. orgID and userID are set into the gin
// context as the middleware would do.
func serveRevert(
	t *testing.T,
	repo *stubRevertWriter,
	registry *RevertRegistry,
	entryIDStr string,
	orgIDStr string,
	userID uuid.UUID,
	body interface{},
) *httptest.ResponseRecorder {
	t.Helper()

	h := NewRevertHandler(registry, repo)

	var bodyBuf *bytes.Buffer
	if body != nil {
		b, err := json.Marshal(body)
		require.NoError(t, err)
		bodyBuf = bytes.NewBuffer(b)
	} else {
		bodyBuf = bytes.NewBuffer(nil)
	}

	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)

	r.POST("/audit-log/:entryId/revert", func(c *gin.Context) {
		if orgIDStr != "" {
			c.Set(ContextKeyOrgID, orgIDStr)
		}
		if userID != uuid.Nil {
			c.Set(ContextKeyUserID, userID)
		}
	}, h.revert)

	req := httptest.NewRequest(http.MethodPost, "/audit-log/"+entryIDStr+"/revert", bodyBuf)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

func makeEntry(orgID uuid.UUID) *models.AuditLogEntry {
	return &models.AuditLogEntry{
		ID:         uuid.New(),
		OrgID:      orgID,
		ProjectID:  uuid.New(),
		ActorID:    uuid.New(),
		Action:     "flag.archived",
		EntityType: "flag",
		EntityID:   uuid.New(),
		OldValue:   `{"enabled":true}`,
		NewValue:   `{"enabled":false}`,
		CreatedAt:  time.Now(),
	}
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestRevert_Success(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	entry := makeEntry(orgID)
	const newAction = "flag.archived.reverted"

	reg := NewRevertRegistry()
	reg.Register(entry.EntityType, entry.Action, func(_ context.Context, _ *models.AuditLogEntry, _ bool) (string, error) {
		return newAction, nil
	})

	repo := &stubRevertWriter{entry: entry}
	w := serveRevert(t, repo, reg, entry.ID.String(), orgID.String(), userID, nil)

	require.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, true, resp["reverted"])
	assert.Equal(t, newAction, resp["action"])
	assert.NotEmpty(t, resp["audit_entry_id"])

	// WriteAuditLog must have been called.
	require.True(t, repo.writeCalled, "WriteAuditLog should be called on success")
	written := repo.writtenEntry
	require.NotNil(t, written)
	assert.Equal(t, newAction, written.Action)
	assert.Equal(t, entry.NewValue, written.OldValue, "OldValue of revert row = NewValue of original")
	assert.Equal(t, entry.OldValue, written.NewValue, "NewValue of revert row = OldValue of original")
	assert.Equal(t, entry.EntityType, written.EntityType)
	assert.Equal(t, entry.EntityID, written.EntityID)
	assert.Equal(t, orgID, written.OrgID)
}

func TestRevert_NotRevertible(t *testing.T) {
	orgID := uuid.New()
	entry := makeEntry(orgID)
	// Empty registry — no handler registered.
	reg := NewRevertRegistry()

	repo := &stubRevertWriter{entry: entry}
	w := serveRevert(t, repo, reg, entry.ID.String(), orgID.String(), uuid.New(), nil)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Contains(t, resp["error"], "not revertible")

	assert.False(t, repo.writeCalled, "WriteAuditLog must NOT be called when action is not revertible")
}

func TestRevert_Race(t *testing.T) {
	orgID := uuid.New()
	entry := makeEntry(orgID)

	reg := NewRevertRegistry()
	reg.Register(entry.EntityType, entry.Action, func(_ context.Context, _ *models.AuditLogEntry, _ bool) (string, error) {
		return "", ErrRevertRace
	})

	repo := &stubRevertWriter{entry: entry}
	w := serveRevert(t, repo, reg, entry.ID.String(), orgID.String(), uuid.New(), nil)

	assert.Equal(t, http.StatusConflict, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "race", resp["code"])

	assert.False(t, repo.writeCalled, "WriteAuditLog must NOT be called on race error")
}

func TestRevert_CrossOrg(t *testing.T) {
	orgA := uuid.New()
	orgB := uuid.New()
	entry := makeEntry(orgA) // entry belongs to org A

	var revertCalled bool
	reg := NewRevertRegistry()
	reg.Register(entry.EntityType, entry.Action, func(_ context.Context, _ *models.AuditLogEntry, _ bool) (string, error) {
		revertCalled = true
		return "should.not.happen", nil
	})

	repo := &stubRevertWriter{entry: entry}
	// Caller's org is B.
	w := serveRevert(t, repo, reg, entry.ID.String(), orgB.String(), uuid.New(), nil)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.False(t, revertCalled, "Revert must NOT be called when org does not match")
	assert.False(t, repo.writeCalled, "WriteAuditLog must NOT be called on cross-org rejection")
}

func TestRevert_NotFound(t *testing.T) {
	orgID := uuid.New()
	reg := NewRevertRegistry()

	repo := &stubRevertWriter{
		entry:  nil,
		getErr: fmt.Errorf("postgres.GetAuditLogEntry: %w", fmt.Errorf("no rows in result set")),
	}

	w := serveRevert(t, repo, reg, uuid.New().String(), orgID.String(), uuid.New(), nil)

	assert.Equal(t, http.StatusNotFound, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Contains(t, resp["error"], "not found")
}

func TestRevert_MalformedEntryID(t *testing.T) {
	reg := NewRevertRegistry()
	repo := &stubRevertWriter{}

	w := serveRevert(t, repo, reg, "not-a-uuid", "some-org", uuid.New(), nil)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "invalid entry id", resp["error"])
}

func TestRevert_ForceFlag(t *testing.T) {
	orgID := uuid.New()
	entry := makeEntry(orgID)

	var capturedForce bool
	reg := NewRevertRegistry()
	reg.Register(entry.EntityType, entry.Action, func(_ context.Context, _ *models.AuditLogEntry, force bool) (string, error) {
		capturedForce = force
		return "flag.archived.reverted", nil
	})

	repo := &stubRevertWriter{entry: entry}
	w := serveRevert(t, repo, reg, entry.ID.String(), orgID.String(), uuid.New(), map[string]bool{"force": true})

	require.Equal(t, http.StatusOK, w.Code)
	assert.True(t, capturedForce, "force=true must be passed through to the revert handler")
}
