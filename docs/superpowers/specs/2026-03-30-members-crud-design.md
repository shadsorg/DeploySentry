# Members CRUD Backend Design

**Date:** 2026-03-30
**Scope:** Org-level and project-level member management

## Summary

Add a dedicated `internal/members/` package with repository, service, and handler layers for managing org and project memberships. Wire existing frontend `MembersPage` to persist changes via the new API endpoints.

## Decisions

- **Approach:** Separate `internal/members/` package (not added to entities)
- **Scope:** Both org-level and project-level members
- **Add flow:** Direct-add only (user must exist); invitation flow deferred
- **Permissions:** Owner and Admin can add/remove/change roles
- **No new pages:** Existing `MembersPage` gets wired to real API

## Data Model & Migration

The `org_members` and `project_members` tables exist but need schema updates.

**Migration for `org_members`:**
- Add `id UUID DEFAULT gen_random_uuid()`
- Add `invited_by UUID REFERENCES users(id)` (for future invitation flow)
- Add `created_at TIMESTAMPTZ NOT NULL DEFAULT now()`
- Add `updated_at TIMESTAMPTZ NOT NULL DEFAULT now()`
- Update role CHECK constraint to include `'viewer'`: `CHECK (role IN ('owner', 'admin', 'member', 'viewer'))`
- Keep composite unique constraint on `(org_id, user_id)`

**Migration for `project_members`:**
- Add `id UUID DEFAULT gen_random_uuid()`
- Add `created_at TIMESTAMPTZ NOT NULL DEFAULT now()`
- Add `updated_at TIMESTAMPTZ NOT NULL DEFAULT now()`
- Keep existing roles: `admin`, `editor`, `viewer`, `deployer`
- Keep composite unique constraint on `(project_id, user_id)`

**Go models** — existing `OrgMember` and `ProjectMember` in `internal/models/` already have these fields.

## Repository Layer

**File:** `internal/members/repository.go` (interface), `internal/platform/database/postgres/members.go` (implementation)

```go
type Repository interface {
    // Org members
    ListOrgMembers(ctx context.Context, orgID uuid.UUID) ([]OrgMemberRow, error)
    GetOrgMember(ctx context.Context, orgID, userID uuid.UUID) (*OrgMemberRow, error)
    AddOrgMember(ctx context.Context, m *models.OrgMember) error
    UpdateOrgMemberRole(ctx context.Context, orgID, userID uuid.UUID, role models.OrgRole) error
    RemoveOrgMember(ctx context.Context, orgID, userID uuid.UUID) error

    // Project members
    ListProjectMembers(ctx context.Context, projectID uuid.UUID) ([]ProjectMemberRow, error)
    GetProjectMember(ctx context.Context, projectID, userID uuid.UUID) (*ProjectMemberRow, error)
    AddProjectMember(ctx context.Context, m *models.ProjectMember) error
    UpdateProjectMemberRole(ctx context.Context, projectID, userID uuid.UUID, role models.ProjectRole) error
    RemoveProjectMember(ctx context.Context, projectID, userID uuid.UUID) error
}
```

**Row types** — `OrgMemberRow` and `ProjectMemberRow` join with `users` table to include `name`, `email`, `avatar_url`. Avoids N+1 queries.

**Key queries:**
- List: `SELECT om.*, u.name, u.email, u.avatar_url FROM org_members om JOIN users u ON om.user_id = u.id WHERE om.org_id = $1 ORDER BY om.joined_at`
- Add: `INSERT INTO org_members (id, org_id, user_id, role, invited_by, joined_at, created_at, updated_at) VALUES (...)`
- Update: `UPDATE org_members SET role = $1, updated_at = now() WHERE org_id = $2 AND user_id = $3`
- Remove: `DELETE FROM org_members WHERE org_id = $1 AND user_id = $2`

Same pattern for project members with `project_id` substituted.

## Service Layer

**File:** `internal/members/service.go`

```go
type Service interface {
    // Org members
    ListOrgMembers(ctx context.Context, orgID uuid.UUID) ([]OrgMemberRow, error)
    AddOrgMember(ctx context.Context, orgID uuid.UUID, email string, role models.OrgRole, addedBy uuid.UUID) error
    UpdateOrgMemberRole(ctx context.Context, orgID, userID uuid.UUID, role models.OrgRole) error
    RemoveOrgMember(ctx context.Context, orgID, userID uuid.UUID) error

    // Project members
    ListProjectMembers(ctx context.Context, projectID uuid.UUID) ([]ProjectMemberRow, error)
    AddProjectMember(ctx context.Context, projectID uuid.UUID, email string, role models.ProjectRole, addedBy uuid.UUID) error
    UpdateProjectMemberRole(ctx context.Context, projectID, userID uuid.UUID, role models.ProjectRole) error
    RemoveProjectMember(ctx context.Context, projectID, userID uuid.UUID) error
}
```

