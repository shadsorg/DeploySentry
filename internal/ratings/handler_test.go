package ratings

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/deploysentry/deploysentry/internal/auth"
	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// Mock service for handler tests
// ---------------------------------------------------------------------------

type mockRatingService struct {
	upsertRatingFn     func(ctx context.Context, rating *models.FlagRating) error
	getRatingFn        func(ctx context.Context, flagID, userID uuid.UUID) (*models.FlagRating, error)
	listRatingsFn      func(ctx context.Context, flagID uuid.UUID, limit, offset int) ([]*models.FlagRating, error)
	deleteRatingFn     func(ctx context.Context, flagID, userID uuid.UUID) error
	getRatingSummaryFn func(ctx context.Context, flagID uuid.UUID) (*models.RatingSummary, error)
	reportErrorsFn     func(ctx context.Context, projectID uuid.UUID, entries []ErrorReportEntry, envID, orgID uuid.UUID) error
	getErrorSummaryFn  func(ctx context.Context, flagID uuid.UUID, period time.Duration) (*models.ErrorSummary, error)
	getErrorsByOrgFn   func(ctx context.Context, flagID uuid.UUID, period time.Duration) ([]*models.OrgErrorBreakdown, error)
	isRatingsEnabledFn func(ctx context.Context, orgID uuid.UUID) (bool, error)
}

func (m *mockRatingService) UpsertRating(ctx context.Context, r *models.FlagRating) error {
	if m.upsertRatingFn != nil {
		return m.upsertRatingFn(ctx, r)
	}
	return nil
}
func (m *mockRatingService) GetRating(ctx context.Context, fid, uid uuid.UUID) (*models.FlagRating, error) {
	if m.getRatingFn != nil {
		return m.getRatingFn(ctx, fid, uid)
	}
	return &models.FlagRating{}, nil
}
func (m *mockRatingService) ListRatings(ctx context.Context, fid uuid.UUID, l, o int) ([]*models.FlagRating, error) {
	if m.listRatingsFn != nil {
		return m.listRatingsFn(ctx, fid, l, o)
	}
	return []*models.FlagRating{}, nil
}
func (m *mockRatingService) DeleteRating(ctx context.Context, fid, uid uuid.UUID) error {
	if m.deleteRatingFn != nil {
		return m.deleteRatingFn(ctx, fid, uid)
	}
	return nil
}
func (m *mockRatingService) GetRatingSummary(ctx context.Context, fid uuid.UUID) (*models.RatingSummary, error) {
	if m.getRatingSummaryFn != nil {
		return m.getRatingSummaryFn(ctx, fid)
	}
	return &models.RatingSummary{}, nil
}
func (m *mockRatingService) ReportErrors(ctx context.Context, projectID uuid.UUID, entries []ErrorReportEntry, envID, orgID uuid.UUID) error {
	if m.reportErrorsFn != nil {
		return m.reportErrorsFn(ctx, projectID, entries, envID, orgID)
	}
	return nil
}
func (m *mockRatingService) GetErrorSummary(ctx context.Context, fid uuid.UUID, p time.Duration) (*models.ErrorSummary, error) {
	if m.getErrorSummaryFn != nil {
		return m.getErrorSummaryFn(ctx, fid, p)
	}
	return &models.ErrorSummary{}, nil
}
func (m *mockRatingService) GetErrorsByOrg(ctx context.Context, fid uuid.UUID, p time.Duration) ([]*models.OrgErrorBreakdown, error) {
	if m.getErrorsByOrgFn != nil {
		return m.getErrorsByOrgFn(ctx, fid, p)
	}
	return []*models.OrgErrorBreakdown{}, nil
}
func (m *mockRatingService) IsRatingsEnabled(ctx context.Context, oid uuid.UUID) (bool, error) {
	if m.isRatingsEnabledFn != nil {
		return m.isRatingsEnabledFn(ctx, oid)
	}
	return true, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func setupRatingRouter(svc RatingService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", uuid.New())         // JWT middleware sets uuid.UUID
		c.Set("org_id", uuid.New().String()) // org_id is stored as string
		c.Set("role", auth.RoleOwner)
		c.Next()
	})
	rbac := auth.NewRBACChecker()
	handler := NewHandler(svc, rbac)
	handler.RegisterRoutes(router.Group("/api"))
	return router
}

