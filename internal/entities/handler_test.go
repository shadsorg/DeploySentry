package entities

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
// Mock service
// ---------------------------------------------------------------------------

type mockEntityService struct {
	createOrgFn         func(ctx context.Context, org *models.Organization, creatorID uuid.UUID) error
	getOrgBySlugFn      func(ctx context.Context, slug string) (*models.Organization, error)
	listOrgsByUserFn    func(ctx context.Context, userID uuid.UUID) ([]*models.Organization, error)
	updateOrgFn         func(ctx context.Context, org *models.Organization) error
	createProjectFn     func(ctx context.Context, project *models.Project) error
	getProjectBySlugFn  func(ctx context.Context, orgID uuid.UUID, slug string) (*models.Project, error)
	listProjectsByOrgFn func(ctx context.Context, orgID uuid.UUID, includeDeleted bool, userID uuid.UUID, orgRole string) ([]*models.Project, error)
	updateProjectFn     func(ctx context.Context, project *models.Project) error
	createAppFn         func(ctx context.Context, app *models.Application) error
	getAppBySlugFn      func(ctx context.Context, projectID uuid.UUID, slug string) (*models.Application, error)
	listAppsByProjectFn func(ctx context.Context, projectID uuid.UUID, includeDeleted bool, userID uuid.UUID, orgRole string) ([]*models.Application, error)
	updateAppFn         func(ctx context.Context, app *models.Application) error
	listEnvironmentsByAppFn func(ctx context.Context, appID uuid.UUID) ([]*models.Environment, error)
	deleteProjectFn         func(ctx context.Context, orgID uuid.UUID, slug string) (*models.DeleteResult, error)
	hardDeleteProjectFn     func(ctx context.Context, orgID uuid.UUID, slug string) error
	restoreProjectFn        func(ctx context.Context, orgID uuid.UUID, slug string) (*models.Project, error)
	deleteAppFn             func(ctx context.Context, projectID uuid.UUID, slug string) (*models.DeleteResult, error)
	hardDeleteAppFn         func(ctx context.Context, projectID uuid.UUID, slug string) error
	restoreAppFn            func(ctx context.Context, projectID uuid.UUID, slug string) (*models.Application, error)
}

func (m *mockEntityService) CreateOrg(ctx context.Context, org *models.Organization, creatorID uuid.UUID) error {
	if m.createOrgFn != nil {
		return m.createOrgFn(ctx, org, creatorID)
	}
	return nil
}

func (m *mockEntityService) GetOrgBySlug(ctx context.Context, slug string) (*models.Organization, error) {
	if m.getOrgBySlugFn != nil {
		return m.getOrgBySlugFn(ctx, slug)
	}
	return &models.Organization{ID: uuid.New(), Name: "Test Org", Slug: slug}, nil
}

func (m *mockEntityService) ListOrgsByUser(ctx context.Context, userID uuid.UUID) ([]*models.Organization, error) {
	if m.listOrgsByUserFn != nil {
		return m.listOrgsByUserFn(ctx, userID)
	}
	return []*models.Organization{}, nil
}

func (m *mockEntityService) UpdateOrg(ctx context.Context, org *models.Organization) error {
	if m.updateOrgFn != nil {
		return m.updateOrgFn(ctx, org)
	}
	return nil
}

func (m *mockEntityService) CreateProject(ctx context.Context, project *models.Project) error {
	if m.createProjectFn != nil {
		return m.createProjectFn(ctx, project)
	}
	return nil
}

func (m *mockEntityService) GetProjectBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*models.Project, error) {
	if m.getProjectBySlugFn != nil {
		return m.getProjectBySlugFn(ctx, orgID, slug)
	}
	return &models.Project{ID: uuid.New(), OrgID: orgID, Name: "Test Project", Slug: slug}, nil
}

