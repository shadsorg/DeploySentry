# Platform Redesign — Remaining Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Complete the platform redesign by building the missing backend CRUD layers (Application, Settings, FlagEnvironmentState), new CLI commands, CLI installer script, and a full web UI overhaul with org-centric navigation.

**Architecture:** Three independent workstreams — (1) Backend: add missing postgres repositories, service layers, and HTTP handlers following the established repo→service→handler pattern with Gin; (2) CLI: add `org`, `app`, `settings`, `apikey` subcommands plus an install script; (3) Frontend: redesign the React SPA with org switcher, accordion sidebar, hierarchical routing, and new pages for applications, per-env flag state, and settings. Each workstream produces working, testable software independently.

**Tech Stack:** Go 1.22+, pgx v5, Gin, Cobra (CLI), React 18, React Router 6, TypeScript, Vite, custom CSS

---

## Phase 1: Backend — Missing Repositories, Services, and Handlers

### Task 1: Application Repository (Postgres)

**Files:**
- Create: `internal/applications/repository.go`
- Create: `internal/platform/database/postgres/applications.go`

- [ ] **Step 1: Create the repository interface**

```go
// internal/applications/repository.go
package applications

import (
	"context"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
)

// ApplicationRepository defines the persistence interface for application entities.
type ApplicationRepository interface {
	Create(ctx context.Context, app *models.Application) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Application, error)
	GetBySlug(ctx context.Context, projectID uuid.UUID, slug string) (*models.Application, error)
	ListByProject(ctx context.Context, projectID uuid.UUID) ([]models.Application, error)
	Update(ctx context.Context, app *models.Application) error
	Delete(ctx context.Context, id uuid.UUID) error
}
```

- [ ] **Step 2: Create the postgres implementation**

```go
// internal/platform/database/postgres/applications.go
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ApplicationRepository struct {
	pool *pgxpool.Pool
}

func NewApplicationRepository(pool *pgxpool.Pool) *ApplicationRepository {
	return &ApplicationRepository{pool: pool}
}

const applicationSelectCols = `
	id, project_id, name, slug,
	COALESCE(description, ''), COALESCE(repo_url, ''),
	created_by, created_at, updated_at`

func scanApplication(row pgx.Row) (*models.Application, error) {
	var a models.Application
	err := row.Scan(
		&a.ID, &a.ProjectID, &a.Name, &a.Slug,
		&a.Description, &a.RepoURL,
		&a.CreatedBy, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &a, nil
}

func (r *ApplicationRepository) Create(ctx context.Context, app *models.Application) error {
	if app.ID == uuid.Nil {
		app.ID = uuid.New()
	}
	now := time.Now().UTC()
	app.CreatedAt = now
	app.UpdatedAt = now

	const q = `
		INSERT INTO applications
			(id, project_id, name, slug, description, repo_url, created_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	_, err := r.pool.Exec(ctx, q,
		app.ID, app.ProjectID, app.Name, app.Slug,
		app.Description, app.RepoURL,
		app.CreatedBy, app.CreatedAt, app.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrConflict
		}
		return fmt.Errorf("postgres.CreateApplication: %w", err)
	}
	return nil
}

func (r *ApplicationRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Application, error) {
	q := `SELECT` + applicationSelectCols + ` FROM applications WHERE id = $1`
	app, err := scanApplication(r.pool.QueryRow(ctx, q, id))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("postgres.GetApplicationByID: %w", err)
	}
	return app, nil
}

func (r *ApplicationRepository) GetBySlug(ctx context.Context, projectID uuid.UUID, slug string) (*models.Application, error) {
	q := `SELECT` + applicationSelectCols + ` FROM applications WHERE project_id = $1 AND slug = $2`
	app, err := scanApplication(r.pool.QueryRow(ctx, q, projectID, slug))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("postgres.GetApplicationBySlug: %w", err)
	}
	return app, nil
}

func (r *ApplicationRepository) ListByProject(ctx context.Context, projectID uuid.UUID) ([]models.Application, error) {
	q := `SELECT` + applicationSelectCols + ` FROM applications WHERE project_id = $1 ORDER BY name ASC`
	rows, err := r.pool.Query(ctx, q, projectID)
	if err != nil {
		return nil, fmt.Errorf("postgres.ListApplicationsByProject: %w", err)
	}
	defer rows.Close()

	var result []models.Application
	for rows.Next() {
		app, err := scanApplication(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres.ListApplicationsByProject: %w", err)
		}
		result = append(result, *app)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres.ListApplicationsByProject: %w", err)
	}
	return result, nil
}

func (r *ApplicationRepository) Update(ctx context.Context, app *models.Application) error {
	app.UpdatedAt = time.Now().UTC()

	const q = `
		UPDATE applications SET
			name = $2, slug = $3, description = $4, repo_url = $5, updated_at = $6
		WHERE id = $1`

	tag, err := r.pool.Exec(ctx, q,
		app.ID, app.Name, app.Slug, app.Description, app.RepoURL, app.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrConflict
		}
		return fmt.Errorf("postgres.UpdateApplication: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *ApplicationRepository) Delete(ctx context.Context, id uuid.UUID) error {
	const q = `DELETE FROM applications WHERE id = $1`
	tag, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("postgres.DeleteApplication: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
```

- [ ] **Step 3: Verify it compiles**

Run: `cd /Users/sgamel/git/DeploySentry && go build ./internal/... 2>&1 | head -20`
Expected: No errors for the applications or postgres packages.

- [ ] **Step 4: Commit**

```bash
git add internal/applications/repository.go internal/platform/database/postgres/applications.go
git commit -m "feat: add Application repository interface and postgres implementation"
```

---

### Task 2: Application Service and Handler

**Files:**
- Create: `internal/applications/service.go`
- Create: `internal/applications/handler.go`
- Modify: `cmd/api/main.go` (wire up routes)

- [ ] **Step 1: Create the service layer**

```go
// internal/applications/service.go
package applications

import (
	"context"
	"fmt"
	"time"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
)

// ApplicationService defines the interface for managing applications.
type ApplicationService interface {
	Create(ctx context.Context, app *models.Application) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Application, error)
	GetBySlug(ctx context.Context, projectID uuid.UUID, slug string) (*models.Application, error)
	ListByProject(ctx context.Context, projectID uuid.UUID) ([]models.Application, error)
	Update(ctx context.Context, app *models.Application) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type applicationService struct {
	repo ApplicationRepository
}

func NewApplicationService(repo ApplicationRepository) ApplicationService {
	return &applicationService{repo: repo}
}

func (s *applicationService) Create(ctx context.Context, app *models.Application) error {
	if app.ID == uuid.Nil {
		app.ID = uuid.New()
	}
	now := time.Now().UTC()
	app.CreatedAt = now
	app.UpdatedAt = now

	if err := app.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	if err := s.repo.Create(ctx, app); err != nil {
		return fmt.Errorf("creating application: %w", err)
	}
	return nil
}

func (s *applicationService) GetByID(ctx context.Context, id uuid.UUID) (*models.Application, error) {
	app, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("getting application: %w", err)
	}
	return app, nil
}

func (s *applicationService) GetBySlug(ctx context.Context, projectID uuid.UUID, slug string) (*models.Application, error) {
	app, err := s.repo.GetBySlug(ctx, projectID, slug)
	if err != nil {
		return nil, fmt.Errorf("getting application by slug: %w", err)
	}
	return app, nil
}

func (s *applicationService) ListByProject(ctx context.Context, projectID uuid.UUID) ([]models.Application, error) {
	apps, err := s.repo.ListByProject(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("listing applications: %w", err)
	}
	return apps, nil
}

func (s *applicationService) Update(ctx context.Context, app *models.Application) error {
	if err := app.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	if err := s.repo.Update(ctx, app); err != nil {
		return fmt.Errorf("updating application: %w", err)
	}
	return nil
}

func (s *applicationService) Delete(ctx context.Context, id uuid.UUID) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("deleting application: %w", err)
	}
	return nil
}
```

- [ ] **Step 2: Create the HTTP handler**

```go
// internal/applications/handler.go
package applications

