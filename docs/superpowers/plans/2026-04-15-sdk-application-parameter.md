# SDK Application Parameter Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a required `application` parameter to both Node and React SDKs, and update backend endpoints to filter flags by application (union of project-level + app-specific flags).

**Architecture:** The SDKs add `application` (slug string) to client options and send it on all API requests. The backend resolves the slug to a UUID via the existing `GetAppBySlug` query, then filters flag queries to return the union set. The flag model already has an optional `application_id` field — no schema changes needed.

**Tech Stack:** TypeScript (Node SDK, React SDK), Go (backend handlers, repository), PostgreSQL

**Spec:** `docs/superpowers/specs/2026-04-15-sdk-application-parameter-design.md`

---

### Task 1: Add `ApplicationID` filter to flag repository `ListFlags`

**Files:**
- Modify: `internal/flags/repository.go:79-86`
- Modify: `internal/platform/database/postgres/flags.go:236-279`

- [ ] **Step 1: Add `ApplicationID` to `ListOptions`**

In `internal/flags/repository.go`, add the field to the `ListOptions` struct:

```go
type ListOptions struct {
	Limit         int        `json:"limit"`
	Offset        int        `json:"offset"`
	EnvironmentID *uuid.UUID `json:"environment_id,omitempty"`
	ApplicationID *uuid.UUID `json:"application_id,omitempty"`
	Archived      *bool      `json:"archived,omitempty"`
	Tag           string     `json:"tag,omitempty"`
}
```

- [ ] **Step 2: Update `ListFlags` query to filter by application**

In `internal/platform/database/postgres/flags.go`, in the `ListFlags` method, add the application filter after the existing `EnvironmentID` filter (around line 243):

```go
if opts.ApplicationID != nil {
	w.Add("(application_id = $%d OR application_id IS NULL)", *opts.ApplicationID)
}
```

This returns the union: flags scoped to the specified application + project-level flags (no application).

- [ ] **Step 3: Verify build compiles**

Run: `cd /Users/sgamel/git/DeploySentry && go build ./...`
Expected: No errors

- [ ] **Step 4: Commit**

```bash
git add internal/flags/repository.go internal/platform/database/postgres/flags.go
git commit -m "feat: add ApplicationID filter to ListFlags query"
```

---

### Task 2: Update `listFlags` handler to accept `application` query param

**Files:**
- Modify: `internal/flags/handler.go:224-270`
- Modify: `internal/entities/service.go` (use existing `GetAppBySlug`)

- [ ] **Step 1: Update the `listFlags` handler to read and resolve `application` param**

In `internal/flags/handler.go`, update the `listFlags` method. After the `projectID` parsing (around line 236), add application resolution:

```go
opts := ListOptions{Limit: 20, Offset: 0}

if appSlug := c.Query("application"); appSlug != "" {
	app, err := h.entityRepo.GetAppBySlug(c.Request.Context(), projectID, appSlug)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve application"})
		return
	}
	if app == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "application not found"})
		return
	}
	opts.ApplicationID = &app.ID
}
```

- [ ] **Step 2: Add `entityRepo` dependency to the flags Handler**

The flags handler needs access to the entity repository to resolve application slugs. Check the flags handler struct in `internal/flags/handler.go` (around line 16). Add an `entityRepo` field:

```go
type Handler struct {
	service      Service
	analyticsSvc AnalyticsService
	ratingSvc    RatingService
	sse          *SSEBroker
	entityRepo   EntityAppResolver
}
```

Add the interface near the top of the file:

```go
// EntityAppResolver resolves application slugs to models. Used by listFlags and evaluate.
type EntityAppResolver interface {
	GetAppBySlug(ctx context.Context, projectID uuid.UUID, slug string) (*models.Application, error)
}
```

Update `NewHandler` to accept the resolver and wire it up. Update the call site in `cmd/api/main.go` to pass the entity repository.

- [ ] **Step 3: Verify build compiles**

Run: `cd /Users/sgamel/git/DeploySentry && go build ./...`
Expected: No errors

- [ ] **Step 4: Commit**

```bash
git add internal/flags/handler.go cmd/api/main.go
git commit -m "feat: listFlags handler accepts application query param"
```

---

### Task 3: Update `evaluate` and `batchEvaluate` to accept `application` field

**Files:**
- Modify: `internal/flags/handler.go:511-614`

- [ ] **Step 1: Add `Application` field to `evaluateRequest` and `batchEvaluateRequest`**

```go
type evaluateRequest struct {
	ProjectID     uuid.UUID                `json:"project_id" binding:"required"`
	EnvironmentID uuid.UUID                `json:"environment_id" binding:"required"`
	Application   string                   `json:"application"`
	FlagKey       string                   `json:"flag_key" binding:"required"`
	Context       models.EvaluationContext `json:"context"`
}

type batchEvaluateRequest struct {
	ProjectID     uuid.UUID                `json:"project_id" binding:"required"`
	EnvironmentID uuid.UUID                `json:"environment_id" binding:"required"`
	Application   string                   `json:"application"`
	FlagKeys      []string                 `json:"flag_keys" binding:"required"`
	Context       models.EvaluationContext `json:"context"`
}
```