func (m *mockEntityService) ListProjectsByOrg(ctx context.Context, orgID uuid.UUID, includeDeleted bool, userID uuid.UUID, orgRole string) ([]*models.Project, error) {
	if m.listProjectsByOrgFn != nil {
		return m.listProjectsByOrgFn(ctx, orgID, includeDeleted, userID, orgRole)
	}
	return []*models.Project{}, nil
}

func (m *mockEntityService) UpdateProject(ctx context.Context, project *models.Project) error {
	if m.updateProjectFn != nil {
		return m.updateProjectFn(ctx, project)
	}
	return nil
}

func (m *mockEntityService) CreateApp(ctx context.Context, app *models.Application) error {
	if m.createAppFn != nil {
		return m.createAppFn(ctx, app)
	}
	return nil
}

func (m *mockEntityService) GetAppBySlug(ctx context.Context, projectID uuid.UUID, slug string) (*models.Application, error) {
	if m.getAppBySlugFn != nil {
		return m.getAppBySlugFn(ctx, projectID, slug)
	}
	return &models.Application{ID: uuid.New(), ProjectID: projectID, Name: "Test App", Slug: slug}, nil
}

func (m *mockEntityService) ListAppsByProject(ctx context.Context, projectID uuid.UUID, includeDeleted bool, userID uuid.UUID, orgRole string) ([]*models.Application, error) {
	if m.listAppsByProjectFn != nil {
		return m.listAppsByProjectFn(ctx, projectID, includeDeleted, userID, orgRole)
	}
	return []*models.Application{}, nil
}

func (m *mockEntityService) UpdateApp(ctx context.Context, app *models.Application) error {
	if m.updateAppFn != nil {
		return m.updateAppFn(ctx, app)
	}
	return nil
}

func (m *mockEntityService) ListEnvironmentsByApp(ctx context.Context, appID uuid.UUID) ([]*models.Environment, error) {
	if m.listEnvironmentsByAppFn != nil {
		return m.listEnvironmentsByAppFn(ctx, appID)
	}
	return []*models.Environment{}, nil
}

func (m *mockEntityService) CreateEnvironment(ctx context.Context, env *OrgEnvironment) error {
	return nil
}

func (m *mockEntityService) GetEnvironmentBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*OrgEnvironment, error) {
	return nil, nil
}

func (m *mockEntityService) UpdateEnvironment(ctx context.Context, env *OrgEnvironment) error {
	return nil
}

func (m *mockEntityService) DeleteEnvironment(ctx context.Context, id uuid.UUID) error {
	return nil
}

func (m *mockEntityService) ListEnvironments(ctx context.Context, orgID uuid.UUID) ([]OrgEnvironment, error) {
	return nil, nil
}

func (m *mockEntityService) DeleteProject(ctx context.Context, orgID uuid.UUID, slug string) (*models.DeleteResult, error) {
	if m.deleteProjectFn != nil {
		return m.deleteProjectFn(ctx, orgID, slug)
	}
	return nil, nil
}

func (m *mockEntityService) HardDeleteProject(ctx context.Context, orgID uuid.UUID, slug string) error {
	if m.hardDeleteProjectFn != nil {
		return m.hardDeleteProjectFn(ctx, orgID, slug)
	}
	return nil
}

func (m *mockEntityService) RestoreProject(ctx context.Context, orgID uuid.UUID, slug string) (*models.Project, error) {
	if m.restoreProjectFn != nil {
		return m.restoreProjectFn(ctx, orgID, slug)
	}
	return nil, nil
}

func (m *mockEntityService) DeleteApp(ctx context.Context, projectID uuid.UUID, slug string) (*models.DeleteResult, error) {
	if m.deleteAppFn != nil {
		return m.deleteAppFn(ctx, projectID, slug)
	}
	return nil, nil
}

func (m *mockEntityService) HardDeleteApp(ctx context.Context, projectID uuid.UUID, slug string) error {
	if m.hardDeleteAppFn != nil {
		return m.hardDeleteAppFn(ctx, projectID, slug)
	}
	return nil
}

