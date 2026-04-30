# YAML Flag Configuration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a YAML export endpoint, SDK file/fallback modes with local rule evaluation, and UI export/preview features so flags work offline.

**Architecture:** The backend gets an export endpoint that serializes all flags, env states, rules, and rule-env states into YAML. The Node SDK adds a YAML file loader and local rule evaluator for `file` and `server-with-fallback` modes. The React SDK adds a `flagData` prop for pre-loaded config. The frontend adds an export button and a per-flag YAML preview tab.

**Tech Stack:** Go + `gopkg.in/yaml.v3` (backend), TypeScript + `js-yaml` (Node SDK), React (frontend)

**Spec:** `docs/superpowers/specs/2026-04-16-yaml-flag-config-design.md`

---

### Task 1: Backend export endpoint

**Files:**
- Create: `internal/flags/export.go`
- Modify: `internal/flags/handler.go` (add handler + route)
- Modify: `internal/flags/service.go` (add service method)

- [ ] **Step 1: Define the YAML export types**

Create `internal/flags/export.go`:

```go
package flags

import "time"

// YAMLExport is the top-level structure for the exported YAML config file.
type YAMLExport struct {
	Version      int               `yaml:"version" json:"version"`
	Project      string            `yaml:"project" json:"project"`
	Application  string            `yaml:"application" json:"application"`
	ExportedAt   string            `yaml:"exported_at" json:"exported_at"`
	Environments []YAMLEnvironment `yaml:"environments" json:"environments"`
	Flags        []YAMLFlag        `yaml:"flags" json:"flags"`
}

// YAMLEnvironment represents an org environment in the export.
type YAMLEnvironment struct {
	ID           string `yaml:"id" json:"id"`
	Name         string `yaml:"name" json:"name"`
	IsProduction bool   `yaml:"is_production" json:"is_production"`
}

// YAMLFlag represents a feature flag in the export.
type YAMLFlag struct {
	Key          string                    `yaml:"key" json:"key"`
	Name         string                    `yaml:"name" json:"name"`
	FlagType     string                    `yaml:"flag_type" json:"flag_type"`
	Category     string                    `yaml:"category" json:"category"`
	DefaultValue string                    `yaml:"default_value" json:"default_value"`
	IsPermanent  bool                      `yaml:"is_permanent" json:"is_permanent"`
	ExpiresAt    string                    `yaml:"expires_at,omitempty" json:"expires_at,omitempty"`
	Environments map[string]YAMLFlagEnv    `yaml:"environments" json:"environments"`
	Rules        []YAMLRule                `yaml:"rules,omitempty" json:"rules,omitempty"`
}

// YAMLFlagEnv represents per-environment flag state.
type YAMLFlagEnv struct {
	Enabled bool   `yaml:"enabled" json:"enabled"`
	Value   string `yaml:"value" json:"value"`
}

// YAMLRule represents a targeting rule in the export.
type YAMLRule struct {
	Attribute    string            `yaml:"attribute" json:"attribute"`
	Operator     string            `yaml:"operator" json:"operator"`
	TargetValues []string          `yaml:"target_values" json:"target_values"`
	Value        string            `yaml:"value" json:"value"`
	Priority     int               `yaml:"priority" json:"priority"`
	Environments map[string]bool   `yaml:"environments" json:"environments"`
}
```

- [ ] **Step 2: Add export service method**

In `internal/flags/service.go`, add to the `FlagService` interface:

```go
ExportFlags(ctx context.Context, projectID uuid.UUID, applicationSlug string, envs []YAMLEnvironment) (*YAMLExport, error)
```

Add the implementation to `flagService`:

```go
func (s *flagService) ExportFlags(ctx context.Context, projectID uuid.UUID, applicationSlug string, envs []YAMLEnvironment) (*YAMLExport, error) {
	// Load all flags for the project
	allFlags, err := s.repo.ListFlags(ctx, projectID, ListOptions{Limit: 10000})
	if err != nil {
		return nil, err
	}

	export := &YAMLExport{
		Version:      1,
		ExportedAt:   time.Now().UTC().Format(time.RFC3339),
		Environments: envs,
	}

	for _, flag := range allFlags {
		yf := YAMLFlag{
			Key:          flag.Key,
			Name:         flag.Name,
			FlagType:     string(flag.FlagType),
			Category:     string(flag.Category),
			DefaultValue: flag.DefaultValue,
			IsPermanent:  flag.IsPermanent,
			Environments: make(map[string]YAMLFlagEnv),
			Rules:        []YAMLRule{},
		}
		if flag.ExpiresAt != nil {
			yf.ExpiresAt = flag.ExpiresAt.Format(time.RFC3339)
		}

		// Load per-env states
		envStates, _ := s.repo.ListFlagEnvStates(ctx, flag.ID)
		for _, es := range envStates {
			for _, env := range envs {
				if es.EnvironmentID.String() == env.ID {
					val := ""
					if es.Value != nil {
						val = fmt.Sprintf("%v", es.Value)
					}
					yf.Environments[env.Name] = YAMLFlagEnv{
						Enabled: es.Enabled,
						Value:   val,
					}
				}
			}
		}

		// Load rules
		rules, _ := s.repo.ListRules(ctx, flag.ID)
		ruleEnvStates, _ := s.repo.ListRuleEnvironmentStates(ctx, flag.ID)

		for _, rule := range rules {
			yr := YAMLRule{
				Attribute:    rule.Attribute,
				Operator:     rule.Operator,
				TargetValues: rule.TargetValues,
				Value:        rule.Value,
				Priority:     rule.Priority,
				Environments: make(map[string]bool),
			}
			for _, res := range ruleEnvStates {
				if res.RuleID == rule.ID {
					for _, env := range envs {
						if res.EnvironmentID.String() == env.ID {
							yr.Environments[env.Name] = res.Enabled
						}
					}
				}
			}
			yf.Rules = append(yf.Rules, yr)
		}

		export.Flags = append(export.Flags, yf)
	}

	return export, nil
}
```

- [ ] **Step 3: Add the export handler**

In `internal/flags/handler.go`, add:

```go
func (h *Handler) exportFlags(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("projectId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project id"})
		return
	}

	appSlug := c.Query("application")
	if appSlug == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "application query parameter is required"})
		return
	}

	format := c.DefaultQuery("format", "yaml")

	// Resolve org environments — need org ID from project
	// The handler has entityRepo which can resolve app; we need org envs
	// For now, load envs from the request context or pass through
	orgIDStr := c.GetString("org_id")
	orgID, _ := uuid.Parse(orgIDStr)

	// Build environment list — this requires the entity service
	// Use the entityRepo to get environments
	var envs []YAMLEnvironment
	// Environments are org-level; we need to get them from the entities service
	// For simplicity, accept environment data as part of the export service call
	// The handler will need access to entity service to list org environments

	export, err := h.service.ExportFlags(c.Request.Context(), projectID, appSlug, envs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to export flags"})
		return
	}

	export.Project = c.Query("project_name")
	export.Application = appSlug

	if format == "json" {
		c.JSON(http.StatusOK, export)
		return
	}

	yamlBytes, err := yaml.Marshal(export)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to serialize yaml"})
		return
	}
	c.Data(http.StatusOK, "application/x-yaml", yamlBytes)
}
```

Add `"gopkg.in/yaml.v3"` to the imports (aliased as `yaml`).

**Note:** The handler needs access to org environments. The cleanest approach is to have the handler call `h.entityRepo` (which already exists on the flags handler — added in the SDK application parameter work) to resolve the application, then use a new method or the existing entities service to list org environments. Read the handler struct and figure out the best way to get org environments. You may need to:
1. Accept `org_id` as a query param, or
2. Resolve it from the project (project has `org_id`), or
3. Add an `entityService` dependency to the flags handler

Choose the simplest approach that works.

- [ ] **Step 4: Register the route**

In `RegisterRoutes`, add at the top level (not inside the `/:id` group):