import (
	"net/http"

	"github.com/deploysentry/deploysentry/internal/auth"
	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Handler provides HTTP endpoints for managing applications.
type Handler struct {
	service ApplicationService
	rbac    *auth.RBACChecker
}

// NewHandler creates a new application HTTP handler.
func NewHandler(service ApplicationService) *Handler {
	return &Handler{
		service: service,
		rbac:    auth.NewRBACChecker(),
	}
}

// RegisterRoutes mounts application API routes.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	projects := rg.Group("/projects/:project_id/applications")
	{
		projects.POST("", auth.RequirePermission(h.rbac, auth.PermProjectManage), h.createApplication)
		projects.GET("", auth.RequirePermission(h.rbac, auth.PermDeployRead), h.listApplications)
	}

	apps := rg.Group("/applications")
	{
		apps.GET("/:id", auth.RequirePermission(h.rbac, auth.PermDeployRead), h.getApplication)
		apps.PUT("/:id", auth.RequirePermission(h.rbac, auth.PermProjectManage), h.updateApplication)
		apps.DELETE("/:id", auth.RequirePermission(h.rbac, auth.PermProjectManage), h.deleteApplication)
	}
}

type createApplicationRequest struct {
	Name        string `json:"name" binding:"required"`
	Slug        string `json:"slug" binding:"required"`
	Description string `json:"description"`
	RepoURL     string `json:"repo_url"`
}

func (h *Handler) createApplication(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("project_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project id"})
		return
	}

	var req createApplicationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var createdBy *uuid.UUID
	if userID, exists := c.Get("user_id"); exists {
		if uid, ok := userID.(uuid.UUID); ok {
			createdBy = &uid
		}
	}

	app := &models.Application{
		ProjectID:   projectID,
		Name:        req.Name,
		Slug:        req.Slug,
		Description: req.Description,
		RepoURL:     req.RepoURL,
		CreatedBy:   createdBy,
	}

	if err := h.service.Create(c.Request.Context(), app); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, app)
}

func (h *Handler) getApplication(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid application id"})
		return
	}

	app, err := h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "application not found"})
		return
	}

	c.JSON(http.StatusOK, app)
}

func (h *Handler) listApplications(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("project_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project id"})
		return
	}

	apps, err := h.service.ListByProject(c.Request.Context(), projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list applications"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"applications": apps})
}

type updateApplicationRequest struct {
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	RepoURL     string `json:"repo_url"`
}

func (h *Handler) updateApplication(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid application id"})
		return
	}

	app, err := h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "application not found"})
		return
	}

	var req updateApplicationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Name != "" {
		app.Name = req.Name
	}
	if req.Slug != "" {
		app.Slug = req.Slug
	}
	if req.Description != "" {
		app.Description = req.Description
	}
	if req.RepoURL != "" {
		app.RepoURL = req.RepoURL
	}

	if err := h.service.Update(c.Request.Context(), app); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, app)
}

func (h *Handler) deleteApplication(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid application id"})
		return
	}

	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}
```

- [ ] **Step 3: Wire into main.go**

In `cmd/api/main.go`, add to imports:
```go
"github.com/deploysentry/deploysentry/internal/applications"
```

After the existing repository declarations (around line 180), add:
```go
appRepo := postgres.NewApplicationRepository(db.Pool)
```

After the existing service declarations (around line 192), add:
```go
appService := applications.NewApplicationService(appRepo)
```

After the existing route registrations (around line 268), add:
```go
applications.NewHandler(appService).RegisterRoutes(api)
```

- [ ] **Step 4: Verify it compiles**

Run: `cd /Users/sgamel/git/DeploySentry && go build ./cmd/api/ 2>&1 | head -20`
Expected: Clean compilation.

- [ ] **Step 5: Commit**

```bash
git add internal/applications/service.go internal/applications/handler.go cmd/api/main.go
git commit -m "feat: add Application service, handler, and route registration"
```

---

### Task 3: Settings Repository, Service, and Handler

**Files:**
- Create: `internal/settings/repository.go`
- Create: `internal/settings/service.go`
- Create: `internal/settings/handler.go`
- Create: `internal/platform/database/postgres/settings.go`
- Modify: `cmd/api/main.go`

- [ ] **Step 1: Create the repository interface**

```go
// internal/settings/repository.go
package settings

import (
	"context"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
)

// SettingRepository defines the persistence interface for settings.
type SettingRepository interface {
	Set(ctx context.Context, setting *models.Setting) error
	Get(ctx context.Context, id uuid.UUID) (*models.Setting, error)
	ListByScope(ctx context.Context, scope string, targetID uuid.UUID) ([]models.Setting, error)
	Delete(ctx context.Context, id uuid.UUID) error
	// Resolve walks up the hierarchy: env → app → project → org, returning the first match.
	Resolve(ctx context.Context, key string, envID, appID, projectID, orgID uuid.UUID) (*models.Setting, error)
}
```

- [ ] **Step 2: Create the postgres implementation**

```go
// internal/platform/database/postgres/settings.go
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SettingRepository struct {
	pool *pgxpool.Pool
}

func NewSettingRepository(pool *pgxpool.Pool) *SettingRepository {
	return &SettingRepository{pool: pool}
}

const settingSelectCols = `id, org_id, project_id, application_id, environment_id, key, value, updated_by, updated_at`

func scanSetting(row pgx.Row) (*models.Setting, error) {
	var s models.Setting
	err := row.Scan(
		&s.ID, &s.OrgID, &s.ProjectID, &s.ApplicationID, &s.EnvironmentID,
		&s.Key, &s.Value, &s.UpdatedBy, &s.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &s, nil
}

func (r *SettingRepository) Set(ctx context.Context, setting *models.Setting) error {
	if setting.ID == uuid.Nil {
		setting.ID = uuid.New()
	}
	setting.UpdatedAt = time.Now().UTC()

	const q = `
		INSERT INTO settings
			(id, org_id, project_id, application_id, environment_id, key, value, updated_by, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT ON CONSTRAINT chk_settings_single_scope DO NOTHING`

	// Use upsert via partial unique indexes. We need to determine which index applies.
	// Since each scope has its own unique index, use a conditional upsert approach.
	var upsertQ string
	var args []interface{}

	switch {
	case setting.OrgID != nil:
		upsertQ = `
			INSERT INTO settings (id, org_id, project_id, application_id, environment_id, key, value, updated_by, updated_at)
			VALUES ($1, $2, NULL, NULL, NULL, $3, $4, $5, $6)
			ON CONFLICT (org_id, key) WHERE org_id IS NOT NULL
			DO UPDATE SET value = EXCLUDED.value, updated_by = EXCLUDED.updated_by, updated_at = EXCLUDED.updated_at
			RETURNING ` + settingSelectCols
		args = []interface{}{setting.ID, *setting.OrgID, setting.Key, setting.Value, setting.UpdatedBy, setting.UpdatedAt}
	case setting.ProjectID != nil:
		upsertQ = `
			INSERT INTO settings (id, org_id, project_id, application_id, environment_id, key, value, updated_by, updated_at)
			VALUES ($1, NULL, $2, NULL, NULL, $3, $4, $5, $6)
			ON CONFLICT (project_id, key) WHERE project_id IS NOT NULL
			DO UPDATE SET value = EXCLUDED.value, updated_by = EXCLUDED.updated_by, updated_at = EXCLUDED.updated_at
			RETURNING ` + settingSelectCols
		args = []interface{}{setting.ID, *setting.ProjectID, setting.Key, setting.Value, setting.UpdatedBy, setting.UpdatedAt}
	case setting.ApplicationID != nil:
		upsertQ = `
			INSERT INTO settings (id, org_id, project_id, application_id, environment_id, key, value, updated_by, updated_at)
			VALUES ($1, NULL, NULL, $2, NULL, $3, $4, $5, $6)
			ON CONFLICT (application_id, key) WHERE application_id IS NOT NULL
			DO UPDATE SET value = EXCLUDED.value, updated_by = EXCLUDED.updated_by, updated_at = EXCLUDED.updated_at
			RETURNING ` + settingSelectCols
		args = []interface{}{setting.ID, *setting.ApplicationID, setting.Key, setting.Value, setting.UpdatedBy, setting.UpdatedAt}
	case setting.EnvironmentID != nil:
		upsertQ = `
			INSERT INTO settings (id, org_id, project_id, application_id, environment_id, key, value, updated_by, updated_at)
			VALUES ($1, NULL, NULL, NULL, $2, $3, $4, $5, $6)
			ON CONFLICT (environment_id, key) WHERE environment_id IS NOT NULL
			DO UPDATE SET value = EXCLUDED.value, updated_by = EXCLUDED.updated_by, updated_at = EXCLUDED.updated_at
			RETURNING ` + settingSelectCols
		args = []interface{}{setting.ID, *setting.EnvironmentID, setting.Key, setting.Value, setting.UpdatedBy, setting.UpdatedAt}
	default:
		return fmt.Errorf("postgres.SetSetting: no scope set")
	}

	result, err := scanSetting(r.pool.QueryRow(ctx, upsertQ, args...))
	if err != nil {
		return fmt.Errorf("postgres.SetSetting: %w", err)
	}
	*setting = *result
	return nil
}

func (r *SettingRepository) Get(ctx context.Context, id uuid.UUID) (*models.Setting, error) {
	q := `SELECT ` + settingSelectCols + ` FROM settings WHERE id = $1`
	s, err := scanSetting(r.pool.QueryRow(ctx, q, id))
	if err != nil {
		return nil, fmt.Errorf("postgres.GetSetting: %w", err)
	}
	return s, nil
}

func (r *SettingRepository) ListByScope(ctx context.Context, scope string, targetID uuid.UUID) ([]models.Setting, error) {
	var col string
	switch scope {
	case "org":
		col = "org_id"
	case "project":
		col = "project_id"
	case "application":
		col = "application_id"
	case "environment":
		col = "environment_id"
	default:
		return nil, fmt.Errorf("postgres.ListSettings: invalid scope %q", scope)
	}

	q := fmt.Sprintf(`SELECT %s FROM settings WHERE %s = $1 ORDER BY key ASC`, settingSelectCols, col)
	rows, err := r.pool.Query(ctx, q, targetID)
	if err != nil {
		return nil, fmt.Errorf("postgres.ListSettings: %w", err)
	}
	defer rows.Close()

	var result []models.Setting
	for rows.Next() {
		s, err := scanSetting(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres.ListSettings: %w", err)
		}
		result = append(result, *s)
	}
	return result, rows.Err()
}

func (r *SettingRepository) Delete(ctx context.Context, id uuid.UUID) error {
	const q = `DELETE FROM settings WHERE id = $1`
	tag, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("postgres.DeleteSetting: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *SettingRepository) Resolve(ctx context.Context, key string, envID, appID, projectID, orgID uuid.UUID) (*models.Setting, error) {
	// Walk up the hierarchy: environment → application → project → org
	scopes := []struct {
		col string
		id  uuid.UUID
	}{
		{"environment_id", envID},
		{"application_id", appID},
		{"project_id", projectID},
		{"org_id", orgID},
	}

	for _, scope := range scopes {
		if scope.id == uuid.Nil {
			continue
		}
		q := fmt.Sprintf(`SELECT %s FROM settings WHERE %s = $1 AND key = $2`, settingSelectCols, scope.col)
		s, err := scanSetting(r.pool.QueryRow(ctx, q, scope.id, key))
		if err == nil {
			return s, nil
		}
		if !errors.Is(err, ErrNotFound) {
			return nil, fmt.Errorf("postgres.ResolveSetting: %w", err)
		}
	}
	return nil, ErrNotFound
}
```

- [ ] **Step 3: Create the service**

```go
// internal/settings/service.go
package settings

import (
	"context"
	"fmt"
	"time"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
)

type SettingService interface {
	Set(ctx context.Context, setting *models.Setting) error
	Get(ctx context.Context, id uuid.UUID) (*models.Setting, error)
	ListByScope(ctx context.Context, scope string, targetID uuid.UUID) ([]models.Setting, error)
	Resolve(ctx context.Context, key string, envID, appID, projectID, orgID uuid.UUID) (*models.Setting, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type settingService struct {
	repo SettingRepository
}

func NewSettingService(repo SettingRepository) SettingService {
	return &settingService{repo: repo}
}

func (s *settingService) Set(ctx context.Context, setting *models.Setting) error {
	if setting.ID == uuid.Nil {
		setting.ID = uuid.New()
	}
	setting.UpdatedAt = time.Now().UTC()

	if err := setting.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	if err := s.repo.Set(ctx, setting); err != nil {
		return fmt.Errorf("setting value: %w", err)
	}
	return nil
}

func (s *settingService) Get(ctx context.Context, id uuid.UUID) (*models.Setting, error) {
	return s.repo.Get(ctx, id)
}

func (s *settingService) ListByScope(ctx context.Context, scope string, targetID uuid.UUID) ([]models.Setting, error) {
	return s.repo.ListByScope(ctx, scope, targetID)
}

func (s *settingService) Resolve(ctx context.Context, key string, envID, appID, projectID, orgID uuid.UUID) (*models.Setting, error) {
	return s.repo.Resolve(ctx, key, envID, appID, projectID, orgID)
}

func (s *settingService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}
```

- [ ] **Step 4: Create the handler**

```go
// internal/settings/handler.go
package settings

import (
	"encoding/json"
	"net/http"

	"github.com/deploysentry/deploysentry/internal/auth"
	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	service SettingService
	rbac    *auth.RBACChecker
}

func NewHandler(service SettingService) *Handler {
	return &Handler{
		service: service,
		rbac:    auth.NewRBACChecker(),
	}
}

func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	s := rg.Group("/settings")
	{
		s.PUT("", auth.RequirePermission(h.rbac, auth.PermProjectManage), h.setSetting)
		s.GET("", auth.RequirePermission(h.rbac, auth.PermDeployRead), h.listSettings)
		s.GET("/resolve", auth.RequirePermission(h.rbac, auth.PermDeployRead), h.resolveSetting)
		s.DELETE("/:id", auth.RequirePermission(h.rbac, auth.PermProjectManage), h.deleteSetting)
	}
}

type setSettingRequest struct {
	Scope    string          `json:"scope" binding:"required"`
	TargetID uuid.UUID       `json:"target_id" binding:"required"`
	Key      string          `json:"key" binding:"required"`
	Value    json.RawMessage `json:"value" binding:"required"`
}

func (h *Handler) setSetting(c *gin.Context) {
	var req setSettingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var updatedBy *uuid.UUID
	if userID, exists := c.Get("user_id"); exists {
		if uid, ok := userID.(uuid.UUID); ok {
			updatedBy = &uid
		}
	}

	setting := &models.Setting{
		Key:       req.Key,
		Value:     req.Value,
		UpdatedBy: updatedBy,
	}

	switch req.Scope {
	case "org":
		setting.OrgID = &req.TargetID
	case "project":
		setting.ProjectID = &req.TargetID
	case "application":
		setting.ApplicationID = &req.TargetID
	case "environment":
		setting.EnvironmentID = &req.TargetID
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "scope must be org, project, application, or environment"})
		return
	}

	if err := h.service.Set(c.Request.Context(), setting); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, setting)
}

func (h *Handler) listSettings(c *gin.Context) {
	scope := c.Query("scope")
	targetIDStr := c.Query("target")
	if scope == "" || targetIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "scope and target query parameters are required"})
		return
	}

	targetID, err := uuid.Parse(targetIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid target id"})
		return
	}

	settings, err := h.service.ListByScope(c.Request.Context(), scope, targetID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list settings"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"settings": settings})
}

func (h *Handler) resolveSetting(c *gin.Context) {
	key := c.Query("key")
	if key == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "key query parameter is required"})
		return
	}

	envID, _ := uuid.Parse(c.Query("environment_id"))
	appID, _ := uuid.Parse(c.Query("application_id"))
	projectID, _ := uuid.Parse(c.Query("project_id"))
	orgID, _ := uuid.Parse(c.Query("org_id"))

	setting, err := h.service.Resolve(c.Request.Context(), key, envID, appID, projectID, orgID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "setting not found"})
		return
	}

	c.JSON(http.StatusOK, setting)
}

