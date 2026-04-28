# Members CRUD Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add org-level and project-level member management with a dedicated `internal/members/` package, wiring the existing frontend MembersPage to persist changes via real API endpoints.

**Architecture:** New `internal/members/` package with repository interface, service (business logic + validation), and Gin handler. The repository wraps the existing `UserRepository` member methods already in `postgres/users.go`. A migration adds missing columns (`id`, `created_at`, `updated_at`) to `org_members` and `project_members` tables.

**Tech Stack:** Go, Gin, PostgreSQL (pgx), React + TypeScript

---

### Task 1: Database Migration

**Files:**
- Create: `migrations/024_add_member_columns.up.sql`
- Create: `migrations/024_add_member_columns.down.sql`

The existing `org_members` table (migration 003) lacks `id`, `invited_by`, `created_at`, `updated_at` columns. The existing `project_members` table (migration 005) lacks `id`, `created_at`, `updated_at`. The Go models and postgres code already reference these columns, so the migration catches the DB schema up.

- [ ] **Step 1: Create the up migration**

```sql
-- migrations/024_add_member_columns.up.sql

-- Add missing columns to org_members
ALTER TABLE org_members
    ADD COLUMN IF NOT EXISTS id         UUID DEFAULT gen_random_uuid(),
    ADD COLUMN IF NOT EXISTS invited_by UUID REFERENCES users(id),
    ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

-- Expand role constraint to include 'viewer'
ALTER TABLE org_members DROP CONSTRAINT IF EXISTS org_members_role_check;
ALTER TABLE org_members ADD CONSTRAINT org_members_role_check
    CHECK (role IN ('owner', 'admin', 'member', 'viewer'));

-- Add missing columns to project_members
ALTER TABLE project_members
    ADD COLUMN IF NOT EXISTS id         UUID DEFAULT gen_random_uuid(),
    ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();
```

- [ ] **Step 2: Create the down migration**

```sql
-- migrations/024_add_member_columns.down.sql

ALTER TABLE project_members
    DROP COLUMN IF EXISTS updated_at,
    DROP COLUMN IF EXISTS created_at,
    DROP COLUMN IF EXISTS id;

ALTER TABLE org_members DROP CONSTRAINT IF EXISTS org_members_role_check;
ALTER TABLE org_members ADD CONSTRAINT org_members_role_check
    CHECK (role IN ('owner', 'admin', 'member'));

ALTER TABLE org_members
    DROP COLUMN IF EXISTS updated_at,
    DROP COLUMN IF EXISTS created_at,
    DROP COLUMN IF EXISTS invited_by,
    DROP COLUMN IF EXISTS id;
```

- [ ] **Step 3: Run the migration**

Run: `make migrate-up`
Expected: Migration 024 applies cleanly.

- [ ] **Step 4: Commit**

```bash
git add migrations/024_add_member_columns.up.sql migrations/024_add_member_columns.down.sql
git commit -m "feat: add id, timestamps, viewer role to member tables (migration 024)"
```

---

### Task 2: Members Repository Interface

**Files:**
- Create: `internal/members/repository.go`

This defines the repository interface that the service depends on. It wraps the existing `UserRepository` methods and adds the user-by-email lookup needed for the "add by email" flow.

- [ ] **Step 1: Create the repository interface**

```go
// internal/members/repository.go
package members

import (
	"context"

	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/google/uuid"
)

// OrgMemberRow extends OrgMember with user profile fields from a JOIN.
type OrgMemberRow struct {
	models.OrgMember
	Name      string `json:"name"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url,omitempty"`
}

// ProjectMemberRow extends ProjectMember with user profile fields from a JOIN.
type ProjectMemberRow struct {
	models.ProjectMember
	Name      string `json:"name"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url,omitempty"`
}

// Repository defines persistence operations for org and project members.
type Repository interface {
	// Org members
	ListOrgMembers(ctx context.Context, orgID uuid.UUID) ([]OrgMemberRow, error)
	GetOrgMember(ctx context.Context, orgID, userID uuid.UUID) (*models.OrgMember, error)
	AddOrgMember(ctx context.Context, m *models.OrgMember) error
	UpdateOrgMemberRole(ctx context.Context, orgID, userID uuid.UUID, role models.OrgRole) error
	RemoveOrgMember(ctx context.Context, orgID, userID uuid.UUID) error
	CountOrgOwners(ctx context.Context, orgID uuid.UUID) (int, error)

	// Project members
	ListProjectMembers(ctx context.Context, projectID uuid.UUID) ([]ProjectMemberRow, error)
	GetProjectMember(ctx context.Context, projectID, userID uuid.UUID) (*models.ProjectMember, error)
	AddProjectMember(ctx context.Context, m *models.ProjectMember) error
	UpdateProjectMemberRole(ctx context.Context, projectID, userID uuid.UUID, role models.ProjectRole) error
	RemoveProjectMember(ctx context.Context, projectID, userID uuid.UUID) error

	// User lookup
	GetUserByEmail(ctx context.Context, email string) (*models.User, error)
}
```

- [ ] **Step 2: Verify compile**

Run: `cd /Users/sgamel/git/DeploySentry && go build ./internal/members/...`
Expected: Compiles (no implementations yet, but the package should parse).

- [ ] **Step 3: Commit**

```bash
git add internal/members/repository.go
git commit -m "feat: add members repository interface"
```

---

### Task 3: PostgreSQL Repository Implementation

**Files:**
- Create: `internal/platform/database/postgres/members.go`

This wraps the existing `UserRepository` member methods and adds the JOIN queries for `OrgMemberRow`/`ProjectMemberRow`, plus `CountOrgOwners` and `GetUserByEmail`.

- [ ] **Step 1: Create the postgres implementation**

```go
// internal/platform/database/postgres/members.go
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/shadsorg/deploysentry/internal/members"
	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// MemberRepository implements members.Repository using PostgreSQL.