func (m *mockEntityService) RestoreApp(ctx context.Context, projectID uuid.UUID, slug string) (*models.Application, error) {
	if m.restoreAppFn != nil {
		return m.restoreAppFn(ctx, projectID, slug)
	}
	return nil, nil
}


// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func setupEntityRouter(svc EntityService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", uuid.New()) // uuid.UUID type, matching JWT middleware
		c.Set("role", auth.RoleOwner)
		c.Next()
	})
	rbac := auth.NewRBACChecker()
	handler := NewHandler(svc, rbac)
	handler.RegisterRoutes(router.Group("/api"))
	return router
}

func marshalEntityJSON(t *testing.T, v interface{}) *bytes.Buffer {
	t.Helper()
	b, err := json.Marshal(v)
	assert.NoError(t, err)
	return bytes.NewBuffer(b)
}

// ---------------------------------------------------------------------------
// Org tests
// ---------------------------------------------------------------------------

func TestCreateOrg_Valid(t *testing.T) {
	svc := &mockEntityService{
		createOrgFn: func(_ context.Context, org *models.Organization, _ uuid.UUID) error {
			assert.Equal(t, "Test", org.Name)
			assert.Equal(t, "test", org.Slug)
			return nil
		},
	}
	router := setupEntityRouter(svc)

	body := map[string]interface{}{"name": "Test", "slug": "test"}
	req := httptest.NewRequest(http.MethodPost, "/api/orgs", marshalEntityJSON(t, body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestCreateOrg_DuplicateSlug(t *testing.T) {
	svc := &mockEntityService{
		createOrgFn: func(_ context.Context, _ *models.Organization, _ uuid.UUID) error {
			return errors.New("duplicate key value violates unique constraint")
		},
	}
	router := setupEntityRouter(svc)

	body := map[string]interface{}{"name": "Test", "slug": "test"}
	req := httptest.NewRequest(http.MethodPost, "/api/orgs", marshalEntityJSON(t, body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
	var resp map[string]string
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "slug already exists", resp["error"])
}

func TestListOrgs(t *testing.T) {
	svc := &mockEntityService{
		listOrgsByUserFn: func(_ context.Context, _ uuid.UUID) ([]*models.Organization, error) {
			return []*models.Organization{
				{ID: uuid.New(), Name: "Org One", Slug: "org-one"},
				{ID: uuid.New(), Name: "Org Two", Slug: "org-two"},
			}, nil
		},
	}
	router := setupEntityRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/orgs", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	orgs, ok := resp["organizations"]
	assert.True(t, ok)
	assert.Len(t, orgs, 2)
}

func TestGetOrg(t *testing.T) {
	orgID := uuid.New()
	svc := &mockEntityService{
		getOrgBySlugFn: func(_ context.Context, slug string) (*models.Organization, error) {
			assert.Equal(t, "test-org", slug)
			return &models.Organization{ID: orgID, Name: "Test Org", Slug: slug}, nil
		},
	}
	router := setupEntityRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/orgs/test-org", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp models.Organization
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, orgID, resp.ID)
	assert.Equal(t, "test-org", resp.Slug)
}

func TestGetOrg_NotFound(t *testing.T) {
	svc := &mockEntityService{
		getOrgBySlugFn: func(_ context.Context, _ string) (*models.Organization, error) {
			return nil, nil
		},
	}
	router := setupEntityRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/orgs/missing-org", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ---------------------------------------------------------------------------
// Project tests
// ---------------------------------------------------------------------------

func TestCreateProject_Valid(t *testing.T) {
	orgID := uuid.New()
	svc := &mockEntityService{
		getOrgBySlugFn: func(_ context.Context, slug string) (*models.Organization, error) {
			return &models.Organization{ID: orgID, Name: "Test Org", Slug: slug}, nil
		},
		createProjectFn: func(_ context.Context, project *models.Project) error {
			assert.Equal(t, orgID, project.OrgID)
			assert.Equal(t, "My Project", project.Name)
			assert.Equal(t, "my-proj", project.Slug)
			return nil
		},
	}
	router := setupEntityRouter(svc)

	body := map[string]interface{}{"name": "My Project", "slug": "my-proj"}
	req := httptest.NewRequest(http.MethodPost, "/api/orgs/test-org/projects", marshalEntityJSON(t, body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestListProjects(t *testing.T) {
	orgID := uuid.New()
	svc := &mockEntityService{
		getOrgBySlugFn: func(_ context.Context, slug string) (*models.Organization, error) {
			return &models.Organization{ID: orgID, Name: "Test Org", Slug: slug}, nil
		},
		listProjectsByOrgFn: func(_ context.Context, id uuid.UUID, _ bool, _ uuid.UUID, _ string) ([]*models.Project, error) {
			assert.Equal(t, orgID, id)
			return []*models.Project{
				{ID: uuid.New(), OrgID: orgID, Name: "Project One", Slug: "project-one"},
			}, nil
		},
	}
	router := setupEntityRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/orgs/test-org/projects", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	projects, ok := resp["projects"]
	assert.True(t, ok)
	assert.Len(t, projects, 1)
}

// ---------------------------------------------------------------------------
// App tests
// ---------------------------------------------------------------------------

func TestCreateApp_Valid(t *testing.T) {
	orgID := uuid.New()
	projectID := uuid.New()
	svc := &mockEntityService{
		getOrgBySlugFn: func(_ context.Context, slug string) (*models.Organization, error) {
			return &models.Organization{ID: orgID, Name: "Test Org", Slug: slug}, nil
		},
		getProjectBySlugFn: func(_ context.Context, id uuid.UUID, slug string) (*models.Project, error) {
			assert.Equal(t, orgID, id)
			return &models.Project{ID: projectID, OrgID: id, Name: "My Project", Slug: slug}, nil
		},
		createAppFn: func(_ context.Context, app *models.Application) error {
			assert.Equal(t, projectID, app.ProjectID)
			assert.Equal(t, "My App", app.Name)
			assert.Equal(t, "my-app", app.Slug)
			return nil
		},
	}
	router := setupEntityRouter(svc)

	body := map[string]interface{}{"name": "My App", "slug": "my-app"}
	req := httptest.NewRequest(http.MethodPost, "/api/orgs/test-org/projects/my-proj/apps", marshalEntityJSON(t, body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestListApps(t *testing.T) {
	orgID := uuid.New()
	projectID := uuid.New()
	svc := &mockEntityService{
		getOrgBySlugFn: func(_ context.Context, slug string) (*models.Organization, error) {
			return &models.Organization{ID: orgID, Name: "Test Org", Slug: slug}, nil
		},
		getProjectBySlugFn: func(_ context.Context, id uuid.UUID, slug string) (*models.Project, error) {
			return &models.Project{ID: projectID, OrgID: id, Name: "My Project", Slug: slug}, nil
		},
		listAppsByProjectFn: func(_ context.Context, id uuid.UUID, _ bool, _ uuid.UUID, _ string) ([]*models.Application, error) {
			assert.Equal(t, projectID, id)
			return []*models.Application{
				{ID: uuid.New(), ProjectID: projectID, Name: "App One", Slug: "app-one"},
				{ID: uuid.New(), ProjectID: projectID, Name: "App Two", Slug: "app-two"},
			}, nil
		},
	}
	router := setupEntityRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/orgs/test-org/projects/my-proj/apps", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	apps, ok := resp["applications"]
	assert.True(t, ok)
	assert.Len(t, apps, 2)
}

// ---------------------------------------------------------------------------
// Delete / Hard-delete / Restore tests
// ---------------------------------------------------------------------------

func TestDeleteProject_NoFlags(t *testing.T) {
	orgID := uuid.New()
	svc := &mockEntityService{
		getOrgBySlugFn: func(_ context.Context, slug string) (*models.Organization, error) {
			return &models.Organization{ID: orgID, Name: "Test Org", Slug: slug}, nil
		},
		deleteProjectFn: func(_ context.Context, id uuid.UUID, slug string) (*models.DeleteResult, error) {
			assert.Equal(t, orgID, id)
			assert.Equal(t, "my-proj", slug)
			return &models.DeleteResult{Deleted: "permanent"}, nil
		},
	}
	router := setupEntityRouter(svc)

	req := httptest.NewRequest(http.MethodDelete, "/api/orgs/test-org/projects/my-proj", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "permanent", resp["deleted"])
}

func TestDeleteProject_ActiveFlags(t *testing.T) {
	orgID := uuid.New()
	svc := &mockEntityService{
		getOrgBySlugFn: func(_ context.Context, slug string) (*models.Organization, error) {
			return &models.Organization{ID: orgID, Name: "Test Org", Slug: slug}, nil
		},
		deleteProjectFn: func(_ context.Context, _ uuid.UUID, _ string) (*models.DeleteResult, error) {
			return &models.DeleteResult{
				ActiveFlags: []models.FlagActivitySummary{
					{Key: "flag-1", Name: "Flag One", LastEvaluated: time.Now()},
				},
			}, nil
		},
	}
	router := setupEntityRouter(svc)

	req := httptest.NewRequest(http.MethodDelete, "/api/orgs/test-org/projects/my-proj", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
	var resp map[string]interface{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "project has flags with recent activity", resp["error"])
	flags, ok := resp["active_flags"].([]interface{})
	assert.True(t, ok)
	assert.Len(t, flags, 1)
}

func TestDeleteProject_SoftDelete(t *testing.T) {
	orgID := uuid.New()
	eligible := time.Now().Add(30 * 24 * time.Hour)
	svc := &mockEntityService{
		getOrgBySlugFn: func(_ context.Context, slug string) (*models.Organization, error) {
			return &models.Organization{ID: orgID, Name: "Test Org", Slug: slug}, nil
		},
		deleteProjectFn: func(_ context.Context, _ uuid.UUID, _ string) (*models.DeleteResult, error) {
			return &models.DeleteResult{
				Deleted:               "soft",
				EligibleForHardDelete: &eligible,
			}, nil
		},
	}
	router := setupEntityRouter(svc)

	req := httptest.NewRequest(http.MethodDelete, "/api/orgs/test-org/projects/my-proj", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "soft", resp["deleted"])
	assert.NotEmpty(t, resp["eligible_for_hard_delete"])
}

func TestHardDeleteProject_TooEarly(t *testing.T) {
	orgID := uuid.New()
	eligible := time.Now().Add(30 * 24 * time.Hour)
	svc := &mockEntityService{
		getOrgBySlugFn: func(_ context.Context, slug string) (*models.Organization, error) {
			return &models.Organization{ID: orgID, Name: "Test Org", Slug: slug}, nil
		},
		hardDeleteProjectFn: func(_ context.Context, _ uuid.UUID, _ string) error {
			return fmt.Errorf("project not eligible for hard delete; eligible at %s", eligible.Format(time.RFC3339))
		},
	}
	router := setupEntityRouter(svc)

	req := httptest.NewRequest(http.MethodDelete, "/api/orgs/test-org/projects/my-proj/permanent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
	var resp map[string]interface{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Contains(t, resp["error"], "eligible at")
}

func TestRestoreProject(t *testing.T) {
	orgID := uuid.New()
	projectID := uuid.New()
	svc := &mockEntityService{
		getOrgBySlugFn: func(_ context.Context, slug string) (*models.Organization, error) {
			return &models.Organization{ID: orgID, Name: "Test Org", Slug: slug}, nil
		},
		restoreProjectFn: func(_ context.Context, id uuid.UUID, slug string) (*models.Project, error) {
			assert.Equal(t, orgID, id)
			return &models.Project{ID: projectID, OrgID: orgID, Name: "Restored", Slug: slug, DeletedAt: nil}, nil
		},
	}
	router := setupEntityRouter(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/orgs/test-org/projects/my-proj/restore", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp models.Project
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, projectID, resp.ID)
	assert.Nil(t, resp.DeletedAt)
}
