# Entity Management API & UI Wiring Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build backend CRUD for orgs/projects/apps and wire the entire web dashboard to use real API calls, eliminating mock data for entity management.

**Architecture:** A single `internal/entities/` package provides handler, service, and repository for the org > project > app hierarchy. The frontend replaces all mock entity imports with React hooks that call the real API. Non-entity mocks (flags, deployments, etc.) stay for now.

**Tech Stack:** Go 1.22+ / Gin / pgx v5 (backend), React 18 / TypeScript / React Router v6 (frontend)

**Spec:** `docs/superpowers/specs/2026-03-29-entity-management-api-wiring-design.md`

---

## File Structure

| Action | Path | Responsibility |
|--------|------|----------------|
| Create | `internal/entities/repository.go` | EntityRepository interface |
| Create | `internal/entities/service.go` | EntityService interface + implementation |
| Create | `internal/entities/handler.go` | HTTP handler + RegisterRoutes |
| Create | `internal/entities/handler_test.go` | Handler tests with mock service |
| Create | `internal/platform/database/postgres/entities.go` | PostgreSQL repository |
| Modify | `cmd/api/main.go` | Wire up entity handler |
| Modify | `web/src/api.ts` | Add `entitiesApi` module |
| Create | `web/src/hooks/useEntities.ts` | useOrgs, useProjects, useApps hooks |
| Modify | `web/src/mocks/hierarchy.ts` | Remove entity exports, keep non-entity mocks |
| Modify | `web/src/components/OrgSwitcher.tsx` | Use hooks |
| Modify | `web/src/components/ProjectSwitcher.tsx` | Use hooks |
| Modify | `web/src/components/AppAccordion.tsx` | Use hooks |
| Modify | `web/src/components/Sidebar.tsx` | Add Projects nav link |
| Modify | `web/src/components/Breadcrumb.tsx` | Use hooks for name resolution |
| Modify | `web/src/components/DefaultRedirect.tsx` | Use API instead of MOCK_ORGS |
| Modify | `web/src/components/LegacyRedirect.tsx` | Use localStorage only, remove mock imports |
| Modify | `web/src/pages/ProjectListPage.tsx` | Use hooks, enable Create button |
| Modify | `web/src/pages/CreateOrgPage.tsx` | Call real API |
| Modify | `web/src/pages/CreateAppPage.tsx` | Call real API |
| Create | `web/src/pages/CreateProjectPage.tsx` | New create project form |
| Modify | `web/src/App.tsx` | Add CreateProjectPage route |
| Modify | `web/src/pages/FlagListPage.tsx` | Replace name resolution imports |
| Modify | `web/src/pages/DeploymentsPage.tsx` | Replace name resolution imports |
| Modify | `web/src/pages/ReleasesPage.tsx` | Replace name resolution imports |
| Modify | `web/src/pages/SDKsPage.tsx` | Replace name resolution imports |
| Modify | `web/src/pages/AnalyticsPage.tsx` | Replace name resolution imports |

---

### Task 1: Backend — Repository Interface & Service

**Files:**
- Create: `internal/entities/repository.go`
- Create: `internal/entities/service.go`

- [ ] **Step 1: Create repository interface**

```go
// internal/entities/repository.go
package entities

import (
	"context"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
)

// EntityRepository defines persistence for orgs, projects, and applications.
type EntityRepository interface {
	CreateOrg(ctx context.Context, org *models.Organization) error
	GetOrgBySlug(ctx context.Context, slug string) (*models.Organization, error)
	ListOrgsByUser(ctx context.Context, userID uuid.UUID) ([]*models.Organization, error)
	UpdateOrg(ctx context.Context, org *models.Organization) error

	CreateProject(ctx context.Context, project *models.Project) error
	GetProjectBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*models.Project, error)
	ListProjectsByOrg(ctx context.Context, orgID uuid.UUID) ([]*models.Project, error)
	UpdateProject(ctx context.Context, project *models.Project) error

	CreateApp(ctx context.Context, app *models.Application) error
	GetAppBySlug(ctx context.Context, projectID uuid.UUID, slug string) (*models.Application, error)
	ListAppsByProject(ctx context.Context, projectID uuid.UUID) ([]*models.Application, error)
	UpdateApp(ctx context.Context, app *models.Application) error

	AddOrgMember(ctx context.Context, orgID, userID uuid.UUID, role string) error
}
```

- [ ] **Step 2: Create service**