type MemberRepository struct {
	pool *pgxpool.Pool
}

// NewMemberRepository creates a new MemberRepository.
func NewMemberRepository(pool *pgxpool.Pool) *MemberRepository {
	return &MemberRepository{pool: pool}
}

// ---------------------------------------------------------------------------
// Org members
// ---------------------------------------------------------------------------

func (r *MemberRepository) ListOrgMembers(ctx context.Context, orgID uuid.UUID) ([]members.OrgMemberRow, error) {
	const q = `
		SELECT om.id, om.org_id, om.user_id, om.role,
			COALESCE(om.invited_by, '00000000-0000-0000-0000-000000000000'::uuid),
			COALESCE(om.joined_at, om.created_at),
			om.created_at, om.updated_at,
			u.name, u.email, COALESCE(u.avatar_url, '')
		FROM org_members om
		JOIN users u ON om.user_id = u.id
		WHERE om.org_id = $1
		ORDER BY om.joined_at`

	rows, err := r.pool.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("postgres.ListOrgMembers: %w", err)
	}
	defer rows.Close()

	var result []members.OrgMemberRow
	for rows.Next() {
		var row members.OrgMemberRow
		if err := rows.Scan(
			&row.ID, &row.OrgID, &row.UserID, &row.Role,
			&row.InvitedBy, &row.JoinedAt, &row.CreatedAt, &row.UpdatedAt,
			&row.Name, &row.Email, &row.AvatarURL,
		); err != nil {
			return nil, fmt.Errorf("postgres.ListOrgMembers scan: %w", err)
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func (r *MemberRepository) GetOrgMember(ctx context.Context, orgID, userID uuid.UUID) (*models.OrgMember, error) {
	const q = `
		SELECT id, org_id, user_id, role,
			COALESCE(invited_by, '00000000-0000-0000-0000-000000000000'::uuid),
			COALESCE(joined_at, created_at),
			created_at, updated_at
		FROM org_members
		WHERE org_id = $1 AND user_id = $2`

	var m models.OrgMember
	err := r.pool.QueryRow(ctx, q, orgID, userID).Scan(
		&m.ID, &m.OrgID, &m.UserID, &m.Role,
		&m.InvitedBy, &m.JoinedAt, &m.CreatedAt, &m.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("postgres.GetOrgMember: %w", err)
	}
	return &m, nil
}

func (r *MemberRepository) AddOrgMember(ctx context.Context, m *models.OrgMember) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	now := time.Now().UTC()
	m.CreatedAt = now
	m.UpdatedAt = now
	if m.JoinedAt.IsZero() {
		m.JoinedAt = now
	}

	const q = `
		INSERT INTO org_members (id, org_id, user_id, role, invited_by, joined_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	_, err := r.pool.Exec(ctx, q,
		m.ID, m.OrgID, m.UserID, m.Role, m.InvitedBy, m.JoinedAt, m.CreatedAt, m.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrConflict
		}
		return fmt.Errorf("postgres.AddOrgMember: %w", err)
	}
	return nil
}

func (r *MemberRepository) UpdateOrgMemberRole(ctx context.Context, orgID, userID uuid.UUID, role models.OrgRole) error {
	const q = `UPDATE org_members SET role = $3, updated_at = $4 WHERE org_id = $1 AND user_id = $2`
	tag, err := r.pool.Exec(ctx, q, orgID, userID, role, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("postgres.UpdateOrgMemberRole: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *MemberRepository) RemoveOrgMember(ctx context.Context, orgID, userID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM org_members WHERE org_id = $1 AND user_id = $2`, orgID, userID)
	if err != nil {
		return fmt.Errorf("postgres.RemoveOrgMember: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *MemberRepository) CountOrgOwners(ctx context.Context, orgID uuid.UUID) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM org_members WHERE org_id = $1 AND role = 'owner'`, orgID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("postgres.CountOrgOwners: %w", err)
	}
	return count, nil
}

// ---------------------------------------------------------------------------
// Project members
// ---------------------------------------------------------------------------

func (r *MemberRepository) ListProjectMembers(ctx context.Context, projectID uuid.UUID) ([]members.ProjectMemberRow, error) {
	const q = `
		SELECT pm.id, pm.project_id, pm.user_id, pm.role,
			pm.created_at, pm.updated_at,
			u.name, u.email, COALESCE(u.avatar_url, '')
		FROM project_members pm
		JOIN users u ON pm.user_id = u.id
		WHERE pm.project_id = $1
		ORDER BY pm.created_at`

	rows, err := r.pool.Query(ctx, q, projectID)
	if err != nil {
		return nil, fmt.Errorf("postgres.ListProjectMembers: %w", err)
	}
	defer rows.Close()

	var result []members.ProjectMemberRow
	for rows.Next() {
		var row members.ProjectMemberRow
		if err := rows.Scan(
			&row.ID, &row.ProjectID, &row.UserID, &row.Role,
			&row.CreatedAt, &row.UpdatedAt,
			&row.Name, &row.Email, &row.AvatarURL,
		); err != nil {
			return nil, fmt.Errorf("postgres.ListProjectMembers scan: %w", err)
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func (r *MemberRepository) GetProjectMember(ctx context.Context, projectID, userID uuid.UUID) (*models.ProjectMember, error) {
	const q = `
		SELECT id, project_id, user_id, role, created_at, updated_at
		FROM project_members
		WHERE project_id = $1 AND user_id = $2`

	var m models.ProjectMember
	err := r.pool.QueryRow(ctx, q, projectID, userID).Scan(
		&m.ID, &m.ProjectID, &m.UserID, &m.Role, &m.CreatedAt, &m.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("postgres.GetProjectMember: %w", err)
	}
	return &m, nil
}

func (r *MemberRepository) AddProjectMember(ctx context.Context, m *models.ProjectMember) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	now := time.Now().UTC()
	m.CreatedAt = now
	m.UpdatedAt = now

	const q = `
		INSERT INTO project_members (id, project_id, user_id, role, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)`

	_, err := r.pool.Exec(ctx, q, m.ID, m.ProjectID, m.UserID, m.Role, m.CreatedAt, m.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrConflict
		}
		return fmt.Errorf("postgres.AddProjectMember: %w", err)
	}
	return nil
}

func (r *MemberRepository) UpdateProjectMemberRole(ctx context.Context, projectID, userID uuid.UUID, role models.ProjectRole) error {
	const q = `UPDATE project_members SET role = $3, updated_at = $4 WHERE project_id = $1 AND user_id = $2`
	tag, err := r.pool.Exec(ctx, q, projectID, userID, role, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("postgres.UpdateProjectMemberRole: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *MemberRepository) RemoveProjectMember(ctx context.Context, projectID, userID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM project_members WHERE project_id = $1 AND user_id = $2`, projectID, userID)
	if err != nil {
		return fmt.Errorf("postgres.RemoveProjectMember: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ---------------------------------------------------------------------------
// User lookup
// ---------------------------------------------------------------------------

func (r *MemberRepository) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	const q = `
		SELECT id, email, name, COALESCE(avatar_url, ''),
			auth_provider, COALESCE(auth_provider_id, ''),
			COALESCE(password_hash, ''), email_verified,
			last_login_at, created_at, updated_at
		FROM users WHERE email = $1`

	var u models.User
	err := r.pool.QueryRow(ctx, q, email).Scan(
		&u.ID, &u.Email, &u.Name, &u.AvatarURL,
		&u.AuthProvider, &u.ProviderID,
		&u.PasswordHash, &u.EmailVerified,
		&u.LastLoginAt, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("postgres.GetUserByEmail: %w", err)
	}
	return &u, nil
}
```

- [ ] **Step 2: Verify compile**

Run: `cd /Users/sgamel/git/DeploySentry && go build ./internal/platform/database/postgres/...`
Expected: Compiles cleanly.

- [ ] **Step 3: Commit**

```bash
git add internal/platform/database/postgres/members.go
git commit -m "feat: add postgres MemberRepository implementation"
```

---

### Task 4: Members Service

**Files:**
- Create: `internal/members/service.go`

Business logic: email-to-user resolution, role validation, last-owner guard.

- [ ] **Step 1: Create the service**

```go
// internal/members/service.go
package members

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/google/uuid"
)

var (
	ErrUserNotFound   = errors.New("user not found")
	ErrAlreadyMember  = errors.New("user is already a member")
	ErrInvalidRole    = errors.New("invalid role")
	ErrLastOwner      = errors.New("cannot remove or demote the last owner")
	ErrOwnerAssign    = errors.New("cannot assign owner role via update; use ownership transfer")
	ErrNotFound       = errors.New("member not found")
)

// Service defines the business-logic layer for member management.
type Service interface {
	// Org members
	ListOrgMembers(ctx context.Context, orgID uuid.UUID) ([]OrgMemberRow, error)
	AddOrgMember(ctx context.Context, orgID uuid.UUID, email string, role models.OrgRole, addedBy uuid.UUID) (*OrgMemberRow, error)
	UpdateOrgMemberRole(ctx context.Context, orgID, userID uuid.UUID, role models.OrgRole) error
	RemoveOrgMember(ctx context.Context, orgID, userID uuid.UUID) error

	// Project members
	ListProjectMembers(ctx context.Context, projectID uuid.UUID) ([]ProjectMemberRow, error)
	AddProjectMember(ctx context.Context, projectID uuid.UUID, email string, role models.ProjectRole, addedBy uuid.UUID) (*ProjectMemberRow, error)
	UpdateProjectMemberRole(ctx context.Context, projectID, userID uuid.UUID, role models.ProjectRole) error
	RemoveProjectMember(ctx context.Context, projectID, userID uuid.UUID) error
}

type memberService struct {
	repo Repository
}

// NewService creates a new member Service.
func NewService(repo Repository) Service {
	return &memberService{repo: repo}
}

// ---------------------------------------------------------------------------
// Org members
// ---------------------------------------------------------------------------

func (s *memberService) ListOrgMembers(ctx context.Context, orgID uuid.UUID) ([]OrgMemberRow, error) {
	rows, err := s.repo.ListOrgMembers(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("listing org members: %w", err)
	}
	return rows, nil
}

func (s *memberService) AddOrgMember(ctx context.Context, orgID uuid.UUID, email string, role models.OrgRole, addedBy uuid.UUID) (*OrgMemberRow, error) {
	if !models.ValidRole(role) {
		return nil, ErrInvalidRole
	}

	user, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, ErrUserNotFound
	}

	m := &models.OrgMember{
		OrgID:     orgID,
		UserID:    user.ID,
		Role:      role,
		InvitedBy: addedBy,
	}

	if err := s.repo.AddOrgMember(ctx, m); err != nil {
		if strings.Contains(err.Error(), "conflict") {
			return nil, ErrAlreadyMember
		}
		return nil, fmt.Errorf("adding org member: %w", err)
	}

	return &OrgMemberRow{
		OrgMember: *m,
		Name:      user.Name,
		Email:     user.Email,
		AvatarURL: user.AvatarURL,
	}, nil
}

func (s *memberService) UpdateOrgMemberRole(ctx context.Context, orgID, userID uuid.UUID, role models.OrgRole) error {
	if !models.ValidRole(role) {
		return ErrInvalidRole
	}
	if role == models.OrgRoleOwner {
		return ErrOwnerAssign
	}

	// Guard: don't demote the last owner
	existing, err := s.repo.GetOrgMember(ctx, orgID, userID)
	if err != nil {
		return ErrNotFound
	}
	if existing.Role == models.OrgRoleOwner {
		count, err := s.repo.CountOrgOwners(ctx, orgID)
		if err != nil {
			return fmt.Errorf("counting owners: %w", err)
		}
		if count <= 1 {
			return ErrLastOwner
		}
	}

	if err := s.repo.UpdateOrgMemberRole(ctx, orgID, userID, role); err != nil {
		return fmt.Errorf("updating org member role: %w", err)
	}
	return nil
}

func (s *memberService) RemoveOrgMember(ctx context.Context, orgID, userID uuid.UUID) error {
	existing, err := s.repo.GetOrgMember(ctx, orgID, userID)
	if err != nil {
		return ErrNotFound
	}
	if existing.Role == models.OrgRoleOwner {
		count, err := s.repo.CountOrgOwners(ctx, orgID)
		if err != nil {
			return fmt.Errorf("counting owners: %w", err)
		}
		if count <= 1 {
			return ErrLastOwner
		}
	}

	if err := s.repo.RemoveOrgMember(ctx, orgID, userID); err != nil {
		return fmt.Errorf("removing org member: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Project members
// ---------------------------------------------------------------------------

func (s *memberService) ListProjectMembers(ctx context.Context, projectID uuid.UUID) ([]ProjectMemberRow, error) {
	rows, err := s.repo.ListProjectMembers(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("listing project members: %w", err)
	}
	return rows, nil
}

func validProjectRole(r models.ProjectRole) bool {
	switch r {
	case models.ProjectRoleAdmin, models.ProjectRoleDeveloper, models.ProjectRoleViewer:
		return true
	}
	return false
}

func (s *memberService) AddProjectMember(ctx context.Context, projectID uuid.UUID, email string, role models.ProjectRole, addedBy uuid.UUID) (*ProjectMemberRow, error) {
	if !validProjectRole(role) {
		return nil, ErrInvalidRole
	}

	user, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, ErrUserNotFound
	}

	m := &models.ProjectMember{
		ProjectID: projectID,
		UserID:    user.ID,
		Role:      role,
	}

	if err := s.repo.AddProjectMember(ctx, m); err != nil {
		return nil, fmt.Errorf("adding project member: %w", err)
	}

	return &ProjectMemberRow{
		ProjectMember: *m,
		Name:          user.Name,
		Email:         user.Email,
		AvatarURL:     user.AvatarURL,
	}, nil
}

func (s *memberService) UpdateProjectMemberRole(ctx context.Context, projectID, userID uuid.UUID, role models.ProjectRole) error {
	if !validProjectRole(role) {
		return ErrInvalidRole
	}
	if err := s.repo.UpdateProjectMemberRole(ctx, projectID, userID, role); err != nil {
		return fmt.Errorf("updating project member role: %w", err)
	}
	return nil
}

func (s *memberService) RemoveProjectMember(ctx context.Context, projectID, userID uuid.UUID) error {
	if err := s.repo.RemoveProjectMember(ctx, projectID, userID); err != nil {
		return fmt.Errorf("removing project member: %w", err)
	}
	return nil
}
```

- [ ] **Step 2: Verify compile**

Run: `cd /Users/sgamel/git/DeploySentry && go build ./internal/members/...`
Expected: Compiles cleanly.

- [ ] **Step 3: Commit**

```bash
git add internal/members/service.go
git commit -m "feat: add members service with business logic and validation"
```

---

### Task 5: Members Handler

**Files:**
- Create: `internal/members/handler.go`

HTTP handler with routes for org and project member CRUD. Uses entities service for slug → ID resolution.

- [ ] **Step 1: Create the handler**

```go
// internal/members/handler.go
package members

import (
	"errors"
	"net/http"

	"github.com/shadsorg/deploysentry/internal/auth"
	"github.com/shadsorg/deploysentry/internal/entities"
	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Handler provides HTTP endpoints for member management.
type Handler struct {
	service   Service
	entities  entities.EntityService
	rbac      *auth.RBACChecker
}

// NewHandler creates a new members HTTP handler.
func NewHandler(service Service, entityService entities.EntityService, rbac *auth.RBACChecker) *Handler {
	return &Handler{service: service, entities: entityService, rbac: rbac}
}

// RegisterRoutes mounts member management routes.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	// Org members: /orgs/:orgSlug/members
	orgMembers := rg.Group("/orgs/:orgSlug/members")
	{
		orgMembers.GET("", auth.RequirePermission(h.rbac, auth.PermOrgManage), h.listOrgMembers)
		orgMembers.POST("", auth.RequirePermission(h.rbac, auth.PermOrgManage), h.addOrgMember)
		orgMembers.PUT("/:userId", auth.RequirePermission(h.rbac, auth.PermOrgManage), h.updateOrgMemberRole)
		orgMembers.DELETE("/:userId", auth.RequirePermission(h.rbac, auth.PermOrgManage), h.removeOrgMember)
	}

	// Project members: /orgs/:orgSlug/projects/:projectSlug/members
	projectMembers := rg.Group("/orgs/:orgSlug/projects/:projectSlug/members")
	{
		projectMembers.GET("", auth.RequirePermission(h.rbac, auth.PermProjectManage), h.listProjectMembers)
		projectMembers.POST("", auth.RequirePermission(h.rbac, auth.PermProjectManage), h.addProjectMember)
		projectMembers.PUT("/:userId", auth.RequirePermission(h.rbac, auth.PermProjectManage), h.updateProjectMemberRole)
		projectMembers.DELETE("/:userId", auth.RequirePermission(h.rbac, auth.PermProjectManage), h.removeProjectMember)
	}
}

// resolveOrgID looks up the org by slug and returns its ID.
func (h *Handler) resolveOrgID(c *gin.Context) (uuid.UUID, bool) {
	org, err := h.entities.GetOrgBySlug(c.Request.Context(), c.Param("orgSlug"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "organization not found"})
		return uuid.Nil, false
	}
	return org.ID, true
}

// resolveProjectID looks up the project by org slug + project slug.
func (h *Handler) resolveProjectID(c *gin.Context) (uuid.UUID, bool) {
	org, err := h.entities.GetOrgBySlug(c.Request.Context(), c.Param("orgSlug"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "organization not found"})
		return uuid.Nil, false
	}
	project, err := h.entities.GetProjectBySlug(c.Request.Context(), org.ID, c.Param("projectSlug"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
		return uuid.Nil, false
	}
	return project.ID, true
}

// ---------------------------------------------------------------------------
// Org member handlers
// ---------------------------------------------------------------------------

func (h *Handler) listOrgMembers(c *gin.Context) {
	orgID, ok := h.resolveOrgID(c)
	if !ok {
		return
	}
	rows, err := h.service.ListOrgMembers(c.Request.Context(), orgID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list members"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"members": rows})
}

type addMemberRequest struct {
	Email string `json:"email" binding:"required"`
	Role  string `json:"role" binding:"required"`
}

func (h *Handler) addOrgMember(c *gin.Context) {
	orgID, ok := h.resolveOrgID(c)
	if !ok {
		return
	}

	var req addMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	uid, _ := c.Get("user_id")
	addedBy, _ := uid.(uuid.UUID)

	row, err := h.service.AddOrgMember(c.Request.Context(), orgID, req.Email, models.OrgRole(req.Role), addedBy)
	if err != nil {
		switch {
		case errors.Is(err, ErrUserNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found with that email"})
		case errors.Is(err, ErrAlreadyMember):
			c.JSON(http.StatusConflict, gin.H{"error": "user is already a member"})
		case errors.Is(err, ErrInvalidRole):
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid role"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to add member"})
		}
		return
	}
	c.JSON(http.StatusCreated, gin.H{"member": row})
}

type updateRoleRequest struct {
	Role string `json:"role" binding:"required"`
}

func (h *Handler) updateOrgMemberRole(c *gin.Context) {
	orgID, ok := h.resolveOrgID(c)
	if !ok {
		return
	}

	userID, err := uuid.Parse(c.Param("userId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})
		return
	}

	var req updateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.UpdateOrgMemberRole(c.Request.Context(), orgID, userID, models.OrgRole(req.Role)); err != nil {
		switch {
		case errors.Is(err, ErrInvalidRole):
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid role"})
		case errors.Is(err, ErrOwnerAssign):
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		case errors.Is(err, ErrLastOwner):
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		case errors.Is(err, ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "member not found"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update role"})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

func (h *Handler) removeOrgMember(c *gin.Context) {
	orgID, ok := h.resolveOrgID(c)
	if !ok {
		return
	}

	userID, err := uuid.Parse(c.Param("userId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})
		return
	}

	if err := h.service.RemoveOrgMember(c.Request.Context(), orgID, userID); err != nil {
		switch {
		case errors.Is(err, ErrLastOwner):
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		case errors.Is(err, ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "member not found"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to remove member"})
		}
		return
	}
	c.Status(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// Project member handlers
// ---------------------------------------------------------------------------

func (h *Handler) listProjectMembers(c *gin.Context) {
	projectID, ok := h.resolveProjectID(c)
	if !ok {
		return
	}
	rows, err := h.service.ListProjectMembers(c.Request.Context(), projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list members"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"members": rows})
}

func (h *Handler) addProjectMember(c *gin.Context) {
	projectID, ok := h.resolveProjectID(c)
	if !ok {
		return
	}

	var req addMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	uid, _ := c.Get("user_id")
	addedBy, _ := uid.(uuid.UUID)

	row, err := h.service.AddProjectMember(c.Request.Context(), projectID, req.Email, models.ProjectRole(req.Role), addedBy)
	if err != nil {
		switch {
		case errors.Is(err, ErrUserNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found with that email"})
		case errors.Is(err, ErrAlreadyMember):
			c.JSON(http.StatusConflict, gin.H{"error": "user is already a member"})
		case errors.Is(err, ErrInvalidRole):
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid role"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to add member"})
		}
		return
	}
	c.JSON(http.StatusCreated, gin.H{"member": row})
}

func (h *Handler) updateProjectMemberRole(c *gin.Context) {
	projectID, ok := h.resolveProjectID(c)
	if !ok {
		return
	}

	userID, err := uuid.Parse(c.Param("userId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})
		return
	}

	var req updateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.UpdateProjectMemberRole(c.Request.Context(), projectID, userID, models.ProjectRole(req.Role)); err != nil {
		switch {
		case errors.Is(err, ErrInvalidRole):
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid role"})
		case errors.Is(err, ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "member not found"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update role"})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

func (h *Handler) removeProjectMember(c *gin.Context) {
	projectID, ok := h.resolveProjectID(c)
	if !ok {
		return
	}

	userID, err := uuid.Parse(c.Param("userId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})
		return
	}

	if err := h.service.RemoveProjectMember(c.Request.Context(), projectID, userID); err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "member not found"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to remove member"})
		}
		return
	}
	c.Status(http.StatusNoContent)
}
```

- [ ] **Step 2: Verify compile**

Run: `cd /Users/sgamel/git/DeploySentry && go build ./internal/members/...`
Expected: Compiles cleanly.

- [ ] **Step 3: Commit**

```bash
git add internal/members/handler.go
git commit -m "feat: add members HTTP handler with org and project routes"
```

---

### Task 6: Wire into main.go

**Files:**
- Modify: `cmd/api/main.go`

- [ ] **Step 1: Add member handler wiring**

Add after the settings handler registration (line 283 in `cmd/api/main.go`):

```go
// After existing repos (line ~186):
memberRepo := postgres.NewMemberRepository(db.Pool)

// After existing services (line ~201):
memberService := members.NewService(memberRepo)

// After existing handler registrations (line ~283):
members.NewHandler(memberService, entityService, rbacChecker).RegisterRoutes(api)
```

Add the import:
```go
"github.com/shadsorg/deploysentry/internal/members"
```

- [ ] **Step 2: Verify compile**

Run: `cd /Users/sgamel/git/DeploySentry && go build ./cmd/api/...`
Expected: Compiles cleanly.

- [ ] **Step 3: Commit**

```bash
git add cmd/api/main.go
git commit -m "feat: wire members handler into API server"
```

---

### Task 7: Update Frontend Types and API Client

**Files:**
- Modify: `web/src/types.ts`
- Modify: `web/src/api.ts`

- [ ] **Step 1: Update Member type in types.ts**

Replace the existing `Member` interface (lines 232-239 of `web/src/types.ts`):

```typescript
export interface Member {
  id: string;
  user_id: string;
  name: string;
  email: string;
  avatar_url?: string;
  role: 'owner' | 'admin' | 'member' | 'viewer';
  joined_at: string;
}
```

- [ ] **Step 2: Update membersApi in api.ts**

Replace the existing `membersApi` (lines 106-110 of `web/src/api.ts`):

```typescript
// Members
export const membersApi = {
  // Org members
  listByOrg: (orgSlug: string) =>
    request<{ members: Member[] }>(`/orgs/${orgSlug}/members`),
  addToOrg: (orgSlug: string, email: string, role: string) =>
    request<{ member: Member }>(`/orgs/${orgSlug}/members`, {
      method: 'POST',
      body: JSON.stringify({ email, role }),
    }),
  updateOrgRole: (orgSlug: string, userId: string, role: string) =>
    request<{ member: Member }>(`/orgs/${orgSlug}/members/${userId}`, {
      method: 'PUT',
      body: JSON.stringify({ role }),
    }),
  removeFromOrg: (orgSlug: string, userId: string) =>
    request<void>(`/orgs/${orgSlug}/members/${userId}`, { method: 'DELETE' }),

  // Project members
  listByProject: (orgSlug: string, projectSlug: string) =>
    request<{ members: Member[] }>(`/orgs/${orgSlug}/projects/${projectSlug}/members`),
  addToProject: (orgSlug: string, projectSlug: string, email: string, role: string) =>
    request<{ member: Member }>(`/orgs/${orgSlug}/projects/${projectSlug}/members`, {
      method: 'POST',
      body: JSON.stringify({ email, role }),
    }),
  updateProjectRole: (orgSlug: string, projectSlug: string, userId: string, role: string) =>
    request<{ member: Member }>(`/orgs/${orgSlug}/projects/${projectSlug}/members/${userId}`, {
      method: 'PUT',
      body: JSON.stringify({ role }),
    }),
  removeFromProject: (orgSlug: string, projectSlug: string, userId: string) =>
    request<void>(`/orgs/${orgSlug}/projects/${projectSlug}/members/${userId}`, { method: 'DELETE' }),
};
```

Add `Member` to the import at the top of `api.ts` (line 1).

- [ ] **Step 3: Verify build**

Run: `cd /Users/sgamel/git/DeploySentry/web && npm run build`
Expected: Build succeeds.

- [ ] **Step 4: Commit**

```bash
git add web/src/types.ts web/src/api.ts
git commit -m "feat: update frontend Member type and API client with full CRUD"
```

---

### Task 8: Wire MembersPage to Real API

**Files:**
- Modify: `web/src/pages/MembersPage.tsx`

Replace the client-only handlers with real API calls. Expand role options to include all 4 roles.

- [ ] **Step 1: Replace MembersPage with API-wired version**

Replace the full contents of `web/src/pages/MembersPage.tsx`:

```tsx
import { useState, useEffect } from 'react';
import { useParams } from 'react-router-dom';
import type { Member } from '@/types';
import { membersApi } from '@/api';

function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' });
}

export default function MembersPage() {
  const { orgSlug } = useParams<{ orgSlug: string }>();
  const [activeTab, setActiveTab] = useState<'members' | 'groups'>('members');
  const [members, setMembers] = useState<Member[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  // Add member form
  const [newEmail, setNewEmail] = useState('');
  const [newRole, setNewRole] = useState<string>('member');

  // Delete confirm
  const [confirmDelete, setConfirmDelete] = useState<string | null>(null);

  async function fetchMembers() {
    if (!orgSlug) return;
    setLoading(true);
    setError(null);
    try {
      const result = await membersApi.listByOrg(orgSlug);
      setMembers(result.members);
    } catch (err: any) {
      setError(err.message || 'Failed to load members');
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    fetchMembers();
  }, [orgSlug]);

  async function handleAddMember() {
    if (!newEmail.trim() || !orgSlug) return;
    setActionError(null);
    try {
      const result = await membersApi.addToOrg(orgSlug, newEmail.trim(), newRole);
      setMembers((prev) => [...prev, result.member]);
      setNewEmail('');
      setNewRole('member');
    } catch (err: any) {
      setActionError(err.message || 'Failed to add member');
    }
  }

  async function handleChangeRole(userId: string, role: string) {
    if (!orgSlug) return;
    setActionError(null);
    try {
      await membersApi.updateOrgRole(orgSlug, userId, role);
      setMembers((prev) => prev.map((m) => (m.user_id === userId ? { ...m, role: role as Member['role'] } : m)));
    } catch (err: any) {
      setActionError(err.message || 'Failed to update role');
    }
  }

  async function handleRemoveMember(userId: string) {
    if (!orgSlug) return;
    setActionError(null);
    try {
      await membersApi.removeFromOrg(orgSlug, userId);
      setMembers((prev) => prev.filter((m) => m.user_id !== userId));
      setConfirmDelete(null);
    } catch (err: any) {
      setActionError(err.message || 'Failed to remove member');
      setConfirmDelete(null);
    }
  }

  return (
    <div className="page-content">
      <h2>Members &amp; Groups</h2>

      <div className="detail-tabs">
        <button
          className={`detail-tab${activeTab === 'members' ? ' active' : ''}`}
          onClick={() => setActiveTab('members')}
        >
          Members
        </button>
        <button
          className={`detail-tab${activeTab === 'groups' ? ' active' : ''}`}
          onClick={() => setActiveTab('groups')}
        >
          Groups
        </button>
      </div>

      {/* ---- Members Tab ---- */}
      {activeTab === 'members' && (
        <div>
          <div className="inline-form-row" style={{ marginBottom: 16 }}>
            <input
              type="email"
              placeholder="Email address"
              value={newEmail}
              onChange={(e) => setNewEmail(e.target.value)}
            />
            <select value={newRole} onChange={(e) => setNewRole(e.target.value)}>
              <option value="member">Member</option>
              <option value="admin">Admin</option>
              <option value="viewer">Viewer</option>
            </select>
            <button className="btn btn-primary" onClick={handleAddMember}>
              Add
            </button>
          </div>

          {error && <p className="form-error" style={{ marginBottom: 8 }}>{error}</p>}
          {actionError && <p className="form-error" style={{ marginBottom: 8 }}>{actionError}</p>}

          {loading ? (
            <p className="text-muted">Loading members...</p>
          ) : members.length === 0 ? (
            <p className="empty-state">No members yet. Add one above.</p>
          ) : (
            <table className="data-table">
              <thead>
                <tr>
                  <th>Name</th>
                  <th>Email</th>
                  <th>Org Role</th>
                  <th>Joined</th>
                  <th>Actions</th>
                </tr>
              </thead>
              <tbody>
                {members.map((m) => (
                  <tr key={m.id}>
                    <td>{m.name}</td>
                    <td>{m.email}</td>
                    <td>
                      <span className={`badge badge-${m.role}`}>{m.role}</span>
                    </td>
                    <td>{formatDate(m.joined_at)}</td>
                    <td>
                      <div style={{ display: 'flex', gap: 8, alignItems: 'center', flexWrap: 'wrap' }}>
                        {m.role === 'owner' ? (
                          <span className="text-muted">Owner</span>
                        ) : (
                          <select
                            value={m.role}
                            onChange={(e) => handleChangeRole(m.user_id, e.target.value)}
                          >
                            <option value="admin">Admin</option>
                            <option value="member">Member</option>
                            <option value="viewer">Viewer</option>
                          </select>
                        )}

                        {confirmDelete === m.user_id ? (
                          <span className="inline-confirm">
                            Are you sure?{' '}
                            <button className="btn btn-sm btn-danger" onClick={() => handleRemoveMember(m.user_id)}>
                              Yes
                            </button>{' '}
                            <button className="btn btn-sm" onClick={() => setConfirmDelete(null)}>
                              No
                            </button>
                          </span>
                        ) : (
                          <button className="btn btn-sm btn-danger" onClick={() => setConfirmDelete(m.user_id)}>
                            Remove
                          </button>
                        )}
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      )}

      {/* ---- Groups Tab ---- */}
      {activeTab === 'groups' && (
        <div>
          <p className="empty-state">Groups management coming soon.</p>
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 2: Verify build**

Run: `cd /Users/sgamel/git/DeploySentry/web && npm run build`
Expected: Build succeeds.

- [ ] **Step 3: Commit**

```bash
git add web/src/pages/MembersPage.tsx
git commit -m "feat: wire MembersPage to real API for add/remove/role changes"
```

---

### Task 9: Final Verification

- [ ] **Step 1: Verify Go builds**

Run: `cd /Users/sgamel/git/DeploySentry && go build ./cmd/api/... && go build ./internal/members/...`
Expected: Both compile cleanly.

- [ ] **Step 2: Run existing tests**

Run: `cd /Users/sgamel/git/DeploySentry && go test ./internal/... 2>&1 | tail -20`
Expected: All existing tests pass.

- [ ] **Step 3: Verify frontend build**

Run: `cd /Users/sgamel/git/DeploySentry/web && npm run build`
Expected: Build succeeds.

- [ ] **Step 4: Verify no import of removed entitiesApi.getOrg in MembersPage**

Run: `grep -n 'entitiesApi' web/src/pages/MembersPage.tsx`
Expected: No matches (MembersPage now uses orgSlug directly with membersApi).
