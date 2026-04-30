# SDK Application Parameter

**Phase**: Design

## Overview

The SDK client options take `project` and `environment` but have no concept of `application`. Flags can be scoped to applications in the backend (`application_id` on `feature_flags`), but the SDK cannot request or evaluate app-scoped flags. This spec adds a required `application` parameter to both SDKs and updates the backend endpoints to filter by application.

## Requirements

### SDK Client Options
- Add `application: string` (required) to `ClientOptions` in the Node SDK and `ProviderProps` in the React SDK.
- The value is the application slug (human-readable string), consistent with how `project` and `environment` are specified.
- The client sends the application slug on all API requests.

### Flag Return Behavior (Union)
When an application is specified:
- Project-level flags (`application_id IS NULL`) are always included.
- App-specific flags (`application_id` matches the resolved application) are included.
- Flags scoped to other applications are excluded.
- Flag keys must be unique within the union — no merge/override logic.

### Backend Endpoint Changes

**`GET /api/v1/flags`** (listFlags):
- Accept optional `application` query parameter (slug string).
- When present: resolve slug to UUID within the project, return union of project-level + app-specific flags.
- When absent: return all flags for the project (current behavior, backward compatible).
- Return 404 if the application slug doesn't resolve.

**`POST /api/v1/flags/evaluate`** (evaluateFlag):
- Accept optional `application` field in request body (slug string).
- When present: resolve slug, verify the requested flag belongs to the union (project-level or matching app). Return 404 if the flag exists but is scoped to a different application.
- When absent: current behavior (project-level only).

**`POST /api/v1/flags/batch-evaluate`** (batchEvaluateFlags):
- Same as evaluate — accept optional `application` field, filter to the union set.

**`GET /api/v1/flags/stream`** (SSE):
- Accept optional `application` query parameter (slug string).
- When present: only emit change events for flags in the union set.
- When absent: emit events for all project flags (current behavior).

### Application Resolution
- The server resolves the application slug within the project context: look up by `(project_id, slug)` in the `applications` table.
- Return `404 Not Found` with `{"error": "application not found"}` if the slug doesn't match.
- Resolution happens once per request (listFlags, evaluate) or once on SSE connection open.

## SDK Changes

### Node SDK (`sdk/node/`)

**`src/types.ts`** — `ClientOptions`:
```typescript
export interface ClientOptions {
  apiKey: string;
  baseURL?: string;
  environment: string;
  project: string;
  application: string;        // NEW — required
  cacheTimeout?: number;
  offlineMode?: boolean;
  sessionId?: string;
}
```

**`src/client.ts`** — `DeploySentryClient`:
- Store `application` from options.
- Validation: throw if `application` is empty.
- `fetchAllFlags()`: append `&application=<slug>` to the query string.
- `evaluate()` / `detail()`: add `application: this.application` to the POST body.
- SSE URL: append `&application=<slug>` to the stream query string.

### React SDK (`sdk/react/`)

**`src/types.ts`** — `ProviderProps`:
```typescript
export interface ProviderProps {
  apiKey: string;
  baseURL: string;
  environment: string;
  project: string;
  application: string;        // NEW — required
  user?: UserContext;
  sessionId?: string;
  children: React.ReactNode;
}
```

**`src/client.ts`** — `DeploySentryClient`:
- Store `application` from constructor options.
- `fetchFlags()`: append `&application=<slug>` to the query URL.
- `buildQueryParams()`: add `application` to the params.
- SSE URL: already built from `buildQueryParams()`, so it picks up automatically.

**`src/provider.tsx`** — `DeploySentryProvider`:
- Pass `application` through to the client constructor.
- Include `application` in the `configKey` memo (recreate client if application changes).

## Backend Changes

### Flag Handler (`internal/flags/handler.go`)

**`listFlags`:**
- Read optional `application` query param.
- When present: resolve to `application_id` via entity service/repo lookup. Query flags with `WHERE (application_id = $resolved OR application_id IS NULL) AND project_id = $project`.
- When absent: current query unchanged.

**`evaluateFlag` / `batchEvaluateFlags`:**
- Add optional `Application string` field to `evaluateRequest` and `batchEvaluateRequest`.
- When present: resolve slug, add application filter to the flag lookup.

**`streamFlags` (SSE):**
- Read optional `application` query param.
- Filter SSE events to the union set.

### Flag Repository (`internal/platform/database/postgres/flags.go`)

**Update `ListFlags`:**
- Add optional `applicationID *uuid.UUID` parameter (or use a filter struct).
- When non-nil: `WHERE project_id = $1 AND (application_id = $2 OR application_id IS NULL)`.
- When nil: current behavior.

**Update `GetFlagByKey`:**
- Add optional `applicationID *uuid.UUID` parameter.
- When non-nil: add `AND (application_id = $1 OR application_id IS NULL)` to the lookup. This prevents evaluating a flag scoped to a different application.

### Entity Repository

No changes — `GetAppBySlug(ctx, projectID, slug)` already exists and is sufficient for resolution.

## Documentation Updates

### `sdk/node/README.md`
- Add `application` to the Configuration table (required).
- Update all code examples to include `application`.

### `sdk/react/README.md`
- Add `application` to the Provider Props table (required).
- Update all code examples to include `application`.

### `sdk/INTEGRATION.md`
- Update the integration checklist code examples.
- Update the CLAUDE.md directive block.
- Update the environment variables table (add `DEPLOYSENTRY_APPLICATION`).

## Out of Scope
- CLI changes.
- Allowing per-evaluation application override (application is fixed per client instance).
- Multi-application clients (one client = one application).
- Changes to flag creation (application_id remains optional when creating flags).

## Checklist
- [ ] Node SDK: add `application` to `ClientOptions`
- [ ] Node SDK: validate `application` in constructor
- [ ] Node SDK: send `application` in `fetchAllFlags`, `evaluate`, `detail`, SSE URL
- [ ] Node SDK: update README examples and config table
- [ ] React SDK: add `application` to `ProviderProps`
- [ ] React SDK: pass `application` through provider → client
- [ ] React SDK: send `application` in `fetchFlags`, `buildQueryParams`
- [ ] React SDK: update README examples and props table
- [ ] Backend: update `listFlags` handler to accept and resolve `application` param
- [ ] Backend: update `evaluateFlag` / `batchEvaluateFlags` to accept `application`
- [ ] Backend: update SSE stream handler to filter by application
- [ ] Backend: update flag repository queries with application filter
- [ ] Backend: tests for application-filtered flag queries
- [ ] Backend: tests for evaluate with application context
- [ ] Update `sdk/INTEGRATION.md` examples and CLAUDE.md block
- [ ] Node SDK contract tests
- [ ] React SDK tests

## Completion Record
<!-- Fill in when phase is set to Complete -->
- **Branch**: ``
- **Committed**: No
- **Pushed**: No
- **CI Checks**:
