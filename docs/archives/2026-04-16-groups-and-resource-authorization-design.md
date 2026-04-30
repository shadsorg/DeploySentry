# Groups & Resource Authorization

**Date**: 2026-04-16
**Status**: Approved

## Overview

Add org-level groups, assign users to groups, and introduce a resource authorization system that controls who can see and modify projects and applications. Replaces the existing `project_members` system with a unified `resource_grants` model.

## Goals

1. Org admins can create groups and assign org members to them
2. Projects and apps can have read/write authorization grants for users or groups
3. If no grants exist on a resource, it is open to all org members (current behavior preserved)
4. If any grant exists, only granted users/groups (and org owners) can access the resource
5. Users without read access cannot see the resource in any listing or navigate to it directly
6. Project-level grants cascade to all apps within that project unless the app has its own grants

## Data Model

### New Tables

#### `groups`

| Column       | Type        | Constraints                                      |
|-------------|-------------|--------------------------------------------------|
| `id`         | UUID        | PK, default `gen_random_uuid()`                  |
| `org_id`     | UUID        | NOT NULL, FK → `organizations(id)` ON DELETE CASCADE |
| `name`       | TEXT        | NOT NULL                                         |
| `slug`       | TEXT        | NOT NULL                                         |
| `description`| TEXT        | default `''`                                     |
| `created_by` | UUID        | FK → `users(id)`                                 |
| `created_at` | TIMESTAMPTZ | NOT NULL, default `now()`                        |
| `updated_at` | TIMESTAMPTZ | NOT NULL, default `now()`                        |

- Unique constraint on `(org_id, slug)`

#### `group_members`

| Column       | Type        | Constraints                                      |
|-------------|-------------|--------------------------------------------------|
| `group_id`   | UUID        | NOT NULL, FK → `groups(id)` ON DELETE CASCADE    |
| `user_id`    | UUID        | NOT NULL, FK → `users(id)` ON DELETE CASCADE     |
| `created_at` | TIMESTAMPTZ | NOT NULL, default `now()`                        |

- PK on `(group_id, user_id)`

#### `resource_grants`

| Column           | Type        | Constraints                                      |
|-----------------|-------------|--------------------------------------------------|
| `id`             | UUID        | PK, default `gen_random_uuid()`                  |
| `org_id`         | UUID        | NOT NULL, FK → `organizations(id)` ON DELETE CASCADE |
| `project_id`     | UUID        | FK → `projects(id)` ON DELETE CASCADE, nullable  |
| `application_id` | UUID        | FK → `applications(id)` ON DELETE CASCADE, nullable |
| `user_id`        | UUID        | FK → `users(id)` ON DELETE CASCADE, nullable     |
| `group_id`       | UUID        | FK → `groups(id)` ON DELETE CASCADE, nullable    |
| `permission`     | TEXT        | NOT NULL, CHECK (`permission` IN ('read', 'write')) |
| `granted_by`     | UUID        | FK → `users(id)`                                 |
| `created_at`     | TIMESTAMPTZ | NOT NULL, default `now()`                        |

- CHECK: exactly one of `project_id` or `application_id` is NOT NULL
- CHECK: exactly one of `user_id` or `group_id` is NOT NULL
- Unique constraint on `(project_id, user_id)` WHERE both NOT NULL
- Unique constraint on `(project_id, group_id)` WHERE both NOT NULL
- Unique constraint on `(application_id, user_id)` WHERE both NOT NULL
- Unique constraint on `(application_id, group_id)` WHERE both NOT NULL

### Dropped Table

#### `project_members`

Replaced by `resource_grants`. Existing rows migrated:
- `role = 'admin'` or `role = 'developer'` → `permission = 'write'`
- `role = 'viewer'` → `permission = 'read'`

## Authorization Logic

### Access Resolution

For a given user attempting to access a resource (project or app):

```
1. If user's org role is 'owner' → ALLOW (full access, always)
2. Determine effective resource:
   - If accessing an app: check if app has any grants
     - If yes → use app grants
     - If no → use parent project grants (cascade)
   - If accessing a project: use project grants
3. If the effective resource has zero grants → ALLOW (resource is open)
4. Check for direct user grant on the effective resource → use that permission
5. Check all groups the user belongs to for grants on the effective resource → use highest permission (write > read)
6. If no grant found → DENY (resource is hidden)
```

### Permission Semantics

- **`read`**: Can see the resource in listings, view its contents (flags, deployments, releases, settings). Combined with org role to determine specific read actions.
- **`write`**: Implies `read`. Can modify the resource contents. Combined with org role to determine specific write actions (e.g., an org `viewer` with `write` grant still cannot perform `org:manage` actions).
- **Org role caps actions**: The resource grant controls access to the resource. The org role controls what actions are available within that access. A `write` grant does not elevate an org `viewer` to admin capabilities.

### Cascade Rules

- Project grants cascade to all apps in that project **unless** the app has its own grants
- An app with zero grants inherits from its project
- An app with one or more grants uses only its own grants (project grants do not merge in)
- This means adding the first grant to an app severs the inheritance from the project for that app

