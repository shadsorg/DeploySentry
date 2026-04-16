# YAML Flag Configuration File

**Phase**: Design

## Overview

Add a YAML-based flag configuration file that serves as a complete snapshot of flag definitions, per-environment values, and targeting rules. The SDK can load this file for offline development, testing, or as a fallback when the server is unreachable. The UI provides a full project export and per-flag YAML preview.

## YAML File Structure

```yaml
version: 1
project: my-project
application: my-web-app
exported_at: "2026-04-16T12:00:00Z"

environments:
  - id: "uuid"
    name: staging
    is_production: false
  - id: "uuid"
    name: production
    is_production: true

flags:
  - key: new-checkout
    name: New Checkout Flow
    flag_type: boolean
    category: release
    default_value: "false"
    is_permanent: false
    expires_at: "2026-06-01T00:00:00Z"
    environments:
      staging:
        enabled: true
        value: "true"
      production:
        enabled: false
        value: "false"
    rules:
      - attribute: userType
        operator: equals
        target_values: ["beta-tester"]
        value: "true"
        priority: 1
        environments:
          staging: true
          production: false
```

### Field Descriptions

- `version` — schema version for forward compatibility (always `1` for now)
- `project` / `application` — identifies the source; SDK validates these match its config
- `exported_at` — ISO timestamp of when the file was generated
- `environments` — org-level environment definitions (id, name, is_production)
- `flags` — array of flag definitions:
  - `key`, `name`, `flag_type`, `category`, `default_value`, `is_permanent`, `expires_at` — flag metadata
  - `environments` — map of environment name → `{enabled, value}` (per-environment flag state)
  - `rules` — array of targeting rules, each with:
    - `attribute`, `operator`, `target_values`, `value`, `priority` — rule definition
    - `environments` — map of environment name → boolean (per-environment rule activation)

## SDK Changes

### New Config Options

**Node SDK (`ClientOptions`):**
```typescript
export interface ClientOptions {
  apiKey: string;
  baseURL?: string;
  environment: string;
  project: string;
  application: string;
  mode?: 'server' | 'file' | 'server-with-fallback';  // default: 'server'
  flagFilePath?: string;                                // default: '.deploysentry/flags.yaml'
  cacheTimeout?: number;
  offlineMode?: boolean;  // deprecated in favor of mode: 'file'
  sessionId?: string;
}
```

**React SDK (`ProviderProps`):**
```typescript
export interface ProviderProps {
  apiKey: string;
  baseURL: string;
  environment: string;
  project: string;
  application: string;
  mode?: 'server' | 'file' | 'server-with-fallback';
  flagFilePath?: string;
  user?: UserContext;
  sessionId?: string;
  children: React.ReactNode;
}
```

