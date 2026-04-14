# Project Edit & Delete Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix project editing to persist all fields, add soft-delete with 14-day flag activity guard, hard-delete with 7-day gate, restore capability, and corresponding web UI changes.

**Architecture:** Add `deleted_at` column to projects table. Extend the existing entity repository/service/handler layers with delete/restore methods. The service layer queries `flag_evaluation_log` to enforce the 14-day activity guard. The web UI adds a gear icon to project cards, edit fields to the project settings page, and a danger zone section for deletion.

**Tech Stack:** Go (Gin, pgx), PostgreSQL, React + TypeScript

---

## File Map

| Action | File | Responsibility |
|--------|------|----------------|
| Create | `migrations/039_add_project_deleted_at.up.sql` | Add `deleted_at` column |
| Create | `migrations/039_add_project_deleted_at.down.sql` | Rollback migration |
| Modify | `internal/models/project.go` | Add `DeletedAt` field |
| Modify | `internal/platform/database/postgres/entities.go` | Fix update query, add soft/hard delete, restore, include_deleted |
| Modify | `internal/platform/database/postgres/flags.go` | Add `HasRecentFlagActivity` query |
| Modify | `internal/entities/repository.go` | Add new interface methods |
| Modify | `internal/entities/service.go` | Add delete/restore business logic |
| Modify | `internal/entities/handler.go` | Add delete/restore routes and handlers |
| Modify | `internal/entities/handler_test.go` | Tests for new handlers |
| Modify | `web/src/types.ts` | Add `deleted_at`, `description`, `repo_url` to Project type |
| Modify | `web/src/api.ts` | Add delete/restore/activity-check API methods, fix updateProject |
| Modify | `web/src/hooks/useEntities.ts` | Add `includeDeleted` param to `useProjects` |
| Modify | `web/src/pages/ProjectListPage.tsx` | Gear icon, deleted card styling, restore button |
| Modify | `web/src/pages/SettingsPage.tsx` | Add description/repo_url fields, danger zone tab for projects |

---

### Task 1: Migration — Add `deleted_at` Column

**Files:**
- Create: `migrations/039_add_project_deleted_at.up.sql`
- Create: `migrations/039_add_project_deleted_at.down.sql`

- [ ] **Step 1: Create up migration**

```sql
ALTER TABLE projects ADD COLUMN deleted_at TIMESTAMPTZ;
```

- [ ] **Step 2: Create down migration**

```sql
ALTER TABLE projects DROP COLUMN deleted_at;
```

- [ ] **Step 3: Run migration**

Run: `make migrate-up`
Expected: Migration 039 applied successfully.

- [ ] **Step 4: Commit**

```bash
git add migrations/039_add_project_deleted_at.up.sql migrations/039_add_project_deleted_at.down.sql
git commit -m "feat: add deleted_at column to projects table"
```

---

### Task 2: Model — Add `DeletedAt` Field

**Files:**
- Modify: `internal/models/project.go:11-20`

- [ ] **Step 1: Add DeletedAt to Project struct**

In `internal/models/project.go`, add the `DeletedAt` field to the `Project` struct after `UpdatedAt`:

```go
type Project struct {
	ID          uuid.UUID  `json:"id" db:"id"`
	OrgID       uuid.UUID  `json:"org_id" db:"org_id"`
	Name        string     `json:"name" db:"name"`
	Slug        string     `json:"slug" db:"slug"`
	Description string     `json:"description,omitempty" db:"description"`
	RepoURL     string     `json:"repo_url,omitempty" db:"repo_url"`
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at" db:"updated_at"`
	DeletedAt   *time.Time `json:"deleted_at,omitempty" db:"deleted_at"`
}
```

- [ ] **Step 2: Verify build**

Run: `cd /Users/sgamel/git/DeploySentry && go build ./...`
Expected: Build succeeds.

- [ ] **Step 3: Commit**

```bash
git add internal/models/project.go
git commit -m "feat: add DeletedAt field to Project model"
```

---

### Task 3: Repository — Fix Existing Queries and Add Delete/Restore Methods

**Files:**
- Modify: `internal/platform/database/postgres/entities.go:134-227`

- [ ] **Step 1: Fix `CreateProject` to include description and repo_url**

Replace the `CreateProject` method (lines 135-155):

```go
func (r *EntityRepository) CreateProject(ctx context.Context, project *models.Project) error {
	const q = `
		INSERT INTO projects (id, org_id, name, slug, description, repo_url, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	_, err := r.pool.Exec(ctx, q,
		project.ID,
		project.OrgID,
		project.Name,
		project.Slug,
		project.Description,
		project.RepoURL,
		project.CreatedAt,
		project.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrConflict
		}
		return fmt.Errorf("postgres.CreateProject: %w", err)
	}
	return nil
}
```

- [ ] **Step 2: Fix `GetProjectBySlug` to select all columns including `deleted_at`**

Replace the `GetProjectBySlug` method (lines 159-180):

```go
func (r *EntityRepository) GetProjectBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*models.Project, error) {
	const q = `
		SELECT id, org_id, name, slug, COALESCE(description, ''), COALESCE(repo_url, ''), created_at, updated_at, deleted_at
		FROM projects WHERE org_id = $1 AND slug = $2`

	var p models.Project
	err := r.pool.QueryRow(ctx, q, orgID, slug).Scan(
		&p.ID,
		&p.OrgID,
		&p.Name,
		&p.Slug,
		&p.Description,
		&p.RepoURL,
		&p.CreatedAt,
		&p.UpdatedAt,
		&p.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("postgres.GetProjectBySlug: %w", err)
	}
	return &p, nil
}
```

- [ ] **Step 3: Fix `ListProjectsByOrg` to select all columns and support `includeDeleted`**

Replace the `ListProjectsByOrg` method (lines 183-213):

```go
func (r *EntityRepository) ListProjectsByOrg(ctx context.Context, orgID uuid.UUID, includeDeleted bool) ([]*models.Project, error) {
	q := `
		SELECT id, org_id, name, slug, COALESCE(description, ''), COALESCE(repo_url, ''), created_at, updated_at, deleted_at
		FROM projects WHERE org_id = $1`
	if !includeDeleted {
		q += ` AND deleted_at IS NULL`
	}
	q += ` ORDER BY name`

	rows, err := r.pool.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("postgres.ListProjectsByOrg: %w", err)
	}
	defer rows.Close()

	result := make([]*models.Project, 0)
	for rows.Next() {
		var p models.Project
		if err := rows.Scan(
			&p.ID,
			&p.OrgID,
			&p.Name,
			&p.Slug,
			&p.Description,
			&p.RepoURL,
			&p.CreatedAt,
			&p.UpdatedAt,
			&p.DeletedAt,
		); err != nil {
			return nil, fmt.Errorf("postgres.ListProjectsByOrg: %w", err)
		}
		result = append(result, &p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres.ListProjectsByOrg: %w", err)
	}
	return result, nil
}
```

- [ ] **Step 4: Fix `UpdateProject` to persist all editable fields**

Replace the `UpdateProject` method (lines 216-227):

```go
func (r *EntityRepository) UpdateProject(ctx context.Context, project *models.Project) error {
	const q = `
		UPDATE projects
		SET name = $1, description = $2, repo_url = $3, updated_at = $4
		WHERE id = $5 AND deleted_at IS NULL`

	tag, err := r.pool.Exec(ctx, q, project.Name, project.Description, project.RepoURL, project.UpdatedAt, project.ID)
	if err != nil {
		return fmt.Errorf("postgres.UpdateProject: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
```

- [ ] **Step 5: Add `SoftDeleteProject`, `HardDeleteProject`, `RestoreProject` methods**

Add after `UpdateProject`:

```go
// SoftDeleteProject sets deleted_at on a project.
func (r *EntityRepository) SoftDeleteProject(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE projects SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL`

	tag, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("postgres.SoftDeleteProject: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// HardDeleteProject permanently deletes a project that has been soft-deleted for at least 7 days.
func (r *EntityRepository) HardDeleteProject(ctx context.Context, id uuid.UUID) error {
	const q = `DELETE FROM projects WHERE id = $1 AND deleted_at <= now() - interval '7 days'`

	tag, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("postgres.HardDeleteProject: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// RestoreProject clears deleted_at on a soft-deleted project.
func (r *EntityRepository) RestoreProject(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE projects SET deleted_at = NULL WHERE id = $1 AND deleted_at IS NOT NULL`

	tag, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("postgres.RestoreProject: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
```

- [ ] **Step 6: Verify build**

Run: `cd /Users/sgamel/git/DeploySentry && go build ./...`
Expected: Build will fail — interface mismatches. That's expected; we fix the interface in the next task.

- [ ] **Step 7: Commit (don't worry about build failure yet)**

```bash
git add internal/platform/database/postgres/entities.go
git commit -m "feat: fix project repo queries, add soft/hard delete and restore"
```

---

### Task 4: Flag Activity Query

**Files:**
- Modify: `internal/platform/database/postgres/flags.go`

- [ ] **Step 1: Add `FlagActivitySummary` type and `HasRecentFlagActivity` method**

Add to the bottom of `internal/platform/database/postgres/flags.go`:

```go
// FlagActivitySummary describes a flag with recent evaluation activity.
type FlagActivitySummary struct {
	Key           string    `json:"key"`
	Name          string    `json:"name"`
	LastEvaluated time.Time `json:"last_evaluated"`
}

// HasRecentFlagActivity returns flags in the given project that have evaluations after `since`.
func (r *FlagRepository) HasRecentFlagActivity(ctx context.Context, projectID uuid.UUID, since time.Time) ([]FlagActivitySummary, error) {
	const q = `
		SELECT f.key, f.name, MAX(l.evaluated_at) AS last_evaluated
		FROM feature_flags f
		JOIN flag_evaluation_log l ON l.flag_id = f.id
		WHERE f.project_id = $1
		  AND f.archived_at IS NULL
		  AND l.evaluated_at >= $2
		GROUP BY f.key, f.name
		ORDER BY last_evaluated DESC`

	rows, err := r.pool.Query(ctx, q, projectID, since)
	if err != nil {
		return nil, fmt.Errorf("postgres.HasRecentFlagActivity: %w", err)
	}
	defer rows.Close()

	var result []FlagActivitySummary
	for rows.Next() {
		var s FlagActivitySummary
		if err := rows.Scan(&s.Key, &s.Name, &s.LastEvaluated); err != nil {
			return nil, fmt.Errorf("postgres.HasRecentFlagActivity: %w", err)
		}
		result = append(result, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres.HasRecentFlagActivity: %w", err)
	}
	return result, nil
}
```

Note: Check the existing imports in `flags.go` — `time` may need to be added.

- [ ] **Step 2: Verify build**

Run: `cd /Users/sgamel/git/DeploySentry && go build ./internal/platform/database/postgres/...`
Expected: Compiles successfully.

- [ ] **Step 3: Commit**

```bash
git add internal/platform/database/postgres/flags.go
git commit -m "feat: add HasRecentFlagActivity query for project deletion guard"
```

---

### Task 5: Repository Interface Update

**Files:**
- Modify: `internal/entities/repository.go`

- [ ] **Step 1: Update `EntityRepository` interface**

Replace the project section of the interface (lines 17-20) with:

```go
	CreateProject(ctx context.Context, project *models.Project) error
	GetProjectBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*models.Project, error)
	ListProjectsByOrg(ctx context.Context, orgID uuid.UUID, includeDeleted bool) ([]*models.Project, error)
	UpdateProject(ctx context.Context, project *models.Project) error
	SoftDeleteProject(ctx context.Context, id uuid.UUID) error
	HardDeleteProject(ctx context.Context, id uuid.UUID) error
	RestoreProject(ctx context.Context, id uuid.UUID) error
```

- [ ] **Step 2: Verify build**

Run: `cd /Users/sgamel/git/DeploySentry && go build ./...`
Expected: Build will fail in service.go and handler_test.go due to updated signatures. Expected — fixed in next tasks.

- [ ] **Step 3: Commit**

```bash
git add internal/entities/repository.go
git commit -m "feat: add project delete/restore methods to EntityRepository interface"
```

---

### Task 6: Service Layer — Delete/Restore Logic

**Files:**
- Modify: `internal/entities/service.go`

- [ ] **Step 1: Add new imports and types**

Add `"fmt"` to the import block in `service.go` if not present. Add the `FlagActivityChecker` interface and update the service struct after the existing imports:

```go
// FlagActivityChecker checks for recent flag evaluation activity.
type FlagActivityChecker interface {
	HasRecentFlagActivity(ctx context.Context, projectID uuid.UUID, since time.Time) ([]FlagActivitySummary, error)
}

// FlagActivitySummary describes a flag with recent evaluation activity.
type FlagActivitySummary struct {
	Key           string    `json:"key"`
	Name          string    `json:"name"`
	LastEvaluated time.Time `json:"last_evaluated"`
}
```

- [ ] **Step 2: Update service struct and constructor**

Replace the `entityService` struct and `NewEntityService` (lines 38-45):

```go
const (
	recentActivityWindow   = 14 * 24 * time.Hour
	hardDeleteEligibleAfter = 7 * 24 * time.Hour
)

type entityService struct {
	repo         EntityRepository
	envRepo      *EnvironmentRepository
	flagActivity FlagActivityChecker
}

func NewEntityService(repo EntityRepository, envRepo *EnvironmentRepository, flagActivity FlagActivityChecker) EntityService {
	return &entityService{repo: repo, envRepo: envRepo, flagActivity: flagActivity}
}
```

- [ ] **Step 3: Add new methods to `EntityService` interface**

Add after the existing `UpdateProject` line (line 20) in the `EntityService` interface:

```go
	SoftDeleteProject(ctx context.Context, orgID uuid.UUID, slug string) ([]FlagActivitySummary, error)
	HardDeleteProject(ctx context.Context, orgID uuid.UUID, slug string) (*time.Time, error)
	RestoreProject(ctx context.Context, orgID uuid.UUID, slug string) error
	ListProjectsByOrg(ctx context.Context, orgID uuid.UUID, includeDeleted bool) ([]*models.Project, error)
```

And remove the old `ListProjectsByOrg` (line 19) that has no `includeDeleted` param.

- [ ] **Step 4: Update existing `ListProjectsByOrg` implementation**

Replace the existing implementation (lines 96-98):

```go
func (s *entityService) ListProjectsByOrg(ctx context.Context, orgID uuid.UUID, includeDeleted bool) ([]*models.Project, error) {
	return s.repo.ListProjectsByOrg(ctx, orgID, includeDeleted)
}
```

- [ ] **Step 5: Add `SoftDeleteProject` implementation**

Add after `UpdateProject`:

```go
func (s *entityService) SoftDeleteProject(ctx context.Context, orgID uuid.UUID, slug string) ([]FlagActivitySummary, error) {
	project, err := s.repo.GetProjectBySlug(ctx, orgID, slug)
	if err != nil {
		return nil, err
	}
	if project == nil {
		return nil, errors.New("project not found")
	}
	if project.DeletedAt != nil {
		return nil, errors.New("project is already deleted")
	}

	since := time.Now().UTC().Add(-recentActivityWindow)
	activeFlags, err := s.flagActivity.HasRecentFlagActivity(ctx, project.ID, since)
	if err != nil {
		return nil, fmt.Errorf("checking flag activity: %w", err)
	}
	if len(activeFlags) > 0 {
		return activeFlags, errors.New("project has flags with recent activity")
	}

	return nil, s.repo.SoftDeleteProject(ctx, project.ID)
}
```

- [ ] **Step 6: Add `HardDeleteProject` implementation**

```go
func (s *entityService) HardDeleteProject(ctx context.Context, orgID uuid.UUID, slug string) (*time.Time, error) {
	project, err := s.repo.GetProjectBySlug(ctx, orgID, slug)
	if err != nil {
		return nil, err
	}
	if project == nil {
		return nil, errors.New("project not found")
	}
	if project.DeletedAt == nil {
		return nil, errors.New("project must be soft-deleted first")
	}

	eligibleAt := project.DeletedAt.Add(hardDeleteEligibleAfter)
	if time.Now().UTC().Before(eligibleAt) {
		return &eligibleAt, errors.New("project must be soft-deleted for at least 7 days")
	}

	return nil, s.repo.HardDeleteProject(ctx, project.ID)
}
```

- [ ] **Step 7: Add `RestoreProject` implementation**

```go
func (s *entityService) RestoreProject(ctx context.Context, orgID uuid.UUID, slug string) error {
	project, err := s.repo.GetProjectBySlug(ctx, orgID, slug)
	if err != nil {
		return err
	}
	if project == nil {
		return errors.New("project not found")
	}
	if project.DeletedAt == nil {
		return errors.New("project is not deleted")
	}
	return s.repo.RestoreProject(ctx, project.ID)
}
```

- [ ] **Step 8: Update all callers of `NewEntityService`**

Find where `NewEntityService` is called (likely `cmd/api/main.go`) and add the `FlagActivityChecker` argument. The postgres `FlagRepository` already satisfies this interface.

Search for `NewEntityService(` and update the call to pass the flag repository:

```go
// Before:
entitySvc := entities.NewEntityService(entityRepo, envRepo)
// After:
entitySvc := entities.NewEntityService(entityRepo, envRepo, flagRepo)
```

- [ ] **Step 9: Verify build**

Run: `cd /Users/sgamel/git/DeploySentry && go build ./...`
Expected: Build will fail in handler_test.go (mock doesn't implement new methods). Expected — fixed in Task 8.

- [ ] **Step 10: Commit**

```bash
git add internal/entities/service.go cmd/api/main.go
git commit -m "feat: add project soft/hard delete and restore service logic"
```

---

### Task 7: Handler — Delete/Restore Routes

**Files:**
- Modify: `internal/entities/handler.go`

- [ ] **Step 1: Register new routes**

In `RegisterRoutes`, add after the `PUT /:projectSlug` line (line 46):

```go
			projects.DELETE("/:projectSlug", auth.RequirePermission(h.rbac, auth.PermProjectManage), h.softDeleteProject)
			projects.DELETE("/:projectSlug/permanent", auth.RequirePermission(h.rbac, auth.PermOrgManage), h.hardDeleteProject)
			projects.POST("/:projectSlug/restore", auth.RequirePermission(h.rbac, auth.PermProjectManage), h.restoreProject)
```

- [ ] **Step 2: Update `listProjects` handler to support `include_deleted`**

Replace the `listProjects` handler (lines 180-193):

```go
func (h *Handler) listProjects(c *gin.Context) {
	org, err := h.service.GetOrgBySlug(c.Request.Context(), c.Param("orgSlug"))
	if err != nil || org == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "organization not found"})
		return
	}

	includeDeleted := c.Query("include_deleted") == "true"
	projects, err := h.service.ListProjectsByOrg(c.Request.Context(), org.ID, includeDeleted)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"projects": projects})
}
```

- [ ] **Step 3: Add `softDeleteProject` handler**

```go
func (h *Handler) softDeleteProject(c *gin.Context) {
	org, err := h.service.GetOrgBySlug(c.Request.Context(), c.Param("orgSlug"))
	if err != nil || org == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "organization not found"})
		return
	}

	activeFlags, err := h.service.SoftDeleteProject(c.Request.Context(), org.ID, c.Param("projectSlug"))
	if err != nil {
		if err.Error() == "project not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		if err.Error() == "project has flags with recent activity" {
			c.JSON(http.StatusConflict, gin.H{
				"error":        err.Error(),
				"active_flags": activeFlags,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}
```

- [ ] **Step 4: Add `hardDeleteProject` handler**

```go
func (h *Handler) hardDeleteProject(c *gin.Context) {
	org, err := h.service.GetOrgBySlug(c.Request.Context(), c.Param("orgSlug"))
	if err != nil || org == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "organization not found"})
		return
	}

	eligibleAt, err := h.service.HardDeleteProject(c.Request.Context(), org.ID, c.Param("projectSlug"))
	if err != nil {
		if err.Error() == "project not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		if eligibleAt != nil {
			c.JSON(http.StatusConflict, gin.H{
				"error":       err.Error(),
				"eligible_at": eligibleAt,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}
```

- [ ] **Step 5: Add `restoreProject` handler**

```go
func (h *Handler) restoreProject(c *gin.Context) {
	org, err := h.service.GetOrgBySlug(c.Request.Context(), c.Param("orgSlug"))
	if err != nil || org == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "organization not found"})
		return
	}

	if err := h.service.RestoreProject(c.Request.Context(), org.ID, c.Param("projectSlug")); err != nil {
		if err.Error() == "project not found" || err.Error() == "project is not deleted" {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	project, _ := h.service.GetProjectBySlug(c.Request.Context(), org.ID, c.Param("projectSlug"))
	c.JSON(http.StatusOK, project)
}
```

- [ ] **Step 6: Verify build (excluding tests)**

Run: `cd /Users/sgamel/git/DeploySentry && go build ./cmd/api/...`
Expected: Compiles successfully.

- [ ] **Step 7: Commit**

```bash
git add internal/entities/handler.go
git commit -m "feat: add project soft/hard delete and restore HTTP handlers"
```

---

### Task 8: Handler Tests

**Files:**
- Modify: `internal/entities/handler_test.go`

- [ ] **Step 1: Update `mockEntityService` with new fields and methods**

Add these fields to the `mockEntityService` struct (after line 32):

```go
	softDeleteProjectFn func(ctx context.Context, orgID uuid.UUID, slug string) ([]FlagActivitySummary, error)
	hardDeleteProjectFn func(ctx context.Context, orgID uuid.UUID, slug string) (*time.Time, error)
	restoreProjectFn    func(ctx context.Context, orgID uuid.UUID, slug string) error
```

Add `time` to the imports.

Update the existing `ListProjectsByOrg` mock method to accept `includeDeleted`:

```go
func (m *mockEntityService) ListProjectsByOrg(ctx context.Context, orgID uuid.UUID, includeDeleted bool) ([]*models.Project, error) {
	if m.listProjectsByOrgFn != nil {
		return m.listProjectsByOrgFn(ctx, orgID)
	}
	return []*models.Project{}, nil
}
```

Add new mock methods:

```go
func (m *mockEntityService) SoftDeleteProject(ctx context.Context, orgID uuid.UUID, slug string) ([]FlagActivitySummary, error) {
	if m.softDeleteProjectFn != nil {
		return m.softDeleteProjectFn(ctx, orgID, slug)
	}
	return nil, nil
}

func (m *mockEntityService) HardDeleteProject(ctx context.Context, orgID uuid.UUID, slug string) (*time.Time, error) {
	if m.hardDeleteProjectFn != nil {
		return m.hardDeleteProjectFn(ctx, orgID, slug)
	}
	return nil, nil
}

func (m *mockEntityService) RestoreProject(ctx context.Context, orgID uuid.UUID, slug string) error {
	if m.restoreProjectFn != nil {
		return m.restoreProjectFn(ctx, orgID, slug)
	}
	return nil
}
```

- [ ] **Step 2: Add soft delete success test**

```go
func TestSoftDeleteProject_Success(t *testing.T) {
	orgID := uuid.New()
	svc := &mockEntityService{
		getOrgBySlugFn: func(_ context.Context, slug string) (*models.Organization, error) {
			return &models.Organization{ID: orgID, Slug: slug}, nil
		},
		softDeleteProjectFn: func(_ context.Context, oid uuid.UUID, slug string) ([]FlagActivitySummary, error) {
			assert.Equal(t, orgID, oid)
			assert.Equal(t, "my-proj", slug)
			return nil, nil
		},
	}
	router := setupEntityRouter(svc)

	req := httptest.NewRequest(http.MethodDelete, "/api/orgs/test-org/projects/my-proj", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}
```

- [ ] **Step 3: Add soft delete blocked by active flags test**

```go
func TestSoftDeleteProject_BlockedByActiveFlags(t *testing.T) {
	orgID := uuid.New()
	svc := &mockEntityService{
		getOrgBySlugFn: func(_ context.Context, slug string) (*models.Organization, error) {
			return &models.Organization{ID: orgID, Slug: slug}, nil
		},
		softDeleteProjectFn: func(_ context.Context, _ uuid.UUID, _ string) ([]FlagActivitySummary, error) {
			return []FlagActivitySummary{
				{Key: "dark-mode", Name: "Dark Mode", LastEvaluated: time.Now()},
			}, errors.New("project has flags with recent activity")
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
	assert.NotNil(t, resp["active_flags"])
}
```

- [ ] **Step 4: Add hard delete too early test**

```go
func TestHardDeleteProject_TooEarly(t *testing.T) {
	orgID := uuid.New()
	eligibleAt := time.Now().Add(7 * 24 * time.Hour)
	svc := &mockEntityService{
		getOrgBySlugFn: func(_ context.Context, slug string) (*models.Organization, error) {
			return &models.Organization{ID: orgID, Slug: slug}, nil
		},
		hardDeleteProjectFn: func(_ context.Context, _ uuid.UUID, _ string) (*time.Time, error) {
			return &eligibleAt, errors.New("project must be soft-deleted for at least 7 days")
		},
	}
	router := setupEntityRouter(svc)

	req := httptest.NewRequest(http.MethodDelete, "/api/orgs/test-org/projects/my-proj/permanent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
	var resp map[string]interface{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "project must be soft-deleted for at least 7 days", resp["error"])
	assert.NotNil(t, resp["eligible_at"])
}
```

- [ ] **Step 5: Add restore test**

```go
func TestRestoreProject_Success(t *testing.T) {
	orgID := uuid.New()
	svc := &mockEntityService{
		getOrgBySlugFn: func(_ context.Context, slug string) (*models.Organization, error) {
			return &models.Organization{ID: orgID, Slug: slug}, nil
		},
		restoreProjectFn: func(_ context.Context, oid uuid.UUID, slug string) error {
			assert.Equal(t, orgID, oid)
			assert.Equal(t, "my-proj", slug)
			return nil
		},
		getProjectBySlugFn: func(_ context.Context, oid uuid.UUID, slug string) (*models.Project, error) {
			return &models.Project{ID: uuid.New(), OrgID: oid, Name: "My Project", Slug: slug}, nil
		},
	}
	router := setupEntityRouter(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/orgs/test-org/projects/my-proj/restore", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
```

- [ ] **Step 6: Run tests**

Run: `cd /Users/sgamel/git/DeploySentry && go test ./internal/entities/ -v -run "TestSoftDelete|TestHardDelete|TestRestore"`
Expected: All new tests pass.

- [ ] **Step 7: Run all entity tests**

Run: `cd /Users/sgamel/git/DeploySentry && go test ./internal/entities/ -v`
Expected: All tests pass (including existing ones).

- [ ] **Step 8: Commit**

```bash
git add internal/entities/handler_test.go
git commit -m "test: add handler tests for project delete and restore"
```

---

### Task 9: Web — Types and API Client

**Files:**
- Modify: `web/src/types.ts:111-116`
- Modify: `web/src/api.ts:316-325`

- [ ] **Step 1: Update `Project` type**

In `web/src/types.ts`, replace the `Project` interface:

```typescript
export interface Project {
  id: string;
  name: string;
  slug: string;
  org_id: string;
  description?: string;
  repo_url?: string;
  deleted_at?: string;
}

export interface FlagActivitySummary {
  key: string;
  name: string;
  last_evaluated: string;
}
```

- [ ] **Step 2: Update API client — fix `updateProject` and add new methods**

In `web/src/api.ts`, replace the projects section (lines 316-325):

```typescript
  // Projects
  listProjects: (orgSlug: string, includeDeleted = false) =>
    request<{ projects: Project[] }>(
      `/orgs/${orgSlug}/projects${includeDeleted ? '?include_deleted=true' : ''}`,
    ),
  getProject: (orgSlug: string, projectSlug: string) =>
    request<Project>(`/orgs/${orgSlug}/projects/${projectSlug}`),
  createProject: (orgSlug: string, data: { name: string; slug: string }) =>
    request<Project>(`/orgs/${orgSlug}/projects`, { method: 'POST', body: JSON.stringify(data) }),
  updateProject: (
    orgSlug: string,
    projectSlug: string,
    data: { name?: string; description?: string; repo_url?: string },
  ) =>
    request<Project>(`/orgs/${orgSlug}/projects/${projectSlug}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    }),
  softDeleteProject: (orgSlug: string, projectSlug: string) =>
    request<void>(`/orgs/${orgSlug}/projects/${projectSlug}`, { method: 'DELETE' }),
  hardDeleteProject: (orgSlug: string, projectSlug: string) =>
    request<void>(`/orgs/${orgSlug}/projects/${projectSlug}/permanent`, { method: 'DELETE' }),
  restoreProject: (orgSlug: string, projectSlug: string) =>
    request<Project>(`/orgs/${orgSlug}/projects/${projectSlug}/restore`, { method: 'POST' }),
```

Add `FlagActivitySummary` to the import from `types.ts` if types are imported, or ensure the type is available.

- [ ] **Step 3: Verify TypeScript compiles**

Run: `cd /Users/sgamel/git/DeploySentry/web && npx tsc --noEmit`
Expected: No type errors.

- [ ] **Step 4: Commit**

```bash
git add web/src/types.ts web/src/api.ts
git commit -m "feat: update Project type and API client for edit/delete"
```

---

### Task 10: Web — Update `useProjects` Hook

**Files:**
- Modify: `web/src/hooks/useEntities.ts:27-52`

- [ ] **Step 1: Update `useProjects` to accept `includeDeleted`**

Replace the `useProjects` function:

```typescript
export function useProjects(orgSlug: string | undefined, includeDeleted = false) {
  const [projects, setProjects] = useState<Project[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(() => {
    if (!orgSlug) {
      setProjects([]);
      setLoading(false);
      return;
    }
    setLoading(true);
    setError(null);
    entitiesApi
      .listProjects(orgSlug, includeDeleted)
      .then((res) => setProjects(res.projects ?? []))
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, [orgSlug, includeDeleted]);

  useEffect(() => {
    refresh();
  }, [refresh]);

  return { projects, loading, error, refresh };
}
```

- [ ] **Step 2: Verify TypeScript compiles**

Run: `cd /Users/sgamel/git/DeploySentry/web && npx tsc --noEmit`
Expected: No type errors.

- [ ] **Step 3: Commit**

```bash
git add web/src/hooks/useEntities.ts
git commit -m "feat: add includeDeleted param to useProjects hook"
```

---

### Task 11: Web — Project List Page (Gear Icon, Deleted Card Styling)

**Files:**
- Modify: `web/src/pages/ProjectListPage.tsx`

- [ ] **Step 1: Rewrite `ProjectListPage` with gear icon, deleted state, and restore**

Replace the entire file:

```tsx
import { useParams, Link } from 'react-router-dom';
import { useProjects } from '@/hooks/useEntities';
import { entitiesApi } from '@/api';

export default function ProjectListPage() {
  const { orgSlug } = useParams();
  const { projects, loading, error, refresh } = useProjects(orgSlug, true);

  if (!orgSlug) return null;
  if (loading) return <div className="page-loading">Loading projects...</div>;
  if (error) return <div className="page-error">Error: {error}</div>;

  const handleRestore = async (slug: string) => {
    try {
      await entitiesApi.restoreProject(orgSlug, slug);
      refresh();
    } catch (err) {
      console.error('Failed to restore project:', err);
    }
  };

  const formatDate = (dateStr: string) => {
    const d = new Date(dateStr);
    return d.toLocaleDateString('en-US', { month: '2-digit', day: '2-digit', year: 'numeric' });
  };

  const getHardDeleteDate = (deletedAt: string) => {
    const d = new Date(deletedAt);
    d.setDate(d.getDate() + 7);
    return formatDate(d.toISOString());
  };

  return (
    <div>
      <div className="page-header-row">
        <h1 className="page-header">Projects</h1>
        <Link to={`/orgs/${orgSlug}/projects/new`} className="btn btn-primary">
          Create Project
        </Link>
      </div>
      {projects.length === 0 ? (
        <div className="empty-state">
          <p>No projects yet.</p>
          <Link to={`/orgs/${orgSlug}/projects/new`} className="btn btn-primary">
            Create Your First Project
          </Link>
        </div>
      ) : (
        <div className="project-card-grid">
          {projects.map((project) => (
            <div
              key={project.id}
              className={`project-card${project.deleted_at ? ' project-card--deleted' : ''}`}
            >
              {project.deleted_at ? (
                <>
                  <div className="project-card-header">
                    <h3 className="project-card-name">{project.name}</h3>
                    <span className="badge badge-danger">Deleted</span>
                  </div>
                  <span className="project-card-slug">{project.slug}</span>
                  <p className="text-sm text-muted" style={{ marginTop: 8 }}>
                    Hard delete available on {getHardDeleteDate(project.deleted_at)}
                  </p>
                  <button
                    className="btn btn-sm"
                    style={{ marginTop: 8 }}
                    onClick={() => handleRestore(project.slug)}
                  >
                    Restore
                  </button>
                </>
              ) : (
                <>
                  <div className="project-card-header">
                    <Link
                      to={`/orgs/${orgSlug}/projects/${project.slug}/flags`}
                      className="project-card-name-link"
                    >
                      <h3 className="project-card-name">{project.name}</h3>
                    </Link>
                    <Link
                      to={`/orgs/${orgSlug}/projects/${project.slug}/settings`}
                      className="project-card-settings"
                      title="Project Settings"
                      onClick={(e) => e.stopPropagation()}
                    >
                      <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor">
                        <path d="M8 4.754a3.246 3.246 0 1 0 0 6.492 3.246 3.246 0 0 0 0-6.492zM5.754 8a2.246 2.246 0 1 1 4.492 0 2.246 2.246 0 0 1-4.492 0z" />
                        <path d="M9.796 1.343c-.527-1.79-3.065-1.79-3.592 0l-.094.319a.873.873 0 0 1-1.255.52l-.292-.16c-1.64-.892-3.433.902-2.54 2.541l.159.292a.873.873 0 0 1-.52 1.255l-.319.094c-1.79.527-1.79 3.065 0 3.592l.319.094a.873.873 0 0 1 .52 1.255l-.16.292c-.892 1.64.901 3.434 2.541 2.54l.292-.159a.873.873 0 0 1 1.255.52l.094.319c.527 1.79 3.065 1.79 3.592 0l.094-.319a.873.873 0 0 1 1.255-.52l.292.16c1.64.893 3.434-.902 2.54-2.541l-.159-.292a.873.873 0 0 1 .52-1.255l.319-.094c1.79-.527 1.79-3.065 0-3.592l-.319-.094a.873.873 0 0 1-.52-1.255l.16-.292c.893-1.64-.902-3.433-2.541-2.54l-.292.159a.873.873 0 0 1-1.255-.52l-.094-.319zm-2.633.283c.246-.835 1.428-.835 1.674 0l.094.319a1.873 1.873 0 0 0 2.693 1.115l.291-.16c.764-.415 1.6.42 1.184 1.185l-.159.292a1.873 1.873 0 0 0 1.116 2.692l.318.094c.835.246.835 1.428 0 1.674l-.319.094a1.873 1.873 0 0 0-1.115 2.693l.16.291c.415.764-.421 1.6-1.185 1.184l-.291-.159a1.873 1.873 0 0 0-2.693 1.116l-.094.318c-.246.835-1.428.835-1.674 0l-.094-.319a1.873 1.873 0 0 0-2.692-1.115l-.292.16c-.764.415-1.6-.421-1.184-1.185l.159-.291A1.873 1.873 0 0 0 1.945 8.93l-.319-.094c-.835-.246-.835-1.428 0-1.674l.319-.094A1.873 1.873 0 0 0 3.06 4.377l-.16-.292c-.415-.764.42-1.6 1.185-1.184l.292.159a1.873 1.873 0 0 0 2.692-1.116l.094-.318z" />
                      </svg>
                    </Link>
                  </div>
                  <Link
                    to={`/orgs/${orgSlug}/projects/${project.slug}/flags`}
                    className="project-card-slug"
                  >
                    {project.slug}
                  </Link>
                </>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 2: Add CSS for deleted card state and card header**

Add to `web/src/styles/globals.css`:

```css
.project-card--deleted {
  opacity: 0.5;
  pointer-events: auto;
}

.project-card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 4px;
}

.project-card-name-link {
  text-decoration: none;
  color: inherit;
}

.project-card-settings {
  color: var(--color-text-muted);
  padding: 4px;
  border-radius: 4px;
  transition: color 0.15s;
}
.project-card-settings:hover {
  color: var(--color-text-primary);
}

.badge-danger {
  background: var(--color-danger-bg);
  color: var(--color-danger);
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 0.75rem;
  font-weight: 500;
}
```

- [ ] **Step 3: Verify dev server renders correctly**

Run: `cd /Users/sgamel/git/DeploySentry/web && npm run dev`
Open browser and navigate to a project list page. Verify:
- Gear icon appears on each project card
- Cards link correctly to flags and settings

- [ ] **Step 4: Commit**

```bash
git add web/src/pages/ProjectListPage.tsx web/src/styles/globals.css
git commit -m "feat: add gear icon and deleted card styling to project list"
```

---

### Task 12: Web — Project Settings Edit Form and Danger Zone

**Files:**
- Modify: `web/src/pages/SettingsPage.tsx`

- [ ] **Step 1: Add danger zone tab for project level**

In `getTabsForLevel`, update the `project` case (line 48):

```typescript
    case 'project':
      return [
        { key: 'general', label: 'Project Settings' },
        { key: 'danger', label: 'Danger Zone' },
      ];
```

In `defaultTab`, update `validTabs` for project (line 22):

```typescript
    project: ['general', 'danger'],
```

- [ ] **Step 2: Add project state for description, repo_url, and delete**

After the existing project form state section (around line 118), add:

```typescript
  const [projectDescription, setProjectDescription] = useState('');
  const [projectRepoUrl, setProjectRepoUrl] = useState('');
  const [deleteLoading, setDeleteLoading] = useState(false);
  const [activeFlags, setActiveFlags] = useState<{ key: string; name: string; last_evaluated: string }[]>([]);
  const [deletionBlocked, setDeletionBlocked] = useState(false);
  const [deleteCheckDone, setDeleteCheckDone] = useState(false);
```

- [ ] **Step 3: Update `handleSaveProjectSettings` to include description and repo_url**

Replace the handler (around line 271):

```typescript
  const handleSaveProjectSettings = async () => {
    if (!orgSlug || !projectSlug) return;
    setSettingsSaving(true);
    try {
      await entitiesApi.updateProject(orgSlug, projectSlug, {
        name: projectName,
        description: projectDescription,
        repo_url: projectRepoUrl,
      });
      setSettingsSuccess(true);
      setTimeout(() => setSettingsSuccess(false), 3000);
    } catch (err) {
      console.error('Failed to save project settings:', err);
    } finally {
      setSettingsSaving(false);
    }
  };
```

- [ ] **Step 4: Add delete handlers**

Add after `handleSaveProjectSettings`:

```typescript
  const handleSoftDeleteProject = async () => {
    if (!orgSlug || !projectSlug) return;
    if (!window.confirm('Are you sure you want to delete this project? It can be restored within 7 days.')) return;
    setDeleteLoading(true);
    try {
      await entitiesApi.softDeleteProject(orgSlug, projectSlug);
      window.location.href = `/orgs/${orgSlug}/projects`;
    } catch (err: any) {
      const body = err?.body;
      if (body?.active_flags) {
        setActiveFlags(body.active_flags);
        setDeletionBlocked(true);
      }
      console.error('Failed to delete project:', err);
    } finally {
      setDeleteLoading(false);
    }
  };

  const handleHardDeleteProject = async () => {
    if (!orgSlug || !projectSlug) return;
    if (!window.confirm('This will PERMANENTLY delete the project and all associated data. This cannot be undone. Are you sure?')) return;
    setDeleteLoading(true);
    try {
      await entitiesApi.hardDeleteProject(orgSlug, projectSlug);
      window.location.href = `/orgs/${orgSlug}/projects`;
    } catch (err) {
      console.error('Failed to permanently delete project:', err);
    } finally {
      setDeleteLoading(false);
    }
  };
```

- [ ] **Step 5: Add description and repo_url fields to the project general tab**

In the project general tab section (around line 741), add description and repo_url fields after the project name input. Replace the entire `activeTab === 'general' && level === 'project'` block:

```tsx
      {activeTab === 'general' && level === 'project' && (
        <div className="card">
          <div className="card-header">
            <span className="card-title">Project Settings</span>
          </div>
          <div className="form-group">
            <label className="form-label">Project Name</label>
            <input
              type="text"
              className="form-input"
              value={projectName}
              onChange={(e) => setProjectName(e.target.value)}
            />
          </div>

          <div className="form-group">
            <label className="form-label">Description</label>
            <textarea
              className="form-input"
              rows={3}
              value={projectDescription}
              onChange={(e) => setProjectDescription(e.target.value)}
            />
          </div>

          <div className="form-group">
            <label className="form-label">Repository URL</label>
            <input
              type="text"
              className="form-input"
              placeholder="https://github.com/org/repo"
              value={projectRepoUrl}
              onChange={(e) => setProjectRepoUrl(e.target.value)}
            />
          </div>

          <div className="form-group">
            <label className="form-label">Default Environment</label>
            <select
              className="form-select"
              value={defaultEnv}
              onChange={(e) => setDefaultEnv(e.target.value)}
            >
              <option value="development">Development</option>
              <option value="staging">Staging</option>
              <option value="production">Production</option>
            </select>
          </div>

          <div className="form-group">
            <label className="form-label">Stale Flag Threshold</label>
            <input
              type="text"
              className="form-input"
              value={staleThreshold}
              onChange={(e) => setStaleThreshold(e.target.value)}
            />
            <div className="form-hint">
              Flags with no evaluation activity beyond this threshold will be marked as stale.
              Examples: 30d, 2w, 90d.
            </div>
          </div>

          <div className="flex items-center gap-3">
            <button
              className="btn btn-primary"
              onClick={handleSaveProjectSettings}
              disabled={settingsSaving}
            >
              {settingsSaving ? 'Saving\u2026' : 'Save'}
            </button>
            {settingsSuccess && <span className="text-sm text-success">Settings saved.</span>}
          </div>
        </div>
      )}
```

- [ ] **Step 6: Add danger zone section for project level**

Add before the existing app-level danger zone block (`activeTab === 'danger'`). Replace the danger zone conditional to handle both project and app levels:

```tsx
      {/* Danger Zone — project level */}
      {activeTab === 'danger' && level === 'project' && (
        <div className="danger-zone">
          <h3>Delete Project</h3>
          {deletionBlocked && activeFlags.length > 0 && (
            <div className="danger-zone-warning" style={{ marginBottom: 16 }}>
              <p style={{ fontWeight: 500, marginBottom: 8 }}>
                Cannot delete: the following flags have been evaluated in the last 14 days.
              </p>
              <ul style={{ margin: 0, paddingLeft: 20 }}>
                {activeFlags.map((flag) => (
                  <li key={flag.key}>
                    <strong>{flag.name}</strong> ({flag.key}) &mdash; last evaluated{' '}
                    {new Date(flag.last_evaluated).toLocaleDateString()}
                  </li>
                ))}
              </ul>
            </div>
          )}
          <p>
            Deleting this project will remove all its applications, flags, deployments, and releases.
            The project can be restored within 7 days.
          </p>
          <button
            className="btn btn-danger"
            onClick={handleSoftDeleteProject}
            disabled={deleteLoading || deletionBlocked}
          >
            {deleteLoading ? 'Deleting\u2026' : 'Delete Project'}
          </button>
        </div>
      )}

      {/* Danger Zone — app level */}
      {activeTab === 'danger' && level === 'app' && (
```

- [ ] **Step 7: Add CSS for danger zone warning**

Add to `web/src/styles/globals.css`:

```css
.danger-zone-warning {
  padding: 12px 16px;
  background: rgba(239, 68, 68, 0.08);
  border: 1px solid var(--color-danger-bg);
  border-radius: var(--radius-lg);
  color: var(--color-text-primary);
}
```

- [ ] **Step 8: Verify dev server renders correctly**

Run: `cd /Users/sgamel/git/DeploySentry/web && npm run dev`
Navigate to a project settings page. Verify:
- Description and repo_url fields appear in the General tab
- Danger Zone tab exists and shows the delete button
- Save button works

- [ ] **Step 9: Commit**

```bash
git add web/src/pages/SettingsPage.tsx web/src/styles/globals.css
git commit -m "feat: add project edit fields and danger zone to settings page"
```

---

### Task 13: Full Build Verification

- [ ] **Step 1: Run Go tests**

Run: `cd /Users/sgamel/git/DeploySentry && go test ./...`
Expected: All tests pass.

- [ ] **Step 2: Run TypeScript type check**

Run: `cd /Users/sgamel/git/DeploySentry/web && npx tsc --noEmit`
Expected: No errors.

- [ ] **Step 3: Run full build**

Run: `cd /Users/sgamel/git/DeploySentry && go build ./...`
Expected: Build succeeds.

- [ ] **Step 4: Manual verification in browser**

Start API and web servers:
```bash
make run-api &
cd web && npm run dev
```

Test:
1. Navigate to project list — verify gear icon shows on cards
2. Click gear icon — verify settings page opens with General and Danger Zone tabs
3. Edit name, description, repo_url — verify save works
4. Click Danger Zone — verify delete button is visible
5. If flags have recent activity — verify warning banner and disabled button

---