```go
rg.GET("/projects/:projectId/export-flags", auth.RequirePermission(h.rbac, auth.PermFlagRead), h.exportFlags)
```

- [ ] **Step 5: Update mock service in handler tests**

Add `ExportFlags` to the mock service in `internal/flags/handler_test.go`:

```go
func (m *mockFlagService) ExportFlags(ctx context.Context, projectID uuid.UUID, applicationSlug string, envs []YAMLEnvironment) (*YAMLExport, error) {
	return &YAMLExport{Version: 1}, nil
}
```

- [ ] **Step 6: Verify build**

Run: `cd /Users/sgamel/git/DeploySentry && go build ./...`
Expected: No errors

- [ ] **Step 7: Commit**

```bash
git add internal/flags/export.go internal/flags/handler.go internal/flags/service.go internal/flags/handler_test.go
git commit -m "feat: add flag export endpoint with YAML/JSON output"
```

---

### Task 2: Node SDK — YAML file loader and local evaluator

**Files:**
- Create: `sdk/node/src/file-loader.ts`
- Create: `sdk/node/src/local-evaluator.ts`
- Modify: `sdk/node/src/types.ts`
- Modify: `sdk/node/src/client.ts`
- Modify: `sdk/node/package.json` (add `js-yaml` dependency)

- [ ] **Step 1: Add `js-yaml` dependency**

```bash
cd /Users/sgamel/git/DeploySentry/sdk/node && npm install js-yaml && npm install -D @types/js-yaml
```

- [ ] **Step 2: Update `ClientOptions` in types.ts**

In `sdk/node/src/types.ts`, add to the `ClientOptions` interface:

```typescript
/** SDK operation mode. Default: 'server'. */
mode?: 'server' | 'file' | 'server-with-fallback';
/** Path to YAML flag config file. Default: '.deploysentry/flags.yaml'. */
flagFilePath?: string;
```

Add the YAML config types:

```typescript
/** Parsed YAML flag configuration file. */
export interface FlagConfig {
  version: number;
  project: string;
  application: string;
  exported_at: string;
  environments: FlagConfigEnvironment[];
  flags: FlagConfigFlag[];
}

export interface FlagConfigEnvironment {
  id: string;
  name: string;
  is_production: boolean;
}

export interface FlagConfigFlag {
  key: string;
  name: string;
  flag_type: string;
  category: string;
  default_value: string;
  is_permanent: boolean;
  expires_at?: string;
  environments: Record<string, { enabled: boolean; value: string }>;
  rules?: FlagConfigRule[];
}

export interface FlagConfigRule {
  attribute: string;
  operator: string;
  target_values: string[];
  value: string;
  priority: number;
  environments: Record<string, boolean>;
}
```

- [ ] **Step 3: Create the file loader**

Create `sdk/node/src/file-loader.ts`:

```typescript
import * as fs from 'fs';
import * as path from 'path';
import * as yaml from 'js-yaml';
import type { FlagConfig } from './types';

const DEFAULT_FLAG_FILE_PATH = '.deploysentry/flags.yaml';

/**
 * Load and parse a YAML flag configuration file.
 * @param filePath Path to the YAML file (relative to cwd or absolute).
 * @returns Parsed flag configuration.
 * @throws If the file doesn't exist or is invalid YAML.
 */
export function loadFlagConfig(filePath?: string): FlagConfig {
  const resolvedPath = path.resolve(filePath ?? DEFAULT_FLAG_FILE_PATH);

  if (!fs.existsSync(resolvedPath)) {
    throw new Error(`Flag config file not found: ${resolvedPath}`);
  }

  const content = fs.readFileSync(resolvedPath, 'utf-8');
  const parsed = yaml.load(content) as FlagConfig;

  if (!parsed || typeof parsed !== 'object' || !parsed.version || !parsed.flags) {
    throw new Error(`Invalid flag config file: ${resolvedPath}`);
  }

  return parsed;
}
```

- [ ] **Step 4: Create the local evaluator**