```go
// internal/entities/service.go
package entities

import (
	"context"
	"errors"
	"time"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/google/uuid"
)

// EntityService defines the interface for entity management.
type EntityService interface {
	CreateOrg(ctx context.Context, org *models.Organization, creatorID uuid.UUID) error
	GetOrgBySlug(ctx context.Context, slug string) (*models.Organization, error)
	ListOrgsByUser(ctx context.Context, userID uuid.UUID) ([]*models.Organization, error)
	UpdateOrg(ctx context.Context, org *models.Organization) error

	CreateProject(ctx context.Context, project *models.Project) error
	GetProjectBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*models.Project, error)
	ListProjectsByOrg(ctx context.Context, orgID uuid.UUID) ([]*models.Project, error)
	UpdateProject(ctx context.Context, project *models.Project) error

	CreateApp(ctx context.Context, app *models.Application) error
	GetAppBySlug(ctx context.Context, projectID uuid.UUID, slug string) (*models.Application, error)
	ListAppsByProject(ctx context.Context, projectID uuid.UUID) ([]*models.Application, error)
	UpdateApp(ctx context.Context, app *models.Application) error
}

type entityService struct {
	repo EntityRepository
}

func NewEntityService(repo EntityRepository) EntityService {
	return &entityService{repo: repo}
}

func (s *entityService) CreateOrg(ctx context.Context, org *models.Organization, creatorID uuid.UUID) error {
	org.ID = uuid.New()
	org.OwnerID = creatorID
	if org.Plan == "" {
		org.Plan = "free"
	}
	now := time.Now().UTC()
	org.CreatedAt = now
	org.UpdatedAt = now
	if err := org.Validate(); err != nil {
		return err
	}
	if err := s.repo.CreateOrg(ctx, org); err != nil {
		return err
	}
	return s.repo.AddOrgMember(ctx, org.ID, creatorID, "owner")
}

func (s *entityService) GetOrgBySlug(ctx context.Context, slug string) (*models.Organization, error) {
	if slug == "" {
		return nil, errors.New("slug is required")
	}
	return s.repo.GetOrgBySlug(ctx, slug)
}

func (s *entityService) ListOrgsByUser(ctx context.Context, userID uuid.UUID) ([]*models.Organization, error) {
	return s.repo.ListOrgsByUser(ctx, userID)
}

func (s *entityService) UpdateOrg(ctx context.Context, org *models.Organization) error {
	org.UpdatedAt = time.Now().UTC()
	return s.repo.UpdateOrg(ctx, org)
}

func (s *entityService) CreateProject(ctx context.Context, project *models.Project) error {
	project.ID = uuid.New()
	now := time.Now().UTC()
	project.CreatedAt = now
	project.UpdatedAt = now
	if err := project.Validate(); err != nil {
		return err
	}
	return s.repo.CreateProject(ctx, project)
}

func (s *entityService) GetProjectBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*models.Project, error) {
	return s.repo.GetProjectBySlug(ctx, orgID, slug)
}

func (s *entityService) ListProjectsByOrg(ctx context.Context, orgID uuid.UUID) ([]*models.Project, error) {
	return s.repo.ListProjectsByOrg(ctx, orgID)
}

func (s *entityService) UpdateProject(ctx context.Context, project *models.Project) error {
	project.UpdatedAt = time.Now().UTC()
	return s.repo.UpdateProject(ctx, project)
}

func (s *entityService) CreateApp(ctx context.Context, app *models.Application) error {
	app.ID = uuid.New()
	now := time.Now().UTC()
	app.CreatedAt = now
	app.UpdatedAt = now
	if err := app.Validate(); err != nil {
		return err
	}
	return s.repo.CreateApp(ctx, app)
}

func (s *entityService) GetAppBySlug(ctx context.Context, projectID uuid.UUID, slug string) (*models.Application, error) {
	return s.repo.GetAppBySlug(ctx, projectID, slug)
}

func (s *entityService) ListAppsByProject(ctx context.Context, projectID uuid.UUID) ([]*models.Application, error) {
	return s.repo.ListAppsByProject(ctx, projectID)
}

func (s *entityService) UpdateApp(ctx context.Context, app *models.Application) error {
	app.UpdatedAt = time.Now().UTC()
	return s.repo.UpdateApp(ctx, app)
}
```

- [ ] **Step 3: Verify compilation**

Run: `cd /Users/sgamel/git/DeploySentry && go build ./internal/entities/`
Expected: No errors

- [ ] **Step 4: Commit**

