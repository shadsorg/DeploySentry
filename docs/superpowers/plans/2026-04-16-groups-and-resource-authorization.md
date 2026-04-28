# Groups & Resource Authorization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add org-level groups, resource authorization grants (read/write) on projects and apps, and visibility filtering so users without access cannot see restricted resources.

**Architecture:** Three new DB tables (`groups`, `group_members`, `resource_grants`) replace `project_members`. Two new backend packages (`internal/groups/`, `internal/grants/`) follow the existing repository/service/handler pattern. A new `RequireResourceAccess` middleware gates project/app routes. Frontend adds group management to MembersPage and Authorization tabs to project/app settings.

**Tech Stack:** Go (Gin, pgxpool), PostgreSQL, React + TypeScript

**Spec:** `docs/superpowers/specs/2026-04-16-groups-and-resource-authorization-design.md`

---

## File Map

### New Files

| File | Responsibility |
|------|---------------|
| `migrations/041_create_groups_and_grants.up.sql` | Create `groups`, `group_members`, `resource_grants` tables; migrate `project_members` data; drop `project_members` |
| `migrations/041_create_groups_and_grants.down.sql` | Reverse migration |
| `internal/models/group.go` | `Group`, `GroupMember`, `ResourceGrant` model structs |
| `internal/groups/repository.go` | `GroupRepository` interface |
| `internal/groups/service.go` | `GroupService` interface + `groupService` implementation |
| `internal/groups/handler.go` | HTTP handlers + `RegisterRoutes` for group CRUD and group member management |
| `internal/platform/database/postgres/groups.go` | `GroupRepository` postgres implementation |
| `internal/grants/repository.go` | `GrantRepository` interface |
| `internal/grants/service.go` | `GrantService` interface + `grantService` implementation |
| `internal/grants/handler.go` | HTTP handlers + `RegisterRoutes` for grant CRUD |
| `internal/grants/middleware.go` | `RequireResourceAccess` middleware |
| `internal/platform/database/postgres/grants.go` | `GrantRepository` postgres implementation (includes visibility queries) |
| `web/src/hooks/useGroups.ts` | React hooks for group API calls |
| `web/src/hooks/useGrants.ts` | React hooks for grant API calls |

### Modified Files

| File | Change |
|------|--------|
| `internal/auth/rbac.go` | Add `PermGroupManage` permission constant; add to owner/admin roles |
| `internal/auth/middleware.go` | Remove `RequireProjectPermission` and `RequireEnvironmentPermission` |
| `internal/entities/repository.go` | Add `userID` and `orgRole` params to `ListProjectsByOrg` and `ListAppsByProject` |
| `internal/entities/service.go` | Pass through new params |
| `internal/entities/handler.go` | Extract `user_id` and `role` from context for list calls |
| `internal/platform/database/postgres/entities.go` | Add visibility-filtering SQL to list queries |
| `internal/members/handler.go` | Remove project member routes from `RegisterRoutes` |
| `internal/members/repository.go` | Remove project member interface methods |
| `internal/members/service.go` | Remove project member service methods |
| `internal/platform/database/postgres/members.go` | Remove project member repository methods |
| `cmd/api/main.go` | Wire `GroupHandler`, `GrantHandler`; pass grant resolver to middleware |
| `web/src/pages/MembersPage.tsx` | Replace Groups tab placeholder with full group management UI |
| `web/src/pages/SettingsPage.tsx` | Add `authorization` tab for project and app levels |
| `web/src/api.ts` | Add group and grant API functions |

---

## Task 1: Database Migration

**Files:**
- Create: `migrations/041_create_groups_and_grants.up.sql`
- Create: `migrations/041_create_groups_and_grants.down.sql`

- [ ] **Step 1: Write the up migration**

```sql
-- 041_create_groups_and_grants.up.sql

-- 1. Groups
CREATE TABLE groups (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id      UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name        TEXT        NOT NULL,
    slug        TEXT        NOT NULL,
    description TEXT        NOT NULL DEFAULT '',
    created_by  UUID        REFERENCES users(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (org_id, slug)
);

CREATE INDEX idx_groups_org_id ON groups(org_id);

-- 2. Group members
CREATE TABLE group_members (
    group_id   UUID        NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    user_id    UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (group_id, user_id)
);

CREATE INDEX idx_group_members_user_id ON group_members(user_id);

-- 3. Resource grants
CREATE TABLE resource_grants (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    project_id      UUID        REFERENCES projects(id) ON DELETE CASCADE,
    application_id  UUID        REFERENCES applications(id) ON DELETE CASCADE,
    user_id         UUID        REFERENCES users(id) ON DELETE CASCADE,
    group_id        UUID        REFERENCES groups(id) ON DELETE CASCADE,
    permission      TEXT        NOT NULL CHECK (permission IN ('read', 'write')),
    granted_by      UUID        REFERENCES users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- Exactly one resource target
    CONSTRAINT chk_one_resource CHECK (
        (project_id IS NOT NULL AND application_id IS NULL)
        OR (project_id IS NULL AND application_id IS NOT NULL)
    ),
    -- Exactly one grantee
    CONSTRAINT chk_one_grantee CHECK (
        (user_id IS NOT NULL AND group_id IS NULL)
        OR (user_id IS NULL AND group_id IS NOT NULL)
    )
);

-- Unique constraints (partial indexes for nullable columns)
CREATE UNIQUE INDEX uq_grant_project_user ON resource_grants(project_id, user_id)
    WHERE project_id IS NOT NULL AND user_id IS NOT NULL;
CREATE UNIQUE INDEX uq_grant_project_group ON resource_grants(project_id, group_id)
    WHERE project_id IS NOT NULL AND group_id IS NOT NULL;
CREATE UNIQUE INDEX uq_grant_app_user ON resource_grants(application_id, user_id)
    WHERE application_id IS NOT NULL AND user_id IS NOT NULL;
CREATE UNIQUE INDEX uq_grant_app_group ON resource_grants(application_id, group_id)
    WHERE application_id IS NOT NULL AND group_id IS NOT NULL;

CREATE INDEX idx_resource_grants_project ON resource_grants(project_id) WHERE project_id IS NOT NULL;
CREATE INDEX idx_resource_grants_app ON resource_grants(application_id) WHERE application_id IS NOT NULL;
CREATE INDEX idx_resource_grants_user ON resource_grants(user_id) WHERE user_id IS NOT NULL;
CREATE INDEX idx_resource_grants_group ON resource_grants(group_id) WHERE group_id IS NOT NULL;

-- 4. Migrate project_members → resource_grants
INSERT INTO resource_grants (org_id, project_id, user_id, permission, created_at)
SELECT p.org_id, pm.project_id, pm.user_id,
    CASE WHEN pm.role IN ('admin', 'developer') THEN 'write' ELSE 'read' END,
    pm.created_at
FROM project_members pm
JOIN projects p ON p.id = pm.project_id;

-- 5. Drop project_members
DROP TABLE project_members;
```

- [ ] **Step 2: Write the down migration**

```sql
-- 041_create_groups_and_grants.down.sql

-- 1. Recreate project_members
CREATE TABLE project_members (
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role       TEXT NOT NULL CHECK (role IN ('admin', 'developer', 'viewer')),
    id         UUID DEFAULT gen_random_uuid(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (project_id, user_id)
);

-- 2. Migrate back (project-level user grants only)
INSERT INTO project_members (project_id, user_id, role, created_at, updated_at)
SELECT rg.project_id, rg.user_id,
    CASE WHEN rg.permission = 'write' THEN 'developer' ELSE 'viewer' END,
    rg.created_at, rg.created_at
FROM resource_grants rg
WHERE rg.project_id IS NOT NULL AND rg.user_id IS NOT NULL;

-- 3. Drop new tables
DROP TABLE resource_grants;
DROP TABLE group_members;
DROP TABLE groups;
```

- [ ] **Step 3: Run the migration**

Run: `make migrate-up`
Expected: Migration 041 applied successfully

- [ ] **Step 4: Verify tables exist**

Run: `psql "$DATABASE_URL" -c "SELECT table_name FROM information_schema.tables WHERE table_schema='deploy' AND table_name IN ('groups','group_members','resource_grants','project_members') ORDER BY table_name;"`
Expected: `groups`, `group_members`, `resource_grants` present; `project_members` absent

- [ ] **Step 5: Commit**

```bash
git add migrations/041_create_groups_and_grants.up.sql migrations/041_create_groups_and_grants.down.sql
git commit -m "feat: add groups, group_members, resource_grants tables; migrate project_members"
```

---

## Task 2: Models

**Files:**
- Create: `internal/models/group.go`

- [ ] **Step 1: Create the Group, GroupMember, and ResourceGrant models**

```go
package models

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// Group represents an org-level group of users.
type Group struct {
	ID          uuid.UUID  `json:"id"`
	OrgID       uuid.UUID  `json:"org_id"`
	Name        string     `json:"name"`
	Slug        string     `json:"slug"`
	Description string     `json:"description"`
	CreatedBy   *uuid.UUID `json:"created_by,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

func (g *Group) Validate() error {
	if g.OrgID == uuid.Nil {
		return errors.New("org_id is required")
	}
	if g.Name == "" {
		return errors.New("name is required")
	}
	if g.Slug == "" {
		return errors.New("slug is required")
	}
	return nil
}