Create `sdk/node/src/local-evaluator.ts`:

```typescript
import type { EvaluationContext, FlagConfig, FlagConfigFlag, FlagConfigRule } from './types';

/**
 * Evaluate a flag locally from the parsed YAML config.
 */
export function evaluateLocal(
  config: FlagConfig,
  environment: string,
  key: string,
  context?: EvaluationContext,
): { value: string; reason: string } {
  const flag = config.flags.find((f) => f.key === key);
  if (!flag) {
    return { value: '', reason: 'flag_not_found' };
  }

  const envState = flag.environments[environment];
  if (!envState || !envState.enabled) {
    return { value: flag.default_value, reason: 'env_disabled' };
  }

  // Evaluate rules in priority order
  if (flag.rules && context) {
    const sortedRules = [...flag.rules].sort((a, b) => a.priority - b.priority);
    for (const rule of sortedRules) {
      if (!rule.environments[environment]) continue;
      if (matchRule(rule, context)) {
        return { value: rule.value, reason: 'rule_match' };
      }
    }
  }

  return { value: envState.value || flag.default_value, reason: 'default' };
}

function matchRule(rule: FlagConfigRule, context: EvaluationContext): boolean {
  const attrValue = context.attributes?.[rule.attribute] ?? '';
  const targets = rule.target_values ?? [];

  switch (rule.operator) {
    case 'equals':
      return targets.length > 0 && attrValue === targets[0];
    case 'not_equals':
      return targets.length > 0 && attrValue !== targets[0];
    case 'in':
      return targets.includes(attrValue);
    case 'not_in':
      return !targets.includes(attrValue);
    case 'contains':
      return targets.length > 0 && attrValue.includes(targets[0]);
    case 'starts_with':
      return targets.length > 0 && attrValue.startsWith(targets[0]);
    case 'ends_with':
      return targets.length > 0 && attrValue.endsWith(targets[0]);
    case 'greater_than':
      return targets.length > 0 && parseFloat(attrValue) > parseFloat(targets[0]);
    case 'less_than':
      return targets.length > 0 && parseFloat(attrValue) < parseFloat(targets[0]);
    default:
      return false;
  }
}
```

- [ ] **Step 5: Update the client to support modes**

In `sdk/node/src/client.ts`, update the constructor and `initialize()`:

Store the mode and file path:
```typescript
private readonly mode: 'server' | 'file' | 'server-with-fallback';
private readonly flagFilePath?: string;
private flagConfig: FlagConfig | null = null;
```

In the constructor:
```typescript
this.mode = options.mode ?? 'server';
this.flagFilePath = options.flagFilePath;
```

Update `initialize()`:
```typescript
async initialize(): Promise<void> {
  if (this.mode === 'file') {
    this.flagConfig = loadFlagConfig(this.flagFilePath);
    this._initialized = true;
    return;
  }

  if (this.offlineMode) {
    this._initialized = true;
    return;
  }

  try {
    const flags = await this.fetchAllFlags();
    this.cache.setMany(flags);
    this.streamClient = new FlagStreamClient({ /* ... existing SSE setup ... */ });
    this.streamClient.connect();
    this._initialized = true;
  } catch (err) {
    if (this.mode === 'server-with-fallback') {
      console.warn('[DeploySentry] Server unavailable, falling back to flag config file');
      this.flagConfig = loadFlagConfig(this.flagFilePath);
      this._initialized = true;
      return;
    }
    throw err;
  }
}
```

Update the private `evaluate()` method to check file mode:
```typescript
private async evaluate<T>(key: string, defaultValue: T, context?: EvaluationContext): Promise<T> {
  if (this.flagConfig) {
    const result = evaluateLocal(this.flagConfig, this.environment, key, context);
    if (result.reason === 'flag_not_found') return defaultValue;
    return result.value as unknown as T;
  }

  if (this.offlineMode) return defaultValue;
  // ... existing server evaluation logic
}
```