```bash
git add internal/entities/repository.go internal/entities/service.go
git commit -m "feat: add entity repository interface and service for orgs/projects/apps"
```

---

### Task 2: Backend — HTTP Handler with Tests

**Files:**
- Create: `internal/entities/handler.go`
- Create: `internal/entities/handler_test.go`

- [ ] **Step 1: Write handler tests**

Create `internal/entities/handler_test.go` with a mock service and tests covering:
- `TestCreateOrg_Valid` — POST /orgs with name+slug → 201
- `TestCreateOrg_DuplicateSlug` — mock returns error → 409
- `TestListOrgs` — GET /orgs → 200 with orgs array
- `TestGetOrg` — GET /orgs/:orgSlug → 200
- `TestGetOrg_NotFound` — unknown slug → 404
- `TestCreateProject_Valid` — POST /orgs/:orgSlug/projects → 201
- `TestListProjects` — GET /orgs/:orgSlug/projects → 200
- `TestCreateApp_Valid` — POST /orgs/:orgSlug/projects/:projectSlug/apps → 201
- `TestListApps` — GET /orgs/:orgSlug/projects/:projectSlug/apps → 200

Follow the same mock pattern as `internal/ratings/handler_test.go`:
- Mock service with function fields
- `setupEntityRouter` helper that sets `user_id` (uuid.UUID) and `role` (auth.RoleOwner) in gin context
- Use `httptest.NewRequest` + `httptest.NewRecorder`

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/sgamel/git/DeploySentry && go test ./internal/entities/ -v`
Expected: FAIL — `NewHandler` not defined

- [ ] **Step 3: Write handler implementation**

Create `internal/entities/handler.go` with:

```go
package entities

import (
	"net/http"
	"strings"

	"github.com/deploysentry/deploysentry/internal/auth"
	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	service EntityService
	rbac    *auth.RBACChecker
}

func NewHandler(service EntityService, rbac *auth.RBACChecker) *Handler {
	return &Handler{service: service, rbac: rbac}
}

