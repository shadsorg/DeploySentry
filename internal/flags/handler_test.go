package flags

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/shadsorg/deploysentry/internal/auth"
	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// Mock
// ---------------------------------------------------------------------------

type mockFlagService struct {
	createFlagFn  func(ctx context.Context, flag *models.FeatureFlag) error
	getFlagFn     func(ctx context.Context, id uuid.UUID) (*models.FeatureFlag, error)
	listFlagsFn   func(ctx context.Context, projectID uuid.UUID, opts ListOptions) ([]*models.FeatureFlag, error)
	updateFlagFn  func(ctx context.Context, flag *models.FeatureFlag) error
	archiveFlagFn func(ctx context.Context, id uuid.UUID) error
	toggleFlagFn  func(ctx context.Context, id uuid.UUID, enabled bool) error
	evaluateFn    func(ctx context.Context, projectID, environmentID uuid.UUID, key string, evalCtx models.EvaluationContext) (*models.FlagEvaluationResult, error)
	addRuleFn           func(ctx context.Context, rule *models.TargetingRule) error
	updateRuleFn        func(ctx context.Context, rule *models.TargetingRule) error
	deleteRuleFn        func(ctx context.Context, ruleID uuid.UUID) error
	listRulesFn         func(ctx context.Context, flagID uuid.UUID) ([]*models.TargetingRule, error)
	listFlagEnvStatesFn func(ctx context.Context, flagID uuid.UUID) ([]*models.FlagEnvironmentState, error)
	setFlagEnvStateFn   func(ctx context.Context, state *models.FlagEnvironmentState) error
	queueDeletionFn     func(ctx context.Context, id uuid.UUID, retention time.Duration) error
	hardDeleteFlagFn    func(ctx context.Context, id uuid.UUID, retention time.Duration) error
	restoreFlagFn       func(ctx context.Context, id uuid.UUID) error
	clearDeleteAfterFn  func(ctx context.Context, id uuid.UUID) error
	// Segment stubs
	createSegmentFn func(ctx context.Context, segment *models.Segment) error
	getSegmentFn    func(ctx context.Context, id uuid.UUID) (*models.Segment, error)
	listSegmentsFn  func(ctx context.Context, projectID uuid.UUID) ([]*models.Segment, error)
	updateSegmentFn func(ctx context.Context, segment *models.Segment) error
	deleteSegmentFn func(ctx context.Context, id uuid.UUID) error
}

func (m *mockFlagService) CreateFlag(ctx context.Context, flag *models.FeatureFlag) error {
	if m.createFlagFn != nil {
		return m.createFlagFn(ctx, flag)
	}
	return nil
}

func (m *mockFlagService) GetFlag(ctx context.Context, id uuid.UUID) (*models.FeatureFlag, error) {
	if m.getFlagFn != nil {
		return m.getFlagFn(ctx, id)
	}
	return &models.FeatureFlag{ID: id}, nil
}

func (m *mockFlagService) ListFlags(ctx context.Context, projectID uuid.UUID, opts ListOptions) ([]*models.FeatureFlag, error) {
	if m.listFlagsFn != nil {
		return m.listFlagsFn(ctx, projectID, opts)
	}
	return []*models.FeatureFlag{}, nil
}

func (m *mockFlagService) UpdateFlag(ctx context.Context, flag *models.FeatureFlag) error {
	if m.updateFlagFn != nil {
		return m.updateFlagFn(ctx, flag)
	}
	return nil
}

func (m *mockFlagService) ArchiveFlag(ctx context.Context, id uuid.UUID) error {
	if m.archiveFlagFn != nil {
		return m.archiveFlagFn(ctx, id)
	}
	return nil
}

func (m *mockFlagService) UnarchiveFlag(ctx context.Context, id uuid.UUID) error {
	return nil
}

func (m *mockFlagService) ToggleFlag(ctx context.Context, id uuid.UUID, enabled bool) error {
	if m.toggleFlagFn != nil {
		return m.toggleFlagFn(ctx, id, enabled)
	}
	return nil
}

func (m *mockFlagService) Evaluate(ctx context.Context, projectID, environmentID uuid.UUID, key string, evalCtx models.EvaluationContext) (*models.FlagEvaluationResult, error) {
	if m.evaluateFn != nil {
		return m.evaluateFn(ctx, projectID, environmentID, key, evalCtx)
	}
	return &models.FlagEvaluationResult{}, nil
}

func (m *mockFlagService) AddRule(ctx context.Context, rule *models.TargetingRule) error {
	if m.addRuleFn != nil {
		return m.addRuleFn(ctx, rule)
	}
	return nil
}

func (m *mockFlagService) UpdateRule(ctx context.Context, rule *models.TargetingRule) error {
	if m.updateRuleFn != nil {
		return m.updateRuleFn(ctx, rule)
	}
	return nil
}

func (m *mockFlagService) GetRule(ctx context.Context, ruleID uuid.UUID) (*models.TargetingRule, error) {
	return nil, nil
}

func (m *mockFlagService) DeleteRule(ctx context.Context, ruleID uuid.UUID) error {
	if m.deleteRuleFn != nil {
		return m.deleteRuleFn(ctx, ruleID)
	}
	return nil
}

func (m *mockFlagService) ListRules(ctx context.Context, flagID uuid.UUID) ([]*models.TargetingRule, error) {
	if m.listRulesFn != nil {
		return m.listRulesFn(ctx, flagID)
	}
	return []*models.TargetingRule{}, nil
}

func (m *mockFlagService) ListFlagEnvStates(ctx context.Context, flagID uuid.UUID) ([]*models.FlagEnvironmentState, error) {
	if m.listFlagEnvStatesFn != nil {
		return m.listFlagEnvStatesFn(ctx, flagID)
	}
	return []*models.FlagEnvironmentState{}, nil
}

func (m *mockFlagService) SetFlagEnvState(ctx context.Context, state *models.FlagEnvironmentState) error {
	if m.setFlagEnvStateFn != nil {
		return m.setFlagEnvStateFn(ctx, state)
	}
	return nil
}

func (m *mockFlagService) BatchEvaluate(ctx context.Context, projectID, environmentID uuid.UUID, keys []string, evalCtx models.EvaluationContext) ([]*models.FlagEvaluationResult, error) {
	results := make([]*models.FlagEvaluationResult, len(keys))
	for i, key := range keys {
		results[i] = &models.FlagEvaluationResult{FlagKey: key, Enabled: true, Value: "on", Reason: "default"}
	}
	return results, nil
}

func (m *mockFlagService) BulkToggle(ctx context.Context, flagIDs []uuid.UUID, enabled bool) error {
	return nil
}

func (m *mockFlagService) DetectStaleFlags(ctx context.Context, projectID uuid.UUID, threshold time.Duration) ([]*models.FeatureFlag, error) {
	return nil, nil
}

func (m *mockFlagService) WarmCache(ctx context.Context, projectID uuid.UUID) error {
	return nil
}

func (m *mockFlagService) CreateSegment(ctx context.Context, segment *models.Segment) error {
	if m.createSegmentFn != nil {
		return m.createSegmentFn(ctx, segment)
	}
	return nil
}

func (m *mockFlagService) GetSegment(ctx context.Context, id uuid.UUID) (*models.Segment, error) {
	if m.getSegmentFn != nil {
		return m.getSegmentFn(ctx, id)
	}
	return &models.Segment{ID: id}, nil
}

func (m *mockFlagService) ListSegments(ctx context.Context, projectID uuid.UUID) ([]*models.Segment, error) {
	if m.listSegmentsFn != nil {
		return m.listSegmentsFn(ctx, projectID)
	}
	return []*models.Segment{}, nil
}

func (m *mockFlagService) UpdateSegment(ctx context.Context, segment *models.Segment) error {
	if m.updateSegmentFn != nil {
		return m.updateSegmentFn(ctx, segment)
	}
	return nil
}

func (m *mockFlagService) DeleteSegment(ctx context.Context, id uuid.UUID) error {
	if m.deleteSegmentFn != nil {
		return m.deleteSegmentFn(ctx, id)
	}
	return nil
}

func (m *mockFlagService) SetRuleEnvironmentState(ctx context.Context, ruleID, environmentID uuid.UUID, enabled bool) (*models.RuleEnvironmentState, error) {
	return &models.RuleEnvironmentState{RuleID: ruleID, EnvironmentID: environmentID, Enabled: enabled}, nil
}

func (m *mockFlagService) ListRuleEnvironmentStates(ctx context.Context, flagID uuid.UUID) ([]*models.RuleEnvironmentState, error) {
	return nil, nil
}

func (m *mockFlagService) ExportFlags(ctx context.Context, projectID uuid.UUID, envs []YAMLEnvironment) (*YAMLExport, error) {
	return &YAMLExport{Version: 1}, nil
}

// Lifecycle stubs — default to a minimal flag so handler tests can exercise
// the response-shape plumbing without the full persistence layer.
func (m *mockFlagService) GetFlagByProjectKey(ctx context.Context, projectID uuid.UUID, key string) (*models.FeatureFlag, error) {
	return &models.FeatureFlag{ID: uuid.New(), ProjectID: projectID, Key: key}, nil
}

func (m *mockFlagService) RecordSmokeTestResult(ctx context.Context, flagID uuid.UUID, status models.LifecycleTestStatus, notes, testRunURL string) (*models.FeatureFlag, error) {
	return &models.FeatureFlag{ID: flagID, SmokeTestStatus: &status}, nil
}

func (m *mockFlagService) RecordUserTestResult(ctx context.Context, flagID uuid.UUID, status models.LifecycleTestStatus, notes string) (*models.FeatureFlag, error) {
	return &models.FeatureFlag{ID: flagID, UserTestStatus: &status}, nil
}

func (m *mockFlagService) ScheduleRemoval(ctx context.Context, flagID uuid.UUID, days int) (*models.FeatureFlag, error) {
	t := time.Now().Add(time.Duration(days) * 24 * time.Hour)
	return &models.FeatureFlag{ID: flagID, ScheduledRemovalAt: &t}, nil
}

func (m *mockFlagService) CancelScheduledRemoval(ctx context.Context, flagID uuid.UUID) (*models.FeatureFlag, error) {
	return &models.FeatureFlag{ID: flagID}, nil
}

func (m *mockFlagService) MarkIterationExhausted(ctx context.Context, flagID uuid.UUID) (*models.FeatureFlag, error) {
	return &models.FeatureFlag{ID: flagID, IterationExhausted: true}, nil
}

func (m *mockFlagService) ListFlagsDueForRemoval(ctx context.Context, now time.Time) ([]*models.FeatureFlag, error) {
	return nil, nil
}

func (m *mockFlagService) MarkFlagRemovalFired(ctx context.Context, flagID uuid.UUID, firedAt time.Time) error {
	return nil
}

func (m *mockFlagService) QueueDeletion(ctx context.Context, id uuid.UUID, retention time.Duration) error {
	if m.queueDeletionFn != nil {
		return m.queueDeletionFn(ctx, id, retention)
	}
	return nil
}

func (m *mockFlagService) HardDeleteFlag(ctx context.Context, id uuid.UUID, retention time.Duration) error {
	if m.hardDeleteFlagFn != nil {
		return m.hardDeleteFlagFn(ctx, id, retention)
	}
	return nil
}

func (m *mockFlagService) RestoreFlag(ctx context.Context, id uuid.UUID) error {
	if m.restoreFlagFn != nil {
		return m.restoreFlagFn(ctx, id)
	}
	return nil
}

func (m *mockFlagService) ClearDeleteAfter(ctx context.Context, id uuid.UUID) error {
	if m.clearDeleteAfterFn != nil {
		return m.clearDeleteAfterFn(ctx, id)
	}
	return nil
}

func (m *mockFlagService) CreateFlagTx(ctx context.Context, _ pgx.Tx, flag *models.FeatureFlag) (uuid.UUID, error) {
	if err := m.CreateFlag(ctx, flag); err != nil {
		return uuid.Nil, err
	}
	return flag.ID, nil
}

func (m *mockFlagService) AddRuleTx(ctx context.Context, _ pgx.Tx, rule *models.TargetingRule) (uuid.UUID, error) {
	if err := m.AddRule(ctx, rule); err != nil {
		return uuid.Nil, err
	}
	return rule.ID, nil
}

func (m *mockFlagService) PublishCreated(_ context.Context, _ *models.FeatureFlag) {}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func setupFlagRouter(svc FlagService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	// Inject a role + user_id so RBAC middleware passes and createFlag's
	// non-nil-actor guard is satisfied. Tests that need to exercise the
	// missing-actor branch should build their own router.
	testUserID := uuid.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", auth.RoleOwner)
		c.Set("user_id", testUserID)
		c.Next()
	})
	rbac := auth.NewRBACChecker()
	handler := NewHandler(svc, rbac, nil, nil, nil, nil, nil)
	handler.RegisterRoutes(router.Group("/api"))
	return router
}

func toJSON(t *testing.T, v interface{}) *bytes.Buffer {
	t.Helper()
	b, err := json.Marshal(v)
	assert.NoError(t, err)
	return bytes.NewBuffer(b)
}

// ---------------------------------------------------------------------------
// POST /flags  (createFlag)
// ---------------------------------------------------------------------------

func TestCreateFlag_Valid(t *testing.T) {
	svc := &mockFlagService{
		createFlagFn: func(_ context.Context, f *models.FeatureFlag) error {
			f.ID = uuid.New()
			return nil
		},
	}
	router := setupFlagRouter(svc)

	body := map[string]interface{}{
		"project_id":     uuid.New().String(),
		"environment_id": uuid.New().String(),
		"key":            "new-feature",
		"name":           "New Feature",
		"flag_type":      "boolean",
	}
	req := httptest.NewRequest(http.MethodPost, "/api/flags", toJSON(t, body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var resp models.FeatureFlag
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "new-feature", resp.Key)
}

// mockEntityResolver implements EntityResolver for slug-based tests.
type mockEntityResolver struct {
	projectsBySlug map[string]*models.Project
}

func (m *mockEntityResolver) GetAppBySlug(_ context.Context, _ uuid.UUID, _ string) (*models.Application, error) {
	return nil, nil
}

func (m *mockEntityResolver) GetProjectBySlug(_ context.Context, _ uuid.UUID, slug string) (*models.Project, error) {
	if p, ok := m.projectsBySlug[slug]; ok {
		return p, nil
	}
	return nil, nil
}

func (m *mockEntityResolver) GetUserName(_ context.Context, _ uuid.UUID) (string, error) {
	return "", nil
}

func setupFlagRouterWithEntities(svc FlagService, entityRepo EntityResolver, orgID uuid.UUID) *gin.Engine {
	return setupFlagRouterWithAuthCtx(svc, entityRepo, orgID, func(c *gin.Context) {
		c.Set("user_id", uuid.New())
	})
}

// setupFlagRouterWithAuthCtx lets a test customize the auth context (e.g. set
// api_key_created_by instead of user_id, or set neither to exercise the
// missing-actor 422 path).
func setupFlagRouterWithAuthCtx(svc FlagService, entityRepo EntityResolver, orgID uuid.UUID, applyAuth func(*gin.Context)) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", auth.RoleOwner)
		c.Set("org_id", orgID.String())
		applyAuth(c)
		c.Next()
	})
	rbac := auth.NewRBACChecker()
	handler := NewHandler(svc, rbac, nil, nil, entityRepo, nil, nil)
	handler.RegisterRoutes(router.Group("/api"))
	return router
}