func marshalJSON(t *testing.T, v interface{}) *bytes.Buffer {
	t.Helper()
	b, err := json.Marshal(v)
	assert.NoError(t, err)
	return bytes.NewBuffer(b)
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestCreateRating_Valid(t *testing.T) {
	flagID := uuid.New()
	svc := &mockRatingService{
		upsertRatingFn: func(_ context.Context, r *models.FlagRating) error {
			assert.Equal(t, flagID, r.FlagID)
			assert.Equal(t, int16(4), r.Rating)
			return nil
		},
	}
	router := setupRatingRouter(svc)

	body := map[string]interface{}{"rating": 4, "comment": "Great feature"}
	req := httptest.NewRequest(http.MethodPost, "/api/flags/"+flagID.String()+"/ratings", marshalJSON(t, body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCreateRating_RatingsDisabled(t *testing.T) {
	svc := &mockRatingService{
		isRatingsEnabledFn: func(_ context.Context, _ uuid.UUID) (bool, error) {
			return false, nil
		},
	}
	router := setupRatingRouter(svc)

	body := map[string]interface{}{"rating": 4}
	req := httptest.NewRequest(http.MethodPost, "/api/flags/"+uuid.New().String()+"/ratings", marshalJSON(t, body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestGetRatingSummary(t *testing.T) {
	flagID := uuid.New()
	svc := &mockRatingService{
		getRatingSummaryFn: func(_ context.Context, id uuid.UUID) (*models.RatingSummary, error) {
			assert.Equal(t, flagID, id)
			return &models.RatingSummary{Average: 4.2, Count: 10, Distribution: map[int16]int{4: 5, 5: 5}}, nil
		},
	}
	router := setupRatingRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/flags/"+flagID.String()+"/ratings/summary", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp models.RatingSummary
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, 4.2, resp.Average)
	assert.Equal(t, 10, resp.Count)
}

func TestDeleteRating(t *testing.T) {
	svc := &mockRatingService{}
	router := setupRatingRouter(svc)

	req := httptest.NewRequest(http.MethodDelete, "/api/flags/"+uuid.New().String()+"/ratings", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestReportErrors_Handler(t *testing.T) {
	// Use a fixed org_id so request body matches the authenticated context.
	fixedOrgID := uuid.New()
	var capturedEntries []ErrorReportEntry
	svc := &mockRatingService{
		reportErrorsFn: func(_ context.Context, _ uuid.UUID, entries []ErrorReportEntry, _, _ uuid.UUID) error {
			capturedEntries = entries
			return nil
		},
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", uuid.New())
		c.Set("org_id", fixedOrgID.String())
		c.Set("role", auth.RoleOwner)
		c.Next()
	})
	rbac := auth.NewRBACChecker()
	handler := NewHandler(svc, rbac)
	handler.RegisterRoutes(router.Group("/api"))

	body := map[string]interface{}{
		"project_id":     uuid.New().String(),
		"environment_id": uuid.New().String(),
		"org_id":         fixedOrgID.String(),
		"stats": []map[string]interface{}{
			{"flag_key": "test-flag", "evaluations": 100, "errors": 2},
		},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/flags/errors/report", marshalJSON(t, body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
	assert.Len(t, capturedEntries, 1)
}

func TestReportErrors_OrgMismatch(t *testing.T) {
	svc := &mockRatingService{}
	router := setupRatingRouter(svc) // random org_id in context

	body := map[string]interface{}{
		"project_id":     uuid.New().String(),
		"environment_id": uuid.New().String(),
		"org_id":         uuid.New().String(), // different org_id
		"stats": []map[string]interface{}{
			{"flag_key": "test-flag", "evaluations": 100, "errors": 2},
		},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/flags/errors/report", marshalJSON(t, body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestGetErrorsByOrg_RequiresAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", uuid.New())
		c.Set("org_id", uuid.New().String())
		c.Set("role", auth.RoleViewer) // not admin/owner
		c.Next()
	})
	rbac := auth.NewRBACChecker()
	handler := NewHandler(&mockRatingService{}, rbac)
	handler.RegisterRoutes(router.Group("/api"))

	req := httptest.NewRequest(http.MethodGet, "/api/flags/"+uuid.New().String()+"/errors/by-org", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}