func (h *Handler) deleteSetting(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid setting id"})
		return
	}

	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}
```

- [ ] **Step 5: Wire into main.go**

Add import `"github.com/deploysentry/deploysentry/internal/settings"`.

After appRepo line, add:
```go
settingRepo := postgres.NewSettingRepository(db.Pool)
```
After appService line, add:
```go
settingService := settings.NewSettingService(settingRepo)
```
After applications route registration, add:
```go
settings.NewHandler(settingService).RegisterRoutes(api)
```

- [ ] **Step 6: Verify compilation**

Run: `cd /Users/sgamel/git/DeploySentry && go build ./cmd/api/ 2>&1 | head -20`

- [ ] **Step 7: Commit**

```bash
git add internal/settings/ internal/platform/database/postgres/settings.go cmd/api/main.go
git commit -m "feat: add Settings repository, service, handler with cascading resolution"
```

---

### Task 4: FlagEnvironmentState Repository and Handler

**Files:**
- Create: `internal/platform/database/postgres/flag_env_state.go`
- Modify: `internal/flags/repository.go` (add methods to interface)
- Modify: `internal/flags/service.go` (add methods)
- Modify: `internal/flags/handler.go` (add endpoints)
- Modify: `internal/platform/database/postgres/flags.go` (implement new methods)
- Modify: `cmd/api/main.go` (no change needed — flags handler already registered)

- [ ] **Step 1: Add FlagEnvironmentState methods to the FlagRepository interface**

In `internal/flags/repository.go`, add these methods to the `FlagRepository` interface:

```go
	// GetFlagEnvironmentStates returns all environment states for a flag.
	GetFlagEnvironmentStates(ctx context.Context, flagID uuid.UUID) ([]models.FlagEnvironmentState, error)

	// SetFlagEnvironmentState creates or updates the state for a flag in an environment.
	SetFlagEnvironmentState(ctx context.Context, state *models.FlagEnvironmentState) error
```

- [ ] **Step 2: Implement in postgres/flags.go**

Append to `internal/platform/database/postgres/flags.go`:

```go
const flagEnvStateSelectCols = `id, flag_id, environment_id, enabled, value, updated_by, updated_at`

func scanFlagEnvState(row pgx.Row) (*models.FlagEnvironmentState, error) {
	var s models.FlagEnvironmentState
	err := row.Scan(&s.ID, &s.FlagID, &s.EnvironmentID, &s.Enabled, &s.Value, &s.UpdatedBy, &s.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &s, nil
}

func (r *FlagRepository) GetFlagEnvironmentStates(ctx context.Context, flagID uuid.UUID) ([]models.FlagEnvironmentState, error) {
	q := `SELECT ` + flagEnvStateSelectCols + ` FROM flag_environment_state WHERE flag_id = $1 ORDER BY environment_id`
	rows, err := r.pool.Query(ctx, q, flagID)
	if err != nil {
		return nil, fmt.Errorf("postgres.GetFlagEnvironmentStates: %w", err)
	}
	defer rows.Close()

	var result []models.FlagEnvironmentState
	for rows.Next() {
		s, err := scanFlagEnvState(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres.GetFlagEnvironmentStates: %w", err)
		}
		result = append(result, *s)
	}
	return result, rows.Err()
}

func (r *FlagRepository) SetFlagEnvironmentState(ctx context.Context, state *models.FlagEnvironmentState) error {
	if state.ID == uuid.Nil {
		state.ID = uuid.New()
	}
	state.UpdatedAt = time.Now().UTC()

	const q = `
		INSERT INTO flag_environment_state (id, flag_id, environment_id, enabled, value, updated_by, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (flag_id, environment_id)
		DO UPDATE SET enabled = EXCLUDED.enabled, value = EXCLUDED.value,
			updated_by = EXCLUDED.updated_by, updated_at = EXCLUDED.updated_at
		RETURNING ` + flagEnvStateSelectCols

	result, err := scanFlagEnvState(r.pool.QueryRow(ctx, q,
		state.ID, state.FlagID, state.EnvironmentID,
		state.Enabled, state.Value, state.UpdatedBy, state.UpdatedAt,
	))
	if err != nil {
		return fmt.Errorf("postgres.SetFlagEnvironmentState: %w", err)
	}
	*state = *result
	return nil
}
```

- [ ] **Step 3: Add service methods**

In `internal/flags/service.go`, add to the `FlagService` interface:

```go
	GetFlagEnvironmentStates(ctx context.Context, flagID uuid.UUID) ([]models.FlagEnvironmentState, error)
	SetFlagEnvironmentState(ctx context.Context, state *models.FlagEnvironmentState) error
```

Add implementations to the concrete `flagService` struct (append to the file):

```go
func (s *flagService) GetFlagEnvironmentStates(ctx context.Context, flagID uuid.UUID) ([]models.FlagEnvironmentState, error) {
	return s.repo.GetFlagEnvironmentStates(ctx, flagID)
}

func (s *flagService) SetFlagEnvironmentState(ctx context.Context, state *models.FlagEnvironmentState) error {
	if err := state.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}
	return s.repo.SetFlagEnvironmentState(ctx, state)
}
```

- [ ] **Step 4: Add handler endpoints**

In `internal/flags/handler.go`, add to `RegisterRoutes` inside the `flags` group:

```go
		flags.GET("/:id/environments", auth.RequirePermission(h.rbac, auth.PermFlagRead), h.getFlagEnvironmentStates)
		flags.PUT("/:id/environments/:env_id", auth.RequirePermission(h.rbac, auth.PermFlagUpdate), h.setFlagEnvironmentState)
```

Add the handler methods:

```go
func (h *Handler) getFlagEnvironmentStates(c *gin.Context) {
	flagID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid flag id"})
		return
	}

	states, err := h.service.GetFlagEnvironmentStates(c.Request.Context(), flagID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get environment states"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"environment_states": states})
}

type setFlagEnvStateRequest struct {
	Enabled bool             `json:"enabled"`
	Value   *json.RawMessage `json:"value"`
}

func (h *Handler) setFlagEnvironmentState(c *gin.Context) {
	flagID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid flag id"})
		return
	}

	envID, err := uuid.Parse(c.Param("env_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid environment id"})
		return
	}

	var req setFlagEnvStateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var updatedBy *uuid.UUID
	if userID, exists := c.Get("user_id"); exists {
		if uid, ok := userID.(uuid.UUID); ok {
			updatedBy = &uid
		}
	}

	state := &models.FlagEnvironmentState{
		FlagID:        flagID,
		EnvironmentID: envID,
		Enabled:       req.Enabled,
		Value:         req.Value,
		UpdatedBy:     updatedBy,
	}

	if err := h.service.SetFlagEnvironmentState(c.Request.Context(), state); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, state)
}
```

Add `"encoding/json"` to the imports if not already present.

- [ ] **Step 5: Verify compilation**

Run: `cd /Users/sgamel/git/DeploySentry && go build ./cmd/api/ 2>&1 | head -20`

- [ ] **Step 6: Commit**

```bash
git add internal/flags/ internal/platform/database/postgres/flags.go
git commit -m "feat: add per-environment flag state endpoints (GET/PUT /flags/:id/environments)"
```

---

## Phase 2: CLI — New Commands and Installer

### Task 5: CLI `org` commands

**Files:**
- Create: `cmd/cli/orgs.go`

- [ ] **Step 1: Create org subcommands**

```go
// cmd/cli/orgs.go
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var orgCmd = &cobra.Command{
	Use:   "org",
	Short: "Manage organizations",
}

var orgCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new organization",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		slug, _ := cmd.Flags().GetString("slug")

		client, err := clientFromConfig()
		if err != nil {
			return err
		}

		body := map[string]string{"name": name, "slug": slug}
		resp, err := client.post("/api/v1/orgs", body)
		if err != nil {
			return fmt.Errorf("creating organization: %w", err)
		}

		if outputFormat == "json" {
			fmt.Println(string(resp))
			return nil
		}

		fmt.Printf("%s Organization created successfully%s\n", colorGreen, colorReset)
		return nil
	},
}