- [ ] **Step 2: Update `evaluate` handler to resolve application and validate flag scope**

In the `evaluate` handler, after binding the request, add application resolution before calling `h.service.Evaluate`:

```go
if req.Application != "" {
	app, err := h.entityRepo.GetAppBySlug(c.Request.Context(), req.ProjectID, req.Application)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve application"})
		return
	}
	if app == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "application not found"})
		return
	}
	// TODO: pass applicationID to Evaluate so it can validate flag scope
	// For now, the flag is looked up by project+environment+key, which is sufficient
	// since flag keys are unique within project+environment
}
```

Apply the same pattern to `batchEvaluate`.

- [ ] **Step 3: Verify build compiles**

Run: `cd /Users/sgamel/git/DeploySentry && go build ./...`
Expected: No errors

- [ ] **Step 4: Commit**

```bash
git add internal/flags/handler.go
git commit -m "feat: evaluate/batchEvaluate accept application field"
```

---

### Task 4: Update SSE stream handler to accept `application` query param

**Files:**
- Modify: `internal/flags/handler.go:939-961`

- [ ] **Step 1: Read and store the `application` param on SSE connect**

The current SSE implementation broadcasts all flag changes to all clients. To filter by application, the stream handler should resolve the application on connect and filter events. For now, document the application param acceptance — full per-client filtering is a follow-up since the SSE broker uses a shared channel.

Add application param reading at the top of `streamFlags`:

```go
func (h *Handler) streamFlags(c *gin.Context) {
	// Read application filter (optional). SDK always sends this.
	// Full per-client filtering is deferred — the client-side cache
	// already filters to the correct set via the initial listFlags call.
	_ = c.Query("application")

	c.Header("Content-Type", "text/event-stream")
	// ... rest unchanged
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/flags/handler.go
git commit -m "feat: SSE stream handler accepts application query param"
```

---

### Task 5: Add `application` to Node SDK `ClientOptions` and client

**Files:**
- Modify: `sdk/node/src/types.ts:65-80`
- Modify: `sdk/node/src/client.ts:33-59, 81, 289-314, 316-322`

- [ ] **Step 1: Add `application` to `ClientOptions`**

In `sdk/node/src/types.ts`, add the field to `ClientOptions`:

```typescript
export interface ClientOptions {
  apiKey: string;
  baseURL?: string;
  environment: string;
  project: string;
  application: string;
  cacheTimeout?: number;
  offlineMode?: boolean;
  sessionId?: string;
}
```

- [ ] **Step 2: Store and validate `application` in the client constructor**

In `sdk/node/src/client.ts`, add the field and validation in the constructor (around line 38):

```typescript
private readonly application: string;
```

In the constructor body (around line 49):

```typescript
if (!options.application) throw new Error('application is required');
this.application = options.application;
```

- [ ] **Step 3: Send `application` in `fetchAllFlags`**

Update the `fetchAllFlags` method (line 318):

```typescript
private async fetchAllFlags(): Promise<Flag[]> {
  const response = await this.request<{ flags: Flag[] }>(
    'GET',
    `/api/v1/flags?project_id=${enc(this.project)}&application=${enc(this.application)}`,
  );
  return response.flags ?? [];
}
```

- [ ] **Step 4: Send `application` in `evaluate` and `detail`**

Update the POST body in the `evaluate` method (around line 299) and `detail` method (around line 195):

```typescript
{
  project_id: this.project,
  environment_id: this.environment,
  application: this.application,
  flag_key: key,
  context: context ?? {},
  ...(this.sessionId ? { session_id: this.sessionId } : {}),
}
```

- [ ] **Step 5: Send `application` in SSE URL**

Update the SSE URL in `initialize()` (around line 81):

```typescript
url: `${this.baseURL}/api/v1/flags/stream?project_id=${enc(this.project)}&environment_id=${enc(this.environment)}&application=${enc(this.application)}`,
```

- [ ] **Step 6: Verify TypeScript compiles**

Run: `cd /Users/sgamel/git/DeploySentry/sdk/node && npx tsc --noEmit`
Expected: No errors

- [ ] **Step 7: Commit**

```bash
git add sdk/node/src/types.ts sdk/node/src/client.ts
git commit -m "feat: add required application param to Node SDK"
```

---

### Task 6: Add `application` to React SDK `ProviderProps` and client

**Files:**
- Modify: `sdk/react/src/types.ts:72-87`
- Modify: `sdk/react/src/client.ts:71-85, 336-348, 350-374`
- Modify: `sdk/react/src/provider.tsx:43-56`

- [ ] **Step 1: Add `application` to `ProviderProps`**

In `sdk/react/src/types.ts`, add to `ProviderProps`:

```typescript
export interface ProviderProps {
  apiKey: string;
  baseURL: string;
  environment: string;
  project: string;
  application: string;
  user?: UserContext;
  sessionId?: string;
  children: React.ReactNode;
}
```

- [ ] **Step 2: Store `application` in the React client constructor**

In `sdk/react/src/client.ts`, add to the constructor options type (around line 71):

