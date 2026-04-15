# Project & Application Edit/Delete Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add edit (fix broken project update) and delete (instant when no flags, soft-delete when flags exist) for both projects and applications, with full UI support.

**Architecture:** Single migration adds `deleted_at` to both tables. Repository gets count, soft-delete, hard-delete, and restore methods. Service implements the decision logic: zero flags → instant hard delete; flags with no recent activity → soft delete; flags with recent activity → 409. Handler exposes DELETE, DELETE /permanent, and POST /restore routes. Frontend gets gear icons, danger zones, and confirmation dialogs.

**Tech Stack:** Go (backend), PostgreSQL (migrations), React + TypeScript (frontend)

**Spec:** `docs/superpowers/specs/2026-04-13-project-edit-delete-design.md`

---

### Task 1: Migration — add `deleted_at` to projects and applications

**Files:**
- Create: `migrations/039_add_deleted_at.up.sql`
- Create: `migrations/039_add_deleted_at.down.sql`

- [ ] **Step 1: Write the up migration**

```sql
ALTER TABLE projects ADD COLUMN deleted_at TIMESTAMPTZ;
ALTER TABLE applications ADD COLUMN deleted_at TIMESTAMPTZ;
```

- [ ] **Step 2: Write the down migration**

```sql
ALTER TABLE applications DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE projects DROP COLUMN IF EXISTS deleted_at;
```

- [ ] **Step 3: Run the migration**

Run: `cd /Users/sgamel/git/DeploySentry && make migrate-up`
Expected: Migration 039 applied successfully

- [ ] **Step 4: Commit**

```bash
git add migrations/039_add_deleted_at.up.sql migrations/039_add_deleted_at.down.sql
git commit -m "feat: add deleted_at column to projects and applications"
```

---

### Task 2: Add `DeletedAt` to Go models

**Files:**
- Modify: `internal/models/project.go:11-20`
- Modify: `internal/models/application.go:11-21`

- [ ] **Step 1: Add `DeletedAt` to Project struct**

In `internal/models/project.go`, add the field to the `Project` struct:

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

- [ ] **Step 2: Add `DeletedAt` to Application struct**

In `internal/models/application.go`, add the field:

```go
type Application struct {
	ID          uuid.UUID  `json:"id"`
	ProjectID   uuid.UUID  `json:"project_id"`
	Name        string     `json:"name"`
	Slug        string     `json:"slug"`
	Description string     `json:"description,omitempty"`
	RepoURL     string     `json:"repo_url,omitempty"`
	CreatedBy   *uuid.UUID `json:"created_by,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	DeletedAt   *time.Time `json:"deleted_at,omitempty"`
}
```

- [ ] **Step 3: Verify build compiles**

Run: `cd /Users/sgamel/git/DeploySentry && go build ./...`
Expected: No errors

- [ ] **Step 4: Commit**

```bash
git add internal/models/project.go internal/models/application.go
git commit -m "feat: add DeletedAt field to Project and Application models"
```

---

### Task 3: Fix project repository — update, get, and list queries

**Files:**
- Modify: `internal/platform/database/postgres/entities.go:136-227`

- [ ] **Step 1: Fix `CreateProject` to include `description` and `repo_url`**

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

Replace the query and scan (lines 159-180):

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

Replace the method signature and implementation (lines 182-213). The interface will be updated in Task 5, but change the concrete method now:

```go
func (r *EntityRepository) ListProjectsByOrg(ctx context.Context, orgID uuid.UUID, includeDeleted bool) ([]*models.Project, error) {
	deletedFilter := " AND deleted_at IS NULL"
	if includeDeleted {
		deletedFilter = ""
	}
	q := `SELECT id, org_id, name, slug, COALESCE(description, ''), COALESCE(repo_url, ''), created_at, updated_at, deleted_at
		FROM projects WHERE org_id = $1` + deletedFilter + ` ORDER BY name`

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

- [ ] **Step 4: Fix `UpdateProject` to persist `description` and `repo_url`**

Replace the method (lines 216-227):

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

- [ ] **Step 5: Verify build compiles** (will fail until interface is updated in Task 5 — that's expected)

Run: `cd /Users/sgamel/git/DeploySentry && go build ./...`
Expected: Compile error on `ListProjectsByOrg` signature mismatch — will be fixed in Task 5

- [ ] **Step 6: Commit**

```bash
git add internal/platform/database/postgres/entities.go
git commit -m "fix: project repo now persists all fields, selects all columns, supports deleted_at"
```

---

### Task 4: Add delete/restore repository methods

**Files:**
- Modify: `internal/platform/database/postgres/entities.go` (append new methods)
- Modify: `internal/platform/database/postgres/flags.go` (add flag count and activity queries)

- [ ] **Step 1: Add `ListAppsByProject` `includeDeleted` support**

Update `ListAppsByProject` to accept `includeDeleted bool` (same pattern as `ListProjectsByOrg`):

```go
func (r *EntityRepository) ListAppsByProject(ctx context.Context, projectID uuid.UUID, includeDeleted bool) ([]*models.Application, error) {
	deletedFilter := " AND deleted_at IS NULL"
	if includeDeleted {
		deletedFilter = ""
	}
	q := `SELECT id, project_id, name, slug, COALESCE(description, ''), COALESCE(repo_url, ''), created_by, created_at, updated_at, deleted_at
		FROM applications WHERE project_id = $1` + deletedFilter + ` ORDER BY name`

	rows, err := r.pool.Query(ctx, q, projectID)
	if err != nil {
		return nil, fmt.Errorf("postgres.ListAppsByProject: %w", err)
	}
	defer rows.Close()

	result := make([]*models.Application, 0)
	for rows.Next() {
		var a models.Application
		if err := rows.Scan(
			&a.ID,
			&a.ProjectID,
			&a.Name,
			&a.Slug,
			&a.Description,
			&a.RepoURL,
			&a.CreatedBy,
			&a.CreatedAt,
			&a.UpdatedAt,
			&a.DeletedAt,
		); err != nil {
			return nil, fmt.Errorf("postgres.ListAppsByProject: %w", err)
		}
		result = append(result, &a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres.ListAppsByProject: %w", err)
	}
	return result, nil
}
```

Also update `GetAppBySlug` to scan `deleted_at`:

```go
func (r *EntityRepository) GetAppBySlug(ctx context.Context, projectID uuid.UUID, slug string) (*models.Application, error) {
	const q = `
		SELECT id, project_id, name, slug, COALESCE(description, ''), COALESCE(repo_url, ''), created_by, created_at, updated_at, deleted_at
		FROM applications WHERE project_id = $1 AND slug = $2`

	var a models.Application
	err := r.pool.QueryRow(ctx, q, projectID, slug).Scan(
		&a.ID,
		&a.ProjectID,
		&a.Name,
		&a.Slug,
		&a.Description,
		&a.RepoURL,
		&a.CreatedBy,
		&a.CreatedAt,
		&a.UpdatedAt,
		&a.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("postgres.GetAppBySlug: %w", err)
	}
	return &a, nil
}
```

- [ ] **Step 2: Add project delete/restore repository methods**

Append to `internal/platform/database/postgres/entities.go`:

```go
// CountFlagsByProject returns the number of flags in a project.
func (r *EntityRepository) CountFlagsByProject(ctx context.Context, projectID uuid.UUID) (int, error) {
	const q = `SELECT COUNT(*) FROM feature_flags WHERE project_id = $1`
	var count int
	err := r.pool.QueryRow(ctx, q, projectID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("postgres.CountFlagsByProject: %w", err)
	}
	return count, nil
}

// SoftDeleteProject sets deleted_at on a project.
func (r *EntityRepository) SoftDeleteProject(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE projects SET deleted_at = now(), updated_at = now() WHERE id = $1 AND deleted_at IS NULL`
	tag, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("postgres.SoftDeleteProject: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// HardDeleteProject permanently deletes a project (cascades to apps, flags, etc.).
func (r *EntityRepository) HardDeleteProject(ctx context.Context, id uuid.UUID) error {
	const q = `DELETE FROM projects WHERE id = $1`
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
	const q = `UPDATE projects SET deleted_at = NULL, updated_at = now() WHERE id = $1 AND deleted_at IS NOT NULL`
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

- [ ] **Step 3: Add application delete/restore repository methods**

Append to `internal/platform/database/postgres/entities.go`:

```go
// CountFlagsByApp returns the number of flags scoped to an application.
func (r *EntityRepository) CountFlagsByApp(ctx context.Context, applicationID uuid.UUID) (int, error) {
	const q = `SELECT COUNT(*) FROM feature_flags WHERE application_id = $1`
	var count int
	err := r.pool.QueryRow(ctx, q, applicationID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("postgres.CountFlagsByApp: %w", err)
	}
	return count, nil
}

// SoftDeleteApp sets deleted_at on an application.
func (r *EntityRepository) SoftDeleteApp(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE applications SET deleted_at = now(), updated_at = now() WHERE id = $1 AND deleted_at IS NULL`
	tag, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("postgres.SoftDeleteApp: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// HardDeleteApp permanently deletes an application.
func (r *EntityRepository) HardDeleteApp(ctx context.Context, id uuid.UUID) error {
	const q = `DELETE FROM applications WHERE id = $1`
	tag, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("postgres.HardDeleteApp: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// RestoreApp clears deleted_at on a soft-deleted application.
func (r *EntityRepository) RestoreApp(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE applications SET deleted_at = NULL, updated_at = now() WHERE id = $1 AND deleted_at IS NOT NULL`
	tag, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("postgres.RestoreApp: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
```

- [ ] **Step 4: Add flag activity query**

Append to `internal/platform/database/postgres/entities.go` (or `flags.go` — but keeping it with entities since it's consumed by the entity service):

```go
// FlagActivitySummary describes a flag with recent evaluation activity.
type FlagActivitySummary struct {
	Key           string    `json:"key"`
	Name          string    `json:"name"`
	LastEvaluated time.Time `json:"last_evaluated"`
}

// HasRecentFlagActivity returns flags with evaluations after `since`.
// When applicationID is nil, checks all flags in the project.
// When applicationID is non-nil, checks only flags with that application_id.
func (r *EntityRepository) HasRecentFlagActivity(ctx context.Context, projectID uuid.UUID, applicationID *uuid.UUID, since time.Time) ([]FlagActivitySummary, error) {
	appFilter := ""
	args := []interface{}{projectID, since}
	if applicationID != nil {
		appFilter = " AND f.application_id = $3"
		args = append(args, *applicationID)
	}

	q := `SELECT f.key, f.name, MAX(l.evaluated_at) as last_evaluated
		FROM feature_flags f
		JOIN flag_evaluation_log l ON f.id = l.flag_id
		WHERE f.project_id = $1 AND l.evaluated_at >= $2` + appFilter + `
		GROUP BY f.key, f.name
		ORDER BY last_evaluated DESC`

	rows, err := r.pool.Query(ctx, q, args...)
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
	return result, rows.Err()
}
```

- [ ] **Step 5: Commit**

```bash
git add internal/platform/database/postgres/entities.go
git commit -m "feat: add delete/restore/count/activity repo methods for projects and apps"
```

---

### Task 5: Update repository and service interfaces

**Files:**
- Modify: `internal/entities/repository.go:10-30`
- Modify: `internal/entities/service.go:12-36`

- [ ] **Step 1: Update `EntityRepository` interface**

Replace the interface in `internal/entities/repository.go`:

```go
type EntityRepository interface {
	CreateOrg(ctx context.Context, org *models.Organization) error
	GetOrgBySlug(ctx context.Context, slug string) (*models.Organization, error)
	ListOrgsByUser(ctx context.Context, userID uuid.UUID) ([]*models.Organization, error)
	UpdateOrg(ctx context.Context, org *models.Organization) error

	CreateProject(ctx context.Context, project *models.Project) error
	GetProjectBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*models.Project, error)
	ListProjectsByOrg(ctx context.Context, orgID uuid.UUID, includeDeleted bool) ([]*models.Project, error)
	UpdateProject(ctx context.Context, project *models.Project) error
	CountFlagsByProject(ctx context.Context, projectID uuid.UUID) (int, error)
	SoftDeleteProject(ctx context.Context, id uuid.UUID) error
	HardDeleteProject(ctx context.Context, id uuid.UUID) error
	RestoreProject(ctx context.Context, id uuid.UUID) error

	CreateApp(ctx context.Context, app *models.Application) error
	GetAppBySlug(ctx context.Context, projectID uuid.UUID, slug string) (*models.Application, error)
	ListAppsByProject(ctx context.Context, projectID uuid.UUID, includeDeleted bool) ([]*models.Application, error)
	UpdateApp(ctx context.Context, app *models.Application) error
	CountFlagsByApp(ctx context.Context, applicationID uuid.UUID) (int, error)
	SoftDeleteApp(ctx context.Context, id uuid.UUID) error
	HardDeleteApp(ctx context.Context, id uuid.UUID) error
	RestoreApp(ctx context.Context, id uuid.UUID) error

	HasRecentFlagActivity(ctx context.Context, projectID uuid.UUID, applicationID *uuid.UUID, since time.Time) ([]postgres.FlagActivitySummary, error)

	ListEnvironmentsByApp(ctx context.Context, appID uuid.UUID) ([]*models.Environment, error)

	AddOrgMember(ctx context.Context, orgID, userID uuid.UUID, role string) error
}
```

Note: You'll need to import the `postgres` package for `FlagActivitySummary`, or move the struct to a shared location (e.g., `internal/models/` or define it in the `entities` package). The cleaner approach is to define it in the entities package:

```go
// FlagActivitySummary describes a flag with recent evaluation activity.
type FlagActivitySummary struct {
	Key           string    `json:"key"`
	Name          string    `json:"name"`
	LastEvaluated time.Time `json:"last_evaluated"`
}
```

Then have the repository method return `[]FlagActivitySummary` using the entities package type.

- [ ] **Step 2: Update `EntityService` interface**

Add delete/restore methods to the interface in `internal/entities/service.go`:

```go
type EntityService interface {
	CreateOrg(ctx context.Context, org *models.Organization, creatorID uuid.UUID) error
	GetOrgBySlug(ctx context.Context, slug string) (*models.Organization, error)
	ListOrgsByUser(ctx context.Context, userID uuid.UUID) ([]*models.Organization, error)
	UpdateOrg(ctx context.Context, org *models.Organization) error

	CreateProject(ctx context.Context, project *models.Project) error
	GetProjectBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*models.Project, error)
	ListProjectsByOrg(ctx context.Context, orgID uuid.UUID, includeDeleted bool) ([]*models.Project, error)
	UpdateProject(ctx context.Context, project *models.Project) error
	DeleteProject(ctx context.Context, orgID uuid.UUID, slug string) (*DeleteResult, error)
	HardDeleteProject(ctx context.Context, orgID uuid.UUID, slug string) error
	RestoreProject(ctx context.Context, orgID uuid.UUID, slug string) (*models.Project, error)

	CreateApp(ctx context.Context, app *models.Application) error
	GetAppBySlug(ctx context.Context, projectID uuid.UUID, slug string) (*models.Application, error)
	ListAppsByProject(ctx context.Context, projectID uuid.UUID, includeDeleted bool) ([]*models.Application, error)
	UpdateApp(ctx context.Context, app *models.Application) error
	DeleteApp(ctx context.Context, projectID uuid.UUID, slug string) (*DeleteResult, error)
	HardDeleteApp(ctx context.Context, projectID uuid.UUID, slug string) error
	RestoreApp(ctx context.Context, projectID uuid.UUID, slug string) (*models.Application, error)

	ListEnvironmentsByApp(ctx context.Context, appID uuid.UUID) ([]*models.Environment, error)

	ListEnvironments(ctx context.Context, orgID uuid.UUID) ([]OrgEnvironment, error)
	CreateEnvironment(ctx context.Context, env *OrgEnvironment) error
	GetEnvironmentBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*OrgEnvironment, error)
	UpdateEnvironment(ctx context.Context, env *OrgEnvironment) error
	DeleteEnvironment(ctx context.Context, id uuid.UUID) error
}
```

- [ ] **Step 3: Define `DeleteResult` type**

Add to `internal/entities/service.go`:

```go
// DeleteResult describes the outcome of a delete operation.
type DeleteResult struct {
	// "permanent" if hard-deleted immediately (no flags), "soft" if soft-deleted.
	Deleted               string                `json:"deleted"`
	EligibleForHardDelete *time.Time            `json:"eligible_for_hard_delete,omitempty"`
	ActiveFlags           []FlagActivitySummary  `json:"active_flags,omitempty"`
}

const (
	recentActivityWindow    = 14 * 24 * time.Hour
	hardDeleteEligibleAfter = 7 * 24 * time.Hour
)
```

- [ ] **Step 4: Verify build compiles** (will fail until service implementations exist in Task 6 — that's expected)

- [ ] **Step 5: Commit**

```bash
git add internal/entities/repository.go internal/entities/service.go
git commit -m "feat: update entity interfaces with delete/restore methods and DeleteResult type"
```

---

### Task 6: Implement service delete/restore logic

**Files:**
- Modify: `internal/entities/service.go` (add method implementations)

- [ ] **Step 1: Update existing `ListProjectsByOrg` and `ListAppsByProject` to pass `includeDeleted`**

```go
func (s *entityService) ListProjectsByOrg(ctx context.Context, orgID uuid.UUID, includeDeleted bool) ([]*models.Project, error) {
	return s.repo.ListProjectsByOrg(ctx, orgID, includeDeleted)
}

func (s *entityService) ListAppsByProject(ctx context.Context, projectID uuid.UUID, includeDeleted bool) ([]*models.Application, error) {
	return s.repo.ListAppsByProject(ctx, projectID, includeDeleted)
}
```

- [ ] **Step 2: Implement `DeleteProject`**

```go
func (s *entityService) DeleteProject(ctx context.Context, orgID uuid.UUID, slug string) (*DeleteResult, error) {
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

	flagCount, err := s.repo.CountFlagsByProject(ctx, project.ID)
	if err != nil {
		return nil, err
	}

	// No flags — instant hard delete
	if flagCount == 0 {
		if err := s.repo.HardDeleteProject(ctx, project.ID); err != nil {
			return nil, err
		}
		return &DeleteResult{Deleted: "permanent"}, nil
	}

	// Has flags — check for recent activity
	since := time.Now().UTC().Add(-recentActivityWindow)
	activeFlags, err := s.repo.HasRecentFlagActivity(ctx, project.ID, nil, since)
	if err != nil {
		return nil, err
	}
	if len(activeFlags) > 0 {
		return &DeleteResult{ActiveFlags: activeFlags}, nil
	}

	// No recent activity — soft delete
	if err := s.repo.SoftDeleteProject(ctx, project.ID); err != nil {
		return nil, err
	}
	eligible := time.Now().UTC().Add(hardDeleteEligibleAfter)
	return &DeleteResult{Deleted: "soft", EligibleForHardDelete: &eligible}, nil
}
```

- [ ] **Step 3: Implement `HardDeleteProject`**

```go
func (s *entityService) HardDeleteProject(ctx context.Context, orgID uuid.UUID, slug string) error {
	project, err := s.repo.GetProjectBySlug(ctx, orgID, slug)
	if err != nil {
		return err
	}
	if project == nil {
		return errors.New("project not found")
	}
	if project.DeletedAt == nil {
		return errors.New("project must be soft-deleted before hard delete")
	}
	eligible := project.DeletedAt.Add(hardDeleteEligibleAfter)
	if time.Now().UTC().Before(eligible) {
		return fmt.Errorf("project must be soft-deleted for at least 7 days; eligible at %s", eligible.Format(time.RFC3339))
	}
	return s.repo.HardDeleteProject(ctx, project.ID)
}
```

- [ ] **Step 4: Implement `RestoreProject`**

```go
func (s *entityService) RestoreProject(ctx context.Context, orgID uuid.UUID, slug string) (*models.Project, error) {
	project, err := s.repo.GetProjectBySlug(ctx, orgID, slug)
	if err != nil {
		return nil, err
	}
	if project == nil {
		return nil, errors.New("project not found")
	}
	if project.DeletedAt == nil {
		return nil, errors.New("project is not deleted")
	}
	if err := s.repo.RestoreProject(ctx, project.ID); err != nil {
		return nil, err
	}
	project.DeletedAt = nil
	return project, nil
}
```

- [ ] **Step 5: Implement `DeleteApp`**

```go
func (s *entityService) DeleteApp(ctx context.Context, projectID uuid.UUID, slug string) (*DeleteResult, error) {
	app, err := s.repo.GetAppBySlug(ctx, projectID, slug)
	if err != nil {
		return nil, err
	}
	if app == nil {
		return nil, errors.New("application not found")
	}
	if app.DeletedAt != nil {
		return nil, errors.New("application is already deleted")
	}

	flagCount, err := s.repo.CountFlagsByApp(ctx, app.ID)
	if err != nil {
		return nil, err
	}

	// No flags — instant hard delete
	if flagCount == 0 {
		if err := s.repo.HardDeleteApp(ctx, app.ID); err != nil {
			return nil, err
		}
		return &DeleteResult{Deleted: "permanent"}, nil
	}

	// Has flags — check for recent activity
	since := time.Now().UTC().Add(-recentActivityWindow)
	activeFlags, err := s.repo.HasRecentFlagActivity(ctx, app.ProjectID, &app.ID, since)
	if err != nil {
		return nil, err
	}
	if len(activeFlags) > 0 {
		return &DeleteResult{ActiveFlags: activeFlags}, nil
	}

	// No recent activity — soft delete
	if err := s.repo.SoftDeleteApp(ctx, app.ID); err != nil {
		return nil, err
	}
	eligible := time.Now().UTC().Add(hardDeleteEligibleAfter)
	return &DeleteResult{Deleted: "soft", EligibleForHardDelete: &eligible}, nil
}
```

- [ ] **Step 6: Implement `HardDeleteApp`**

```go
func (s *entityService) HardDeleteApp(ctx context.Context, projectID uuid.UUID, slug string) error {
	app, err := s.repo.GetAppBySlug(ctx, projectID, slug)
	if err != nil {
		return err
	}
	if app == nil {
		return errors.New("application not found")
	}
	if app.DeletedAt == nil {
		return errors.New("application must be soft-deleted before hard delete")
	}
	eligible := app.DeletedAt.Add(hardDeleteEligibleAfter)
	if time.Now().UTC().Before(eligible) {
		return fmt.Errorf("application must be soft-deleted for at least 7 days; eligible at %s", eligible.Format(time.RFC3339))
	}
	return s.repo.HardDeleteApp(ctx, app.ID)
}
```

- [ ] **Step 7: Implement `RestoreApp`**

```go
func (s *entityService) RestoreApp(ctx context.Context, projectID uuid.UUID, slug string) (*models.Application, error) {
	app, err := s.repo.GetAppBySlug(ctx, projectID, slug)
	if err != nil {
		return nil, err
	}
	if app == nil {
		return nil, errors.New("application not found")
	}
	if app.DeletedAt == nil {
		return nil, errors.New("application is not deleted")
	}
	if err := s.repo.RestoreApp(ctx, app.ID); err != nil {
		return nil, err
	}
	app.DeletedAt = nil
	return app, nil
}
```

- [ ] **Step 8: Verify build compiles**

Run: `cd /Users/sgamel/git/DeploySentry && go build ./...`
Expected: No errors

- [ ] **Step 9: Commit**

```bash
git add internal/entities/service.go
git commit -m "feat: implement delete/restore service logic for projects and applications"
```

---

### Task 7: Add delete/restore HTTP handlers and routes

**Files:**
- Modify: `internal/entities/handler.go:25-58` (routes)
- Modify: `internal/entities/handler.go` (append handlers)

- [ ] **Step 1: Register new routes**

In `RegisterRoutes`, add delete/restore routes for projects (after the PUT on line 46):

```go
projects.DELETE("/:projectSlug", auth.RequirePermission(h.rbac, auth.PermProjectManage), h.deleteProject)
projects.DELETE("/:projectSlug/permanent", auth.RequirePermission(h.rbac, auth.PermOrgManage), h.hardDeleteProject)
projects.POST("/:projectSlug/restore", auth.RequirePermission(h.rbac, auth.PermProjectManage), h.restoreProject)
```

And for applications (after the PUT on line 53):

```go
apps.DELETE("/:appSlug", auth.RequirePermission(h.rbac, auth.PermProjectManage), h.deleteApp)
apps.DELETE("/:appSlug/permanent", auth.RequirePermission(h.rbac, auth.PermOrgManage), h.hardDeleteApp)
apps.POST("/:appSlug/restore", auth.RequirePermission(h.rbac, auth.PermProjectManage), h.restoreApp)
```

- [ ] **Step 2: Update `listProjects` handler to support `include_deleted`**

In the `listProjects` handler, read the query param and pass it through:

```go
func (h *Handler) listProjects(c *gin.Context) {
	org, err := h.resolveOrg(c)
	if err != nil {
		return
	}
	includeDeleted := c.Query("include_deleted") == "true"
	projects, err := h.service.ListProjectsByOrg(c.Request.Context(), org.ID, includeDeleted)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list projects"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"projects": projects})
}
```

Apply the same pattern to `listApps`.

- [ ] **Step 3: Add `deleteProject` handler**

```go
func (h *Handler) deleteProject(c *gin.Context) {
	org, err := h.resolveOrg(c)
	if err != nil {
		return
	}
	result, err := h.service.DeleteProject(c.Request.Context(), org.ID, c.Param("projectSlug"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if len(result.ActiveFlags) > 0 {
		c.JSON(http.StatusConflict, gin.H{
			"error":        "project has flags with recent activity",
			"active_flags": result.ActiveFlags,
		})
		return
	}
	c.JSON(http.StatusOK, result)
}
```

- [ ] **Step 4: Add `hardDeleteProject` handler**

```go
func (h *Handler) hardDeleteProject(c *gin.Context) {
	org, err := h.resolveOrg(c)
	if err != nil {
		return
	}
	err = h.service.HardDeleteProject(c.Request.Context(), org.ID, c.Param("projectSlug"))
	if err != nil {
		if strings.Contains(err.Error(), "eligible at") {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
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
	org, err := h.resolveOrg(c)
	if err != nil {
		return
	}
	project, err := h.service.RestoreProject(c.Request.Context(), org.ID, c.Param("projectSlug"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, project)
}
```

- [ ] **Step 6: Add `deleteApp`, `hardDeleteApp`, `restoreApp` handlers**

Follow the same pattern as project handlers, but resolve the project first, then pass `project.ID` and `appSlug` to the service:

```go
func (h *Handler) deleteApp(c *gin.Context) {
	org, err := h.resolveOrg(c)
	if err != nil {
		return
	}
	project, err := h.service.GetProjectBySlug(c.Request.Context(), org.ID, c.Param("projectSlug"))
	if err != nil || project == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
		return
	}
	result, err := h.service.DeleteApp(c.Request.Context(), project.ID, c.Param("appSlug"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if len(result.ActiveFlags) > 0 {
		c.JSON(http.StatusConflict, gin.H{
			"error":        "application has flags with recent activity",
			"active_flags": result.ActiveFlags,
		})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handler) hardDeleteApp(c *gin.Context) {
	org, err := h.resolveOrg(c)
	if err != nil {
		return
	}
	project, err := h.service.GetProjectBySlug(c.Request.Context(), org.ID, c.Param("projectSlug"))
	if err != nil || project == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
		return
	}
	err = h.service.HardDeleteApp(c.Request.Context(), project.ID, c.Param("appSlug"))
	if err != nil {
		if strings.Contains(err.Error(), "eligible at") {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) restoreApp(c *gin.Context) {
	org, err := h.resolveOrg(c)
	if err != nil {
		return
	}
	project, err := h.service.GetProjectBySlug(c.Request.Context(), org.ID, c.Param("projectSlug"))
	if err != nil || project == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
		return
	}
	app, err := h.service.RestoreApp(c.Request.Context(), project.ID, c.Param("appSlug"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, app)
}
```

- [ ] **Step 7: Verify build compiles**

Run: `cd /Users/sgamel/git/DeploySentry && go build ./...`
Expected: No errors

- [ ] **Step 8: Commit**

```bash
git add internal/entities/handler.go
git commit -m "feat: add delete/restore HTTP handlers and routes for projects and apps"
```

---

### Task 8: Update frontend types and API client

**Files:**
- Modify: `web/src/types.ts:111-116, 166-175`
- Modify: `web/src/api.ts` (entities section)

- [ ] **Step 1: Update `Project` interface**

In `web/src/types.ts`, replace the Project interface (lines 111-116):

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
```

- [ ] **Step 2: Add `deleted_at` to `Application` interface**

```typescript
export interface Application {
  id: string;
  project_id: string;
  name: string;
  slug: string;
  description?: string;
  repo_url?: string;
  created_at: string;
  updated_at: string;
  deleted_at?: string;
}
```

- [ ] **Step 3: Add `FlagActivitySummary` and `DeleteResult` types**

```typescript
export interface FlagActivitySummary {
  key: string;
  name: string;
  last_evaluated: string;
}

export interface DeleteResult {
  deleted?: 'permanent' | 'soft';
  eligible_for_hard_delete?: string;
  active_flags?: FlagActivitySummary[];
}
```

- [ ] **Step 4: Fix `updateProject` API method and add delete methods**

In `web/src/api.ts`, update `updateProject` to accept all fields:

```typescript
updateProject: (orgSlug: string, projectSlug: string, data: { name?: string; description?: string; repo_url?: string }) =>
  request<Project>(`/orgs/${orgSlug}/projects/${projectSlug}`, {
    method: 'PUT',
    body: JSON.stringify(data),
  }),
```

Add delete/restore methods for projects:

```typescript
deleteProject: (orgSlug: string, projectSlug: string) =>
  request<DeleteResult>(`/orgs/${orgSlug}/projects/${projectSlug}`, { method: 'DELETE' }),

hardDeleteProject: (orgSlug: string, projectSlug: string) =>
  request<void>(`/orgs/${orgSlug}/projects/${projectSlug}/permanent`, { method: 'DELETE' }),

restoreProject: (orgSlug: string, projectSlug: string) =>
  request<Project>(`/orgs/${orgSlug}/projects/${projectSlug}/restore`, { method: 'POST' }),
```

Add delete/restore methods for applications:

```typescript
deleteApp: (orgSlug: string, projectSlug: string, appSlug: string) =>
  request<DeleteResult>(`/orgs/${orgSlug}/projects/${projectSlug}/apps/${appSlug}`, { method: 'DELETE' }),

hardDeleteApp: (orgSlug: string, projectSlug: string, appSlug: string) =>
  request<void>(`/orgs/${orgSlug}/projects/${projectSlug}/apps/${appSlug}/permanent`, { method: 'DELETE' }),

restoreApp: (orgSlug: string, projectSlug: string, appSlug: string) =>
  request<Application>(`/orgs/${orgSlug}/projects/${projectSlug}/apps/${appSlug}/restore`, { method: 'POST' }),
```

- [ ] **Step 5: Update `listProjects` to support `include_deleted`**

```typescript
listProjects: (orgSlug: string, includeDeleted = false) =>
  request<{ projects: Project[] }>(`/orgs/${orgSlug}/projects${includeDeleted ? '?include_deleted=true' : ''}`),
```

- [ ] **Step 6: Commit**

```bash
git add web/src/types.ts web/src/api.ts
git commit -m "feat: add delete/restore types and API methods for projects and apps"
```

---

### Task 9: Update frontend hooks

**Files:**
- Modify: `web/src/hooks/useEntities.ts`

- [ ] **Step 1: Update `useProjects` to support `includeDeleted`**

Update the hook to accept and pass the parameter:

```typescript
export function useProjects(orgSlug: string, includeDeleted = false) {
  const [projects, setProjects] = useState<Project[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchProjects = useCallback(async () => {
    if (!orgSlug) return;
    setLoading(true);
    try {
      const data = await entitiesApi.listProjects(orgSlug, includeDeleted);
      setProjects(data.projects || []);
      setError(null);
    } catch (err: any) {
      setError(err.message || 'Failed to load projects');
    } finally {
      setLoading(false);
    }
  }, [orgSlug, includeDeleted]);

  useEffect(() => {
    fetchProjects();
  }, [fetchProjects]);

  return { projects, loading, error, refetch: fetchProjects };
}
```

Apply the same pattern to `useApps` if it exists and needs the parameter.

- [ ] **Step 2: Commit**

```bash
git add web/src/hooks/useEntities.ts
git commit -m "feat: useProjects hook supports includeDeleted param"
```

---

### Task 10: Update ProjectListPage with gear icon and deleted styling

**Files:**
- Modify: `web/src/pages/ProjectListPage.tsx`

- [ ] **Step 1: Update ProjectListPage**

Add gear icon linking to settings, show deleted projects with dimmed styling and restore button. The exact implementation depends on the current page structure — follow the existing card pattern. Key changes:

- Fetch with `includeDeleted: true`
- Add a gear icon (⚙️ or Settings icon from the existing icon library) on each card, linking to `/orgs/${orgSlug}/projects/${projectSlug}/settings`
- For deleted projects: add `.opacity-50` or similar dimmed class, show a "Deleted" badge, show "Restore" button that calls `entitiesApi.restoreProject`, show "Hard delete available on" date text
- Add refetch callback after restore

- [ ] **Step 2: Verify in browser**

Run: `cd /Users/sgamel/git/DeploySentry && make run-web`
Navigate to the project list page. Verify gear icons appear. If test data includes soft-deleted projects, verify dimmed styling.

- [ ] **Step 3: Commit**

```bash
git add web/src/pages/ProjectListPage.tsx
git commit -m "feat: project list shows gear icon, deleted projects with restore option"
```

---

### Task 11: Update ApplicationsListPage with gear icon and deleted styling

**Files:**
- Modify: `web/src/pages/ApplicationsListPage.tsx`

- [ ] **Step 1: Update ApplicationsListPage**

Same pattern as ProjectListPage:
- Gear icon linking to app settings
- Deleted app styling (dimmed, badge, restore button, hard-delete date)
- Refetch after restore

- [ ] **Step 2: Verify in browser**

- [ ] **Step 3: Commit**

```bash
git add web/src/pages/ApplicationsListPage.tsx
git commit -m "feat: app list shows gear icon, deleted apps with restore option"
```

---

### Task 12: Update SettingsPage with project edit form and danger zone

**Files:**
- Modify: `web/src/pages/SettingsPage.tsx`

- [ ] **Step 1: Fix project settings edit form**

Update the project-level "General" tab to include editable fields for `name`, `description`, and `repo_url`. The save handler should call `entitiesApi.updateProject` with all three fields.

- [ ] **Step 2: Add project danger zone**

Add a danger zone section at the bottom of the project settings page:

- **No flags**: Show delete button. Confirmation dialog: "This project has no flags and will be permanently deleted. This cannot be undone."
- **Flags with recent activity**: Show warning listing active flags. Delete button disabled.
- **Flags, no activity**: Show delete button. Confirmation: "This will soft-delete the project. It can be restored within 7 days."
- **Already soft-deleted, 7+ days old**: Show hard delete button. Confirmation: "This will permanently delete the project and all associated data. This cannot be undone."

Follow the existing danger zone CSS pattern used for environments.

- [ ] **Step 3: Add application danger zone**

Same pattern as project danger zone, but for the application settings tab. Call `entitiesApi.deleteApp` / `entitiesApi.hardDeleteApp`.

- [ ] **Step 4: Verify in browser**

Test: project settings edit saves all fields. Test: delete button behavior with and without flags.

- [ ] **Step 5: Commit**

```bash
git add web/src/pages/SettingsPage.tsx
git commit -m "feat: project/app settings with full edit form and danger zone"
```

---

### Task 13: Backend tests

**Files:**
- Modify: `internal/entities/handler_test.go`

- [ ] **Step 1: Add mock methods for new service interface**

Add mock implementations for `DeleteProject`, `HardDeleteProject`, `RestoreProject`, `DeleteApp`, `HardDeleteApp`, `RestoreApp` to the mock service in the test file.

- [ ] **Step 2: Write test: delete project with no flags (instant delete)**

```go
func TestDeleteProject_NoFlags(t *testing.T) {
	// Setup: mock DeleteProject returns DeleteResult{Deleted: "permanent"}
	// Expect: 200 with {"deleted": "permanent"}
}
```

- [ ] **Step 3: Write test: delete project blocked by active flags**

```go
func TestDeleteProject_ActiveFlags(t *testing.T) {
	// Setup: mock DeleteProject returns DeleteResult with ActiveFlags populated
	// Expect: 409 with active_flags array
}
```

- [ ] **Step 4: Write test: soft delete project (flags, no activity)**

```go
func TestDeleteProject_SoftDelete(t *testing.T) {
	// Setup: mock DeleteProject returns DeleteResult{Deleted: "soft", EligibleForHardDelete: <date>}
	// Expect: 200 with {"deleted": "soft", "eligible_for_hard_delete": "..."}
}
```

- [ ] **Step 5: Write test: hard delete too early**

```go
func TestHardDeleteProject_TooEarly(t *testing.T) {
	// Setup: mock HardDeleteProject returns error with "eligible at"
	// Expect: 409
}
```

- [ ] **Step 6: Write test: restore project**

```go
func TestRestoreProject(t *testing.T) {
	// Setup: mock RestoreProject returns project with nil DeletedAt
	// Expect: 200 with project JSON
}
```

- [ ] **Step 7: Run tests**

Run: `cd /Users/sgamel/git/DeploySentry && go test ./internal/entities/...`
Expected: All tests pass

- [ ] **Step 8: Commit**

```bash
git add internal/entities/handler_test.go
git commit -m "test: add tests for project/app delete, hard delete, and restore handlers"
```

---

### Task 14: Full build verification

- [ ] **Step 1: Run all Go tests**

Run: `cd /Users/sgamel/git/DeploySentry && go test ./...`
Expected: All tests pass

- [ ] **Step 2: Build Go backend**

Run: `cd /Users/sgamel/git/DeploySentry && go build ./...`
Expected: No errors

- [ ] **Step 3: Run web dev server and verify**

Run: `cd /Users/sgamel/git/DeploySentry && make run-web`
Verify:
- Project list shows gear icons
- Project settings edit form saves name, description, repo_url
- Delete flow works (test with a project with no flags for instant delete)
- Application list shows gear icons
- Application settings edit and delete work

- [ ] **Step 4: Final commit if any cleanup needed**