func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	orgs := rg.Group("/orgs")
	{
		orgs.POST("", h.createOrg)   // any authenticated user
		orgs.GET("", h.listOrgs)     // any authenticated user
		orgs.GET("/:orgSlug", h.getOrg)
		orgs.PUT("/:orgSlug", auth.RequirePermission(h.rbac, auth.PermOrgManage), h.updateOrg)

		projects := orgs.Group("/:orgSlug/projects")
		{
			projects.POST("", auth.RequirePermission(h.rbac, auth.PermOrgManage), h.createProject)
			projects.GET("", h.listProjects)
			projects.GET("/:projectSlug", h.getProject)
			projects.PUT("/:projectSlug", auth.RequirePermission(h.rbac, auth.PermProjectManage), h.updateProject)

			apps := projects.Group("/:projectSlug/apps")
			{
				apps.POST("", auth.RequirePermission(h.rbac, auth.PermProjectManage), h.createApp)
				apps.GET("", h.listApps)
				apps.GET("/:appSlug", h.getApp)
				apps.PUT("/:appSlug", auth.RequirePermission(h.rbac, auth.PermProjectManage), h.updateApp)
			}
		}
	}
}
```

Each handler method follows this pattern:
1. Parse slug params
2. Resolve parent entity (for nested routes)
3. Bind JSON request body (for POST/PUT)
4. Call service method
5. Return JSON response

For `createOrg`: extract `user_id` from context using `c.Get("user_id")` with type assertion to `uuid.UUID` (same pattern as flags handler). Call `service.CreateOrg(ctx, org, userID)`.

For slug resolution in nested routes: call `service.GetOrgBySlug` to get the org, then use `org.ID` for project queries. For app routes: resolve org → resolve project → use `project.ID`.

Duplicate slug errors (unique constraint violations) should return 409. Not-found from slug resolution returns 404. Use `strings.Contains(err.Error(), "duplicate")` or check for postgres unique violation error to distinguish.

- [ ] **Step 4: Run tests**

Run: `cd /Users/sgamel/git/DeploySentry && go test ./internal/entities/ -v`
Expected: All tests PASS

- [ ] **Step 5: Commit**

```bash
git add internal/entities/handler.go internal/entities/handler_test.go
git commit -m "feat: add entity management HTTP handler with tests"
```

---

### Task 3: Backend — PostgreSQL Repository & Wiring

**Files:**
- Create: `internal/platform/database/postgres/entities.go`
- Modify: `cmd/api/main.go`

- [ ] **Step 1: Implement PostgreSQL repository**

Create `internal/platform/database/postgres/entities.go` implementing all `EntityRepository` methods. Key implementation details:

- `CreateOrg`: INSERT with service-provided UUID (org.ID set by service layer), unique constraint on slug returns error
- `GetOrgBySlug`: SELECT WHERE slug = $1, return nil error on pgx.ErrNoRows → handler returns 404
- `ListOrgsByUser`: SELECT orgs JOIN org_members WHERE org_members.user_id = $1
- `CreateProject`: INSERT, unique constraint on (org_id, slug) handles duplicates
- `GetProjectBySlug`: SELECT WHERE org_id = $1 AND slug = $2
- `ListProjectsByOrg`: SELECT WHERE org_id = $1 ORDER BY name
- `CreateApp`: INSERT, unique constraint on (project_id, slug) handles duplicates
- `GetAppBySlug`: SELECT WHERE project_id = $1 AND slug = $2
- `ListAppsByProject`: SELECT WHERE project_id = $1 ORDER BY name
- `AddOrgMember`: INSERT into org_members (org_id, user_id, role, joined_at)

For `CreateOrg` transactional safety: The service calls `CreateOrg` then `AddOrgMember` — both use the same context. For true transactional safety, use `pool.Begin(ctx)` in the repository's `CreateOrg` method and have it also insert the member row within the same transaction. OR: add a `CreateOrgWithOwner` method that wraps both in a transaction. The simpler approach: keep the service calling both methods separately — if `AddOrgMember` fails, the org exists without an owner, which is detectable and fixable. For v1 this is acceptable.

- [ ] **Step 2: Wire up in main.go**

Read `cmd/api/main.go`. Add:

1. Import: `"github.com/deploysentry/deploysentry/internal/entities"`
2. In Repositories section: `entityRepo := postgres.NewEntityRepository(db.Pool)`
3. In Services section: `entityService := entities.NewEntityService(entityRepo)`
4. In Routes section (after existing handlers): `entities.NewHandler(entityService, rbacChecker).RegisterRoutes(api)`

- [ ] **Step 3: Verify compilation**

Run: `cd /Users/sgamel/git/DeploySentry && go build ./cmd/api/`
Expected: No errors

- [ ] **Step 4: Run all tests**

Run: `cd /Users/sgamel/git/DeploySentry && go test ./internal/entities/ -v`
Expected: All tests PASS

- [ ] **Step 5: Commit**

```bash
git add internal/platform/database/postgres/entities.go cmd/api/main.go
git commit -m "feat: add PostgreSQL entity repository and wire up in API server"
```

---

### Task 4: Frontend — API Module & Hooks

**Files:**
- Modify: `web/src/api.ts`
- Create: `web/src/hooks/useEntities.ts`

- [ ] **Step 1: Add entitiesApi to api.ts**

Replace the existing `orgsApi` and `applicationsApi` blocks in `web/src/api.ts` with:

```typescript
// Entities (Orgs / Projects / Apps)
export const entitiesApi = {
  // Orgs
  listOrgs: () => request<{ organizations: Organization[] }>('/orgs'),
  getOrg: (slug: string) => request<Organization>(`/orgs/${slug}`),
  createOrg: (data: { name: string; slug: string }) =>
    request<Organization>('/orgs', { method: 'POST', body: JSON.stringify(data) }),
  updateOrg: (slug: string, data: { name: string }) =>
    request<Organization>(`/orgs/${slug}`, { method: 'PUT', body: JSON.stringify(data) }),

  // Projects
  listProjects: (orgSlug: string) =>
    request<{ projects: Project[] }>(`/orgs/${orgSlug}/projects`),
  getProject: (orgSlug: string, projectSlug: string) =>
    request<Project>(`/orgs/${orgSlug}/projects/${projectSlug}`),
  createProject: (orgSlug: string, data: { name: string; slug: string }) =>
    request<Project>(`/orgs/${orgSlug}/projects`, { method: 'POST', body: JSON.stringify(data) }),
  updateProject: (orgSlug: string, projectSlug: string, data: { name: string }) =>
    request<Project>(`/orgs/${orgSlug}/projects/${projectSlug}`, { method: 'PUT', body: JSON.stringify(data) }),

  // Apps
  listApps: (orgSlug: string, projectSlug: string) =>
    request<{ applications: Application[] }>(`/orgs/${orgSlug}/projects/${projectSlug}/apps`),
  getApp: (orgSlug: string, projectSlug: string, appSlug: string) =>
    request<Application>(`/orgs/${orgSlug}/projects/${projectSlug}/apps/${appSlug}`),
  createApp: (orgSlug: string, projectSlug: string, data: { name: string; slug: string; description?: string }) =>
    request<Application>(`/orgs/${orgSlug}/projects/${projectSlug}/apps`, { method: 'POST', body: JSON.stringify(data) }),
  updateApp: (orgSlug: string, projectSlug: string, appSlug: string, data: { name: string; description?: string }) =>
    request<Application>(`/orgs/${orgSlug}/projects/${projectSlug}/apps/${appSlug}`, { method: 'PUT', body: JSON.stringify(data) }),
};
```

Remove the old `orgsApi` and `applicationsApi` blocks.

- [ ] **Step 2: Create entity hooks**

Create `web/src/hooks/useEntities.ts`:

```typescript
import { useState, useEffect, useCallback } from 'react';
import { entitiesApi } from '@/api';
import type { Organization, Project, Application } from '@/types';