Note: React SDK `file` mode uses a bundled JSON version of the YAML (since browsers can't read the filesystem). The export endpoint also supports `?format=json` for this use case. The React provider accepts `flagData` as an alternative to `flagFilePath` for passing pre-loaded config.

### Mode Behavior

**`server` (default):**
- Current behavior. API calls, SSE streaming, cache.
- `flagFilePath` is ignored.

**`file`:**
- Load the YAML file at `initialize()` time. Parse it into the in-memory flag store.
- No API calls, no SSE connection.
- Evaluate targeting rules locally against the provided context.
- The `environment` config option determines which environment's values and rule activations to use.
- If the file doesn't exist or is invalid, `initialize()` throws.

**`server-with-fallback`:**
- Try server first (normal `initialize()` flow).
- If `initialize()` fails (network error, connection refused, timeout), log a warning and fall back to the YAML file.
- Once fallen back, behave like `file` mode for the lifetime of the client.
- If the YAML file also doesn't exist, `initialize()` throws.

### Local Rule Evaluation

When in `file` or fallback mode, the SDK evaluates targeting rules locally:

1. Find the flag by key in the parsed YAML.
2. Look up the environment block matching the client's `environment` config.
3. If the flag is not enabled for that environment, return `default_value`.
4. Iterate rules in priority order. For each rule:
   - Check if the rule is enabled for this environment (from `rules[].environments[envName]`).
   - If enabled, evaluate the rule condition against the provided `EvaluationContext`:
     - Match `attribute` against `context.attributes[attribute]`
     - Apply `operator`: equals, not_equals, in, not_in, contains, starts_with, ends_with, greater_than, less_than
   - If matched, return the rule's `value`.
5. No rules matched → return the environment's `value` (or `default_value`).

### Node SDK File Loading

The Node SDK uses `fs.readFileSync` (or async `fs.readFile`) to load the YAML file. It uses the `yaml` npm package (or `js-yaml`) for parsing. The file path is resolved relative to `process.cwd()`.

### React SDK File Loading

The React SDK cannot read the filesystem. Two approaches:

1. **Build-time import** — the consuming app imports the YAML as a module (Vite/Webpack support YAML imports with plugins). The parsed object is passed as `flagData` prop to the provider.
2. **Fetch from public dir** — place the exported JSON file in the `public/` directory. The SDK fetches it via HTTP at init time.

The `ProviderProps` adds an optional `flagData` prop as an alternative to `flagFilePath`:
```typescript
flagData?: object;  // Pre-loaded flag config (parsed YAML/JSON)
```

## Backend — Export Endpoint

### New Endpoint

`GET /api/v1/projects/:projectId/export-flags`

**Query parameters:**
- `application` (required) — application slug to scope the export
- `format` (optional) — `yaml` (default) or `json`

**Response:** The complete flag configuration as YAML (or JSON), with `Content-Type: application/x-yaml` (or `application/json`).

**Auth:** Requires valid API key or bearer token.

### Implementation

The handler:
1. Resolves the project by ID.
2. Resolves the application by slug within the project.
3. Loads all org environments.
4. Loads all flags for the project, filtered to the application (union: project-level + app-specific).
5. For each flag, loads per-environment states and targeting rules with their per-environment activations.
6. Assembles the YAML structure and serializes.

Uses the `gopkg.in/yaml.v3` package for YAML serialization (already a common Go YAML library).

## Frontend Changes

### Project Settings Tab — Export Button

Add an "Export Flags" section to the project settings page (or as a button on the project's Applications tab):

- "Export Flag Config" button
- On click: calls `GET /projects/:projectId/export-flags?application=<selectedApp>&format=yaml`
- Triggers a file download of `flags.yaml`
- If the project has multiple applications, show a dropdown to select which application to export for

### Flag Detail Page — YAML Tab

Add a "YAML" tab to the flag detail page tabs (alongside Targeting Rules and Environments):

- Renders a read-only `<pre>` code block showing the flag's YAML representation
- Generated client-side from the flag data already loaded on the page
- No download button — view only, for debugging/reference
- Includes the flag's environments and rules sections

### Frontend API Method

```typescript
exportFlags: (projectId: string, application: string, format: 'yaml' | 'json' = 'yaml') => {
  const token = localStorage.getItem('ds_token') || '';
  return fetch(`/api/v1/projects/${projectId}/export-flags?application=${application}&format=${format}`, {
    headers: {
      Authorization: token.startsWith('ds_') ? `ApiKey ${token}` : `Bearer ${token}`,
    },
  }).then((res) => res.text());
},
```

## Future: Automated PR on Flag Change (Not Built)

A future enhancement could automate keeping the YAML file in sync:

1. Configure a target repo and file path in project settings (repo URL, branch, file path).
2. When flag values change (toggle, rule update, env state change), a webhook triggers a worker.
3. The worker calls the export endpoint, generates the updated YAML, and submits a PR to the configured repo via the GitHub/GitLab API.
4. The PR includes a summary of what changed (which flags, which environments).

This is documented here for future reference but is **not in scope** for this implementation.

## Out of Scope

- YAML import (uploading a YAML file to set flag values) — the YAML is read-only from the SDK's perspective
- Automated PR generation
- YAML file watching / hot reload in the SDK
- YAML schema validation CLI tool
- Encryption of sensitive flag values in the YAML file

## Checklist

### Backend
- [ ] Export endpoint: `GET /projects/:projectId/export-flags`
- [ ] YAML serialization of flags, environments, rules, rule env states
- [ ] JSON format option (`?format=json`)
- [ ] Route registration

### Node SDK
- [ ] Add `mode` and `flagFilePath` to `ClientOptions`
- [ ] YAML file loader (fs + yaml parser)
- [ ] Local rule evaluator (attribute matching with all operators)
- [ ] `server-with-fallback` mode: try server, catch, fall back to file
- [ ] Deprecate `offlineMode` in favor of `mode: 'file'`
- [ ] Update README

### React SDK
- [ ] Add `mode`, `flagFilePath`, `flagData` to `ProviderProps`
- [ ] Local rule evaluator (shared logic with Node SDK or duplicated)
- [ ] `server-with-fallback` mode
- [ ] Update README

### Frontend
- [ ] Export button on project settings page
- [ ] Application selector for export (if multiple apps)
- [ ] File download trigger
- [ ] YAML tab on flag detail page (read-only code block)
- [ ] API method for export

### Documentation
- [ ] Update `sdk/INTEGRATION.md` with YAML config file usage
- [ ] Update `sdk/node/README.md` with mode options
- [ ] Update `sdk/react/README.md` with mode options
- [ ] Document future automated PR feature

## Completion Record
<!-- Fill in when phase is set to Complete -->
- **Branch**: ``
- **Committed**: No
- **Pushed**: No
- **CI Checks**:
