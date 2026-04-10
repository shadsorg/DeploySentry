# SettingsPage Backend Wiring — Design Spec

## Overview

Wire the SettingsPage's five tabs (Environments, Webhooks, Notifications, Project General, App General/Danger) to real backend APIs. Currently all tabs use mock data or local-only state. After this work, every tab persists data through the API.

## Scope

| Tab | Backend Status | Work Required |
|-----|---------------|---------------|
| Environments | Read-only endpoint exists | New CRUD endpoints (org-level + app overrides), new migration, frontend wiring |
| Webhooks | Full CRUD API exists | Frontend-only: add `webhooksApi`, hook, replace mock data |
| Notifications | Channel dispatch exists, no preferences API | New preferences HTTP handler, frontend wiring |
| Project General | `settingsApi` exists | Frontend-only: wire Save button to `settingsApi.set()` |
| App General/Danger | `settingsApi` + `entitiesApi` exist | Frontend-only: wire Save/Delete buttons |

## 1. Environments

### Data Model

Environments are defined at the **org level**. Apps inherit org environments but can add app-level overrides (e.g., a staging variant specific to one app).

**New migration — `environments` table:**

```sql
CREATE TABLE environments (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id      UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    slug        TEXT NOT NULL,
    is_production BOOLEAN NOT NULL DEFAULT false,
    sort_order  INT NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (org_id, slug)
);

CREATE INDEX idx_environments_org_id ON environments(org_id);
```

**New migration — `app_environment_overrides` table:**

```sql
CREATE TABLE app_environment_overrides (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    app_id          UUID NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    environment_id  UUID NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    config          JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (app_id, environment_id)
);

CREATE INDEX idx_app_env_overrides_app_id ON app_environment_overrides(app_id);
```

The `config` JSONB column stores app-specific overrides (e.g., custom connection strings, feature toggles per environment).

### Backend API

New routes on the entities handler:

| Method | Endpoint | Permission | Description |
|--------|----------|------------|-------------|
| GET | `/orgs/:orgSlug/environments` | PermOrgRead | List org-level environments |
| POST | `/orgs/:orgSlug/environments` | PermOrgManage | Create org-level environment |
| PUT | `/orgs/:orgSlug/environments/:envSlug` | PermOrgManage | Update org-level environment |
| DELETE | `/orgs/:orgSlug/environments/:envSlug` | PermOrgManage | Delete org-level environment |
| POST | `/orgs/:orgSlug/projects/:projectSlug/apps/:appSlug/environments` | PermProjectManage | Create app-level override |
| DELETE | `/orgs/:orgSlug/projects/:projectSlug/apps/:appSlug/environments/:envSlug` | PermProjectManage | Remove app-level override |

The existing `GET /orgs/:orgSlug/projects/:projectSlug/apps/:appSlug/environments` endpoint is updated to return org environments merged with any app-level overrides for that app. Each returned environment includes an `override` field (null if inherited, object if overridden).

**Create environment request:**
```json
{
  "name": "Staging",
  "slug": "staging",
  "is_production": false
}
```

**Create app override request:**
```json
{
  "environment_id": "<env-uuid>",
  "config": {
    "database_url": "postgres://staging-app-specific/...",
    "feature_x_enabled": true
  }
}
```

### Frontend

- Add `environmentsApi` functions to `api.ts`: `listOrgEnvironments(orgSlug)`, `createEnvironment(orgSlug, data)`, `updateEnvironment(orgSlug, envSlug, data)`, `deleteEnvironment(orgSlug, envSlug)`
- Update `useEnvironments` hook to call org-level endpoint
- Replace local state with API-backed state
- Inline add form calls `createEnvironment()` on submit
- Delete button calls `deleteEnvironment()` with confirmation
- Remove the "local to this session" warning note

## 2. Webhooks

### Backend