## API Endpoints

### Groups

| Method   | Path                                          | Description              | Permission Required |
|----------|-----------------------------------------------|--------------------------|---------------------|
| `GET`    | `/api/v1/orgs/:orgSlug/groups`                | List groups              | `org:manage` or org member |
| `POST`   | `/api/v1/orgs/:orgSlug/groups`                | Create group             | `org:manage`        |
| `GET`    | `/api/v1/orgs/:orgSlug/groups/:groupSlug`     | Get group detail         | `org:manage` or org member |
| `PUT`    | `/api/v1/orgs/:orgSlug/groups/:groupSlug`     | Update group             | `org:manage`        |
| `DELETE` | `/api/v1/orgs/:orgSlug/groups/:groupSlug`     | Delete group             | `org:manage`        |
| `GET`    | `/api/v1/orgs/:orgSlug/groups/:groupSlug/members` | List group members   | `org:manage` or org member |
| `POST`   | `/api/v1/orgs/:orgSlug/groups/:groupSlug/members` | Add member to group  | `org:manage`        |
| `DELETE` | `/api/v1/orgs/:orgSlug/groups/:groupSlug/members/:userId` | Remove member | `org:manage`       |

### Resource Grants

| Method   | Path                                          | Description                      | Permission Required |
|----------|-----------------------------------------------|----------------------------------|---------------------|
| `GET`    | `/api/v1/orgs/:orgSlug/projects/:projectSlug/grants` | List project grants       | `org:manage` (owner/admin) |
| `POST`   | `/api/v1/orgs/:orgSlug/projects/:projectSlug/grants` | Add grant to project      | `org:manage` (owner/admin) |
| `DELETE` | `/api/v1/orgs/:orgSlug/projects/:projectSlug/grants/:grantId` | Remove grant       | `org:manage` (owner/admin) |
| `GET`    | `/api/v1/orgs/:orgSlug/projects/:projectSlug/apps/:appSlug/grants` | List app grants | `org:manage` (owner/admin) |
| `POST`   | `/api/v1/orgs/:orgSlug/projects/:projectSlug/apps/:appSlug/grants` | Add grant to app  | `org:manage` (owner/admin) |
| `DELETE` | `/api/v1/orgs/:orgSlug/projects/:projectSlug/apps/:appSlug/grants/:grantId` | Remove grant | `org:manage` (owner/admin) |

### Grant Request Body

```json
{
  "user_id": "uuid-or-null",
  "group_id": "uuid-or-null",
  "permission": "read | write"
}
```

Exactly one of `user_id` or `group_id` must be provided.

## Visibility Filtering

### Project Listing

The project list query (`GET /api/v1/orgs/:orgSlug/projects`) must filter results:

```sql
SELECT p.* FROM projects p
WHERE p.org_id = $1
  AND p.deleted_at IS NULL
  AND (
    -- org owner sees everything
    $2 = 'owner'
    -- project has no grants (open)
    OR NOT EXISTS (
      SELECT 1 FROM resource_grants rg WHERE rg.project_id = p.id
    )
    -- user has direct grant
    OR EXISTS (
      SELECT 1 FROM resource_grants rg
      WHERE rg.project_id = p.id AND rg.user_id = $3
    )
    -- user is in a group with a grant
    OR EXISTS (
      SELECT 1 FROM resource_grants rg
      JOIN group_members gm ON gm.group_id = rg.group_id
      WHERE rg.project_id = p.id AND gm.user_id = $3
    )
  )
```

### Application Listing

Similar pattern, but with cascade logic: if the app has no grants, check the parent project grants instead.

```sql
SELECT a.* FROM applications a
WHERE a.project_id = $1
  AND a.deleted_at IS NULL
  AND (
    $2 = 'owner'
    -- app has no grants AND project has no grants (fully open)
    OR (
      NOT EXISTS (SELECT 1 FROM resource_grants rg WHERE rg.application_id = a.id)
      AND NOT EXISTS (SELECT 1 FROM resource_grants rg WHERE rg.project_id = a.project_id)
    )
    -- app has no grants, inherit project grant
    OR (
      NOT EXISTS (SELECT 1 FROM resource_grants rg WHERE rg.application_id = a.id)
      AND (
        EXISTS (
          SELECT 1 FROM resource_grants rg
          WHERE rg.project_id = a.project_id AND rg.user_id = $3
        )
        OR EXISTS (
          SELECT 1 FROM resource_grants rg
          JOIN group_members gm ON gm.group_id = rg.group_id
          WHERE rg.project_id = a.project_id AND gm.user_id = $3
        )
      )
    )
    -- app has its own grants, check those
    OR EXISTS (
      SELECT 1 FROM resource_grants rg
      WHERE rg.application_id = a.id AND rg.user_id = $3
    )
    OR EXISTS (
      SELECT 1 FROM resource_grants rg
      JOIN group_members gm ON gm.group_id = rg.group_id
      WHERE rg.application_id = a.id AND gm.user_id = $3
    )
  )
```