export function useOrgs() {
  const [orgs, setOrgs] = useState<Organization[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(() => {
    setLoading(true);
    setError(null);
    entitiesApi.listOrgs()
      .then((res) => setOrgs(res.organizations ?? []))
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, []);

  useEffect(() => { refresh(); }, [refresh]);

  return { orgs, loading, error, refresh };
}

export function useProjects(orgSlug: string | undefined) {
  const [projects, setProjects] = useState<Project[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(() => {
    if (!orgSlug) { setProjects([]); setLoading(false); return; }
    setLoading(true);
    setError(null);
    entitiesApi.listProjects(orgSlug)
      .then((res) => setProjects(res.projects ?? []))
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, [orgSlug]);

  useEffect(() => { refresh(); }, [refresh]);

  return { projects, loading, error, refresh };
}

export function useApps(orgSlug: string | undefined, projectSlug: string | undefined) {
  const [apps, setApps] = useState<Application[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(() => {
    if (!orgSlug || !projectSlug) { setApps([]); setLoading(false); return; }
    setLoading(true);
    setError(null);
    entitiesApi.listApps(orgSlug, projectSlug)
      .then((res) => setApps(res.applications ?? []))
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, [orgSlug, projectSlug]);

  useEffect(() => { refresh(); }, [refresh]);

  return { apps, loading, error, refresh };
}
```

- [ ] **Step 3: Verify TypeScript compilation**

Run: `cd /Users/sgamel/git/DeploySentry/web && npx tsc --noEmit`
Expected: No type errors (may have pre-existing ones — focus on new files)

- [ ] **Step 4: Commit**

```bash
git add web/src/api.ts web/src/hooks/useEntities.ts
git commit -m "feat: add entitiesApi module and useOrgs/useProjects/useApps hooks"
```

---

### Task 5: Frontend — Strip Entity Exports from Mock File

**Files:**
- Modify: `web/src/mocks/hierarchy.ts`

- [ ] **Step 1: Remove entity-related exports**

From `web/src/mocks/hierarchy.ts`, remove:
- `MOCK_ORGS` constant and its data
- `MOCK_PROJECTS` constant and its data
- `getMockOrgs()` function
- `getMockProjects()` function
- `getMockApps()` function (but keep `MOCK_APPLICATIONS` — see below)
- `getOrgName()` function
- `getProjectName()` function
- `getAppName()` function

Keep everything else: `MOCK_APPLICATIONS` (still used by `FlagDetailPage` and `MembersPage` which aren't being wired this pass), `MOCK_FLAG_ENV_STATE`, `MOCK_DEPLOYMENT_DETAIL`, `MOCK_DEPLOYMENT_EVENTS`, `MOCK_RELEASE_DETAIL`, `MOCK_RELEASE_FLAG_CHANGES`, `MOCK_ENVIRONMENTS`, `MOCK_MEMBERS`, `MOCK_GROUPS`, `MOCK_API_KEYS`, `getMockEnvironments()`, `getEnvironmentName()`.

Note: `getMockApps()` is removed because it depended on `MOCK_PROJECTS` for lookup. `MOCK_APPLICATIONS` stays as raw data for pages that reference it directly.

Also remove the `Organization`, `Application`, `Project` type imports if they are no longer needed. Keep the remaining type imports.

- [ ] **Step 2: Fix compilation errors**

After removing, the TypeScript compiler will flag every file that still imports the removed exports. These are fixed in Tasks 6-8.

- [ ] **Step 3: Commit**

```bash
git add web/src/mocks/hierarchy.ts
git commit -m "refactor: remove entity mock data from hierarchy.ts"
```

---

### Task 6: Frontend — Wire Core Navigation Components

**Files:**
- Modify: `web/src/components/OrgSwitcher.tsx`
- Modify: `web/src/components/ProjectSwitcher.tsx`
- Modify: `web/src/components/AppAccordion.tsx`
- Modify: `web/src/components/Sidebar.tsx`
- Modify: `web/src/components/Breadcrumb.tsx`

- [ ] **Step 1: Update OrgSwitcher**

Replace mock imports with hooks:

```typescript
import { useOrgs } from '@/hooks/useEntities';

export default function OrgSwitcher() {
  const { orgSlug } = useParams();
  const navigate = useNavigate();
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  const { orgs } = useOrgs();

  // ... (keep existing outside-click handler)

  const currentOrg = orgs.find((o) => o.slug === orgSlug);

  // In the render, replace getOrgName(orgSlug) with:
  // currentOrg?.name ?? orgSlug ?? 'Select Org'

  // Replace orgs.map with the same orgs from the hook
```

- [ ] **Step 2: Update ProjectSwitcher**

Replace mock imports with hooks:

```typescript
import { useProjects } from '@/hooks/useEntities';

// Replace: const projects = useMemo(() => orgSlug ? getMockProjects(orgSlug) : [], [orgSlug]);
// With:    const { projects } = useProjects(orgSlug);

// Replace: getProjectName(projectSlug)
// With:    projects.find(p => p.slug === projectSlug)?.name ?? projectSlug ?? 'Select Project'
```

- [ ] **Step 3: Update AppAccordion**

Replace mock imports with hooks:

```typescript
import { useApps } from '@/hooks/useEntities';

// Replace: const apps = getMockApps(projectSlug);
// With:    const { apps } = useApps(orgSlug, projectSlug);
```

- [ ] **Step 4: Update Sidebar — add Projects link**

Add a "Projects" nav link in the Organization section so users can always navigate back:

```typescript
{orgSlug && (
  <>
    <div className="sidebar-section">Organization</div>
    <NavLink
      to={`/orgs/${orgSlug}/projects`}
      className={({ isActive }) => `nav-item${isActive ? ' active' : ''}`}
    >
      <span className="nav-icon">□</span>
      Projects
    </NavLink>
    {/* ... existing Members, API Keys, Settings links */}
  </>
)}
```

- [ ] **Step 5: Update Breadcrumb**

Replace mock name resolution with params-based resolution. Since the breadcrumb doesn't have access to the entity lists without adding hooks (which would add API calls), use a simpler approach — display the slug as the label and let the pages themselves show the full names. OR: use the hooks.

Simpler approach:

```typescript
// Remove: import { getOrgName, getProjectName, getAppName } from '@/mocks/hierarchy';

// Replace getOrgName(orgSlug) with orgSlug
// Replace getProjectName(projectSlug) with projectSlug
// Replace getAppName(appSlug) with appSlug
```

This is acceptable because slugs are human-readable by design. If you want full names, add the hooks — but that means 3 API calls per page load just for the breadcrumb.

- [ ] **Step 6: Verify the web app compiles**

Run: `cd /Users/sgamel/git/DeploySentry/web && npx tsc --noEmit`
Expected: No new type errors from these files

- [ ] **Step 7: Commit**

```bash
git add web/src/components/OrgSwitcher.tsx web/src/components/ProjectSwitcher.tsx \
  web/src/components/AppAccordion.tsx web/src/components/Sidebar.tsx \
  web/src/components/Breadcrumb.tsx
git commit -m "feat: wire navigation components to real API via hooks"
```

---

### Task 7: Frontend — Wire Pages & Redirects

**Files:**
- Modify: `web/src/pages/ProjectListPage.tsx`
- Modify: `web/src/pages/CreateOrgPage.tsx`
- Modify: `web/src/pages/CreateAppPage.tsx`
- Create: `web/src/pages/CreateProjectPage.tsx`
- Modify: `web/src/App.tsx`
- Modify: `web/src/components/DefaultRedirect.tsx`
- Modify: `web/src/components/LegacyRedirect.tsx`

- [ ] **Step 1: Update ProjectListPage**

```typescript
import { useParams, Link } from 'react-router-dom';
import { useProjects } from '@/hooks/useEntities';

export default function ProjectListPage() {
  const { orgSlug } = useParams();
  const { projects, loading, error } = useProjects(orgSlug);

  if (!orgSlug) return null;
  if (loading) return <div className="page-loading">Loading projects...</div>;
  if (error) return <div className="page-error">Error: {error}</div>;

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
            <Link
              key={project.id}
              to={`/orgs/${orgSlug}/projects/${project.slug}/flags`}
              className="project-card"
            >
              <h3 className="project-card-name">{project.name}</h3>
              <span className="project-card-slug">{project.slug}</span>
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 2: Update CreateOrgPage**

```typescript
import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { entitiesApi } from '@/api';

export default function CreateOrgPage() {
  const navigate = useNavigate();
  const [name, setName] = useState('');
  const [slug, setSlug] = useState('');
  const [error, setError] = useState('');
  const [submitting, setSubmitting] = useState(false);

  function handleNameChange(value: string) {
    setName(value);
    setSlug(value.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, ''));
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!name || !slug) return;
    setSubmitting(true);
    setError('');
    try {
      await entitiesApi.createOrg({ name, slug });
      localStorage.setItem('ds_last_org', slug);
      navigate(`/orgs/${slug}/projects`);
    } catch (err: any) {
      setError(err.message || 'Failed to create organization');
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="page-center">
      <div className="form-card">
        <h1 className="page-header">Create Organization</h1>
        {error && <div className="form-error">{error}</div>}
        <form onSubmit={handleSubmit}>
          <div className="form-group">
            <label className="form-label">Organization Name</label>
            <input type="text" className="form-input" value={name}
              onChange={(e) => handleNameChange(e.target.value)} placeholder="Acme Corp" required />
          </div>
          <div className="form-group">
            <label className="form-label">Slug</label>
            <input type="text" className="form-input" value={slug}
              onChange={(e) => setSlug(e.target.value)} placeholder="acme-corp" required />
          </div>
          <button type="submit" className="btn btn-primary" style={{ width: '100%' }}
            disabled={submitting}>
            {submitting ? 'Creating...' : 'Create Organization'}
          </button>
        </form>
      </div>
    </div>
  );
}
```

- [ ] **Step 3: Create CreateProjectPage**

Create `web/src/pages/CreateProjectPage.tsx` — same pattern as CreateOrgPage but calls `entitiesApi.createProject(orgSlug, { name, slug })`. Navigate to `/orgs/${orgSlug}/projects/${slug}/flags` on success.

- [ ] **Step 4: Update CreateAppPage**

Replace any mock imports. Call `entitiesApi.createApp(orgSlug, projectSlug, { name, slug, description })`. Add error state and submitting state. Navigate on success.

- [ ] **Step 5: Update DefaultRedirect**

```typescript
import { useEffect, useState } from 'react';
import { Navigate } from 'react-router-dom';
import { entitiesApi } from '@/api';

export default function DefaultRedirect() {
  const [target, setTarget] = useState<string | null>(null);

  useEffect(() => {
    const lastOrg = localStorage.getItem('ds_last_org');
    if (lastOrg) {
      setTarget(`/orgs/${lastOrg}/projects`);
      return;
    }
    entitiesApi.listOrgs().then((res) => {
      const orgs = res.organizations ?? [];
      if (orgs.length > 0) {
        setTarget(`/orgs/${orgs[0].slug}/projects`);
      } else {
        setTarget('/orgs/new');
      }
    }).catch(() => setTarget('/orgs/new'));
  }, []);

  if (!target) return <div className="page-loading">Loading...</div>;
  return <Navigate to={target} replace />;
}
```

- [ ] **Step 6: Update LegacyRedirect**

Remove mock imports. Use only localStorage for redirect resolution — if no localStorage values exist, redirect to `/orgs/new`:

```typescript
import { Navigate } from 'react-router-dom';

interface LegacyRedirectProps { to: string; }

export default function LegacyRedirect({ to }: LegacyRedirectProps) {
  const lastOrg = localStorage.getItem('ds_last_org') || '';
  const lastProject = localStorage.getItem('ds_last_project') || '';
  const lastApp = localStorage.getItem('ds_last_app') || '';

  if (!lastOrg) return <Navigate to="/orgs/new" replace />;
  if (to === 'settings') return <Navigate to={`/orgs/${lastOrg}/settings`} replace />;
  if (!lastProject) return <Navigate to={`/orgs/${lastOrg}/projects`} replace />;

  if ((to === 'deployments' || to === 'releases') && lastApp) {
    return <Navigate to={`/orgs/${lastOrg}/projects/${lastProject}/apps/${lastApp}/${to}`} replace />;
  }
  if (to === 'deployments' || to === 'releases') {
    return <Navigate to={`/orgs/${lastOrg}/projects/${lastProject}/flags`} replace />;
  }
  return <Navigate to={`/orgs/${lastOrg}/projects/${lastProject}/${to}`} replace />;
}
```

- [ ] **Step 7: Add route in App.tsx**

Import `CreateProjectPage` and add route **before** `projects/:projectSlug`:

```typescript
import CreateProjectPage from './pages/CreateProjectPage';

// Inside the /orgs/:orgSlug HierarchyLayout, add before "projects/:projectSlug":
<Route path="projects/new" element={<CreateProjectPage />} />
```

- [ ] **Step 8: Commit**

```bash
git add web/src/pages/ProjectListPage.tsx web/src/pages/CreateOrgPage.tsx \
  web/src/pages/CreateProjectPage.tsx web/src/pages/CreateAppPage.tsx \
  web/src/components/DefaultRedirect.tsx web/src/components/LegacyRedirect.tsx \
  web/src/App.tsx
git commit -m "feat: wire pages and redirects to real API, add CreateProjectPage"
```

---

### Task 8: Frontend — Fix Remaining Mock Imports

**Files:**
- Modify: `web/src/pages/FlagListPage.tsx`
- Modify: `web/src/pages/DeploymentsPage.tsx`
- Modify: `web/src/pages/ReleasesPage.tsx`
- Modify: `web/src/pages/SDKsPage.tsx`
- Modify: `web/src/pages/AnalyticsPage.tsx`
- Modify: `web/src/pages/MembersPage.tsx`

These pages import entity name resolution functions (`getProjectName`, `getAppName`) that no longer exist. Replace with slug display from `useParams()`.

- [ ] **Step 1: Fix FlagListPage**

Remove `import { getProjectName, getAppName } from '@/mocks/hierarchy'`. Use `useParams()` to get `projectSlug` and `appSlug` directly. Where the page displays `getProjectName(projectSlug)`, replace with `projectSlug`. Same for `getAppName`.

- [ ] **Step 2: Fix DeploymentsPage**

Remove `import { getAppName } from '@/mocks/hierarchy'`. Replace `getAppName(appSlug)` with `appSlug`.

- [ ] **Step 3: Fix ReleasesPage**

Same pattern — remove import, use slug from params.

- [ ] **Step 4: Fix SDKsPage**

Remove `import { getProjectName } from '@/mocks/hierarchy'`. Replace with slug from params.

- [ ] **Step 5: Fix AnalyticsPage**

Remove `import { getProjectName } from '@/mocks/hierarchy'`. Replace with slug from params.

- [ ] **Step 6: Fix MembersPage**

Remove `getAppName` import (this helper function was deleted). The remaining imports (`MOCK_MEMBERS`, `MOCK_GROUPS`, `MOCK_ENVIRONMENTS`, `MOCK_APPLICATIONS`, `getEnvironmentName`) all stay — these constants remain in `hierarchy.ts`. Replace uses of `getAppName(...)` with the app slug from params or inline lookup from `MOCK_APPLICATIONS`.

- [ ] **Step 7: Verify full compilation**

Run: `cd /Users/sgamel/git/DeploySentry/web && npx tsc --noEmit`
Expected: No type errors

Run: `cd /Users/sgamel/git/DeploySentry/web && npm run build`
Expected: Build succeeds

- [ ] **Step 8: Commit**

```bash
git add web/src/pages/FlagListPage.tsx web/src/pages/DeploymentsPage.tsx \
  web/src/pages/ReleasesPage.tsx web/src/pages/SDKsPage.tsx \
  web/src/pages/AnalyticsPage.tsx web/src/pages/MembersPage.tsx
git commit -m "fix: replace removed mock name helpers with slug-based display"
```

---

### Task 9: Documentation

**Files:**
- Modify: `docs/Current_Initiatives.md`

- [ ] **Step 1: Update current initiatives**

Add a row:
```
| Entity Management API & UI Wiring | Implementation | [Link](./superpowers/specs/2026-03-29-entity-management-api-wiring-design.md) |
```

- [ ] **Step 2: Commit**

```bash
git add docs/Current_Initiatives.md
git commit -m "docs: add entity management initiative to current initiatives"
```