No changes needed. Full CRUD already exists at `/api/v1/webhooks`:

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/webhooks` | Create webhook |
| GET | `/webhooks` | List webhooks (supports project_id, is_active, events filters) |
| GET | `/webhooks/:id` | Get webhook |
| PUT | `/webhooks/:id` | Update webhook |
| DELETE | `/webhooks/:id` | Delete webhook |
| POST | `/webhooks/:id/test` | Test webhook (sends test payload) |
| GET | `/webhooks/:id/deliveries` | Get delivery history |

### Frontend

- Add `webhooksApi` to `api.ts` wrapping all 7 endpoints
- Add `useWebhooks()` hook returning `{ webhooks, loading, error, refresh }`
- Remove `MOCK_WEBHOOKS` constant from SettingsPage
- Replace static table with API-driven list
- "Add Webhook" button expands an inline form row:
  - URL text input (required)
  - Events multi-select (options fetched from backend)
  - Active toggle (default: true)
  - Save / Cancel buttons
- Each existing webhook row gets:
  - Edit button → expands inline edit form (same as add, pre-filled)
  - Delete button → confirmation dialog, then DELETE call
  - Test button → calls test endpoint, shows inline success/failure feedback
- Loading state: spinner in table body
- Error state: inline error banner with retry button

## 3. Notifications

### Backend

New notification preferences HTTP handler exposing the existing `PreferenceStore` interface.

**New file:** `internal/notifications/preferences_handler.go`

Routes registered under `/api/v1/notifications`:

| Method | Endpoint | Permission | Description |
|--------|----------|------------|-------------|
| GET | `/notifications/preferences` | PermSettingsRead | Get notification preferences for org |
| PUT | `/notifications/preferences` | PermSettingsWrite | Save notification preferences |
| DELETE | `/notifications/preferences` | PermSettingsWrite | Reset to env var defaults |

**GET response shape:**

```json
{
  "channels": {
    "slack": {
      "enabled": true,
      "webhook_url": "https://hooks.slack.com/...",
      "channel": "#deploys",
      "source": "api"
    },
    "email": {
      "enabled": false,
      "smtp_host": "smtp.example.com",
      "smtp_port": 587,
      "username": "alerts@example.com",
      "password": "",
      "from": "alerts@example.com",
      "source": "config"
    },
    "pagerduty": {
      "enabled": false,
      "routing_key": "",
      "source": "config"
    }
  },
  "event_routing": {
    "deploy.started": ["slack"],
    "deploy.completed": ["slack", "email"],
    "deploy.failed": ["slack", "email", "pagerduty"],
    "flag.toggled": ["slack"],
    "release.created": ["slack"]
  }
}
```

The `source` field on each channel indicates whether the value comes from `"config"` (environment variables) or `"api"` (user-set via the preferences endpoint). When `source` is `"config"`, the frontend shows the value as a placeholder/default. The `password` field is always returned as empty string for security; only a non-empty PUT value overwrites it.

**PUT request shape:**

```json
{
  "channels": {
    "slack": {
      "enabled": true,
      "webhook_url": "https://hooks.slack.com/new-url",
      "channel": "#alerts"
    }
  },
  "event_routing": {
    "deploy.failed": ["slack", "pagerduty"]
  }
}
```

PUT is a merge operation — only provided fields are updated. Omitted channels/events retain their current values.

**DELETE** resets all API-set preferences, reverting to environment variable defaults.

### Frontend

- Add `notificationsApi` to `api.ts`: `getPreferences()`, `savePreferences(data)`, `resetPreferences()`
- Load preferences on mount via `getPreferences()`
- Pre-fill channel config forms from response; show `source: "config"` values as placeholder text
- Event routing checkboxes reflect `event_routing` map
- "Save Notification Settings" button calls `savePreferences()` with current form state
- Add "Reset to Defaults" button that calls `resetPreferences()` with confirmation
- Show save success/error feedback inline

## 4. Project General Settings

### Backend

No changes. Uses existing `settingsApi`:
- `GET /settings?scope=project&target={projectId}` — load current values
- `PUT /settings` — save each setting (key: `project.name`, `project.default_environment`, `project.stale_threshold`)

### Frontend

- On mount: call `settingsApi.list("project", projectId)` and populate form fields
- "Save" button: call `settingsApi.set()` for each changed field
- Show save success/error feedback inline

## 5. App General & Danger Zone

### Backend

No changes. Uses existing `settingsApi` and `entitiesApi`.

### Frontend

**General tab:**
- On mount: call `entitiesApi.getApp()` to populate name, description, repo URL
- "Save" button: call `entitiesApi.updateApp()` with changed fields

**Danger zone:**
- "Delete Application" button: confirmation dialog (type app name to confirm), then call `entitiesApi.deleteApp()`, redirect to project page on success

## Error Handling Pattern

All tabs follow the same pattern:
- **Loading**: Spinner in content area
- **Error on fetch**: Inline error banner with "Retry" button
- **Error on save**: Inline error message below the save button, form remains editable
- **Success on save**: Brief inline success message (auto-dismiss after 3s)
- **Optimistic deletes**: Remove item from UI immediately, rollback + error message on failure

## File Map

| Action | File | Responsibility |
|--------|------|----------------|
| Create | `migrations/035_create_environments.up.sql` | Environments + overrides tables |
| Create | `migrations/035_create_environments.down.sql` | Rollback |
| Modify | `internal/entities/handler.go` | Add environment CRUD routes |
| Modify | `internal/entities/service.go` | Add environment service methods |
| Create | `internal/entities/environment_repository.go` | Environment DB queries |
| Create | `internal/notifications/preferences_handler.go` | Notification preferences HTTP handler |
| Modify | `internal/notifications/service.go` | Wire preferences into channel config |
| Modify | `cmd/api/main.go` | Register notification preferences routes |
| Modify | `web/src/api.ts` | Add webhooksApi, notificationsApi, environmentsApi |
| Modify | `web/src/hooks/useEntities.ts` | Update useEnvironments hook |
| Create | `web/src/hooks/useWebhooks.ts` | Webhook data hook |
| Create | `web/src/hooks/useNotifications.ts` | Notification preferences hook |
| Modify | `web/src/pages/SettingsPage.tsx` | Wire all 5 tabs to real APIs |

## Out of Scope

- Webhook delivery history UI (backend exists, frontend deferred)
- Environment-scoped settings resolution UI
- Notification channel health/status monitoring
- App-level environment override management UI (backend endpoints included, frontend deferred to a future pass)