### Direct Access Protection

Middleware must also enforce authorization on direct access (e.g., `GET /projects/:slug`), not just listings. A user without access gets 404 (not 403) to avoid leaking resource existence.

## Middleware Changes

### New: `RequireResourceAccess`

Replaces `RequireProjectPermission`. Sits in the middleware chain after `ResolveOrgRole`.

```go
func RequireResourceAccess(resolver GrantResolver, requiredPerm string) gin.HandlerFunc
```

- `requiredPerm` is `"read"` or `"write"`
- Resolves the user's effective permission on the resource using the access resolution logic above
- Sets `resource_permission` on context (`"read"`, `"write"`, or `"full"` for owners)
- Returns 404 if denied

### Remove: `RequireProjectPermission`

No longer needed — `RequireResourceAccess` handles both project and app access.

### Existing `RequirePermission` stays

Still used for action-level checks (e.g., `flag:create`, `deploy:promote`). These check the org role, not the resource grant. The two layers compose:

1. `RequireResourceAccess("write")` — can the user access this resource with write?
2. `RequirePermission(PermFlagCreate)` — does the user's org role allow flag creation?

Both must pass.

## UI Changes

### Members Page — Groups Tab

Replace the "coming soon" placeholder with:

- **Group list**: Table with name, description, member count, actions (edit, delete)
- **Create group**: Button opens form with name, slug (auto-generated from name), description
- **Group detail**: Click a group row to see/manage its members
  - List of members with name, email, "Remove" button
  - "Add member" dropdown that searches existing org members
- **Delete group**: Confirmation dialog warning that this removes all grants associated with the group

### Project Settings — Authorization Tab

New tab in project settings:

- **Empty state**: "This project is open to all organization members. Add a user or group to restrict access."
- **Grant table**: Columns: Name (user or group), Type (User/Group), Permission (Read/Write), Actions (Remove)
- **Add grant**: Button with form:
  - Searchable dropdown for user or group (searches org members and groups)
  - Permission select: Read / Write
- **Remove grant**: Inline button with confirmation
- **Warning banner** when grants exist: "Access to this project is restricted. Only users and groups listed below (and org owners) can access it."

### App Settings — Authorization Tab

Same pattern as project, with additional context:

- **Empty state**: "This app inherits access from its project. Add a user or group to override with app-specific access."
- **Inheritance indicator**: When app has no grants, show "Inheriting from project" with a link to the project authorization tab
- **Warning when adding first grant**: "Adding a grant here will override the project-level access for this app. Only the users and groups listed here (and org owners) will have access."

## Migration Plan

### Migration: `041_create_groups_and_grants.up.sql`

1. Create `groups` table
2. Create `group_members` table
3. Create `resource_grants` table with all constraints and unique indexes
4. Migrate `project_members` data into `resource_grants`:
   ```sql
   INSERT INTO resource_grants (org_id, project_id, user_id, permission, created_at)
   SELECT p.org_id, pm.project_id, pm.user_id,
     CASE WHEN pm.role IN ('admin', 'developer') THEN 'write' ELSE 'read' END,
     pm.created_at
   FROM project_members pm
   JOIN projects p ON p.id = pm.project_id;
   ```
5. Drop `project_members` table

### Migration: `041_create_groups_and_grants.down.sql`

1. Recreate `project_members` table
2. Migrate `resource_grants` data back (project-level user grants only)
3. Drop `resource_grants`, `group_members`, `groups`

## Backend Package Structure

### New: `internal/groups/`

- `repository.go` — CRUD for groups and group_members
- `service.go` — business logic (slug generation, validation, duplicate checks)
- `handler.go` — HTTP handlers, route registration

### New: `internal/grants/`

- `repository.go` — CRUD for resource_grants, visibility queries
- `service.go` — access resolution logic, cascade evaluation
- `handler.go` — HTTP handlers for grant management
- `middleware.go` — `RequireResourceAccess` middleware

### Modified: `internal/entities/`

- Repository: update project/app listing queries to accept user ID and org role for visibility filtering
- Handler: wire `RequireResourceAccess` middleware into project and app routes

### Modified: `internal/auth/`

- Remove `RequireProjectPermission` middleware
- Update `rbac.go` to add `PermGroupManage` permission constant
- Add `group:manage` to appropriate roles (owner, admin)

### Removed: `internal/members/`

- Remove project member handlers and routes (replaced by grants)
- Keep org member handlers and routes

## Testing

- Unit tests for access resolution logic (all branches: owner bypass, open resource, direct grant, group grant, cascade, deny)
- Unit tests for group CRUD
- Unit tests for grant CRUD with constraint validation
- Integration tests for visibility filtering SQL queries
- Integration tests for middleware chain (resource access + permission check composition)

## Out of Scope

- Nested groups
- Group-level roles (groups are just membership containers)
- Audit log for grant changes (can be added later)
- Bulk grant operations
- API key authorization against resource grants (API keys already have scoped access)