Add imports at the top:
```typescript
import { loadFlagConfig } from './file-loader';
import { evaluateLocal } from './local-evaluator';
import type { FlagConfig } from './types';
```

- [ ] **Step 6: Export new modules from index.ts**

In `sdk/node/src/index.ts`, add:

```typescript
export { loadFlagConfig } from './file-loader';
export { evaluateLocal } from './local-evaluator';
export type { FlagConfig, FlagConfigFlag, FlagConfigRule, FlagConfigEnvironment } from './types';
```

- [ ] **Step 7: Verify TypeScript compiles**

Run: `cd /Users/sgamel/git/DeploySentry/sdk/node && npx tsc --noEmit`

- [ ] **Step 8: Commit**

```bash
git add sdk/node/
git commit -m "feat: Node SDK file/fallback mode with local rule evaluation"
```

---

### Task 3: React SDK — `flagData` prop support

**Files:**
- Modify: `sdk/react/src/types.ts`
- Create: `sdk/react/src/local-evaluator.ts`
- Modify: `sdk/react/src/client.ts`
- Modify: `sdk/react/src/provider.tsx`

- [ ] **Step 1: Add types to React SDK**

In `sdk/react/src/types.ts`, add the same `FlagConfig*` types as the Node SDK (duplicate — React SDK has zero runtime dependency on Node SDK):

```typescript
export interface FlagConfig {
  version: number;
  project: string;
  application: string;
  exported_at: string;
  environments: FlagConfigEnvironment[];
  flags: FlagConfigFlag[];
}

export interface FlagConfigEnvironment {
  id: string;
  name: string;
  is_production: boolean;
}

export interface FlagConfigFlag {
  key: string;
  name: string;
  flag_type: string;
  category: string;
  default_value: string;
  is_permanent: boolean;
  expires_at?: string;
  environments: Record<string, { enabled: boolean; value: string }>;
  rules?: FlagConfigRule[];
}

export interface FlagConfigRule {
  attribute: string;
  operator: string;
  target_values: string[];
  value: string;
  priority: number;
  environments: Record<string, boolean>;
}
```

Add to `ProviderProps`:
```typescript
/** SDK operation mode. Default: 'server'. */
mode?: 'server' | 'file' | 'server-with-fallback';
/** Pre-loaded flag config data (parsed YAML/JSON). Used in file mode for browsers. */
flagData?: FlagConfig;
```

- [ ] **Step 2: Create the local evaluator (same logic as Node SDK)**