```typescript
constructor(options: {
  apiKey: string;
  baseURL: string;
  environment: string;
  project: string;
  application: string;
  user?: UserContext;
  sessionId?: string;
}) {
```

Add the field storage:

```typescript
private readonly application: string;
```

And in the constructor body:

```typescript
this.application = options.application;
```

- [ ] **Step 3: Add `application` to `buildQueryParams`**

In `sdk/react/src/client.ts`, update `buildQueryParams` (around line 336):

```typescript
private buildQueryParams(): URLSearchParams {
  const params = new URLSearchParams({
    project_id: this.project,
    environment_id: this.environment,
    application: this.application,
  });
  if (this.user?.id) {
    params.set('userId', this.user.id);
  }
  if (this.user?.attributes) {
    params.set('attributes', JSON.stringify(this.user.attributes));
  }
  return params;
}
```

- [ ] **Step 4: Add `application` to `fetchFlags` URL**

In `sdk/react/src/client.ts`, update `fetchFlags` (around line 352):

```typescript
const url = `${this.baseURL}/api/v1/flags?project_id=${encodeURIComponent(this.project)}&application=${encodeURIComponent(this.application)}`;
```

- [ ] **Step 5: Update provider to pass `application` through**

In `sdk/react/src/provider.tsx`, update the `configKey` memo (line 44):

```typescript
const configKey = useMemo(
  () => JSON.stringify({ apiKey, baseURL, environment, project, application, sessionId }),
  [apiKey, baseURL, environment, project, application, sessionId],
);
```

And the client construction (line 49):

```typescript
const instance = new DeploySentryClient({
  apiKey,
  baseURL,
  environment,
  project,
  application,
  user,
  sessionId,
});
```

- [ ] **Step 6: Verify TypeScript compiles**

Run: `cd /Users/sgamel/git/DeploySentry/sdk/react && npx tsc --noEmit`
Expected: No errors

- [ ] **Step 7: Commit**

```bash
git add sdk/react/src/types.ts sdk/react/src/client.ts sdk/react/src/provider.tsx
git commit -m "feat: add required application param to React SDK"
```

---

### Task 7: Update SDK tests

**Files:**
- Modify: `sdk/node/src/__tests__/contract.test.ts`
- Modify: `sdk/react/src/__tests__/client.test.ts`

- [ ] **Step 1: Update Node SDK contract tests**

Add `application: 'test-app'` to all `DeploySentryClient` constructor calls in the test file. Verify the application param is sent in API requests.

- [ ] **Step 2: Update React SDK client tests**

Add `application: 'test-app'` to all client constructor calls. Verify the application param appears in fetch URLs and query params.

- [ ] **Step 3: Run Node SDK tests**

Run: `cd /Users/sgamel/git/DeploySentry/sdk/node && npm test`
Expected: All tests pass

- [ ] **Step 4: Run React SDK tests**

Run: `cd /Users/sgamel/git/DeploySentry/sdk/react && npm test`
Expected: All tests pass

- [ ] **Step 5: Commit**

```bash
git add sdk/node/src/__tests__/contract.test.ts sdk/react/src/__tests__/client.test.ts
git commit -m "test: update SDK tests for required application param"
```

---

### Task 8: Update SDK documentation

**Files:**
- Modify: `sdk/node/README.md`
- Modify: `sdk/react/README.md`
- Modify: `sdk/INTEGRATION.md`

- [ ] **Step 1: Update Node SDK README**

In the Configuration table, add `application` as required:

```markdown
| `application`  | string   | Yes      | -                              | Application identifier                 |
```

Update all Quick Start and example code blocks to include `application: 'my-web-app'`.

- [ ] **Step 2: Update React SDK README**

In the Provider Props table, add `application`:

```markdown
| `application` | string        | Yes      | Application identifier                     |
```

Update all provider examples to include `application="my-web-app"`.

- [ ] **Step 3: Update INTEGRATION.md**

Update all code examples in the integration checklist and the CLAUDE.md directive block to include the `application` parameter. Add `DEPLOYSENTRY_APPLICATION` to the environment variables table.

- [ ] **Step 4: Commit**

```bash
git add sdk/node/README.md sdk/react/README.md sdk/INTEGRATION.md
git commit -m "docs: add application param to SDK docs and integration guide"
```

---

### Task 9: Full build verification

- [ ] **Step 1: Build Go backend**

Run: `cd /Users/sgamel/git/DeploySentry && go build ./...`
Expected: No errors

- [ ] **Step 2: Run Go tests**

Run: `cd /Users/sgamel/git/DeploySentry && go test ./...`
Expected: All tests pass

- [ ] **Step 3: Build Node SDK**

Run: `cd /Users/sgamel/git/DeploySentry/sdk/node && npm run build`
Expected: No errors

- [ ] **Step 4: Build React SDK**

Run: `cd /Users/sgamel/git/DeploySentry/sdk/react && npm run build`
Expected: No errors

- [ ] **Step 5: Run all SDK tests**

Run: `cd /Users/sgamel/git/DeploySentry/sdk/node && npm test && cd ../react && npm test`
Expected: All tests pass