func TestCreateFlag_ProjectSlug(t *testing.T) {
	orgID := uuid.New()
	projectID := uuid.New()
	svc := &mockFlagService{
		createFlagFn: func(_ context.Context, f *models.FeatureFlag) error {
			f.ID = uuid.New()
			assert.Equal(t, projectID, f.ProjectID)
			return nil
		},
	}
	entities := &mockEntityResolver{
		projectsBySlug: map[string]*models.Project{
			"my-project": {ID: projectID, Slug: "my-project"},
		},
	}
	router := setupFlagRouterWithEntities(svc, entities, orgID)

	body := map[string]interface{}{
		"project_id": "my-project",
		"key":        "slug-feature",
		"name":       "Slug Feature",
		"flag_type":  "boolean",
	}
	req := httptest.NewRequest(http.MethodPost, "/api/flags", toJSON(t, body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var resp models.FeatureFlag
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "slug-feature", resp.Key)
	assert.Equal(t, projectID, resp.ProjectID)
}

func TestCreateFlag_APIKeyAuth_FallsBackToCreatedBy(t *testing.T) {
	orgID := uuid.New()
	projectID := uuid.New()
	apiKeyOwner := uuid.New()
	var captured uuid.UUID
	svc := &mockFlagService{
		createFlagFn: func(_ context.Context, f *models.FeatureFlag) error {
			f.ID = uuid.New()
			captured = f.CreatedBy
			return nil
		},
	}
	entities := &mockEntityResolver{
		projectsBySlug: map[string]*models.Project{
			"my-project": {ID: projectID, Slug: "my-project"},
		},
	}
	router := setupFlagRouterWithAuthCtx(svc, entities, orgID, func(c *gin.Context) {
		// Mimic API-key middleware: no user_id, only api_key_created_by.
		c.Set("api_key_created_by", apiKeyOwner.String())
	})

	body := map[string]interface{}{
		"project_id": "my-project",
		"key":        "apikey-feature",
		"name":       "API Key Feature",
		"flag_type":  "boolean",
	}
	req := httptest.NewRequest(http.MethodPost, "/api/flags", toJSON(t, body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, apiKeyOwner, captured, "created_by should fall back to the api key's minter")
}

func TestCreateFlag_NoActor_Returns422(t *testing.T) {
	orgID := uuid.New()
	projectID := uuid.New()
	svc := &mockFlagService{
		createFlagFn: func(_ context.Context, _ *models.FeatureFlag) error {
			t.Fatal("service should not be called when actor is missing")
			return nil
		},
	}
	entities := &mockEntityResolver{
		projectsBySlug: map[string]*models.Project{
			"my-project": {ID: projectID, Slug: "my-project"},
		},
	}
	router := setupFlagRouterWithAuthCtx(svc, entities, orgID, func(_ *gin.Context) {
		// Neither user_id nor api_key_created_by set.
	})

	body := map[string]interface{}{
		"project_id": "my-project",
		"key":        "no-actor",
		"name":       "No Actor",
		"flag_type":  "boolean",
	}
	req := httptest.NewRequest(http.MethodPost, "/api/flags", toJSON(t, body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestCreateFlag_InvalidJSON(t *testing.T) {
	router := setupFlagRouter(&mockFlagService{})

	req := httptest.NewRequest(http.MethodPost, "/api/flags", bytes.NewBufferString("{bad json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateFlag_ServiceError(t *testing.T) {
	svc := &mockFlagService{
		createFlagFn: func(_ context.Context, _ *models.FeatureFlag) error {
			return errors.New("duplicate key")
		},
	}
	router := setupFlagRouter(svc)

	body := map[string]interface{}{
		"project_id":     uuid.New().String(),
		"environment_id": uuid.New().String(),
		"key":            "dup-feature",
		"name":           "Dup Feature",
		"flag_type":      "boolean",
	}
	req := httptest.NewRequest(http.MethodPost, "/api/flags", toJSON(t, body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

// ---------------------------------------------------------------------------
// GET /flags/:id  (getFlag)
// ---------------------------------------------------------------------------

func TestGetFlag_Valid(t *testing.T) {
	flagID := uuid.New()
	svc := &mockFlagService{
		getFlagFn: func(_ context.Context, id uuid.UUID) (*models.FeatureFlag, error) {
			return &models.FeatureFlag{ID: id, Key: "my-flag", Name: "My Flag"}, nil
		},
	}
	router := setupFlagRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/flags/"+flagID.String(), nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp models.FeatureFlag
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, flagID, resp.ID)
}

func TestGetFlag_InvalidUUID(t *testing.T) {
	router := setupFlagRouter(&mockFlagService{})

	req := httptest.NewRequest(http.MethodGet, "/api/flags/not-a-uuid", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_GetFlag_NotFound(t *testing.T) {
	svc := &mockFlagService{
		getFlagFn: func(_ context.Context, _ uuid.UUID) (*models.FeatureFlag, error) {
			return nil, errors.New("not found")
		},
	}
	router := setupFlagRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/flags/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ---------------------------------------------------------------------------
// GET /flags?project_id=  (listFlags)
// ---------------------------------------------------------------------------

func TestListFlags_Valid(t *testing.T) {
	projectID := uuid.New()
	svc := &mockFlagService{
		listFlagsFn: func(_ context.Context, pid uuid.UUID, _ ListOptions) ([]*models.FeatureFlag, error) {
			return []*models.FeatureFlag{
				{ID: uuid.New(), ProjectID: pid, Key: "flag-1"},
				{ID: uuid.New(), ProjectID: pid, Key: "flag-2"},
			}, nil
		},
	}
	router := setupFlagRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/flags?project_id="+projectID.String(), nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]json.RawMessage
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Contains(t, resp, "flags")
}

func TestListFlags_MissingProjectID(t *testing.T) {
	router := setupFlagRouter(&mockFlagService{})

	req := httptest.NewRequest(http.MethodGet, "/api/flags", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestListFlags_InvalidProjectID(t *testing.T) {
	router := setupFlagRouter(&mockFlagService{})

	req := httptest.NewRequest(http.MethodGet, "/api/flags?project_id=bad", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ---------------------------------------------------------------------------
// PUT /flags/:id  (updateFlag)
// ---------------------------------------------------------------------------

func TestUpdateFlag_Valid(t *testing.T) {
	flagID := uuid.New()
	svc := &mockFlagService{
		getFlagFn: func(_ context.Context, id uuid.UUID) (*models.FeatureFlag, error) {
			return &models.FeatureFlag{ID: id, Key: "old-key", Name: "Old Name"}, nil
		},
		updateFlagFn: func(_ context.Context, _ *models.FeatureFlag) error {
			return nil
		},
	}
	router := setupFlagRouter(svc)

	body := map[string]interface{}{
		"name": "Updated Name",
	}
	req := httptest.NewRequest(http.MethodPut, "/api/flags/"+flagID.String(), toJSON(t, body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp models.FeatureFlag
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "Updated Name", resp.Name)
}

func TestUpdateFlag_InvalidID(t *testing.T) {
	router := setupFlagRouter(&mockFlagService{})

	body := map[string]interface{}{"name": "x"}
	req := httptest.NewRequest(http.MethodPut, "/api/flags/not-valid", toJSON(t, body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateFlag_NotFound(t *testing.T) {
	svc := &mockFlagService{
		getFlagFn: func(_ context.Context, _ uuid.UUID) (*models.FeatureFlag, error) {
			return nil, errors.New("not found")
		},
	}
	router := setupFlagRouter(svc)

	body := map[string]interface{}{"name": "x"}
	req := httptest.NewRequest(http.MethodPut, "/api/flags/"+uuid.New().String(), toJSON(t, body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ---------------------------------------------------------------------------
// POST /flags/:id/archive  (archiveFlag)
// ---------------------------------------------------------------------------

func TestArchiveFlag_Valid(t *testing.T) {
	svc := &mockFlagService{
		archiveFlagFn: func(_ context.Context, _ uuid.UUID) error {
			return nil
		},
	}
	router := setupFlagRouter(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/flags/"+uuid.New().String()+"/archive", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestArchiveFlag_InvalidID(t *testing.T) {
	router := setupFlagRouter(&mockFlagService{})

	req := httptest.NewRequest(http.MethodPost, "/api/flags/bad-id/archive", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestArchiveFlag_ServiceError(t *testing.T) {
	svc := &mockFlagService{
		archiveFlagFn: func(_ context.Context, _ uuid.UUID) error {
			return errors.New("archive failed")
		},
	}
	router := setupFlagRouter(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/flags/"+uuid.New().String()+"/archive", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

// ---------------------------------------------------------------------------
// POST /flags/:id/toggle  (toggleFlag)
// ---------------------------------------------------------------------------

func TestToggleFlag_Valid(t *testing.T) {
	svc := &mockFlagService{
		toggleFlagFn: func(_ context.Context, _ uuid.UUID, _ bool) error {
			return nil
		},
	}
	router := setupFlagRouter(svc)

	body := map[string]interface{}{"enabled": true}
	req := httptest.NewRequest(http.MethodPost, "/api/flags/"+uuid.New().String()+"/toggle", toJSON(t, body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, true, resp["enabled"])
}

func TestToggleFlag_InvalidID(t *testing.T) {
	router := setupFlagRouter(&mockFlagService{})

	body := map[string]interface{}{"enabled": true}
	req := httptest.NewRequest(http.MethodPost, "/api/flags/bad-id/toggle", toJSON(t, body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ---------------------------------------------------------------------------
// POST /flags/evaluate  (evaluate)
// ---------------------------------------------------------------------------

func TestEvaluate_Valid(t *testing.T) {
	svc := &mockFlagService{
		evaluateFn: func(_ context.Context, _, _ uuid.UUID, key string, _ models.EvaluationContext) (*models.FlagEvaluationResult, error) {
			return &models.FlagEvaluationResult{
				FlagKey: key,
				Enabled: true,
				Value:   "on",
				Reason:  "default",
			}, nil
		},
	}
	router := setupFlagRouter(svc)

	body := map[string]interface{}{
		"project_id":     uuid.New().String(),
		"environment_id": uuid.New().String(),
		"flag_key":       "my-flag",
		"context":        map[string]interface{}{"user_id": "u1"},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/flags/evaluate", toJSON(t, body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp models.FlagEvaluationResult
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "my-flag", resp.FlagKey)
	assert.True(t, resp.Enabled)
}

func TestEvaluate_InvalidJSON(t *testing.T) {
	router := setupFlagRouter(&mockFlagService{})

	req := httptest.NewRequest(http.MethodPost, "/api/flags/evaluate", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestEvaluate_NotFound(t *testing.T) {
	svc := &mockFlagService{
		evaluateFn: func(_ context.Context, _, _ uuid.UUID, _ string, _ models.EvaluationContext) (*models.FlagEvaluationResult, error) {
			return nil, errors.New("flag not found")
		},
	}
	router := setupFlagRouter(svc)

	body := map[string]interface{}{
		"project_id":     uuid.New().String(),
		"environment_id": uuid.New().String(),
		"flag_key":       "missing",
		"context":        map[string]interface{}{},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/flags/evaluate", toJSON(t, body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ---------------------------------------------------------------------------
// POST /flags/:id/rules  (addRule)
// ---------------------------------------------------------------------------

func TestAddRule_Valid(t *testing.T) {
	flagID := uuid.New()
	pct := 50
	svc := &mockFlagService{
		addRuleFn: func(_ context.Context, r *models.TargetingRule) error {
			r.ID = uuid.New()
			return nil
		},
	}
	router := setupFlagRouter(svc)

	body := map[string]interface{}{
		"rule_type":  "percentage",
		"priority":   1,
		"value":      "on",
		"percentage": pct,
	}
	req := httptest.NewRequest(http.MethodPost, "/api/flags/"+flagID.String()+"/rules", toJSON(t, body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestAddRule_InvalidFlagID(t *testing.T) {
	router := setupFlagRouter(&mockFlagService{})

	body := map[string]interface{}{
		"rule_type": "percentage",
		"priority":  1,
	}
	req := httptest.NewRequest(http.MethodPost, "/api/flags/bad-uuid/rules", toJSON(t, body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ---------------------------------------------------------------------------
// PUT /flags/:id/rules/:ruleId  (updateRule)
// ---------------------------------------------------------------------------

func TestUpdateRule_Valid(t *testing.T) {
	flagID := uuid.New()
	ruleID := uuid.New()
	pct := 75
	svc := &mockFlagService{
		updateRuleFn: func(_ context.Context, _ *models.TargetingRule) error {
			return nil
		},
	}
	router := setupFlagRouter(svc)

	body := map[string]interface{}{
		"rule_type":  "percentage",
		"priority":   2,
		"value":      "on",
		"percentage": pct,
	}
	req := httptest.NewRequest(http.MethodPut, "/api/flags/"+flagID.String()+"/rules/"+ruleID.String(), toJSON(t, body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestUpdateRule_InvalidRuleID(t *testing.T) {
	router := setupFlagRouter(&mockFlagService{})

	body := map[string]interface{}{
		"rule_type": "percentage",
	}
	req := httptest.NewRequest(http.MethodPut, "/api/flags/"+uuid.New().String()+"/rules/bad-uuid", toJSON(t, body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateRule_InvalidFlagID(t *testing.T) {
	router := setupFlagRouter(&mockFlagService{})

	body := map[string]interface{}{
		"rule_type": "percentage",
	}
	req := httptest.NewRequest(http.MethodPut, "/api/flags/bad-uuid/rules/"+uuid.New().String(), toJSON(t, body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// The handler parses ruleId first; with a valid ruleId but invalid flagId
	// the handler should return 400 for the invalid flag id.
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ---------------------------------------------------------------------------
// DELETE /flags/:id/rules/:ruleId  (deleteRule)
// ---------------------------------------------------------------------------

func TestDeleteRule_Valid(t *testing.T) {
	svc := &mockFlagService{
		deleteRuleFn: func(_ context.Context, _ uuid.UUID) error {
			return nil
		},
	}
	router := setupFlagRouter(svc)

	req := httptest.NewRequest(http.MethodDelete, "/api/flags/"+uuid.New().String()+"/rules/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestDeleteRule_InvalidRuleID(t *testing.T) {
	router := setupFlagRouter(&mockFlagService{})

	req := httptest.NewRequest(http.MethodDelete, "/api/flags/"+uuid.New().String()+"/rules/bad-uuid", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ---------------------------------------------------------------------------
// GET /flags/:id/rules  (listRules)
// ---------------------------------------------------------------------------

func TestListRules_Valid(t *testing.T) {
	flagID := uuid.New()
	ruleID := uuid.New()
	svc := &mockFlagService{
		listRulesFn: func(_ context.Context, fID uuid.UUID) ([]*models.TargetingRule, error) {
			assert.Equal(t, flagID, fID)
			pct := 50
			return []*models.TargetingRule{
				{
					ID:         ruleID,
					FlagID:     fID,
					RuleType:   "percentage",
					Priority:   1,
					Value:      "on",
					Percentage: &pct,
					Enabled:    true,
				},
			}, nil
		},
	}
	router := setupFlagRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/flags/"+flagID.String()+"/rules", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]json.RawMessage
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Contains(t, resp, "rules")

	var rules []models.TargetingRule
	assert.NoError(t, json.Unmarshal(resp["rules"], &rules))
	assert.Len(t, rules, 1)
	assert.Equal(t, ruleID, rules[0].ID)
}

func TestListRules_InvalidFlagID(t *testing.T) {
	router := setupFlagRouter(&mockFlagService{})

	req := httptest.NewRequest(http.MethodGet, "/api/flags/bad-uuid/rules", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestListRules_ServiceError(t *testing.T) {
	svc := &mockFlagService{
		listRulesFn: func(_ context.Context, _ uuid.UUID) ([]*models.TargetingRule, error) {
			return nil, errors.New("db down")
		},
	}
	router := setupFlagRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/flags/"+uuid.New().String()+"/rules", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// ---------------------------------------------------------------------------
// mockRatingSvc
// ---------------------------------------------------------------------------

type mockRatingSvc struct {
	getRatingSummaryFn func(ctx context.Context, flagID uuid.UUID) (*models.RatingSummary, error)
	getErrorSummaryFn  func(ctx context.Context, flagID uuid.UUID, period time.Duration) (*models.ErrorSummary, error)
	isRatingsEnabledFn func(ctx context.Context, orgID uuid.UUID) (bool, error)
}

func (m *mockRatingSvc) GetRatingSummary(ctx context.Context, flagID uuid.UUID) (*models.RatingSummary, error) {
	if m.getRatingSummaryFn != nil {
		return m.getRatingSummaryFn(ctx, flagID)
	}
	return nil, nil
}

func (m *mockRatingSvc) GetErrorSummary(ctx context.Context, flagID uuid.UUID, period time.Duration) (*models.ErrorSummary, error) {
	if m.getErrorSummaryFn != nil {
		return m.getErrorSummaryFn(ctx, flagID, period)
	}
	return nil, nil
}

func (m *mockRatingSvc) IsRatingsEnabled(ctx context.Context, orgID uuid.UUID) (bool, error) {
	if m.isRatingsEnabledFn != nil {
		return m.isRatingsEnabledFn(ctx, orgID)
	}
	return false, nil
}

// ---------------------------------------------------------------------------
// GET /flags/:id with rating service (augmented response)
// ---------------------------------------------------------------------------

func setupFlagRouterWithRatings(svc FlagService, ratingSvc FlagRatingSvc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	orgID := uuid.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", auth.RoleOwner)
		c.Set("org_id", orgID.String())
		c.Next()
	})
	rbac := auth.NewRBACChecker()
	handler := NewHandler(svc, rbac, nil, nil, nil, nil, nil)
	handler.SetRatingService(ratingSvc)
	handler.RegisterRoutes(router.Group("/api"))
	return router
}

func TestGetFlag_WithRatingService_IncludesRatingSummaryAndErrorRate(t *testing.T) {
	flagID := uuid.New()
	flagSvc := &mockFlagService{
		getFlagFn: func(_ context.Context, id uuid.UUID) (*models.FeatureFlag, error) {
			return &models.FeatureFlag{ID: id, Key: "my-flag", Name: "My Flag"}, nil
		},
	}
	ratingSvc := &mockRatingSvc{
		isRatingsEnabledFn: func(_ context.Context, _ uuid.UUID) (bool, error) {
			return true, nil
		},
		getRatingSummaryFn: func(_ context.Context, _ uuid.UUID) (*models.RatingSummary, error) {
			return &models.RatingSummary{
				Average:      4.2,
				Count:        10,
				Distribution: map[int16]int{5: 6, 4: 3, 3: 1},
			}, nil
		},
		getErrorSummaryFn: func(_ context.Context, _ uuid.UUID, _ time.Duration) (*models.ErrorSummary, error) {
			return &models.ErrorSummary{
				Percentage: 1.5,
				Period:     "7d",
			}, nil
		},
	}

	router := setupFlagRouterWithRatings(flagSvc, ratingSvc)

	req := httptest.NewRequest(http.MethodGet, "/api/flags/"+flagID.String(), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]json.RawMessage
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.Contains(t, resp, "rating_summary", "expected rating_summary in response")
	assert.Contains(t, resp, "error_rate", "expected error_rate in response")

	var ratingSummary models.RatingSummary
	assert.NoError(t, json.Unmarshal(resp["rating_summary"], &ratingSummary))
	assert.Equal(t, 4.2, ratingSummary.Average)
	assert.Equal(t, 10, ratingSummary.Count)

	var errorSummary models.ErrorSummary
	assert.NoError(t, json.Unmarshal(resp["error_rate"], &errorSummary))
	assert.Equal(t, 1.5, errorSummary.Percentage)
	assert.Equal(t, "7d", errorSummary.Period)
}

func TestGetFlag_WithRatingService_RatingsDisabled_OmitsRatingSummary(t *testing.T) {
	flagID := uuid.New()
	flagSvc := &mockFlagService{
		getFlagFn: func(_ context.Context, id uuid.UUID) (*models.FeatureFlag, error) {
			return &models.FeatureFlag{ID: id, Key: "my-flag"}, nil
		},
	}
	ratingSvc := &mockRatingSvc{
		isRatingsEnabledFn: func(_ context.Context, _ uuid.UUID) (bool, error) {
			return false, nil // ratings disabled for this org
		},
		getErrorSummaryFn: func(_ context.Context, _ uuid.UUID, _ time.Duration) (*models.ErrorSummary, error) {
			return &models.ErrorSummary{Percentage: 0.5, Period: "7d"}, nil
		},
	}

	router := setupFlagRouterWithRatings(flagSvc, ratingSvc)

	req := httptest.NewRequest(http.MethodGet, "/api/flags/"+flagID.String(), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]json.RawMessage
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.NotContains(t, resp, "rating_summary", "rating_summary should be absent when ratings are disabled")
	assert.Contains(t, resp, "error_rate", "error_rate should still be present")
}

func TestGetFlag_WithoutRatingService_PlainResponse(t *testing.T) {
	flagID := uuid.New()
	flagSvc := &mockFlagService{
		getFlagFn: func(_ context.Context, id uuid.UUID) (*models.FeatureFlag, error) {
			return &models.FeatureFlag{ID: id, Key: "plain-flag"}, nil
		},
	}
	// No rating service — use the plain router.
	router := setupFlagRouter(flagSvc)

	req := httptest.NewRequest(http.MethodGet, "/api/flags/"+flagID.String(), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]json.RawMessage
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.NotContains(t, resp, "rating_summary")
	assert.NotContains(t, resp, "error_rate")
}

// ---------------------------------------------------------------------------
// GET /flags/:id/environments  (listFlagEnvStates)
// ---------------------------------------------------------------------------

func TestListFlagEnvStates_Valid(t *testing.T) {
	flagID := uuid.New()
	envID := uuid.New()
	svc := &mockFlagService{
		listFlagEnvStatesFn: func(_ context.Context, fID uuid.UUID) ([]*models.FlagEnvironmentState, error) {
			assert.Equal(t, flagID, fID)
			return []*models.FlagEnvironmentState{
				{ID: uuid.New(), FlagID: fID, EnvironmentID: envID, Enabled: true},
			}, nil
		},
	}
	router := setupFlagRouter(svc)
	req := httptest.NewRequest(http.MethodGet, "/api/flags/"+flagID.String()+"/environments", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]json.RawMessage
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Contains(t, resp, "environment_states")
}

func TestListFlagEnvStates_InvalidFlagID(t *testing.T) {
	router := setupFlagRouter(&mockFlagService{})
	req := httptest.NewRequest(http.MethodGet, "/api/flags/bad-uuid/environments", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ---------------------------------------------------------------------------
// PUT /flags/:id/environments/:envId  (setFlagEnvState)
// ---------------------------------------------------------------------------

func TestSetFlagEnvState_Valid(t *testing.T) {
	flagID := uuid.New()
	envID := uuid.New()
	svc := &mockFlagService{
		setFlagEnvStateFn: func(_ context.Context, state *models.FlagEnvironmentState) error {
			assert.Equal(t, flagID, state.FlagID)
			assert.Equal(t, envID, state.EnvironmentID)
			assert.True(t, state.Enabled)
			return nil
		},
	}
	router := setupFlagRouter(svc)
	body := map[string]interface{}{"enabled": true}
	req := httptest.NewRequest(http.MethodPut, "/api/flags/"+flagID.String()+"/environments/"+envID.String(), toJSON(t, body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSetFlagEnvState_InvalidFlagID(t *testing.T) {
	router := setupFlagRouter(&mockFlagService{})
	body := map[string]interface{}{"enabled": true}
	req := httptest.NewRequest(http.MethodPut, "/api/flags/bad-uuid/environments/"+uuid.New().String(), toJSON(t, body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSetFlagEnvState_InvalidEnvID(t *testing.T) {
	router := setupFlagRouter(&mockFlagService{})
	body := map[string]interface{}{"enabled": true}
	req := httptest.NewRequest(http.MethodPut, "/api/flags/"+uuid.New().String()+"/environments/bad-uuid", toJSON(t, body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSetFlagEnvState_ServiceError(t *testing.T) {
	svc := &mockFlagService{
		setFlagEnvStateFn: func(_ context.Context, _ *models.FlagEnvironmentState) error {
			return errors.New("db error")
		},
	}
	router := setupFlagRouter(svc)
	body := map[string]interface{}{"enabled": true}
	req := httptest.NewRequest(http.MethodPut, "/api/flags/"+uuid.New().String()+"/environments/"+uuid.New().String(), toJSON(t, body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

// ---------------------------------------------------------------------------
// broadcastEvent helper
// ---------------------------------------------------------------------------

func TestBroadcastEvent(t *testing.T) {
	broker := NewSSEBroker()
	handler := &Handler{sse: broker, service: &mockFlagService{}}

	ch := broker.Subscribe()
	defer broker.Unsubscribe(ch)

	flagID := uuid.New()
	handler.broadcastEvent("flag.updated", flagID, "my-flag")

	select {
	case msg := <-ch:
		var event struct {
			Event     string `json:"event"`
			FlagID    string `json:"flag_id"`
			FlagKey   string `json:"flag_key"`
			Timestamp string `json:"timestamp"`
		}
		if err := json.Unmarshal([]byte(msg), &event); err != nil {
			t.Fatalf("failed to unmarshal SSE event: %v", err)
		}
		if event.Event != "flag.updated" {
			t.Errorf("event = %q, want %q", event.Event, "flag.updated")
		}
		if event.FlagID != flagID.String() {
			t.Errorf("flag_id = %q, want %q", event.FlagID, flagID.String())
		}
		if event.FlagKey != "my-flag" {
			t.Errorf("flag_key = %q, want %q", event.FlagKey, "my-flag")
		}
		if event.Timestamp == "" {
			t.Error("timestamp should not be empty")
		}
	default:
		t.Error("expected to receive SSE event")
	}
}

// ---------------------------------------------------------------------------
// PUT /flags/:id/rules/:ruleId with rollout field  (updateRule + RolloutAttacher)
// ---------------------------------------------------------------------------

type fakeFlagRolloutAttacher struct {
	called   bool
	lastPrev int
}

func (f *fakeFlagRolloutAttacher) AttachFromRuleRequest(_ context.Context, _ *models.TargetingRule, prev int, _ *RolloutAttachRequest, _ uuid.UUID) error {
	f.called = true
	f.lastPrev = prev
	return nil
}

func setupFlagRouterWithRollouts(svc FlagService, attacher RolloutAttacher) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", auth.RoleOwner)
		c.Next()
	})
	rbac := auth.NewRBACChecker()
	handler := NewHandlerWithRollouts(svc, rbac, nil, nil, nil, nil, nil, attacher)
	handler.RegisterRoutes(router.Group("/api"))
	return router
}

func TestUpdateRule_WithRolloutField_AttacherCalled(t *testing.T) {
	flagID := uuid.New()
	ruleID := uuid.New()

	svc := &mockFlagService{
		updateRuleFn: func(_ context.Context, _ *models.TargetingRule) error {
			return nil
		},
	}
	attacher := &fakeFlagRolloutAttacher{}
	router := setupFlagRouterWithRollouts(svc, attacher)

	body := map[string]interface{}{
		"rule_type":  "percentage",
		"priority":   1,
		"value":      "on",
		"percentage": 50,
		"rollout": map[string]interface{}{
			"strategy_name": "slow",
		},
	}
	req := httptest.NewRequest(http.MethodPut, "/api/flags/"+flagID.String()+"/rules/"+ruleID.String(), toJSON(t, body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
	assert.True(t, attacher.called, "expected AttachFromRuleRequest to be called")
	// previousPercentage is currently assumed 0 (GetRule not yet on FlagService)
	assert.Equal(t, 0, attacher.lastPrev)
}

// ---------------------------------------------------------------------------
// POST /flags/:id/queue-deletion  (queueFlagDeletion)
// ---------------------------------------------------------------------------

func TestHandler_QueueFlagDeletion_ArchivedFlag(t *testing.T) {
	flagID := uuid.New()
	callCount := 0
	svc := &mockFlagService{
		getFlagFn: func(_ context.Context, id uuid.UUID) (*models.FeatureFlag, error) {
			callCount++
			return &models.FeatureFlag{ID: id, Key: "my-flag", Archived: true}, nil
		},
		queueDeletionFn: func(_ context.Context, _ uuid.UUID, _ time.Duration) error {
			return nil
		},
	}
	router := setupFlagRouter(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/flags/"+flagID.String()+"/queue-deletion", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandler_QueueFlagDeletion_NotArchived(t *testing.T) {
	flagID := uuid.New()
	svc := &mockFlagService{
		getFlagFn: func(_ context.Context, id uuid.UUID) (*models.FeatureFlag, error) {
			return &models.FeatureFlag{ID: id, Key: "active-flag", Archived: false}, nil
		},
	}
	router := setupFlagRouter(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/flags/"+flagID.String()+"/queue-deletion", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestHandler_QueueFlagDeletion_InvalidID(t *testing.T) {
	router := setupFlagRouter(&mockFlagService{})

	req := httptest.NewRequest(http.MethodPost, "/api/flags/bad-id/queue-deletion", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ---------------------------------------------------------------------------
// DELETE /flags/:id?force=true  (hardDeleteFlag)
// ---------------------------------------------------------------------------

func TestHandler_HardDeleteFlag_Success(t *testing.T) {
	flagID := uuid.New()
	svc := &mockFlagService{
		getFlagFn: func(_ context.Context, id uuid.UUID) (*models.FeatureFlag, error) {
			return &models.FeatureFlag{ID: id, Key: "my-flag", Archived: true}, nil
		},
		hardDeleteFlagFn: func(_ context.Context, _ uuid.UUID, _ time.Duration) error {
			return nil
		},
	}
	router := setupFlagRouter(svc)

	req := httptest.NewRequest(http.MethodDelete, "/api/flags/"+flagID.String()+"?force=true", nil)
	req.Header.Set("X-Confirm-Slug", "my-flag")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandler_HardDeleteFlag_MissingForce(t *testing.T) {
	flagID := uuid.New()
	router := setupFlagRouter(&mockFlagService{})

	req := httptest.NewRequest(http.MethodDelete, "/api/flags/"+flagID.String(), nil)
	req.Header.Set("X-Confirm-Slug", "my-flag")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_HardDeleteFlag_SlugMismatch(t *testing.T) {
	flagID := uuid.New()
	svc := &mockFlagService{
		getFlagFn: func(_ context.Context, id uuid.UUID) (*models.FeatureFlag, error) {
			return &models.FeatureFlag{ID: id, Key: "my-flag", Archived: true}, nil
		},
	}
	router := setupFlagRouter(svc)

	req := httptest.NewRequest(http.MethodDelete, "/api/flags/"+flagID.String()+"?force=true", nil)
	req.Header.Set("X-Confirm-Slug", "wrong-slug")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_HardDeleteFlag_RetentionNotElapsed(t *testing.T) {
	flagID := uuid.New()
	svc := &mockFlagService{
		getFlagFn: func(_ context.Context, id uuid.UUID) (*models.FeatureFlag, error) {
			return &models.FeatureFlag{ID: id, Key: "my-flag", Archived: true}, nil
		},
		hardDeleteFlagFn: func(_ context.Context, _ uuid.UUID, _ time.Duration) error {
			return errors.New("hard delete: postgres.HardDeleteFlag: not found")
		},
	}
	router := setupFlagRouter(svc)

	req := httptest.NewRequest(http.MethodDelete, "/api/flags/"+flagID.String()+"?force=true", nil)
	req.Header.Set("X-Confirm-Slug", "my-flag")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestHandler_HardDeleteFlag_NotArchived(t *testing.T) {
	flagID := uuid.New()
	svc := &mockFlagService{
		getFlagFn: func(_ context.Context, id uuid.UUID) (*models.FeatureFlag, error) {
			return &models.FeatureFlag{ID: id, Key: "my-flag", Archived: false}, nil
		},
	}
	router := setupFlagRouter(svc)

	req := httptest.NewRequest(http.MethodDelete, "/api/flags/"+flagID.String()+"?force=true", nil)
	req.Header.Set("X-Confirm-Slug", "my-flag")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
}

// ---------------------------------------------------------------------------
// POST /flags/:id/restore  (restoreFlag)
// ---------------------------------------------------------------------------

func TestHandler_RestoreFlag_Success(t *testing.T) {
	flagID := uuid.New()
	callCount := 0
	svc := &mockFlagService{
		getFlagFn: func(_ context.Context, id uuid.UUID) (*models.FeatureFlag, error) {
			callCount++
			return &models.FeatureFlag{ID: id, Key: "my-flag", Archived: true}, nil
		},
		restoreFlagFn: func(_ context.Context, _ uuid.UUID) error {
			return nil
		},
	}
	router := setupFlagRouter(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/flags/"+flagID.String()+"/restore", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandler_RestoreFlag_InvalidID(t *testing.T) {
	router := setupFlagRouter(&mockFlagService{})

	req := httptest.NewRequest(http.MethodPost, "/api/flags/bad-id/restore", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