Create `sdk/react/src/local-evaluator.ts` with the same `evaluateLocal` and `matchRule` functions as the Node SDK (copy from Task 2 Step 4, but import types from the React SDK's own types file).

- [ ] **Step 3: Update the React client**

In `sdk/react/src/client.ts`, add:

- A `flagConfig` field and constructor option
- In `init()`, if `flagConfig` is set, populate the flags store from it and skip API calls
- In `boolValue()`, `stringValue()`, etc., if `flagConfig` is set, use `evaluateLocal()`

- [ ] **Step 4: Update the provider**

In `sdk/react/src/provider.tsx`, pass `mode` and `flagData` to the client constructor.

- [ ] **Step 5: Verify TypeScript compiles**

Run: `cd /Users/sgamel/git/DeploySentry/sdk/react && npx tsc --noEmit`

- [ ] **Step 6: Commit**

```bash
git add sdk/react/
git commit -m "feat: React SDK file/flagData mode with local rule evaluation"
```

---

### Task 4: Frontend — Export button and YAML preview tab

**Files:**
- Modify: `web/src/api.ts`
- Modify: `web/src/pages/FlagDetailPage.tsx`
- Modify: `web/src/pages/ProjectPage.tsx` or `web/src/pages/SettingsPage.tsx`

- [ ] **Step 1: Add export API method**

In `web/src/api.ts`, add to `flagsApi`:

```typescript
exportFlags: async (projectId: string, application: string, format: 'yaml' | 'json' = 'yaml') => {
  const token = localStorage.getItem('ds_token') || '';
  const res = await fetch(`${BASE}/projects/${projectId}/export-flags?application=${application}&format=${format}`, {
    headers: {
      Authorization: token.startsWith('ds_') ? `ApiKey ${token}` : `Bearer ${token}`,
    },
  });
  if (!res.ok) throw new Error(`Export failed: ${res.status}`);
  return res.text();
},
```

- [ ] **Step 2: Add YAML preview tab to FlagDetailPage**

In `web/src/pages/FlagDetailPage.tsx`:

Update the `activeTab` type to include `'yaml'`:
```typescript
const [activeTab, setActiveTab] = useState<'rules' | 'environments' | 'yaml'>('rules');
```

Add a third tab button:
```tsx
<button
  className={`detail-tab${activeTab === 'yaml' ? ' active' : ''}`}
  onClick={() => setActiveTab('yaml')}
>
  YAML
</button>
```

Add the YAML tab content (generates YAML client-side from loaded flag data):

```tsx
{activeTab === 'yaml' && flag && (
  <div className="card">
    <p className="text-muted" style={{ marginBottom: 12, fontSize: 13 }}>
      Read-only preview of this flag's configuration. Use the project-level export for the full file.
    </p>
    <pre style={{
      background: 'var(--color-bg-secondary)',
      border: '1px solid var(--color-border)',
      borderRadius: 6,
      padding: 16,
      fontSize: 13,
      overflow: 'auto',
      maxHeight: 500,
    }}>
      {generateFlagYaml(flag, rules, environments, envStates, ruleEnvStates)}
    </pre>
  </div>
)}
```

Add the `generateFlagYaml` helper function before the component:

```typescript
function generateFlagYaml(
  flag: Flag,
  rules: TargetingRule[],
  environments: OrgEnvironment[],
  envStates: FlagEnvironmentState[],
  ruleEnvStates: RuleEnvironmentState[],
): string {
  const lines: string[] = [];
  lines.push(`- key: ${flag.key}`);
  lines.push(`  name: ${flag.name}`);
  lines.push(`  flag_type: ${flag.flag_type}`);
  lines.push(`  category: ${flag.category}`);
  lines.push(`  default_value: "${flag.default_value}"`);
  lines.push(`  is_permanent: ${flag.is_permanent}`);
  if (flag.expires_at) lines.push(`  expires_at: "${flag.expires_at}"`);

  lines.push(`  environments:`);
  for (const env of environments) {
    const state = envStates.find((s) => s.environment_id === env.id);
    lines.push(`    ${env.name}:`);
    lines.push(`      enabled: ${state?.enabled ?? false}`);
    lines.push(`      value: "${state?.value != null ? String(state.value) : flag.default_value}"`);
  }

  if (rules.length > 0) {
    lines.push(`  rules:`);
    for (const rule of rules) {
      lines.push(`    - attribute: ${rule.attribute}`);
      lines.push(`      operator: ${rule.operator}`);
      lines.push(`      target_values: [${(rule.target_values ?? []).map((v) => `"${v}"`).join(', ')}]`);
      lines.push(`      value: "${rule.value}"`);
      lines.push(`      priority: ${rule.priority}`);
      lines.push(`      environments:`);
      for (const env of environments) {
        const res = ruleEnvStates.find((s) => s.rule_id === rule.id && s.environment_id === env.id);
        lines.push(`        ${env.name}: ${res?.enabled ?? false}`);
      }
    }
  }

  return lines.join('\n');
}
```

- [ ] **Step 3: Add export button to project settings**

In the project settings page (`web/src/pages/SettingsPage.tsx`), find the project-level settings section. Add an "Export Flags" button that:
1. Calls `flagsApi.exportFlags(projectId, appSlug)`
2. Creates a Blob and triggers a download as `flags.yaml`

```tsx
const handleExportFlags = async () => {
  try {
    const yamlContent = await flagsApi.exportFlags(projectId, appSlug);
    const blob = new Blob([yamlContent], { type: 'application/x-yaml' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = 'flags.yaml';
    a.click();
    URL.revokeObjectURL(url);
  } catch (err) {
    console.error('Export failed:', err);
  }
};
```

Add the button in the project settings general tab or as a standalone section.

- [ ] **Step 4: Verify TypeScript compiles**

Run: `cd /Users/sgamel/git/DeploySentry/web && npx tsc --noEmit`

- [ ] **Step 5: Commit**

```bash
git add web/src/
git commit -m "feat: YAML preview tab on flag detail and export button on project settings"
```

---

### Task 5: Update SDK documentation

**Files:**
- Modify: `sdk/node/README.md`
- Modify: `sdk/react/README.md`
- Modify: `sdk/INTEGRATION.md`

- [ ] **Step 1: Update Node SDK README**

Add a new section after Configuration:

```markdown
## Offline / File Mode

The SDK can load flag configurations from a local YAML file instead of (or as fallback to) the server.

### Modes

| Mode | Behavior |
| --- | --- |
| `server` (default) | API calls + SSE streaming |
| `file` | Load from YAML file, evaluate locally. No server contact. |
| `server-with-fallback` | Try server first. If unavailable, fall back to YAML file. |

### Usage

\`\`\`typescript
// File mode — local development, CI, testing
const client = new DeploySentryClient({
  apiKey: 'not-used',
  environment: 'staging',
  project: 'my-project',
  application: 'my-web-app',
  mode: 'file',
  flagFilePath: '.deploysentry/flags.yaml', // default
});

// Fallback mode — production resilience
const client = new DeploySentryClient({
  apiKey: 'ds_live_xxx',
  environment: 'production',
  project: 'my-project',
  application: 'my-web-app',
  mode: 'server-with-fallback',
});
\`\`\`

### Generating the YAML file

Export from the DeploySentry dashboard: Project Settings → Export Flags. Place the downloaded `flags.yaml` at `.deploysentry/flags.yaml` in your project root.
```

Add `mode` and `flagFilePath` to the Configuration table.

- [ ] **Step 2: Update React SDK README**

Add similar section, noting that browsers can't read files directly:

```markdown
## Offline / File Mode

Pass pre-loaded flag config via the `flagData` prop:

\`\`\`tsx
import flagConfig from './.deploysentry/flags.json';

<DeploySentryProvider
  apiKey="not-used"
  baseURL=""
  environment="staging"
  project="my-project"
  application="my-web-app"
  mode="file"
  flagData={flagConfig}
>
  <App />
</DeploySentryProvider>
\`\`\`
```

- [ ] **Step 3: Update INTEGRATION.md**

Add a section about the YAML config file and offline modes. Update the CLAUDE.md directive block to mention `mode` options.

- [ ] **Step 4: Commit**

```bash
git add sdk/node/README.md sdk/react/README.md sdk/INTEGRATION.md
git commit -m "docs: add offline/file mode documentation to SDK READMEs and integration guide"
```

---

### Task 6: Full verification

- [ ] **Step 1: Build Go backend**

Run: `cd /Users/sgamel/git/DeploySentry && go build ./...`

- [ ] **Step 2: Run Go tests**

Run: `cd /Users/sgamel/git/DeploySentry && go test ./internal/flags/...`

- [ ] **Step 3: Build and test Node SDK**

Run: `cd /Users/sgamel/git/DeploySentry/sdk/node && npx tsc --noEmit && npm test`

- [ ] **Step 4: Build and test React SDK**

Run: `cd /Users/sgamel/git/DeploySentry/sdk/react && npx tsc --noEmit && npm test`

- [ ] **Step 5: Verify web frontend**

Run: `cd /Users/sgamel/git/DeploySentry/web && npx tsc --noEmit`

- [ ] **Step 6: Test in browser**

1. Go to a flag detail page → click YAML tab → see read-only YAML preview
2. Go to project settings → click Export Flags → `flags.yaml` downloads
3. Place the file at `.deploysentry/flags.yaml` in a test project
4. Initialize the Node SDK with `mode: 'file'` → verify it loads and evaluates flags locally