var orgListCmd = &cobra.Command{
	Use:   "list",
	Short: "List organizations",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := clientFromConfig()
		if err != nil {
			return err
		}

		resp, err := client.get("/api/v1/orgs")
		if err != nil {
			return fmt.Errorf("listing organizations: %w", err)
		}

		if outputFormat == "json" {
			fmt.Println(string(resp))
			return nil
		}

		var result struct {
			Organizations []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
				Slug string `json:"slug"`
			} `json:"organizations"`
		}
		if err := json.Unmarshal(resp, &result); err != nil {
			return fmt.Errorf("parsing response: %w", err)
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "ID\tNAME\tSLUG\n")
		for _, org := range result.Organizations {
			fmt.Fprintf(w, "%s\t%s\t%s\n", org.ID, org.Name, org.Slug)
		}
		return w.Flush()
	},
}

var orgSwitchCmd = &cobra.Command{
	Use:   "switch <slug>",
	Short: "Set active organization",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		slug := args[0]
		cfg, err := loadConfig()
		if err != nil {
			cfg = &cliConfig{}
		}
		cfg.ActiveOrg = slug
		if err := saveConfig(cfg); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}
		fmt.Printf("%s Switched to organization: %s%s\n", colorGreen, slug, colorReset)
		return nil
	},
}

func init() {
	orgCreateCmd.Flags().String("name", "", "Organization name (required)")
	orgCreateCmd.Flags().String("slug", "", "Organization slug (required)")
	_ = orgCreateCmd.MarkFlagRequired("name")
	_ = orgCreateCmd.MarkFlagRequired("slug")

	orgCmd.AddCommand(orgCreateCmd, orgListCmd, orgSwitchCmd)
	rootCmd.AddCommand(orgCmd)
}
```

- [ ] **Step 2: Add ActiveOrg to config struct**

In `cmd/cli/config.go`, add `ActiveOrg string` to the `cliConfig` struct if not already present. Also add `ActiveApp string`.

- [ ] **Step 3: Verify compilation**

Run: `cd /Users/sgamel/git/DeploySentry && go build ./cmd/cli/ 2>&1 | head -20`

- [ ] **Step 4: Commit**

```bash
git add cmd/cli/orgs.go cmd/cli/config.go
git commit -m "feat: add CLI org create/list/switch commands"
```

---

### Task 6: CLI `app` commands

**Files:**
- Create: `cmd/cli/apps.go`

- [ ] **Step 1: Create app subcommands**

```go
// cmd/cli/apps.go
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var appCmd = &cobra.Command{
	Use:   "app",
	Short: "Manage applications",
}

var appCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new application",
	RunE: func(cmd *cobra.Command, args []string) error {
		project, _ := cmd.Flags().GetString("project")
		if project == "" {
			project = getProject()
		}
		name, _ := cmd.Flags().GetString("name")
		slug, _ := cmd.Flags().GetString("slug")
		description, _ := cmd.Flags().GetString("description")
		repoURL, _ := cmd.Flags().GetString("repo-url")

		client, err := clientFromConfig()
		if err != nil {
			return err
		}

		body := map[string]string{
			"name":        name,
			"slug":        slug,
			"description": description,
			"repo_url":    repoURL,
		}

		org := getOrg()
		resp, err := client.post(fmt.Sprintf("/api/v1/projects/%s/applications", project), body)
		if err != nil {
			return fmt.Errorf("creating application: %w", err)
		}

		_ = org // org used for context display
		if outputFormat == "json" {
			fmt.Println(string(resp))
			return nil
		}

		fmt.Printf("%s Application created successfully%s\n", colorGreen, colorReset)
		return nil
	},
}

var appListCmd = &cobra.Command{
	Use:   "list",
	Short: "List applications in a project",
	RunE: func(cmd *cobra.Command, args []string) error {
		project, _ := cmd.Flags().GetString("project")
		if project == "" {
			project = getProject()
		}

		client, err := clientFromConfig()
		if err != nil {
			return err
		}

		resp, err := client.get(fmt.Sprintf("/api/v1/projects/%s/applications", project))
		if err != nil {
			return fmt.Errorf("listing applications: %w", err)
		}

		if outputFormat == "json" {
			fmt.Println(string(resp))
			return nil
		}

		var result struct {
			Applications []struct {
				ID          string `json:"id"`
				Name        string `json:"name"`
				Slug        string `json:"slug"`
				Description string `json:"description"`
			} `json:"applications"`
		}
		if err := json.Unmarshal(resp, &result); err != nil {
			return fmt.Errorf("parsing response: %w", err)
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "ID\tNAME\tSLUG\tDESCRIPTION\n")
		for _, a := range result.Applications {
			desc := a.Description
			if len(desc) > 40 {
				desc = desc[:37] + "..."
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", a.ID, a.Name, a.Slug, desc)
		}
		return w.Flush()
	},
}

var appDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete an application",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := clientFromConfig()
		if err != nil {
			return err
		}

		_, err = client.delete(fmt.Sprintf("/api/v1/applications/%s", args[0]))
		if err != nil {
			return fmt.Errorf("deleting application: %w", err)
		}

		fmt.Printf("%s Application deleted%s\n", colorGreen, colorReset)
		return nil
	},
}

func init() {
	appCreateCmd.Flags().String("project", "", "Project ID or slug")
	appCreateCmd.Flags().String("name", "", "Application name (required)")
	appCreateCmd.Flags().String("slug", "", "Application slug (required)")
	appCreateCmd.Flags().String("description", "", "Description")
	appCreateCmd.Flags().String("repo-url", "", "Repository URL")
	_ = appCreateCmd.MarkFlagRequired("name")
	_ = appCreateCmd.MarkFlagRequired("slug")

	appListCmd.Flags().String("project", "", "Project ID or slug")

	appCmd.AddCommand(appCreateCmd, appListCmd, appDeleteCmd)
	rootCmd.AddCommand(appCmd)
}
```

- [ ] **Step 2: Verify compilation**

Run: `cd /Users/sgamel/git/DeploySentry && go build ./cmd/cli/ 2>&1 | head -20`

- [ ] **Step 3: Commit**

```bash
git add cmd/cli/apps.go
git commit -m "feat: add CLI app create/list/delete commands"
```

---

### Task 7: CLI `settings` and `apikey` commands

**Files:**
- Create: `cmd/cli/settings.go`
- Create: `cmd/cli/apikeys.go`

- [ ] **Step 1: Create settings subcommands**

```go
// cmd/cli/settings.go
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var settingsCmd = &cobra.Command{
	Use:   "settings",
	Short: "Manage hierarchical settings",
}

var settingsSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value at a scope",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		scope, _ := cmd.Flags().GetString("scope")
		target, _ := cmd.Flags().GetString("target")

		client, err := clientFromConfig()
		if err != nil {
			return err
		}

		// Wrap the value as JSON
		rawValue := json.RawMessage(fmt.Sprintf("%q", args[1]))
		body := map[string]interface{}{
			"scope":     scope,
			"target_id": target,
			"key":       args[0],
			"value":     rawValue,
		}

		resp, err := client.put("/api/v1/settings", body)
		if err != nil {
			return fmt.Errorf("setting value: %w", err)
		}

		if outputFormat == "json" {
			fmt.Println(string(resp))
			return nil
		}

		fmt.Printf("%s Setting saved: %s%s\n", colorGreen, args[0], colorReset)
		return nil
	},
}

var settingsGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Get a setting value (resolved through hierarchy if no scope specified)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		scope, _ := cmd.Flags().GetString("scope")
		target, _ := cmd.Flags().GetString("target")

		client, err := clientFromConfig()
		if err != nil {
			return err
		}

		var path string
		if scope == "" {
			// Resolve through hierarchy
			path = fmt.Sprintf("/api/v1/settings/resolve?key=%s", args[0])
		} else {
			path = fmt.Sprintf("/api/v1/settings?scope=%s&target=%s", scope, target)
		}

		resp, err := client.get(path)
		if err != nil {
			return fmt.Errorf("getting setting: %w", err)
		}

		fmt.Println(string(resp))
		return nil
	},
}

var settingsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List settings at a scope",
	RunE: func(cmd *cobra.Command, args []string) error {
		scope, _ := cmd.Flags().GetString("scope")
		target, _ := cmd.Flags().GetString("target")

		client, err := clientFromConfig()
		if err != nil {
			return err
		}

		resp, err := client.get(fmt.Sprintf("/api/v1/settings?scope=%s&target=%s", scope, target))
		if err != nil {
			return fmt.Errorf("listing settings: %w", err)
		}

		if outputFormat == "json" {
			fmt.Println(string(resp))
			return nil
		}

		var result struct {
			Settings []struct {
				Key   string          `json:"key"`
				Value json.RawMessage `json:"value"`
			} `json:"settings"`
		}
		if err := json.Unmarshal(resp, &result); err != nil {
			return fmt.Errorf("parsing response: %w", err)
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "KEY\tVALUE\n")
		for _, s := range result.Settings {
			fmt.Fprintf(w, "%s\t%s\n", s.Key, string(s.Value))
		}
		return w.Flush()
	},
}

func init() {
	settingsSetCmd.Flags().String("scope", "", "Scope: org, project, application, environment (required)")
	settingsSetCmd.Flags().String("target", "", "Target ID for the scope (required)")
	_ = settingsSetCmd.MarkFlagRequired("scope")
	_ = settingsSetCmd.MarkFlagRequired("target")

	settingsGetCmd.Flags().String("scope", "", "Scope (omit to resolve through hierarchy)")
	settingsGetCmd.Flags().String("target", "", "Target ID")

	settingsListCmd.Flags().String("scope", "", "Scope (required)")
	settingsListCmd.Flags().String("target", "", "Target ID (required)")
	_ = settingsListCmd.MarkFlagRequired("scope")
	_ = settingsListCmd.MarkFlagRequired("target")

	settingsCmd.AddCommand(settingsSetCmd, settingsGetCmd, settingsListCmd)
	rootCmd.AddCommand(settingsCmd)
}
```

- [ ] **Step 2: Create apikey subcommands**

```go
// cmd/cli/apikeys.go
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var apikeyCmd = &cobra.Command{
	Use:   "apikey",
	Short: "Manage API keys",
}

var apikeyCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new API key",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		scopes, _ := cmd.Flags().GetStringSlice("scopes")

		client, err := clientFromConfig()
		if err != nil {
			return err
		}

		body := map[string]interface{}{
			"name":   name,
			"scopes": scopes,
		}

		resp, err := client.post("/api/v1/api-keys", body)
		if err != nil {
			return fmt.Errorf("creating API key: %w", err)
		}

		if outputFormat == "json" {
			fmt.Println(string(resp))
			return nil
		}

		var result struct {
			APIKey struct {
				Name   string `json:"name"`
				Prefix string `json:"prefix"`
			} `json:"api_key"`
			Token string `json:"token"`
		}
		if err := json.Unmarshal(resp, &result); err != nil {
			return fmt.Errorf("parsing response: %w", err)
		}

		fmt.Printf("%s API key created: %s%s\n", colorGreen, result.APIKey.Name, colorReset)
		fmt.Printf("Token: %s%s%s\n", colorYellow, result.Token, colorReset)
		fmt.Println("Save this token — it will not be shown again.")
		return nil
	},
}

var apikeyListCmd = &cobra.Command{
	Use:   "list",
	Short: "List API keys",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := clientFromConfig()
		if err != nil {
			return err
		}

		resp, err := client.get("/api/v1/api-keys")
		if err != nil {
			return fmt.Errorf("listing API keys: %w", err)
		}

		if outputFormat == "json" {
			fmt.Println(string(resp))
			return nil
		}

		var result struct {
			APIKeys []struct {
				ID     string   `json:"id"`
				Name   string   `json:"name"`
				Prefix string   `json:"prefix"`
				Scopes []string `json:"scopes"`
			} `json:"api_keys"`
		}
		if err := json.Unmarshal(resp, &result); err != nil {
			return fmt.Errorf("parsing response: %w", err)
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "ID\tNAME\tPREFIX\tSCOPES\n")
		for _, k := range result.APIKeys {
			fmt.Fprintf(w, "%s\t%s\t%s\t%v\n", k.ID, k.Name, k.Prefix, k.Scopes)
		}
		return w.Flush()
	},
}

var apikeyRevokeCmd = &cobra.Command{
	Use:   "revoke <id>",
	Short: "Revoke an API key",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := clientFromConfig()
		if err != nil {
			return err
		}

		_, err = client.delete(fmt.Sprintf("/api/v1/api-keys/%s", args[0]))
		if err != nil {
			return fmt.Errorf("revoking API key: %w", err)
		}

		fmt.Printf("%s API key revoked%s\n", colorGreen, colorReset)
		return nil
	},
}

func init() {
	apikeyCreateCmd.Flags().String("name", "", "Key name (required)")
	apikeyCreateCmd.Flags().StringSlice("scopes", nil, "Permissions (e.g. flags:read,deploys:write)")
	_ = apikeyCreateCmd.MarkFlagRequired("name")

	apikeyCmd.AddCommand(apikeyCreateCmd, apikeyListCmd, apikeyRevokeCmd)
	rootCmd.AddCommand(apikeyCmd)
}
```

- [ ] **Step 3: Verify compilation**

Run: `cd /Users/sgamel/git/DeploySentry && go build ./cmd/cli/ 2>&1 | head -20`

- [ ] **Step 4: Commit**

```bash
git add cmd/cli/settings.go cmd/cli/apikeys.go
git commit -m "feat: add CLI settings and apikey commands"
```

---

### Task 8: CLI Install Script

**Files:**
- Create: `scripts/install.sh`

- [ ] **Step 1: Write the installer**

```bash
#!/bin/sh
# DeploySentry CLI Installer
# Usage: curl -fsSL https://dr-sentry.com/get-cli | sh

set -e

INSTALL_DIR="${HOME}/.deploysentry/bin"
BINARY_NAME="deploysentry"
BASE_URL="${DEPLOYSENTRY_RELEASES_URL:-https://github.com/deploysentry/deploysentry/releases/download}"
VERSION="${VERSION:-latest}"

# Detect OS and architecture
detect_platform() {
    OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
    ARCH="$(uname -m)"

    case "$OS" in
        linux)  OS="linux" ;;
        darwin) OS="darwin" ;;
        *)      echo "Error: unsupported OS: $OS"; exit 1 ;;
    esac

    case "$ARCH" in
        x86_64|amd64)  ARCH="amd64" ;;
        arm64|aarch64) ARCH="arm64" ;;
        *)             echo "Error: unsupported architecture: $ARCH"; exit 1 ;;
    esac
}

# Resolve latest version tag
resolve_version() {
    if [ "$VERSION" = "latest" ]; then
        VERSION=$(curl -fsSL "https://api.github.com/repos/deploysentry/deploysentry/releases/latest" \
            | grep '"tag_name"' | sed -E 's/.*"v?([^"]+)".*/\1/')
        if [ -z "$VERSION" ]; then
            echo "Error: could not determine latest version"
            exit 1
        fi
    fi
}

# Download and verify binary
download() {
    BINARY_URL="${BASE_URL}/v${VERSION}/${BINARY_NAME}-${OS}-${ARCH}"
    CHECKSUM_URL="${BASE_URL}/v${VERSION}/checksums.txt"

    echo "Downloading DeploySentry CLI v${VERSION} (${OS}/${ARCH})..."

    TMPDIR=$(mktemp -d)
    trap 'rm -rf "$TMPDIR"' EXIT

    curl -fsSL -o "${TMPDIR}/${BINARY_NAME}" "$BINARY_URL"
    curl -fsSL -o "${TMPDIR}/checksums.txt" "$CHECKSUM_URL" 2>/dev/null || true

    # Verify checksum if available
    if [ -f "${TMPDIR}/checksums.txt" ]; then
        EXPECTED=$(grep "${BINARY_NAME}-${OS}-${ARCH}" "${TMPDIR}/checksums.txt" | awk '{print $1}')
        if [ -n "$EXPECTED" ]; then
            ACTUAL=$(shasum -a 256 "${TMPDIR}/${BINARY_NAME}" | awk '{print $1}')
            if [ "$EXPECTED" != "$ACTUAL" ]; then
                echo "Error: checksum verification failed"
                echo "  Expected: $EXPECTED"
                echo "  Actual:   $ACTUAL"
                exit 1
            fi
            echo "Checksum verified."
        fi
    fi

    # Install
    mkdir -p "$INSTALL_DIR"
    mv "${TMPDIR}/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
    chmod +x "${INSTALL_DIR}/${BINARY_NAME}"
}

