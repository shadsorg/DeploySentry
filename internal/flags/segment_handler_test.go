package flags

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/deploysentry/deploysentry/internal/auth"
	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupSegmentRouter creates a test router with both flag and segment routes registered.
func setupSegmentRouter(svc FlagService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", auth.RoleOwner)
		c.Next()
	})
	rbac := auth.NewRBACChecker()
	handler := NewHandler(svc, rbac, nil, nil, nil)
	handler.RegisterRoutes(router.Group("/api"))
	handler.RegisterSegmentRoutes(router.Group("/api"))
	return router
}

// ---------------------------------------------------------------------------
// POST /segments  (createSegment)
// ---------------------------------------------------------------------------

func TestCreateSegment_Valid(t *testing.T) {
	projectID := uuid.New()
	var capturedSegment *models.Segment

	svc := &mockFlagService{
		createSegmentFn: func(_ context.Context, s *models.Segment) error {
			capturedSegment = s
			return nil
		},
	}
	router := setupSegmentRouter(svc)

	body := map[string]interface{}{
		"project_id":  projectID.String(),
		"key":         "beta-users",
		"name":        "Beta Users",
		"description": "Users enrolled in the beta program",
		"combine_op":  "AND",
		"conditions": []map[string]interface{}{
			{"attribute": "plan", "operator": "eq", "value": "beta", "priority": 1},
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/segments", toJSONBuf(t, body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	require.NotNil(t, capturedSegment)
	assert.Equal(t, "beta-users", capturedSegment.Key)
	assert.Equal(t, "Beta Users", capturedSegment.Name)
	assert.Equal(t, "AND", capturedSegment.CombineOp)
	assert.Equal(t, projectID, capturedSegment.ProjectID)
	assert.Len(t, capturedSegment.Conditions, 1)
	assert.Equal(t, "plan", capturedSegment.Conditions[0].Attribute)
}

func TestCreateSegment_DefaultCombineOp(t *testing.T) {
	var capturedSegment *models.Segment
	svc := &mockFlagService{
		createSegmentFn: func(_ context.Context, s *models.Segment) error {
			capturedSegment = s
			return nil
		},
	}
	router := setupSegmentRouter(svc)

	body := map[string]interface{}{
		"project_id": uuid.New().String(),
		"key":        "my-segment",
		"name":       "My Segment",
		// combine_op omitted — should default to "AND"
	}

	req := httptest.NewRequest(http.MethodPost, "/api/segments", toJSONBuf(t, body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	require.NotNil(t, capturedSegment)
	assert.Equal(t, "AND", capturedSegment.CombineOp)
}

func TestCreateSegment_MissingRequiredFields(t *testing.T) {
	svc := &mockFlagService{}
	router := setupSegmentRouter(svc)

	// Missing key and name
	body := map[string]interface{}{
		"project_id": uuid.New().String(),
	}

	req := httptest.NewRequest(http.MethodPost, "/api/segments", toJSONBuf(t, body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateSegment_ServiceError(t *testing.T) {
	svc := &mockFlagService{
		createSegmentFn: func(_ context.Context, s *models.Segment) error {
			return errors.New("database error")
		},
	}
	router := setupSegmentRouter(svc)

	body := map[string]interface{}{
		"project_id": uuid.New().String(),
		"key":        "segment-key",
		"name":       "Segment Name",
	}

	req := httptest.NewRequest(http.MethodPost, "/api/segments", toJSONBuf(t, body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

// ---------------------------------------------------------------------------
// GET /segments/:segmentID  (getSegment)
// ---------------------------------------------------------------------------

func TestGetSegment_Valid(t *testing.T) {
	segmentID := uuid.New()
	projectID := uuid.New()

	svc := &mockFlagService{
		getSegmentFn: func(_ context.Context, id uuid.UUID) (*models.Segment, error) {
			return &models.Segment{
				ID:        id,
				ProjectID: projectID,
				Key:       "my-segment",
				Name:      "My Segment",
				CombineOp: "AND",
			}, nil
		},
	}
	router := setupSegmentRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/segments/"+segmentID.String(), nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp models.Segment
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, segmentID, resp.ID)
	assert.Equal(t, "my-segment", resp.Key)
}

func TestGetSegment_InvalidID(t *testing.T) {
	svc := &mockFlagService{}
	router := setupSegmentRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/segments/not-a-uuid", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetSegment_NotFound(t *testing.T) {
	svc := &mockFlagService{
		getSegmentFn: func(_ context.Context, id uuid.UUID) (*models.Segment, error) {
			return nil, errors.New("not found")
		},
	}
	router := setupSegmentRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/segments/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ---------------------------------------------------------------------------
// GET /segments  (listSegments)
// ---------------------------------------------------------------------------

func TestListSegments_Valid(t *testing.T) {
	projectID := uuid.New()
	svc := &mockFlagService{
		listSegmentsFn: func(_ context.Context, pid uuid.UUID) ([]*models.Segment, error) {
			return []*models.Segment{
				{ID: uuid.New(), ProjectID: pid, Key: "seg-a", Name: "Seg A", CombineOp: "AND"},
				{ID: uuid.New(), ProjectID: pid, Key: "seg-b", Name: "Seg B", CombineOp: "OR"},
			}, nil
		},
	}
	router := setupSegmentRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/segments?project_id="+projectID.String(), nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	segs, ok := resp["segments"].([]interface{})
	require.True(t, ok)
	assert.Len(t, segs, 2)
}

func TestListSegments_MissingProjectID(t *testing.T) {
	svc := &mockFlagService{}
	router := setupSegmentRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/segments", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ---------------------------------------------------------------------------
// PUT /segments/:segmentID  (updateSegment)
// ---------------------------------------------------------------------------

func TestUpdateSegment_Valid(t *testing.T) {
	segmentID := uuid.New()
	projectID := uuid.New()

	svc := &mockFlagService{
		getSegmentFn: func(_ context.Context, id uuid.UUID) (*models.Segment, error) {
			return &models.Segment{
				ID:        id,
				ProjectID: projectID,
				Key:       "original-key",
				Name:      "Original Name",
				CombineOp: "AND",
			}, nil
		},
		updateSegmentFn: func(_ context.Context, s *models.Segment) error {
			return nil
		},
	}
	router := setupSegmentRouter(svc)

	body := map[string]interface{}{
		"name":       "Updated Name",
		"combine_op": "OR",
	}

	req := httptest.NewRequest(http.MethodPut, "/api/segments/"+segmentID.String(), toJSONBuf(t, body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp models.Segment
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "Updated Name", resp.Name)
	assert.Equal(t, "OR", resp.CombineOp)
}

// ---------------------------------------------------------------------------
// DELETE /segments/:segmentID  (deleteSegment)
// ---------------------------------------------------------------------------

func TestDeleteSegment_Valid(t *testing.T) {
	segmentID := uuid.New()
	var deletedID uuid.UUID

	svc := &mockFlagService{
		deleteSegmentFn: func(_ context.Context, id uuid.UUID) error {
			deletedID = id
			return nil
		},
	}
	router := setupSegmentRouter(svc)

	req := httptest.NewRequest(http.MethodDelete, "/api/segments/"+segmentID.String(), nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, segmentID, deletedID)
}

func TestDeleteSegment_InvalidID(t *testing.T) {
	svc := &mockFlagService{}
	router := setupSegmentRouter(svc)

	req := httptest.NewRequest(http.MethodDelete, "/api/segments/bad-id", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ---------------------------------------------------------------------------
// Helpers (local to this file to avoid duplicate declarations)
// ---------------------------------------------------------------------------

func toJSONBuf(t *testing.T, v interface{}) *bytes.Buffer {
	t.Helper()
	b, err := json.Marshal(v)
	assert.NoError(t, err)
	return bytes.NewBuffer(b)
}