// GroupMember represents a user's membership in a group.
type GroupMember struct {
	GroupID   uuid.UUID `json:"group_id"`
	UserID    uuid.UUID `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
}

// ResourcePermission represents the permission level on a resource.
type ResourcePermission string

const (
	PermissionRead  ResourcePermission = "read"
	PermissionWrite ResourcePermission = "write"
)

// ResourceGrant represents a read/write grant for a user or group on a project or application.
type ResourceGrant struct {
	ID            uuid.UUID          `json:"id"`
	OrgID         uuid.UUID          `json:"org_id"`
	ProjectID     *uuid.UUID         `json:"project_id,omitempty"`
	ApplicationID *uuid.UUID         `json:"application_id,omitempty"`
	UserID        *uuid.UUID         `json:"user_id,omitempty"`
	GroupID       *uuid.UUID         `json:"group_id,omitempty"`
	Permission    ResourcePermission `json:"permission"`
	GrantedBy     *uuid.UUID         `json:"granted_by,omitempty"`
	CreatedAt     time.Time          `json:"created_at"`
}

func (rg *ResourceGrant) Validate() error {
	if rg.OrgID == uuid.Nil {
		return errors.New("org_id is required")
	}
	if rg.ProjectID == nil && rg.ApplicationID == nil {
		return errors.New("exactly one of project_id or application_id is required")
	}
	if rg.ProjectID != nil && rg.ApplicationID != nil {
		return errors.New("exactly one of project_id or application_id is required")
	}
	if rg.UserID == nil && rg.GroupID == nil {
		return errors.New("exactly one of user_id or group_id is required")
	}
	if rg.UserID != nil && rg.GroupID != nil {
		return errors.New("exactly one of user_id or group_id is required")
	}
	if rg.Permission != PermissionRead && rg.Permission != PermissionWrite {
		return errors.New("permission must be 'read' or 'write'")
	}
	return nil
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/models/...`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add internal/models/group.go
git commit -m "feat: add Group, GroupMember, ResourceGrant models"
```

---

## Task 3: Groups Repository + Postgres Implementation

**Files:**
- Create: `internal/groups/repository.go`
- Create: `internal/platform/database/postgres/groups.go`

- [ ] **Step 1: Define the GroupRepository interface**

```go
package groups

import (
	"context"

	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/google/uuid"
)

// GroupMemberRow extends GroupMember with user details from a JOIN.
type GroupMemberRow struct {
	models.GroupMember
	Name      string `json:"name"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url,omitempty"`
}

// GroupRow extends Group with a member count.
type GroupRow struct {
	models.Group
	MemberCount int `json:"member_count"`
}

// Repository defines persistence for groups and group members.
type Repository interface {
	ListByOrg(ctx context.Context, orgID uuid.UUID) ([]GroupRow, error)
	GetBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*models.Group, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.Group, error)
	Create(ctx context.Context, g *models.Group) error
	Update(ctx context.Context, g *models.Group) error
	Delete(ctx context.Context, id uuid.UUID) error

	ListMembers(ctx context.Context, groupID uuid.UUID) ([]GroupMemberRow, error)
	AddMember(ctx context.Context, groupID, userID uuid.UUID) error
	RemoveMember(ctx context.Context, groupID, userID uuid.UUID) error
	IsMember(ctx context.Context, groupID, userID uuid.UUID) (bool, error)
}
```

- [ ] **Step 2: Implement the postgres GroupRepository**

```go
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/shadsorg/deploysentry/internal/groups"
	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type GroupRepository struct {
	pool *pgxpool.Pool
}

func NewGroupRepository(pool *pgxpool.Pool) *GroupRepository {
	return &GroupRepository{pool: pool}
}

func (r *GroupRepository) ListByOrg(ctx context.Context, orgID uuid.UUID) ([]groups.GroupRow, error) {
	const q = `
		SELECT g.id, g.org_id, g.name, g.slug, g.description, g.created_by,
			g.created_at, g.updated_at,
			(SELECT COUNT(*) FROM group_members gm WHERE gm.group_id = g.id)::int AS member_count
		FROM groups g
		WHERE g.org_id = $1
		ORDER BY g.name`

	rows, err := r.pool.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("postgres.GroupRepository.ListByOrg: %w", err)
	}
	defer rows.Close()

	var result []groups.GroupRow
	for rows.Next() {
		var row groups.GroupRow
		if err := rows.Scan(
			&row.ID, &row.OrgID, &row.Name, &row.Slug, &row.Description, &row.CreatedBy,
			&row.CreatedAt, &row.UpdatedAt,
			&row.MemberCount,
		); err != nil {
			return nil, fmt.Errorf("postgres.GroupRepository.ListByOrg scan: %w", err)
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func (r *GroupRepository) GetBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*models.Group, error) {
	const q = `
		SELECT id, org_id, name, slug, description, created_by, created_at, updated_at
		FROM groups WHERE org_id = $1 AND slug = $2`

	var g models.Group
	err := r.pool.QueryRow(ctx, q, orgID, slug).Scan(
		&g.ID, &g.OrgID, &g.Name, &g.Slug, &g.Description, &g.CreatedBy,
		&g.CreatedAt, &g.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres.GroupRepository.GetBySlug: %w", err)
	}
	return &g, nil
}

func (r *GroupRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Group, error) {
	const q = `
		SELECT id, org_id, name, slug, description, created_by, created_at, updated_at
		FROM groups WHERE id = $1`

	var g models.Group
	err := r.pool.QueryRow(ctx, q, id).Scan(
		&g.ID, &g.OrgID, &g.Name, &g.Slug, &g.Description, &g.CreatedBy,
		&g.CreatedAt, &g.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres.GroupRepository.GetByID: %w", err)
	}
	return &g, nil
}

func (r *GroupRepository) Create(ctx context.Context, g *models.Group) error {
	const q = `
		INSERT INTO groups (id, org_id, name, slug, description, created_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	_, err := r.pool.Exec(ctx, q,
		g.ID, g.OrgID, g.Name, g.Slug, g.Description, g.CreatedBy,
		g.CreatedAt, g.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres.GroupRepository.Create: %w", err)
	}
	return nil
}

func (r *GroupRepository) Update(ctx context.Context, g *models.Group) error {
	const q = `
		UPDATE groups SET name = $2, slug = $3, description = $4, updated_at = now()
		WHERE id = $1`

	_, err := r.pool.Exec(ctx, q, g.ID, g.Name, g.Slug, g.Description)
	if err != nil {
		return fmt.Errorf("postgres.GroupRepository.Update: %w", err)
	}
	return nil
}

func (r *GroupRepository) Delete(ctx context.Context, id uuid.UUID) error {
	const q = `DELETE FROM groups WHERE id = $1`
	_, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("postgres.GroupRepository.Delete: %w", err)
	}
	return nil
}

func (r *GroupRepository) ListMembers(ctx context.Context, groupID uuid.UUID) ([]groups.GroupMemberRow, error) {
	const q = `
		SELECT gm.group_id, gm.user_id, gm.created_at,
			u.name, u.email, COALESCE(u.avatar_url, '')
		FROM group_members gm
		JOIN users u ON gm.user_id = u.id
		WHERE gm.group_id = $1
		ORDER BY u.name`

	rows, err := r.pool.Query(ctx, q, groupID)
	if err != nil {
		return nil, fmt.Errorf("postgres.GroupRepository.ListMembers: %w", err)
	}
	defer rows.Close()

	var result []groups.GroupMemberRow
	for rows.Next() {
		var row groups.GroupMemberRow
		if err := rows.Scan(
			&row.GroupID, &row.UserID, &row.CreatedAt,
			&row.Name, &row.Email, &row.AvatarURL,
		); err != nil {
			return nil, fmt.Errorf("postgres.GroupRepository.ListMembers scan: %w", err)
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func (r *GroupRepository) AddMember(ctx context.Context, groupID, userID uuid.UUID) error {
	const q = `INSERT INTO group_members (group_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`
	_, err := r.pool.Exec(ctx, q, groupID, userID)
	if err != nil {
		return fmt.Errorf("postgres.GroupRepository.AddMember: %w", err)
	}
	return nil
}

func (r *GroupRepository) RemoveMember(ctx context.Context, groupID, userID uuid.UUID) error {
	const q = `DELETE FROM group_members WHERE group_id = $1 AND user_id = $2`
	_, err := r.pool.Exec(ctx, q, groupID, userID)
	if err != nil {
		return fmt.Errorf("postgres.GroupRepository.RemoveMember: %w", err)
	}
	return nil
}

func (r *GroupRepository) IsMember(ctx context.Context, groupID, userID uuid.UUID) (bool, error) {
	const q = `SELECT EXISTS(SELECT 1 FROM group_members WHERE group_id = $1 AND user_id = $2)`
	var exists bool
	err := r.pool.QueryRow(ctx, q, groupID, userID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("postgres.GroupRepository.IsMember: %w", err)
	}
	return exists, nil
}
```

- [ ] **Step 3: Verify it compiles**

Run: `go build ./internal/groups/... ./internal/platform/database/postgres/...`
Expected: No errors

- [ ] **Step 4: Commit**

```bash
git add internal/groups/repository.go internal/platform/database/postgres/groups.go
git commit -m "feat: add GroupRepository interface and postgres implementation"
```

---

## Task 4: Groups Service

**Files:**
- Create: `internal/groups/service.go`

- [ ] **Step 1: Implement the GroupService**

```go
package groups

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/google/uuid"
)

var (
	ErrGroupNotFound  = errors.New("group not found")
	ErrSlugTaken      = errors.New("group slug already exists in this organization")
	ErrMemberNotFound = errors.New("user is not a member of this group")
)

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

// Service defines business logic for groups.
type Service interface {
	ListByOrg(ctx context.Context, orgID uuid.UUID) ([]GroupRow, error)
	GetBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*models.Group, error)
	Create(ctx context.Context, orgID uuid.UUID, name, description string, createdBy uuid.UUID) (*models.Group, error)
	Update(ctx context.Context, orgID uuid.UUID, slug string, name, description string) (*models.Group, error)
	Delete(ctx context.Context, orgID uuid.UUID, slug string) error

	ListMembers(ctx context.Context, orgID uuid.UUID, groupSlug string) ([]GroupMemberRow, error)
	AddMember(ctx context.Context, orgID uuid.UUID, groupSlug string, userID uuid.UUID) error
	RemoveMember(ctx context.Context, orgID uuid.UUID, groupSlug string, userID uuid.UUID) error
}

type groupService struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &groupService{repo: repo}
}

func generateSlug(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = slugRe.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

func (s *groupService) ListByOrg(ctx context.Context, orgID uuid.UUID) ([]GroupRow, error) {
	return s.repo.ListByOrg(ctx, orgID)
}

func (s *groupService) GetBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*models.Group, error) {
	g, err := s.repo.GetBySlug(ctx, orgID, slug)
	if err != nil {
		return nil, err
	}
	if g == nil {
		return nil, ErrGroupNotFound
	}
	return g, nil
}

func (s *groupService) Create(ctx context.Context, orgID uuid.UUID, name, description string, createdBy uuid.UUID) (*models.Group, error) {
	slug := generateSlug(name)

	existing, err := s.repo.GetBySlug(ctx, orgID, slug)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ErrSlugTaken
	}

	now := time.Now()
	g := &models.Group{
		ID:          uuid.New(),
		OrgID:       orgID,
		Name:        name,
		Slug:        slug,
		Description: description,
		CreatedBy:   &createdBy,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := g.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, g); err != nil {
		return nil, err
	}
	return g, nil
}

func (s *groupService) Update(ctx context.Context, orgID uuid.UUID, slug string, name, description string) (*models.Group, error) {
	g, err := s.repo.GetBySlug(ctx, orgID, slug)
	if err != nil {
		return nil, err
	}
	if g == nil {
		return nil, ErrGroupNotFound
	}

	newSlug := generateSlug(name)
	if newSlug != g.Slug {
		existing, err := s.repo.GetBySlug(ctx, orgID, newSlug)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			return nil, ErrSlugTaken
		}
	}

	g.Name = name
	g.Slug = newSlug
	g.Description = description

	if err := s.repo.Update(ctx, g); err != nil {
		return nil, err
	}
	return g, nil
}

func (s *groupService) Delete(ctx context.Context, orgID uuid.UUID, slug string) error {
	g, err := s.repo.GetBySlug(ctx, orgID, slug)
	if err != nil {
		return err
	}
	if g == nil {
		return ErrGroupNotFound
	}
	return s.repo.Delete(ctx, g.ID)
}

func (s *groupService) resolveGroup(ctx context.Context, orgID uuid.UUID, groupSlug string) (*models.Group, error) {
	g, err := s.repo.GetBySlug(ctx, orgID, groupSlug)
	if err != nil {
		return nil, err
	}
	if g == nil {
		return nil, ErrGroupNotFound
	}
	return g, nil
}

func (s *groupService) ListMembers(ctx context.Context, orgID uuid.UUID, groupSlug string) ([]GroupMemberRow, error) {
	g, err := s.resolveGroup(ctx, orgID, groupSlug)
	if err != nil {
		return nil, err
	}
	return s.repo.ListMembers(ctx, g.ID)
}

func (s *groupService) AddMember(ctx context.Context, orgID uuid.UUID, groupSlug string, userID uuid.UUID) error {
	g, err := s.resolveGroup(ctx, orgID, groupSlug)
	if err != nil {
		return err
	}
	return s.repo.AddMember(ctx, g.ID, userID)
}

func (s *groupService) RemoveMember(ctx context.Context, orgID uuid.UUID, groupSlug string, userID uuid.UUID) error {
	g, err := s.resolveGroup(ctx, orgID, groupSlug)
	if err != nil {
		return err
	}
	return s.repo.RemoveMember(ctx, g.ID, userID)
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/groups/...`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add internal/groups/service.go
git commit -m "feat: add GroupService with slug generation and CRUD logic"
```

---

## Task 5: Groups Handler

**Files:**
- Create: `internal/groups/handler.go`

- [ ] **Step 1: Implement the Groups HTTP handler**

```go
package groups

import (
	"errors"
	"net/http"

	"github.com/shadsorg/deploysentry/internal/auth"
	"github.com/shadsorg/deploysentry/internal/entities"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	service  Service
	entities entities.EntityService
	rbac     *auth.RBACChecker
}

func NewHandler(service Service, entityService entities.EntityService, rbac *auth.RBACChecker) *Handler {
	return &Handler{service: service, entities: entityService, rbac: rbac}
}

func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/orgs/:orgSlug/groups")
	{
		g.GET("", h.listGroups)
		g.POST("", auth.RequirePermission(h.rbac, auth.PermGroupManage), h.createGroup)
		g.GET("/:groupSlug", h.getGroup)
		g.PUT("/:groupSlug", auth.RequirePermission(h.rbac, auth.PermGroupManage), h.updateGroup)
		g.DELETE("/:groupSlug", auth.RequirePermission(h.rbac, auth.PermGroupManage), h.deleteGroup)
		g.GET("/:groupSlug/members", h.listGroupMembers)
		g.POST("/:groupSlug/members", auth.RequirePermission(h.rbac, auth.PermGroupManage), h.addGroupMember)
		g.DELETE("/:groupSlug/members/:userId", auth.RequirePermission(h.rbac, auth.PermGroupManage), h.removeGroupMember)
	}
}

func (h *Handler) resolveOrgID(c *gin.Context) (uuid.UUID, bool) {
	org, err := h.entities.GetOrgBySlug(c.Request.Context(), c.Param("orgSlug"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "organization not found"})
		return uuid.Nil, false
	}
	return org.ID, true
}

type createGroupRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

type updateGroupRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

type addGroupMemberRequest struct {
	UserID string `json:"user_id" binding:"required"`
}

func (h *Handler) listGroups(c *gin.Context) {
	orgID, ok := h.resolveOrgID(c)
	if !ok {
		return
	}
	rows, err := h.service.ListByOrg(c.Request.Context(), orgID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"groups": rows})
}

func (h *Handler) createGroup(c *gin.Context) {
	orgID, ok := h.resolveOrgID(c)
	if !ok {
		return
	}
	var req createGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	uid, _ := c.Get("user_id")
	createdBy, _ := uid.(uuid.UUID)

	g, err := h.service.Create(c.Request.Context(), orgID, req.Name, req.Description, createdBy)
	if err != nil {
		switch {
		case errors.Is(err, ErrSlugTaken):
			c.JSON(http.StatusConflict, gin.H{"error": "a group with this name already exists"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}
	c.JSON(http.StatusCreated, g)
}

func (h *Handler) getGroup(c *gin.Context) {
	orgID, ok := h.resolveOrgID(c)
	if !ok {
		return
	}
	g, err := h.service.GetBySlug(c.Request.Context(), orgID, c.Param("groupSlug"))
	if err != nil {
		if errors.Is(err, ErrGroupNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "group not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, g)
}

func (h *Handler) updateGroup(c *gin.Context) {
	orgID, ok := h.resolveOrgID(c)
	if !ok {
		return
	}
	var req updateGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	g, err := h.service.Update(c.Request.Context(), orgID, c.Param("groupSlug"), req.Name, req.Description)
	if err != nil {
		switch {
		case errors.Is(err, ErrGroupNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "group not found"})
		case errors.Is(err, ErrSlugTaken):
			c.JSON(http.StatusConflict, gin.H{"error": "a group with this name already exists"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, g)
}

func (h *Handler) deleteGroup(c *gin.Context) {
	orgID, ok := h.resolveOrgID(c)
	if !ok {
		return
	}
	err := h.service.Delete(c.Request.Context(), orgID, c.Param("groupSlug"))
	if err != nil {
		if errors.Is(err, ErrGroupNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "group not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) listGroupMembers(c *gin.Context) {
	orgID, ok := h.resolveOrgID(c)
	if !ok {
		return
	}
	rows, err := h.service.ListMembers(c.Request.Context(), orgID, c.Param("groupSlug"))
	if err != nil {
		if errors.Is(err, ErrGroupNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "group not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"members": rows})
}

func (h *Handler) addGroupMember(c *gin.Context) {
	orgID, ok := h.resolveOrgID(c)
	if !ok {
		return
	}
	var req addGroupMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user_id"})
		return
	}
	if err := h.service.AddMember(c.Request.Context(), orgID, c.Param("groupSlug"), userID); err != nil {
		if errors.Is(err, ErrGroupNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "group not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) removeGroupMember(c *gin.Context) {
	orgID, ok := h.resolveOrgID(c)
	if !ok {
		return
	}
	userID, err := uuid.Parse(c.Param("userId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user_id"})
		return
	}
	if err := h.service.RemoveMember(c.Request.Context(), orgID, c.Param("groupSlug"), userID); err != nil {
		if errors.Is(err, ErrGroupNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "group not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/groups/...`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add internal/groups/handler.go
git commit -m "feat: add Groups HTTP handler with CRUD and member management routes"
```

---

## Task 6: Grants Repository + Postgres Implementation

**Files:**
- Create: `internal/grants/repository.go`
- Create: `internal/platform/database/postgres/grants.go`

- [ ] **Step 1: Define the GrantRepository interface**

```go
package grants

import (
	"context"

	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/google/uuid"
)

// GrantRow extends ResourceGrant with the grantee name for display.
type GrantRow struct {
	models.ResourceGrant
	GranteeName string `json:"grantee_name"`
	GranteeType string `json:"grantee_type"` // "user" or "group"
}

// Repository defines persistence for resource grants.
type Repository interface {
	ListByProject(ctx context.Context, projectID uuid.UUID) ([]GrantRow, error)
	ListByApp(ctx context.Context, applicationID uuid.UUID) ([]GrantRow, error)
	Create(ctx context.Context, g *models.ResourceGrant) error
	Delete(ctx context.Context, id uuid.UUID) error

	// Access resolution
	HasAnyGrants(ctx context.Context, projectID *uuid.UUID, applicationID *uuid.UUID) (bool, error)
	GetUserPermission(ctx context.Context, userID uuid.UUID, projectID *uuid.UUID, applicationID *uuid.UUID) (*models.ResourcePermission, error)
	GetUserGroupPermission(ctx context.Context, userID uuid.UUID, projectID *uuid.UUID, applicationID *uuid.UUID) (*models.ResourcePermission, error)
}
```

- [ ] **Step 2: Implement the postgres GrantRepository**

```go
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/shadsorg/deploysentry/internal/grants"
	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type GrantRepository struct {
	pool *pgxpool.Pool
}

func NewGrantRepository(pool *pgxpool.Pool) *GrantRepository {
	return &GrantRepository{pool: pool}
}

func (r *GrantRepository) ListByProject(ctx context.Context, projectID uuid.UUID) ([]grants.GrantRow, error) {
	const q = `
		SELECT rg.id, rg.org_id, rg.project_id, rg.application_id,
			rg.user_id, rg.group_id, rg.permission, rg.granted_by, rg.created_at,
			CASE
				WHEN rg.user_id IS NOT NULL THEN u.name
				WHEN rg.group_id IS NOT NULL THEN g.name
			END AS grantee_name,
			CASE
				WHEN rg.user_id IS NOT NULL THEN 'user'
				WHEN rg.group_id IS NOT NULL THEN 'group'
			END AS grantee_type
		FROM resource_grants rg
		LEFT JOIN users u ON rg.user_id = u.id
		LEFT JOIN groups g ON rg.group_id = g.id
		WHERE rg.project_id = $1
		ORDER BY grantee_type, grantee_name`

	rows, err := r.pool.Query(ctx, q, projectID)
	if err != nil {
		return nil, fmt.Errorf("postgres.GrantRepository.ListByProject: %w", err)
	}
	defer rows.Close()

	var result []grants.GrantRow
	for rows.Next() {
		var row grants.GrantRow
		if err := rows.Scan(
			&row.ID, &row.OrgID, &row.ProjectID, &row.ApplicationID,
			&row.UserID, &row.GroupID, &row.Permission, &row.GrantedBy, &row.CreatedAt,
			&row.GranteeName, &row.GranteeType,
		); err != nil {
			return nil, fmt.Errorf("postgres.GrantRepository.ListByProject scan: %w", err)
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func (r *GrantRepository) ListByApp(ctx context.Context, applicationID uuid.UUID) ([]grants.GrantRow, error) {
	const q = `
		SELECT rg.id, rg.org_id, rg.project_id, rg.application_id,
			rg.user_id, rg.group_id, rg.permission, rg.granted_by, rg.created_at,
			CASE
				WHEN rg.user_id IS NOT NULL THEN u.name
				WHEN rg.group_id IS NOT NULL THEN g.name
			END AS grantee_name,
			CASE
				WHEN rg.user_id IS NOT NULL THEN 'user'
				WHEN rg.group_id IS NOT NULL THEN 'group'
			END AS grantee_type
		FROM resource_grants rg
		LEFT JOIN users u ON rg.user_id = u.id
		LEFT JOIN groups g ON rg.group_id = g.id
		WHERE rg.application_id = $1
		ORDER BY grantee_type, grantee_name`

	rows, err := r.pool.Query(ctx, q, applicationID)
	if err != nil {
		return nil, fmt.Errorf("postgres.GrantRepository.ListByApp: %w", err)
	}
	defer rows.Close()

	var result []grants.GrantRow
	for rows.Next() {
		var row grants.GrantRow
		if err := rows.Scan(
			&row.ID, &row.OrgID, &row.ProjectID, &row.ApplicationID,
			&row.UserID, &row.GroupID, &row.Permission, &row.GrantedBy, &row.CreatedAt,
			&row.GranteeName, &row.GranteeType,
		); err != nil {
			return nil, fmt.Errorf("postgres.GrantRepository.ListByApp scan: %w", err)
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func (r *GrantRepository) Create(ctx context.Context, g *models.ResourceGrant) error {
	const q = `
		INSERT INTO resource_grants (id, org_id, project_id, application_id, user_id, group_id, permission, granted_by, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	_, err := r.pool.Exec(ctx, q,
		g.ID, g.OrgID, g.ProjectID, g.ApplicationID,
		g.UserID, g.GroupID, g.Permission, g.GrantedBy, g.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres.GrantRepository.Create: %w", err)
	}
	return nil
}

func (r *GrantRepository) Delete(ctx context.Context, id uuid.UUID) error {
	const q = `DELETE FROM resource_grants WHERE id = $1`
	_, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("postgres.GrantRepository.Delete: %w", err)
	}
	return nil
}

func (r *GrantRepository) HasAnyGrants(ctx context.Context, projectID *uuid.UUID, applicationID *uuid.UUID) (bool, error) {
	var q string
	var arg uuid.UUID
	if projectID != nil {
		q = `SELECT EXISTS(SELECT 1 FROM resource_grants WHERE project_id = $1)`
		arg = *projectID
	} else {
		q = `SELECT EXISTS(SELECT 1 FROM resource_grants WHERE application_id = $1)`
		arg = *applicationID
	}

	var exists bool
	err := r.pool.QueryRow(ctx, q, arg).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("postgres.GrantRepository.HasAnyGrants: %w", err)
	}
	return exists, nil
}

func (r *GrantRepository) GetUserPermission(ctx context.Context, userID uuid.UUID, projectID *uuid.UUID, applicationID *uuid.UUID) (*models.ResourcePermission, error) {
	var q string
	var arg uuid.UUID
	if projectID != nil {
		q = `SELECT permission FROM resource_grants WHERE project_id = $1 AND user_id = $2`
		arg = *projectID
	} else {
		q = `SELECT permission FROM resource_grants WHERE application_id = $1 AND user_id = $2`
		arg = *applicationID
	}

	var perm models.ResourcePermission
	err := r.pool.QueryRow(ctx, q, arg, userID).Scan(&perm)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres.GrantRepository.GetUserPermission: %w", err)
	}
	return &perm, nil
}

func (r *GrantRepository) GetUserGroupPermission(ctx context.Context, userID uuid.UUID, projectID *uuid.UUID, applicationID *uuid.UUID) (*models.ResourcePermission, error) {
	var q string
	var arg uuid.UUID
	if projectID != nil {
		q = `SELECT rg.permission FROM resource_grants rg
			JOIN group_members gm ON gm.group_id = rg.group_id
			WHERE rg.project_id = $1 AND gm.user_id = $2
			ORDER BY CASE WHEN rg.permission = 'write' THEN 0 ELSE 1 END
			LIMIT 1`
		arg = *projectID
	} else {
		q = `SELECT rg.permission FROM resource_grants rg
			JOIN group_members gm ON gm.group_id = rg.group_id
			WHERE rg.application_id = $1 AND gm.user_id = $2
			ORDER BY CASE WHEN rg.permission = 'write' THEN 0 ELSE 1 END
			LIMIT 1`
		arg = *applicationID
	}

	var perm models.ResourcePermission
	err := r.pool.QueryRow(ctx, q, arg, userID).Scan(&perm)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres.GrantRepository.GetUserGroupPermission: %w", err)
	}
	return &perm, nil
}
```

- [ ] **Step 3: Verify it compiles**

Run: `go build ./internal/grants/... ./internal/platform/database/postgres/...`
Expected: No errors

- [ ] **Step 4: Commit**

```bash
git add internal/grants/repository.go internal/platform/database/postgres/grants.go
git commit -m "feat: add GrantRepository interface and postgres implementation"
```

---

## Task 7: Grants Service

**Files:**
- Create: `internal/grants/service.go`

- [ ] **Step 1: Implement the GrantService with access resolution logic**

```go
package grants

import (
	"context"
	"errors"
	"time"

	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/google/uuid"
)

var (
	ErrGrantNotFound = errors.New("grant not found")
	ErrDuplicate     = errors.New("grant already exists for this grantee on this resource")
)

// Service defines business logic for resource grants and access resolution.
type Service interface {
	ListByProject(ctx context.Context, projectID uuid.UUID) ([]GrantRow, error)
	ListByApp(ctx context.Context, applicationID uuid.UUID) ([]GrantRow, error)
	Create(ctx context.Context, grant *models.ResourceGrant) (*models.ResourceGrant, error)
	Delete(ctx context.Context, id uuid.UUID) error

	// ResolveAccess determines the effective permission for a user on a resource.
	// Returns nil if the user has no access (deny).
	// orgRole is the user's org-level role string (e.g., "owner", "admin").
	ResolveAccess(ctx context.Context, userID uuid.UUID, orgRole string, projectID *uuid.UUID, applicationID *uuid.UUID) (*models.ResourcePermission, error)
}

type grantService struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &grantService{repo: repo}
}

func (s *grantService) ListByProject(ctx context.Context, projectID uuid.UUID) ([]GrantRow, error) {
	return s.repo.ListByProject(ctx, projectID)
}

func (s *grantService) ListByApp(ctx context.Context, applicationID uuid.UUID) ([]GrantRow, error) {
	return s.repo.ListByApp(ctx, applicationID)
}

func (s *grantService) Create(ctx context.Context, grant *models.ResourceGrant) (*models.ResourceGrant, error) {
	if err := grant.Validate(); err != nil {
		return nil, err
	}
	grant.ID = uuid.New()
	grant.CreatedAt = time.Now()

	if err := s.repo.Create(ctx, grant); err != nil {
		// Unique constraint violation → duplicate
		if isDuplicateErr(err) {
			return nil, ErrDuplicate
		}
		return nil, err
	}
	return grant, nil
}

func (s *grantService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

func (s *grantService) ResolveAccess(ctx context.Context, userID uuid.UUID, orgRole string, projectID *uuid.UUID, applicationID *uuid.UUID) (*models.ResourcePermission, error) {
	// 1. Org owners always have full access
	if orgRole == "owner" {
		perm := models.PermissionWrite
		return &perm, nil
	}

	// 2. Determine the effective resource for apps (cascade logic)
	effectiveProjectID := projectID
	effectiveAppID := applicationID

	if applicationID != nil {
		appHasGrants, err := s.repo.HasAnyGrants(ctx, nil, applicationID)
		if err != nil {
			return nil, err
		}
		if appHasGrants {
			// App has its own grants — use app grants only
			effectiveProjectID = nil
		} else {
			// App has no grants — cascade to project
			effectiveAppID = nil
			// projectID must be provided by the caller when checking app access
			if effectiveProjectID == nil {
				// If no project ID available, treat as open
				perm := models.PermissionWrite
				return &perm, nil
			}
		}
	}

	// 3. Check if the effective resource has any grants at all
	hasGrants, err := s.repo.HasAnyGrants(ctx, effectiveProjectID, effectiveAppID)
	if err != nil {
		return nil, err
	}
	if !hasGrants {
		// No grants = open resource, allow full access
		perm := models.PermissionWrite
		return &perm, nil
	}

	// 4. Check direct user grant
	perm, err := s.repo.GetUserPermission(ctx, userID, effectiveProjectID, effectiveAppID)
	if err != nil {
		return nil, err
	}
	if perm != nil {
		return perm, nil
	}

	// 5. Check group-based grants (returns highest: write > read)
	perm, err = s.repo.GetUserGroupPermission(ctx, userID, effectiveProjectID, effectiveAppID)
	if err != nil {
		return nil, err
	}
	if perm != nil {
		return perm, nil
	}

	// 6. No grant found — deny
	return nil, nil
}

// isDuplicateErr checks for postgres unique constraint violations.
func isDuplicateErr(err error) bool {
	return err != nil && (errors.Is(err, context.DeadlineExceeded) == false) &&
		(len(err.Error()) > 0 && contains(err.Error(), "duplicate key") || contains(err.Error(), "23505"))
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && searchString(s, sub)
}

func searchString(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/grants/...`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add internal/grants/service.go
git commit -m "feat: add GrantService with access resolution and cascade logic"
```

---

## Task 8: RequireResourceAccess Middleware

**Files:**
- Create: `internal/grants/middleware.go`

- [ ] **Step 1: Implement the middleware**

```go
package grants

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RequireResourceAccess returns middleware that checks the user has the required
// permission on the project or app identified by context values.
// requiredPerm is "read" or "write".
// If denied, returns 404 (not 403) to avoid leaking resource existence.
func RequireResourceAccess(svc Service, requiredPerm string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDVal, exists := c.Get("user_id")
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}
		userID, ok := userIDVal.(uuid.UUID)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid user identity"})
			return
		}

		roleVal, _ := c.Get("role")
		orgRole := ""
		if r, ok := roleVal.(string); ok {
			orgRole = r
		} else if r, ok := roleVal.(interface{ String() string }); ok {
			orgRole = r.String()
		}

		// Determine resource IDs from context (set by earlier middleware or handlers)
		var projectID *uuid.UUID
		var applicationID *uuid.UUID

		if pidStr, exists := c.Get("project_id"); exists {
			if pid, ok := pidStr.(uuid.UUID); ok {
				projectID = &pid
			} else if pidStr, ok := pidStr.(string); ok {
				if pid, err := uuid.Parse(pidStr); err == nil {
					projectID = &pid
				}
			}
		}

		if aidStr, exists := c.Get("application_id"); exists {
			if aid, ok := aidStr.(uuid.UUID); ok {
				applicationID = &aid
			} else if aidStr, ok := aidStr.(string); ok {
				if aid, err := uuid.Parse(aidStr); err == nil {
					applicationID = &aid
				}
			}
		}

		// If no resource identified yet, let the handler deal with it
		if projectID == nil && applicationID == nil {
			c.Next()
			return
		}

		perm, err := svc.ResolveAccess(c.Request.Context(), userID, orgRole, projectID, applicationID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "access check failed"})
			return
		}

		if perm == nil {
			// Denied — return 404 to hide resource existence
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}

		// Check if the granted permission satisfies the requirement
		if requiredPerm == "write" && string(*perm) != "write" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "write access required"})
			return
		}

		c.Set("resource_permission", string(*perm))
		c.Next()
	}
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/grants/...`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add internal/grants/middleware.go
git commit -m "feat: add RequireResourceAccess middleware with 404-on-deny"
```

---

## Task 9: Grants Handler

**Files:**
- Create: `internal/grants/handler.go`

- [ ] **Step 1: Implement the Grants HTTP handler**

```go
package grants

import (
	"errors"
	"net/http"

	"github.com/shadsorg/deploysentry/internal/auth"
	"github.com/shadsorg/deploysentry/internal/entities"
	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	service  Service
	entities entities.EntityService
	rbac     *auth.RBACChecker
}

func NewHandler(service Service, entityService entities.EntityService, rbac *auth.RBACChecker) *Handler {
	return &Handler{service: service, entities: entityService, rbac: rbac}
}

func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	// Project grants
	pg := rg.Group("/orgs/:orgSlug/projects/:projectSlug/grants")
	pg.Use(auth.RequirePermission(h.rbac, auth.PermOrgManage))
	{
		pg.GET("", h.listProjectGrants)
		pg.POST("", h.createProjectGrant)
		pg.DELETE("/:grantId", h.deleteGrant)
	}

	// App grants
	ag := rg.Group("/orgs/:orgSlug/projects/:projectSlug/apps/:appSlug/grants")
	ag.Use(auth.RequirePermission(h.rbac, auth.PermOrgManage))
	{
		ag.GET("", h.listAppGrants)
		ag.POST("", h.createAppGrant)
		ag.DELETE("/:grantId", h.deleteGrant)
	}
}

type createGrantRequest struct {
	UserID     *string `json:"user_id"`
	GroupID    *string `json:"group_id"`
	Permission string  `json:"permission" binding:"required"`
}

func (h *Handler) resolveOrg(c *gin.Context) (uuid.UUID, bool) {
	org, err := h.entities.GetOrgBySlug(c.Request.Context(), c.Param("orgSlug"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "organization not found"})
		return uuid.Nil, false
	}
	return org.ID, true
}

func (h *Handler) resolveProject(c *gin.Context, orgID uuid.UUID) (uuid.UUID, bool) {
	project, err := h.entities.GetProjectBySlug(c.Request.Context(), orgID, c.Param("projectSlug"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
		return uuid.Nil, false
	}
	return project.ID, true
}

func (h *Handler) resolveApp(c *gin.Context, projectID uuid.UUID) (uuid.UUID, bool) {
	app, err := h.entities.GetAppBySlug(c.Request.Context(), projectID, c.Param("appSlug"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "application not found"})
		return uuid.Nil, false
	}
	return app.ID, true
}

func (h *Handler) listProjectGrants(c *gin.Context) {
	orgID, ok := h.resolveOrg(c)
	if !ok {
		return
	}
	projectID, ok := h.resolveProject(c, orgID)
	if !ok {
		return
	}
	rows, err := h.service.ListByProject(c.Request.Context(), projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"grants": rows})
}

func (h *Handler) listAppGrants(c *gin.Context) {
	orgID, ok := h.resolveOrg(c)
	if !ok {
		return
	}
	projectID, ok := h.resolveProject(c, orgID)
	if !ok {
		return
	}
	appID, ok := h.resolveApp(c, projectID)
	if !ok {
		return
	}
	rows, err := h.service.ListByApp(c.Request.Context(), appID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"grants": rows})
}

func (h *Handler) createProjectGrant(c *gin.Context) {
	orgID, ok := h.resolveOrg(c)
	if !ok {
		return
	}
	projectID, ok := h.resolveProject(c, orgID)
	if !ok {
		return
	}
	h.createGrant(c, orgID, &projectID, nil)
}

func (h *Handler) createAppGrant(c *gin.Context) {
	orgID, ok := h.resolveOrg(c)
	if !ok {
		return
	}
	projectID, ok := h.resolveProject(c, orgID)
	if !ok {
		return
	}
	appID, ok := h.resolveApp(c, projectID)
	if !ok {
		return
	}
	h.createGrant(c, orgID, nil, &appID)
}

func (h *Handler) createGrant(c *gin.Context, orgID uuid.UUID, projectID *uuid.UUID, appID *uuid.UUID) {
	var req createGrantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Permission != "read" && req.Permission != "write" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "permission must be 'read' or 'write'"})
		return
	}
	if (req.UserID == nil) == (req.GroupID == nil) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "exactly one of user_id or group_id is required"})
		return
	}

	grant := &models.ResourceGrant{
		OrgID:         orgID,
		ProjectID:     projectID,
		ApplicationID: appID,
		Permission:    models.ResourcePermission(req.Permission),
	}

	if req.UserID != nil {
		uid, err := uuid.Parse(*req.UserID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user_id"})
			return
		}
		grant.UserID = &uid
	}
	if req.GroupID != nil {
		gid, err := uuid.Parse(*req.GroupID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid group_id"})
			return
		}
		grant.GroupID = &gid
	}

	uid, _ := c.Get("user_id")
	if grantedBy, ok := uid.(uuid.UUID); ok {
		grant.GrantedBy = &grantedBy
	}

	result, err := h.service.Create(c.Request.Context(), grant)
	if err != nil {
		if errors.Is(err, ErrDuplicate) {
			c.JSON(http.StatusConflict, gin.H{"error": "grant already exists for this grantee"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, result)
}

func (h *Handler) deleteGrant(c *gin.Context) {
	grantID, err := uuid.Parse(c.Param("grantId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid grant_id"})
		return
	}
	if err := h.service.Delete(c.Request.Context(), grantID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/grants/...`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add internal/grants/handler.go
git commit -m "feat: add Grants HTTP handler for project and app grant CRUD"
```

---

## Task 10: Update RBAC Permissions

**Files:**
- Modify: `internal/auth/rbac.go`

- [ ] **Step 1: Add the PermGroupManage constant**

In `internal/auth/rbac.go`, add to the permission constants block:

```go
PermGroupManage Permission = "group:manage"
```

- [ ] **Step 2: Add PermGroupManage to owner and admin roles**

In the `rolePermissions` map, add `PermGroupManage` to the permission lists for `RoleOwner`, `RoleOrgOwner`, `RoleAdmin`, and `RoleOrgAdmin`.

- [ ] **Step 3: Verify it compiles**

Run: `go build ./internal/auth/...`
Expected: No errors

- [ ] **Step 4: Commit**

```bash
git add internal/auth/rbac.go
git commit -m "feat: add PermGroupManage to RBAC permission constants and role mappings"
```

---

## Task 11: Remove Project Members from Members Package

**Files:**
- Modify: `internal/members/repository.go` — remove project member interface methods
- Modify: `internal/members/service.go` — remove project member service methods
- Modify: `internal/members/handler.go` — remove project member routes
- Modify: `internal/platform/database/postgres/members.go` — remove project member repository methods

- [ ] **Step 1: Remove project member methods from the Repository interface**

In `internal/members/repository.go`, remove:
- The `ProjectMemberRow` struct
- All project member methods from the `Repository` interface: `ListProjectMembers`, `GetProjectMember`, `AddProjectMember`, `UpdateProjectMemberRole`, `RemoveProjectMember`

- [ ] **Step 2: Remove project member methods from the Service interface and implementation**

In `internal/members/service.go`, remove:
- All project member methods from the `Service` interface
- All project member method implementations on `memberService`
- The `ValidProjectRole` function if it exists

- [ ] **Step 3: Remove project member routes from the handler**

In `internal/members/handler.go`, remove:
- The `resolveProjectID` helper method
- The `projectMembers` route group from `RegisterRoutes`
- All project member handler methods: `listProjectMembers`, `addProjectMember`, `updateProjectMemberRole`, `removeProjectMember`

- [ ] **Step 4: Remove project member SQL methods from postgres implementation**

In `internal/platform/database/postgres/members.go`, remove all project member methods: `ListProjectMembers`, `GetProjectMember`, `AddProjectMember`, `UpdateProjectMemberRole`, `RemoveProjectMember`.

- [ ] **Step 5: Verify it compiles**

Run: `go build ./...`
Expected: No errors (if there are compile errors from callers, fix them — they should be in test files or main.go which will be updated in Task 13)

- [ ] **Step 6: Commit**

```bash
git add internal/members/repository.go internal/members/service.go internal/members/handler.go internal/platform/database/postgres/members.go
git commit -m "refactor: remove project_members from members package (replaced by resource_grants)"
```

---

## Task 12: Update Entity Listing Queries for Visibility Filtering

**Files:**
- Modify: `internal/entities/repository.go` — add userID and orgRole params to list methods
- Modify: `internal/entities/service.go` — pass through new params
- Modify: `internal/entities/handler.go` — extract user_id and role from context
- Modify: `internal/platform/database/postgres/entities.go` — add visibility-filtering SQL

- [ ] **Step 1: Update the EntityRepository interface**

In `internal/entities/repository.go`, change the signatures:

```go
ListProjectsByOrg(ctx context.Context, orgID uuid.UUID, includeDeleted bool, userID uuid.UUID, orgRole string) ([]*models.Project, error)
ListAppsByProject(ctx context.Context, projectID uuid.UUID, includeDeleted bool, userID uuid.UUID, orgRole string) ([]*models.Application, error)
```

- [ ] **Step 2: Update the EntityService interface and implementation**

In `internal/entities/service.go`, update:

```go
ListProjectsByOrg(ctx context.Context, orgID uuid.UUID, includeDeleted bool, userID uuid.UUID, orgRole string) ([]*models.Project, error)
ListAppsByProject(ctx context.Context, projectID uuid.UUID, includeDeleted bool, userID uuid.UUID, orgRole string) ([]*models.Application, error)
```

And pass through in the implementations:

```go
func (s *entityService) ListProjectsByOrg(ctx context.Context, orgID uuid.UUID, includeDeleted bool, userID uuid.UUID, orgRole string) ([]*models.Project, error) {
	return s.repo.ListProjectsByOrg(ctx, orgID, includeDeleted, userID, orgRole)
}

func (s *entityService) ListAppsByProject(ctx context.Context, projectID uuid.UUID, includeDeleted bool, userID uuid.UUID, orgRole string) ([]*models.Application, error) {
	return s.repo.ListAppsByProject(ctx, projectID, includeDeleted, userID, orgRole)
}
```

- [ ] **Step 3: Update the handler to extract user_id and role from context**

In `internal/entities/handler.go`, update `listProjects`:

```go
func (h *Handler) listProjects(c *gin.Context) {
	org, err := h.service.GetOrgBySlug(c.Request.Context(), c.Param("orgSlug"))
	if err != nil || org == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "organization not found"})
		return
	}

	uid, _ := c.Get("user_id")
	userID, _ := uid.(uuid.UUID)
	roleVal, _ := c.Get("role")
	orgRole := fmt.Sprintf("%v", roleVal)

	includeDeleted := c.Query("include_deleted") == "true"
	projects, err := h.service.ListProjectsByOrg(c.Request.Context(), org.ID, includeDeleted, userID, orgRole)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"projects": projects})
}
```

Apply the same pattern to `listApps` — extract `userID` and `orgRole`, pass to `ListAppsByProject`.

- [ ] **Step 4: Update the postgres ListProjectsByOrg query with visibility filtering**

In `internal/platform/database/postgres/entities.go`, update `ListProjectsByOrg`:

```go
func (r *EntityRepository) ListProjectsByOrg(ctx context.Context, orgID uuid.UUID, includeDeleted bool, userID uuid.UUID, orgRole string) ([]*models.Project, error) {
	deletedFilter := " AND p.deleted_at IS NULL"
	if includeDeleted {
		deletedFilter = ""
	}
	q := `SELECT p.id, p.org_id, p.name, p.slug, COALESCE(p.description, ''), COALESCE(p.repo_url, ''), p.created_at, p.updated_at, p.deleted_at
		FROM projects p
		WHERE p.org_id = $1` + deletedFilter + `
		AND (
			$3 = 'owner'
			OR NOT EXISTS (SELECT 1 FROM resource_grants rg WHERE rg.project_id = p.id)
			OR EXISTS (SELECT 1 FROM resource_grants rg WHERE rg.project_id = p.id AND rg.user_id = $2)
			OR EXISTS (
				SELECT 1 FROM resource_grants rg
				JOIN group_members gm ON gm.group_id = rg.group_id
				WHERE rg.project_id = p.id AND gm.user_id = $2
			)
		)
		ORDER BY p.name`

	rows, err := r.pool.Query(ctx, q, orgID, userID, orgRole)
	// ... rest of scan logic unchanged, just update column references to use p. prefix
```

- [ ] **Step 5: Update the postgres ListAppsByProject query with visibility filtering**

In `internal/platform/database/postgres/entities.go`, update `ListAppsByProject`:

```go
func (r *EntityRepository) ListAppsByProject(ctx context.Context, projectID uuid.UUID, includeDeleted bool, userID uuid.UUID, orgRole string) ([]*models.Application, error) {
	deletedFilter := " AND a.deleted_at IS NULL"
	if includeDeleted {
		deletedFilter = ""
	}
	q := `SELECT a.id, a.project_id, a.name, a.slug, COALESCE(a.description, ''), COALESCE(a.repo_url, ''), a.created_by, a.created_at, a.updated_at, a.deleted_at
		FROM applications a
		WHERE a.project_id = $1` + deletedFilter + `
		AND (
			$3 = 'owner'
			OR (
				NOT EXISTS (SELECT 1 FROM resource_grants rg WHERE rg.application_id = a.id)
				AND NOT EXISTS (SELECT 1 FROM resource_grants rg WHERE rg.project_id = a.project_id)
			)
			OR (
				NOT EXISTS (SELECT 1 FROM resource_grants rg WHERE rg.application_id = a.id)
				AND (
					EXISTS (SELECT 1 FROM resource_grants rg WHERE rg.project_id = a.project_id AND rg.user_id = $2)
					OR EXISTS (
						SELECT 1 FROM resource_grants rg
						JOIN group_members gm ON gm.group_id = rg.group_id
						WHERE rg.project_id = a.project_id AND gm.user_id = $2
					)
				)
			)
			OR EXISTS (SELECT 1 FROM resource_grants rg WHERE rg.application_id = a.id AND rg.user_id = $2)
			OR EXISTS (
				SELECT 1 FROM resource_grants rg
				JOIN group_members gm ON gm.group_id = rg.group_id
				WHERE rg.application_id = a.id AND gm.user_id = $2
			)
		)
		ORDER BY a.name`

	rows, err := r.pool.Query(ctx, q, projectID, userID, orgRole)
	// ... rest of scan logic unchanged, just update column references to use a. prefix
```

- [ ] **Step 6: Fix all callers**

Search for all calls to `ListProjectsByOrg` and `ListAppsByProject` across the codebase. Update each call to pass `userID` and `orgRole`. For internal/backend callers that don't have a user context (e.g., background jobs), pass `uuid.Nil` and `"owner"` to bypass filtering.

- [ ] **Step 7: Verify it compiles and tests pass**

Run: `go build ./... && make test`
Expected: Compiles and existing tests pass

- [ ] **Step 8: Commit**

```bash
git add internal/entities/ internal/platform/database/postgres/entities.go
git commit -m "feat: add visibility filtering to project and app listing queries"
```

---

## Task 13: Wire Everything in main.go

**Files:**
- Modify: `cmd/api/main.go`

- [ ] **Step 1: Add imports and construct new repositories/services/handlers**

Add these to the dependency construction section of `cmd/api/main.go`:

```go
// Groups
groupRepo := postgres.NewGroupRepository(db.Pool)
groupService := groups.NewService(groupRepo)
groupHandler := groups.NewHandler(groupService, entityService, rbac)

// Grants
grantRepo := postgres.NewGrantRepository(db.Pool)
grantService := grants.NewService(grantRepo)
grantHandler := grants.NewHandler(grantService, entityService, rbac)
```

- [ ] **Step 2: Register routes**

Add to the route registration section (after existing handler registrations):

```go
groupHandler.RegisterRoutes(api)
grantHandler.RegisterRoutes(api)
```

- [ ] **Step 3: Verify it compiles and the server starts**

Run: `go build ./cmd/api/... && make run-api`
Expected: Server starts without errors

- [ ] **Step 4: Commit**

```bash
git add cmd/api/main.go
git commit -m "feat: wire groups and grants handlers into API server"
```

---

## Task 14: Remove RequireProjectPermission from Auth Middleware

**Files:**
- Modify: `internal/auth/middleware.go`

- [ ] **Step 1: Remove RequireProjectPermission and RequireEnvironmentPermission**

Delete the `RequireProjectPermission` and `RequireEnvironmentPermission` functions from `internal/auth/middleware.go`.

- [ ] **Step 2: Find and update all callers**

Search for `RequireProjectPermission` and `RequireEnvironmentPermission` across the codebase. Replace with `grants.RequireResourceAccess(grantService, "read")` or `grants.RequireResourceAccess(grantService, "write")` as appropriate based on the original permission level:
- Read-only endpoints → `RequireResourceAccess(grantService, "read")`
- Mutation endpoints → `RequireResourceAccess(grantService, "write")`

This may require passing the `grantService` to handlers that use this middleware, or wiring it at the route-registration level in `main.go`.

- [ ] **Step 3: Verify it compiles**

Run: `go build ./...`
Expected: No errors

- [ ] **Step 4: Commit**

```bash
git add internal/auth/middleware.go cmd/api/main.go
git commit -m "refactor: replace RequireProjectPermission with RequireResourceAccess"
```

---

## Task 15: Frontend API Functions

**Files:**
- Modify: `web/src/api.ts`

- [ ] **Step 1: Add group API functions**

Add to `web/src/api.ts`:

```typescript
// ── Groups ──────────────────────────────────────────────────

export interface Group {
  id: string;
  org_id: string;
  name: string;
  slug: string;
  description: string;
  member_count: number;
  created_at: string;
}

export interface GroupMember {
  group_id: string;
  user_id: string;
  name: string;
  email: string;
  avatar_url?: string;
  created_at: string;
}

export const groupsApi = {
  list: (orgSlug: string) =>
    api.get<{ groups: Group[] }>(`/orgs/${orgSlug}/groups`).then((r) => r.data.groups),

  create: (orgSlug: string, data: { name: string; description?: string }) =>
    api.post<Group>(`/orgs/${orgSlug}/groups`, data).then((r) => r.data),

  update: (orgSlug: string, groupSlug: string, data: { name: string; description?: string }) =>
    api.put<Group>(`/orgs/${orgSlug}/groups/${groupSlug}`, data).then((r) => r.data),

  delete: (orgSlug: string, groupSlug: string) =>
    api.delete(`/orgs/${orgSlug}/groups/${groupSlug}`),

  listMembers: (orgSlug: string, groupSlug: string) =>
    api.get<{ members: GroupMember[] }>(`/orgs/${orgSlug}/groups/${groupSlug}/members`).then((r) => r.data.members),

  addMember: (orgSlug: string, groupSlug: string, userId: string) =>
    api.post(`/orgs/${orgSlug}/groups/${groupSlug}/members`, { user_id: userId }),

  removeMember: (orgSlug: string, groupSlug: string, userId: string) =>
    api.delete(`/orgs/${orgSlug}/groups/${groupSlug}/members/${userId}`),
};
```

- [ ] **Step 2: Add grant API functions**

```typescript
// ── Grants ──────────────────────────────────────────────────

export interface ResourceGrant {
  id: string;
  org_id: string;
  project_id?: string;
  application_id?: string;
  user_id?: string;
  group_id?: string;
  permission: 'read' | 'write';
  grantee_name: string;
  grantee_type: 'user' | 'group';
  created_at: string;
}

export const grantsApi = {
  listProjectGrants: (orgSlug: string, projectSlug: string) =>
    api.get<{ grants: ResourceGrant[] }>(`/orgs/${orgSlug}/projects/${projectSlug}/grants`).then((r) => r.data.grants),

  createProjectGrant: (orgSlug: string, projectSlug: string, data: { user_id?: string; group_id?: string; permission: string }) =>
    api.post<ResourceGrant>(`/orgs/${orgSlug}/projects/${projectSlug}/grants`, data).then((r) => r.data),

  listAppGrants: (orgSlug: string, projectSlug: string, appSlug: string) =>
    api.get<{ grants: ResourceGrant[] }>(`/orgs/${orgSlug}/projects/${projectSlug}/apps/${appSlug}/grants`).then((r) => r.data.grants),

  createAppGrant: (orgSlug: string, projectSlug: string, appSlug: string, data: { user_id?: string; group_id?: string; permission: string }) =>
    api.post<ResourceGrant>(`/orgs/${orgSlug}/projects/${projectSlug}/apps/${appSlug}/grants`, data).then((r) => r.data),

  deleteGrant: (orgSlug: string, projectSlug: string, grantId: string, appSlug?: string) => {
    const base = appSlug
      ? `/orgs/${orgSlug}/projects/${projectSlug}/apps/${appSlug}/grants/${grantId}`
      : `/orgs/${orgSlug}/projects/${projectSlug}/grants/${grantId}`;
    return api.delete(base);
  },
};
```

- [ ] **Step 3: Commit**

```bash
git add web/src/api.ts
git commit -m "feat: add groups and grants API client functions"
```

---

## Task 16: React Hooks for Groups and Grants

**Files:**
- Create: `web/src/hooks/useGroups.ts`
- Create: `web/src/hooks/useGrants.ts`

- [ ] **Step 1: Create useGroups hook**

```typescript
import { useState, useEffect, useCallback } from 'react';
import { groupsApi, Group, GroupMember } from '../api';

export function useGroups(orgSlug: string | undefined) {
  const [groups, setGroups] = useState<Group[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(() => {
    if (!orgSlug) return;
    setLoading(true);
    groupsApi
      .list(orgSlug)
      .then(setGroups)
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false));
  }, [orgSlug]);

  useEffect(() => { refresh(); }, [refresh]);

  return { groups, loading, error, refresh };
}

export function useGroupMembers(orgSlug: string | undefined, groupSlug: string | undefined) {
  const [members, setMembers] = useState<GroupMember[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(() => {
    if (!orgSlug || !groupSlug) return;
    setLoading(true);
    groupsApi
      .listMembers(orgSlug, groupSlug)
      .then(setMembers)
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false));
  }, [orgSlug, groupSlug]);

  useEffect(() => { refresh(); }, [refresh]);

  return { members, loading, error, refresh };
}
```

- [ ] **Step 2: Create useGrants hook**

```typescript
import { useState, useEffect, useCallback } from 'react';
import { grantsApi, ResourceGrant } from '../api';

export function useGrants(
  orgSlug: string | undefined,
  projectSlug: string | undefined,
  appSlug?: string | undefined,
) {
  const [grants, setGrants] = useState<ResourceGrant[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(() => {
    if (!orgSlug || !projectSlug) return;
    setLoading(true);
    const fetcher = appSlug
      ? grantsApi.listAppGrants(orgSlug, projectSlug, appSlug)
      : grantsApi.listProjectGrants(orgSlug, projectSlug);
    fetcher
      .then(setGrants)
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false));
  }, [orgSlug, projectSlug, appSlug]);

  useEffect(() => { refresh(); }, [refresh]);

  return { grants, loading, error, refresh };
}
```

- [ ] **Step 3: Commit**

```bash
git add web/src/hooks/useGroups.ts web/src/hooks/useGrants.ts
git commit -m "feat: add useGroups and useGrants React hooks"
```

---

## Task 17: Groups Tab UI in MembersPage

**Files:**
- Modify: `web/src/pages/MembersPage.tsx`

- [ ] **Step 1: Replace the Groups tab placeholder**

Replace the groups tab content (the `<p className="empty-state">Groups management coming soon.</p>` block) with the full group management UI. This includes:

1. **State variables** at the top of the component:
```typescript
const [selectedGroup, setSelectedGroup] = useState<Group | null>(null);
const [showCreateGroup, setShowCreateGroup] = useState(false);
const [newGroupName, setNewGroupName] = useState('');
const [newGroupDesc, setNewGroupDesc] = useState('');
const [confirmDeleteGroup, setConfirmDeleteGroup] = useState<string | null>(null);
const [addMemberUserId, setAddMemberUserId] = useState('');
```

2. **Import the hooks and API**:
```typescript
import { useGroups, useGroupMembers } from '../hooks/useGroups';
import { groupsApi, Group } from '../api';
```

3. **Hook calls**:
```typescript
const { groups, loading: groupsLoading, refresh: refreshGroups } = useGroups(orgSlug);
const { members: groupMembers, loading: gmLoading, refresh: refreshGM } = useGroupMembers(
  orgSlug,
  selectedGroup?.slug,
);
```

4. **Groups tab content** — two views:
   - **Group list view** (when `selectedGroup` is null): Table with name, description, member count, edit/delete actions. "Create Group" button at top with inline form. Click a row to select it.
   - **Group detail view** (when `selectedGroup` is set): Back button, group name heading, member table with name/email/remove, "Add Member" form with user ID input.

The UI follows the exact same patterns as the Members tab: inline form rows, confirmation pattern for deletes, table layout.

- [ ] **Step 2: Verify the page renders**

Run: `make run-web` and navigate to the Members page, Groups tab.
Expected: Groups tab shows the group list (empty if no groups yet). Create group form works. Clicking a group shows its members.

- [ ] **Step 3: Commit**

```bash
git add web/src/pages/MembersPage.tsx
git commit -m "feat: implement Groups tab UI with CRUD and member management"
```

---

## Task 18: Authorization Tab in SettingsPage

**Files:**
- Modify: `web/src/pages/SettingsPage.tsx`

- [ ] **Step 1: Add 'authorization' to the SettingsTab type and tab lists**

Update the `SettingsTab` type:
```typescript
type SettingsTab = 'environments' | 'webhooks' | 'notifications' | 'general' | 'danger' | 'authorization';
```

Update `getTabsForLevel` to add the authorization tab for project and app levels:
```typescript
case 'project':
  return [
    { key: 'general', label: 'Project Settings' },
    { key: 'authorization', label: 'Authorization' },
    { key: 'danger', label: 'Danger Zone' },
  ];
case 'app':
  return [
    { key: 'general', label: 'General' },
    { key: 'authorization', label: 'Authorization' },
    { key: 'danger', label: 'Danger Zone' },
  ];
```

Update `defaultTab` `validTabs` to include `'authorization'` for project and app.

- [ ] **Step 2: Add imports and hooks**

```typescript
import { useGrants } from '../hooks/useGrants';
import { useGroups } from '../hooks/useGroups';
import { grantsApi, ResourceGrant } from '../api';
```

- [ ] **Step 3: Add the Authorization tab content**

Add state variables:
```typescript
const [addGrantType, setAddGrantType] = useState<'user' | 'group'>('user');
const [addGrantId, setAddGrantId] = useState('');
const [addGrantPerm, setAddGrantPerm] = useState<'read' | 'write'>('read');
const [confirmDeleteGrant, setConfirmDeleteGrant] = useState<string | null>(null);
```

Add hook calls:
```typescript
const { grants, loading: grantsLoading, refresh: refreshGrants } = useGrants(orgSlug, projectSlug, level === 'app' ? appSlug : undefined);
const { groups: orgGroups } = useGroups(orgSlug);
```

Add the tab content block:
```typescript
{activeTab === 'authorization' && (
  <div className="settings-section">
    {grants.length === 0 ? (
      <div className="empty-state">
        <p>
          {level === 'app'
            ? 'This app inherits access from its project. Add a user or group to override with app-specific access.'
            : 'This project is open to all organization members. Add a user or group to restrict access.'}
        </p>
      </div>
    ) : (
      <>
        <div className="alert alert-warning" style={{ marginBottom: 16 }}>
          Access to this {level === 'app' ? 'app' : 'project'} is restricted. Only users and groups listed below (and org owners) can access it.
        </div>
        <table className="data-table">
          <thead>
            <tr>
              <th>Name</th>
              <th>Type</th>
              <th>Permission</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            {grants.map((g) => (
              <tr key={g.id}>
                <td>{g.grantee_name}</td>
                <td>{g.grantee_type}</td>
                <td>{g.permission}</td>
                <td>
                  {confirmDeleteGrant === g.id ? (
                    <span className="inline-confirm">
                      Remove access?{' '}
                      <button className="btn btn-sm btn-danger" onClick={() => handleDeleteGrant(g.id)}>Yes</button>{' '}
                      <button className="btn btn-sm" onClick={() => setConfirmDeleteGrant(null)}>No</button>
                    </span>
                  ) : (
                    <button className="btn btn-sm btn-danger" onClick={() => setConfirmDeleteGrant(g.id)}>Remove</button>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </>
    )}

    <h3 style={{ marginTop: 24 }}>Add Access</h3>
    <div className="inline-form-row">
      <select className="form-select" value={addGrantType} onChange={(e) => setAddGrantType(e.target.value as 'user' | 'group')}>
        <option value="user">User</option>
        <option value="group">Group</option>
      </select>
      <input
        type="text"
        className="form-input"
        placeholder={addGrantType === 'user' ? 'User ID' : 'Group ID'}
        value={addGrantId}
        onChange={(e) => setAddGrantId(e.target.value)}
      />
      <select className="form-select" value={addGrantPerm} onChange={(e) => setAddGrantPerm(e.target.value as 'read' | 'write')}>
        <option value="read">Read</option>
        <option value="write">Write</option>
      </select>
      <button className="btn btn-primary" onClick={handleAddGrant}>Add</button>
    </div>
  </div>
)}
```

Add handler functions:
```typescript
const handleAddGrant = async () => {
  if (!orgSlug || !projectSlug || !addGrantId) return;
  const data = {
    user_id: addGrantType === 'user' ? addGrantId : undefined,
    group_id: addGrantType === 'group' ? addGrantId : undefined,
    permission: addGrantPerm,
  };
  try {
    if (level === 'app' && appSlug) {
      await grantsApi.createAppGrant(orgSlug, projectSlug, appSlug, data);
    } else {
      await grantsApi.createProjectGrant(orgSlug, projectSlug, data);
    }
    setAddGrantId('');
    refreshGrants();
  } catch (e: any) {
    alert(e.response?.data?.error || e.message);
  }
};

const handleDeleteGrant = async (grantId: string) => {
  if (!orgSlug || !projectSlug) return;
  try {
    await grantsApi.deleteGrant(orgSlug, projectSlug, grantId, level === 'app' ? appSlug : undefined);
    setConfirmDeleteGrant(null);
    refreshGrants();
  } catch (e: any) {
    alert(e.response?.data?.error || e.message);
  }
};
```

- [ ] **Step 4: Verify the page renders**

Run: `make run-web` and navigate to Project Settings → Authorization tab, and App Settings → Authorization tab.
Expected: Both tabs render. Empty state shows appropriate message. Adding/removing grants works.

- [ ] **Step 5: Commit**

```bash
git add web/src/pages/SettingsPage.tsx
git commit -m "feat: add Authorization tab to project and app settings pages"
```

---

## Task 19: End-to-End Verification

- [ ] **Step 1: Start all services**

Run: `make dev-up && make migrate-up && make run-api`

- [ ] **Step 2: Start the web dashboard**

Run: `make run-web`

- [ ] **Step 3: Test group management**

1. Navigate to Members page → Groups tab
2. Create a group (e.g., "Backend Team")
3. Add a member to the group
4. Remove the member
5. Delete the group

- [ ] **Step 4: Test project authorization**

1. Navigate to Project Settings → Authorization tab
2. Verify empty state message: "This project is open to all organization members..."
3. Add a user grant with `write` permission
4. Verify the warning banner appears
5. In a different browser/incognito (as a different user), verify the project is hidden from the project list
6. Remove the grant
7. Verify the project reappears for the other user

- [ ] **Step 5: Test app authorization with cascade**

1. Add a user grant on a project (write)
2. Verify all apps in that project are visible to the granted user
3. Add a different user grant on a specific app
4. Verify that app now uses its own grants (project grant no longer applies to this app)
5. Remove the app-level grant
6. Verify the app falls back to project-level grants

- [ ] **Step 6: Test org owner bypass**

1. As an org owner, verify all projects and apps are always visible regardless of grants

- [ ] **Step 7: Run all tests**

Run: `make test`
Expected: All tests pass

- [ ] **Step 8: Commit any fixes**

```bash
git add -A
git commit -m "fix: address issues found during e2e verification"
```

---

## Task 20: Update Documentation

**Files:**
- Modify: `docs/CURRENT_INITIATIVES.md`

- [ ] **Step 1: Update current initiatives**

Add the Groups & Resource Authorization initiative to the active table:

```markdown
| Groups & Resource Authorization | Implementation | [Spec](./superpowers/specs/2026-04-16-groups-and-resource-authorization-design.md) / [Plan](./superpowers/plans/2026-04-16-groups-and-resource-authorization.md) | Migration 041, new groups/grants packages, visibility filtering |
```

- [ ] **Step 2: Commit**

```bash
git add docs/CURRENT_INITIATIVES.md
git commit -m "docs: add Groups & Resource Authorization to current initiatives"
```