# Check if install dir is in PATH
check_path() {
    case ":$PATH:" in
        *":${INSTALL_DIR}:"*) return 0 ;;
        *) return 1 ;;
    esac
}

main() {
    detect_platform
    resolve_version
    download

    echo ""
    echo "DeploySentry CLI installed successfully!"
    echo ""

    if ! check_path; then
        echo "Add to your PATH:"
        echo ""
        echo "  # bash"
        echo "  echo 'export PATH=\"\$HOME/.deploysentry/bin:\$PATH\"' >> ~/.bashrc"
        echo ""
        echo "  # zsh"
        echo "  echo 'export PATH=\"\$HOME/.deploysentry/bin:\$PATH\"' >> ~/.zshrc"
        echo ""
        echo "  # fish"
        echo "  fish_add_path ~/.deploysentry/bin"
        echo ""
    fi

    echo "Next steps:"
    echo "  1. deploysentry auth login"
    echo "  2. deploysentry org list"
    echo ""
}

main
```

- [ ] **Step 2: Make executable**

Run: `chmod +x /Users/sgamel/git/DeploySentry/scripts/install.sh`

- [ ] **Step 3: Commit**

```bash
git add scripts/install.sh
git commit -m "feat: add curl|sh CLI installer script"
```

---

## Phase 3: Frontend — Web UI Overhaul

### Task 9: Update TypeScript types and API client

**Files:**
- Modify: `web/src/types.ts`
- Modify: `web/src/api.ts`

- [ ] **Step 1: Update types.ts**

Add these types/interfaces and update existing ones:

```typescript
// Update ReleaseStatus to match backend
export type ReleaseStatus = 'draft' | 'rolling_out' | 'paused' | 'completed' | 'rolled_back';

export interface Organization {
  id: string;
  name: string;
  slug: string;
  created_at: string;
  updated_at: string;
}

export interface Application {
  id: string;
  project_id: string;
  name: string;
  slug: string;
  description: string;
  repo_url: string;
  created_by: string;
  created_at: string;
  updated_at: string;
}

export interface FlagEnvironmentState {
  id: string;
  flag_id: string;
  environment_id: string;
  enabled: boolean;
  value: any;
  updated_by: string;
  updated_at: string;
}

export interface Setting {
  id: string;
  org_id?: string;
  project_id?: string;
  application_id?: string;
  environment_id?: string;
  key: string;
  value: any;
  updated_by?: string;
  updated_at: string;
}

// Update Deployment to be application-scoped
export interface Deployment {
  id: string;
  application_id: string;
  environment_id: string;
  version: string;
  commit_sha: string;
  artifact: string;
  strategy: DeployStrategy;
  status: DeployStatus;
  traffic_percent: number;
  created_by: string;
  created_at: string;
  updated_at: string;
  started_at: string | null;
  completed_at: string | null;
}

// Update Release to be flag-change bundle
export interface Release {
  id: string;
  application_id: string;
  name: string;
  description: string;
  session_sticky: boolean;
  sticky_header: string;
  traffic_percent: number;
  status: ReleaseStatus;
  created_by: string;
  created_at: string;
  updated_at: string;
  started_at: string | null;
  completed_at: string | null;
}

export interface ReleaseFlagChange {
  id: string;
  release_id: string;
  flag_id: string;
  environment_id: string;
  previous_value: any;
  new_value: any;
  previous_enabled: boolean | null;
  new_enabled: boolean | null;
  applied_at: string | null;
  created_at: string;
}

// Update Environment to be application-scoped
export interface Environment {
  id: string;
  name: string;
  slug: string;
  application_id: string;
}
```

- [ ] **Step 2: Update api.ts with new API namespaces**

Add these new API groups:

```typescript
// Organizations
export const orgsApi = {
  list: () => request<{ organizations: Organization[] }>('/orgs'),
  create: (data: { name: string; slug: string }) =>
    request<Organization>('/orgs', { method: 'POST', body: JSON.stringify(data) }),
  getMembers: (orgId: string) =>
    request<{ members: any[] }>(`/orgs/${orgId}/members`),
};

// Applications
export const applicationsApi = {
  list: (projectId: string) =>
    request<{ applications: Application[] }>(`/projects/${projectId}/applications`),
  get: (id: string) => request<Application>(`/applications/${id}`),
  create: (projectId: string, data: { name: string; slug: string; description?: string; repo_url?: string }) =>
    request<Application>(`/projects/${projectId}/applications`, { method: 'POST', body: JSON.stringify(data) }),
  update: (id: string, data: Partial<Application>) =>
    request<Application>(`/applications/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  delete: (id: string) =>
    request<void>(`/applications/${id}`, { method: 'DELETE' }),
};

// Flag Environment State
export const flagEnvStateApi = {
  list: (flagId: string) =>
    request<{ environment_states: FlagEnvironmentState[] }>(`/flags/${flagId}/environments`),
  set: (flagId: string, envId: string, data: { enabled: boolean; value?: any }) =>
    request<FlagEnvironmentState>(`/flags/${flagId}/environments/${envId}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    }),
};

// Settings
export const settingsApi = {
  list: (scope: string, targetId: string) =>
    request<{ settings: Setting[] }>(`/settings?scope=${scope}&target=${targetId}`),
  resolve: (key: string, params: { org_id?: string; project_id?: string; application_id?: string; environment_id?: string }) => {
    const qs = new URLSearchParams({ key });
    Object.entries(params).forEach(([k, v]) => { if (v) qs.set(k, v); });
    return request<Setting>(`/settings/resolve?${qs}`);
  },
  set: (data: { scope: string; target_id: string; key: string; value: any }) =>
    request<Setting>('/settings', { method: 'PUT', body: JSON.stringify(data) }),
  delete: (id: string) =>
    request<void>(`/settings/${id}`, { method: 'DELETE' }),
};
```

Update existing `deploymentsApi` to use application-scoped endpoints:

```typescript
export const deploymentsApi = {
  list: (appId: string) =>
    request<{ deployments: Deployment[] }>(`/applications/${appId}/deployments`),
  get: (id: string) => request<Deployment>(`/deployments/${id}`),
  create: (appId: string, data: { environment_id: string; version: string; strategy: string; commit_sha?: string; artifact?: string }) =>
    request<Deployment>(`/applications/${appId}/deployments`, { method: 'POST', body: JSON.stringify(data) }),
  promote: (id: string) =>
    request<Deployment>(`/deployments/${id}/promote`, { method: 'POST' }),
  rollback: (id: string) =>
    request<Deployment>(`/deployments/${id}/rollback`, { method: 'POST' }),
  pause: (id: string) =>
    request<Deployment>(`/deployments/${id}/pause`, { method: 'POST' }),
  resume: (id: string) =>
    request<Deployment>(`/deployments/${id}/resume`, { method: 'POST' }),
};