**Dependencies:**
- `Repository` for persistence
- `UserLookup` interface for email → user resolution: `GetUserByEmail(ctx, email) (*models.User, error)`

**Business rules:**
- `AddOrgMember` resolves email to user ID; returns error if user not found
- Role validation before DB operations
- Cannot remove the last owner of an org
- Cannot demote the last owner
- Cannot assign `owner` role via update endpoint (ownership transfer is separate)
- Sets `id`, `joined_at`, `created_at`, `updated_at` on add

## Handler Layer & Routes

**File:** `internal/members/handler.go`

**Dependencies:** `Service`, `entities.Service` (for slug → ID resolution), `auth.RBACChecker`

### Org member routes

| Method | Route | Permission | Description |
|--------|-------|------------|-------------|
| GET | `/orgs/:orgSlug/members` | `PermOrgManage` | List org members |
| POST | `/orgs/:orgSlug/members` | `PermOrgManage` | Add member by email |
| PUT | `/orgs/:orgSlug/members/:userId` | `PermOrgManage` | Update member role |
| DELETE | `/orgs/:orgSlug/members/:userId` | `PermOrgManage` | Remove member |

### Project member routes

| Method | Route | Permission | Description |
|--------|-------|------------|-------------|
| GET | `/orgs/:orgSlug/projects/:projectSlug/members` | `PermProjectManage` | List project members |
| POST | `/orgs/:orgSlug/projects/:projectSlug/members` | `PermProjectManage` | Add member by email |
| PUT | `/orgs/:orgSlug/projects/:projectSlug/members/:userId` | `PermProjectManage` | Update member role |
| DELETE | `/orgs/:orgSlug/projects/:projectSlug/members/:userId` | `PermProjectManage` | Remove member |

### Request/response shapes

```
POST body:    { "email": "user@example.com", "role": "admin" }
PUT body:     { "role": "member" }
GET response: { "members": [{ "id", "user_id", "name", "email", "avatar_url", "role", "joined_at" }] }
```

### Error responses

- User not found by email → 404
- Already a member → 409 Conflict
- Invalid role → 422
- Last owner removal → 422 with message

## Frontend Updates

### `web/src/api.ts`

Expand `membersApi` with full CRUD for both org and project members:

```typescript
export const membersApi = {
  // Org members
  listByOrg: (orgId: string) => request<{ members: Member[] }>(`/orgs/${orgId}/members`),
  addToOrg: (orgId: string, email: string, role: string) =>
    request<{ member: Member }>(`/orgs/${orgId}/members`, { method: 'POST', body: { email, role } }),
  updateOrgRole: (orgId: string, userId: string, role: string) =>
    request<{ member: Member }>(`/orgs/${orgId}/members/${userId}`, { method: 'PUT', body: { role } }),
  removeFromOrg: (orgId: string, userId: string) =>
    request<void>(`/orgs/${orgId}/members/${userId}`, { method: 'DELETE' }),

  // Project members
  listByProject: (projectId: string) =>
    request<{ members: Member[] }>(`/projects/${projectId}/members`),
  addToProject: (projectId: string, email: string, role: string) =>
    request<{ member: Member }>(`/projects/${projectId}/members`, { method: 'POST', body: { email, role } }),
  updateProjectRole: (projectId: string, userId: string, role: string) =>
    request<{ member: Member }>(`/projects/${projectId}/members/${userId}`, { method: 'PUT', body: { role } }),
  removeFromProject: (projectId: string, userId: string) =>
    request<void>(`/projects/${projectId}/members/${userId}`, { method: 'DELETE' }),
};
```

### `web/src/types.ts`

Update `Member` interface to match backend response:

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

### `web/src/pages/MembersPage.tsx`

Wire existing UI actions (add, remove, role change) to the new API endpoints instead of local state. Add error handling for API failures.

## Wiring

In `cmd/api/main.go`:
- Create `memberRepo := postgres.NewMemberRepository(db.Pool)`
- Create `memberService := members.NewService(memberRepo, userLookup)`
- Register `members.NewHandler(memberService, entityService, rbacChecker).RegisterRoutes(api)`