export const releasesApi = {
  list: (appId: string) =>
    request<{ releases: Release[] }>(`/applications/${appId}/releases`),
  get: (id: string) => request<Release>(`/releases/${id}`),
  create: (appId: string, data: { name: string; description?: string; session_sticky?: boolean; sticky_header?: string }) =>
    request<Release>(`/applications/${appId}/releases`, { method: 'POST', body: JSON.stringify(data) }),
  delete: (id: string) =>
    request<void>(`/releases/${id}`, { method: 'DELETE' }),
  start: (id: string) =>
    request<{ status: string }>(`/releases/${id}/start`, { method: 'POST' }),
  promote: (id: string, trafficPercent: number) =>
    request<{ status: string }>(`/releases/${id}/promote`, {
      method: 'POST',
      body: JSON.stringify({ traffic_percent: trafficPercent }),
    }),
  pause: (id: string) =>
    request<{ status: string }>(`/releases/${id}/pause`, { method: 'POST' }),
  rollback: (id: string) =>
    request<{ status: string }>(`/releases/${id}/rollback`, { method: 'POST' }),
  complete: (id: string) =>
    request<{ status: string }>(`/releases/${id}/complete`, { method: 'POST' }),
  addFlagChange: (releaseId: string, data: { flag_id: string; environment_id: string; new_enabled?: boolean }) =>
    request<ReleaseFlagChange>(`/releases/${releaseId}/flag-changes`, {
      method: 'POST',
      body: JSON.stringify(data),
    }),
  listFlagChanges: (releaseId: string) =>
    request<{ flag_changes: ReleaseFlagChange[] }>(`/releases/${releaseId}/flag-changes`),
};
```

- [ ] **Step 3: Commit**

```bash
cd /Users/sgamel/git/DeploySentry && git add web/src/types.ts web/src/api.ts
git commit -m "feat: update frontend types and API client for org hierarchy"
```

---

### Task 10: Org Switcher and Sidebar Redesign

**Files:**
- Create: `web/src/components/OrgSwitcher.tsx`
- Rewrite: `web/src/components/Sidebar.tsx`
- Modify: `web/src/components/Layout.tsx`
- Create: `web/src/components/Breadcrumb.tsx`
- Modify: `web/src/styles/globals.css`

- [ ] **Step 1: Create OrgSwitcher component**

Create `web/src/components/OrgSwitcher.tsx` — a dropdown that shows current org and allows switching. Uses `orgsApi.list()`. Stores selected org in `localStorage('ds_org')`. Dispatches a custom event `org-changed` so the sidebar can refresh.

- [ ] **Step 2: Create Breadcrumb component**

Create `web/src/components/Breadcrumb.tsx` — parses the current URL path and renders a breadcrumb trail (`Org > Project > App > Page`). Uses `useLocation()` from react-router.

- [ ] **Step 3: Rewrite Sidebar with accordion navigation**

Rewrite `web/src/components/Sidebar.tsx`:
- OrgSwitcher at top
- Project accordion (fetches projects for selected org)
- Under each project: Application accordion
- Under each application: Deployments, Releases, Flags, Settings links
- Project-level section: Feature Flags, Settings
- Org-level section: Members, API Keys, Settings

Each accordion section uses `useState` for expand/collapse. Links use `NavLink` with the new route structure.

- [ ] **Step 4: Update Layout to include Breadcrumb**

In `web/src/components/Layout.tsx`, add `<Breadcrumb />` above the `<Outlet />` inside `<main>`.

- [ ] **Step 5: Add CSS for new sidebar patterns**

In `web/src/styles/globals.css`, add styles for:
- `.org-switcher` dropdown
- `.sidebar-accordion`, `.sidebar-accordion-header`, `.sidebar-accordion-content`
- `.sidebar-section-label`
- `.breadcrumb`

- [ ] **Step 6: Verify dev build**

Run: `cd /Users/sgamel/git/DeploySentry/web && npm run build 2>&1 | tail -10`

- [ ] **Step 7: Commit**

```bash
cd /Users/sgamel/git/DeploySentry && git add web/src/components/ web/src/styles/globals.css
git commit -m "feat: add org switcher, accordion sidebar, and breadcrumb navigation"
```

---

### Task 11: Update Router and Add New Pages

**Files:**
- Modify: `web/src/App.tsx`
- Create: `web/src/pages/ApplicationsPage.tsx`
- Create: `web/src/pages/ApplicationDetailPage.tsx`
- Create: `web/src/pages/OrgSettingsPage.tsx`
- Create: `web/src/pages/MembersPage.tsx`
- Modify: `web/src/pages/DeploymentsPage.tsx`
- Modify: `web/src/pages/ReleasesPage.tsx`
- Modify: `web/src/pages/FlagDetailPage.tsx`
- Modify: `web/src/pages/SettingsPage.tsx`

- [ ] **Step 1: Update App.tsx router**

Replace the flat route structure with the hierarchical one from the spec:

```tsx
<Route element={<RequireAuth><Layout /></RequireAuth>}>
  <Route index element={<DashboardPage />} />

  {/* Org-level routes */}
  <Route path="orgs/:orgSlug/members" element={<MembersPage />} />
  <Route path="orgs/:orgSlug/api-keys" element={<SettingsPage tab="api-keys" />} />
  <Route path="orgs/:orgSlug/settings" element={<OrgSettingsPage />} />

  {/* Project-level routes */}
  <Route path="orgs/:orgSlug/projects/:projectSlug/flags" element={<FlagListPage />} />
  <Route path="orgs/:orgSlug/projects/:projectSlug/flags/:id" element={<FlagDetailPage />} />
  <Route path="orgs/:orgSlug/projects/:projectSlug/settings" element={<SettingsPage scope="project" />} />

  {/* Application-level routes */}
  <Route path="orgs/:orgSlug/projects/:projectSlug/apps/:appSlug/deployments" element={<DeploymentsPage />} />
  <Route path="orgs/:orgSlug/projects/:projectSlug/apps/:appSlug/deployments/:id" element={<DeploymentDetailPage />} />
  <Route path="orgs/:orgSlug/projects/:projectSlug/apps/:appSlug/releases" element={<ReleasesPage />} />
  <Route path="orgs/:orgSlug/projects/:projectSlug/apps/:appSlug/releases/:id" element={<ReleaseDetailPage />} />
  <Route path="orgs/:orgSlug/projects/:projectSlug/apps/:appSlug/flags" element={<FlagListPage scope="app" />} />
  <Route path="orgs/:orgSlug/projects/:projectSlug/apps/:appSlug/flags/:id" element={<FlagDetailPage />} />
  <Route path="orgs/:orgSlug/projects/:projectSlug/apps/:appSlug/settings" element={<SettingsPage scope="application" />} />

  {/* Keep existing routes as fallbacks during transition */}
  <Route path="flags" element={<FlagListPage />} />
  <Route path="flags/new" element={<FlagCreatePage />} />
  <Route path="flags/:id" element={<FlagDetailPage />} />
  <Route path="deployments" element={<DeploymentsPage />} />
  <Route path="releases" element={<ReleasesPage />} />
  <Route path="analytics" element={<AnalyticsPage />} />
  <Route path="sdks" element={<SDKsPage />} />
  <Route path="settings" element={<SettingsPage />} />
</Route>
```

- [ ] **Step 2: Create ApplicationsPage**

Simple list page using `applicationsApi.list()`. Shows app name, slug, description, repo URL. "Create Application" button.

- [ ] **Step 3: Create MembersPage**

Uses `orgsApi.getMembers()`. Table with name, email, role. Invite member form.

- [ ] **Step 4: Create OrgSettingsPage**

Uses `settingsApi.list('org', orgId)`. Key/value editor.

- [ ] **Step 5: Update FlagDetailPage with Environments tab**

Add a third tab "Environments" that uses `flagEnvStateApi.list(flagId)`. Shows a table with environment name, enabled toggle, value, last updated. Toggle calls `flagEnvStateApi.set()`.

- [ ] **Step 6: Update DeploymentsPage and ReleasesPage**

Update to read `appSlug` from URL params and use application-scoped API calls.

- [ ] **Step 7: Verify build**

Run: `cd /Users/sgamel/git/DeploySentry/web && npm run build 2>&1 | tail -10`

- [ ] **Step 8: Commit**

```bash
cd /Users/sgamel/git/DeploySentry && git add web/src/
git commit -m "feat: add hierarchical routing, new pages, and per-environment flag state UI"
```

---

## Review Checkpoint

After completing all 11 tasks, verify:

- [ ] `go build ./cmd/api/` compiles cleanly
- [ ] `go build ./cmd/cli/` compiles cleanly
- [ ] `cd web && npm run build` succeeds
- [ ] `go test ./internal/...` passes existing tests
- [ ] New endpoints respond correctly: `GET /api/v1/projects/:id/applications`, `PUT /api/v1/settings`, `GET /api/v1/flags/:id/environments`
