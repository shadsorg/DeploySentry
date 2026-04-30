# Targeting Rules with Per-Environment Activation

**Phase**: Design

## Overview

Build out the targeting rules system so rules can be created as definitions (metadata only), then enabled/disabled independently per environment. The Targeting Rules tab on the flag detail page gets an inline "Add Rule" form. The Environments tab shows each environment with an accordion listing all rules and per-environment toggles.

## Current State

- `flag_targeting_rules` table exists with rule definitions (attribute, operator, target_values, value, priority, etc.)
- Backend has `CreateRule`, `ListRules`, `UpdateRule`, `DeleteRule` in the flag service/repo
- Frontend `FlagDetailPage` has a Targeting Rules tab (read-only table) and an Environments tab (per-env flag toggle)
- The `enabled` field on `flag_targeting_rules` is a global boolean — no per-environment control
- The evaluator checks `rule.Enabled` globally when applying rules

## Design

### Database

**New table: `rule_environment_state`**

```sql
CREATE TABLE rule_environment_state (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rule_id         UUID NOT NULL REFERENCES flag_targeting_rules(id) ON DELETE CASCADE,
    environment_id  UUID NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    enabled         BOOLEAN NOT NULL DEFAULT false,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (rule_id, environment_id)
);

CREATE INDEX idx_rule_env_state_rule_id ON rule_environment_state(rule_id);
CREATE INDEX idx_rule_env_state_env_id ON rule_environment_state(environment_id);
```

**Semantics:**
- When a rule is created, no `rule_environment_state` rows exist — the rule is disabled in all environments.
- Enabling a rule for an environment inserts/updates a row with `enabled = true`.
- Disabling sets `enabled = false` (row stays for audit trail).
- Deleting a rule cascades to delete all its `rule_environment_state` rows.

### Backend

#### New Model

```go
type RuleEnvironmentState struct {
    ID            uuid.UUID `json:"id" db:"id"`
    RuleID        uuid.UUID `json:"rule_id" db:"rule_id"`
    EnvironmentID uuid.UUID `json:"environment_id" db:"environment_id"`
    Enabled       bool      `json:"enabled" db:"enabled"`
    CreatedAt     time.Time `json:"created_at" db:"created_at"`
    UpdatedAt     time.Time `json:"updated_at" db:"updated_at"`
}
```

#### New Endpoints

| Method | Path | Description |
|--------|------|-------------|
| PUT | `/api/v1/flags/:flagId/rules/:ruleId/environments/:envId` | Set enabled state for a rule in an environment |
| GET | `/api/v1/flags/:flagId/rules/environment-states` | List all rule-environment states for a flag's rules |

**PUT request body:**
```json
{"enabled": true}
```

**PUT response:** the `RuleEnvironmentState` row.

**GET response:**
```json
{
  "rule_environment_states": [
    {"id": "...", "rule_id": "...", "environment_id": "...", "enabled": true, ...}
  ]
}
```

#### Repository Methods

- `SetRuleEnvironmentState(ctx, ruleID, environmentID uuid.UUID, enabled bool) (*RuleEnvironmentState, error)` — upsert on `(rule_id, environment_id)`.
- `ListRuleEnvironmentStates(ctx, flagID uuid.UUID) ([]RuleEnvironmentState, error)` — joins `flag_targeting_rules` to get all states for a flag's rules.

#### Evaluator Change

In `evaluator.go`, the rule application loop currently checks `rule.Enabled`. Change this to check per-environment state:

1. After loading rules, load `rule_environment_state` rows for the flag's rules filtered to the current `environmentID`.
2. Build a set of enabled rule IDs.
3. In the rule loop, skip rules not in the enabled set (instead of checking `rule.Enabled`).

The global `rule.Enabled` field on `flag_targeting_rules` becomes unused for evaluation. It can remain in the schema for backward compatibility but is ignored by the evaluator.

#### Handler Methods

Add to the existing flags handler:
- `setRuleEnvState` — handles PUT, parses ruleId/envId from URL, calls repo upsert.
- `listRuleEnvStates` — handles GET, parses flagId, calls repo list.

Register routes:
```go
rules.PUT("/:ruleId/environments/:envId", h.setRuleEnvState)
rules.GET("/environment-states", h.listRuleEnvStates)
```

### Frontend

#### Types (`web/src/types.ts`)

```typescript
export interface RuleEnvironmentState {
  id: string;
  rule_id: string;
  environment_id: string;
  enabled: boolean;
  created_at: string;
  updated_at: string;
}
```

Add operators to support:
```typescript
export type RuleOperator = 'equals' | 'not_equals' | 'in' | 'not_in' | 'contains' | 'starts_with' | 'ends_with' | 'greater_than' | 'less_than';
```

#### API (`web/src/api.ts`)

Add to `flagsApi`:
```typescript
setRuleEnvState: (flagId: string, ruleId: string, envId: string, data: { enabled: boolean }) =>
  request<RuleEnvironmentState>(`/flags/${flagId}/rules/${ruleId}/environments/${envId}`, {
    method: 'PUT',
    body: JSON.stringify(data),
  }),
listRuleEnvStates: (flagId: string) =>
  request<{ rule_environment_states: RuleEnvironmentState[] }>(`/flags/${flagId}/rules/environment-states`),
deleteRule: (flagId: string, ruleId: string) =>
  request<void>(`/flags/${flagId}/rules/${ruleId}`, { method: 'DELETE' }),
```

#### Targeting Rules Tab

Replace the current read-only table with:

1. **Rule list** — table showing priority, condition summary, serve value, delete button per row.
2. **Add Rule form** — inline form that expands below the table when "Add Rule" is clicked:
   - Attribute name (`form-input`, text, required)
   - Operator (`form-select`, dropdown with all operators)
   - Target value(s) (`form-input`, text, comma-separated for `in`/`not_in`)
   - Serve value (`form-input`, text, required)
   - Priority (`form-input`, number, defaults to `rules.length + 1`)
   - Submit button → calls `flagsApi.addRule()`, then refetches rules
   - Cancel button → collapses form

**Condition summary format:** `{attribute} {operator} {target_values} → {value}`

Example: `userType equals beta-tester → true`

#### Environments Tab

Replace the current flat table with:

For each org environment:
- **Row header:** environment name, production badge, flag enabled toggle (existing `flagEnvStateApi.set`), flag value
- **Expandable accordion:** when clicked, shows a list of all targeting rules with a toggle per rule
  - Each rule row: condition summary, enabled toggle
  - Toggle calls `flagsApi.setRuleEnvState(flagId, ruleId, envId, {enabled})`
  - Rules with no `rule_environment_state` row default to disabled (toggle is off)

### Rule Creation Flow

1. User clicks "Add Rule" on Targeting Rules tab
2. Fills in: attribute=`userType`, operator=`equals`, target_values=`beta-tester`, value=`true`
3. Submits → rule created, appears in the table, disabled everywhere
4. User switches to Environments tab
5. Expands "staging" environment accordion
6. Toggles the new rule on for staging
7. The rule is now active only in staging — evaluations in staging with `userType=beta-tester` get `true`

### Evaluation Flow (Updated)

1. SDK sends evaluate request with `project_id`, `environment_id`, `flag_key`, `context`
2. Evaluator loads flag, checks if flag is disabled/archived → return default
3. Loads targeting rules for the flag
4. Loads `rule_environment_state` rows for the flag's rules, filtered to the request's `environment_id`
5. For each rule in priority order:
   - Skip if no `rule_environment_state` row exists for this environment, or if `enabled = false`
   - Evaluate the rule's condition against the context
   - If matched → return the rule's serve value
6. No rules matched → return the flag's default value

## Out of Scope

- Other rule types (percentage, segment, schedule, user_target) — attribute only for now
- Bulk enable/disable rules across environments
- Rule ordering UI (drag-and-drop priority)
- Rule versioning/history
- Compound conditions (AND/OR of multiple attributes) — the `conditions` JSONB field exists but is not used here

## Checklist

### Database
- [ ] Migration: create `rule_environment_state` table

### Backend Model
- [ ] Add `RuleEnvironmentState` struct to models

### Backend Repository
- [ ] `SetRuleEnvironmentState` (upsert)
- [ ] `ListRuleEnvironmentStates` (by flag ID)
- [ ] `ListRuleEnvironmentStatesByEnv` (by flag ID + environment ID, for evaluator)

### Backend Handler
- [ ] `PUT /flags/:flagId/rules/:ruleId/environments/:envId` handler
- [ ] `GET /flags/:flagId/rules/environment-states` handler
- [ ] Register routes

### Backend Evaluator
- [ ] Load per-environment rule states in Evaluate()
- [ ] Skip rules not enabled for the current environment

### Frontend Types & API
- [ ] Add `RuleEnvironmentState` type
- [ ] Add `RuleOperator` type
- [ ] Add `setRuleEnvState`, `listRuleEnvStates`, `deleteRule` API methods

### Frontend — Targeting Rules Tab
- [ ] Add Rule inline form (attribute, operator, target values, serve value, priority)
- [ ] Delete rule button per row
- [ ] Refetch rules after add/delete

### Frontend — Environments Tab
- [ ] Accordion per environment showing targeting rules
- [ ] Per-rule per-environment toggle
- [ ] Fetch and display rule environment states

### Tests
- [ ] Backend: create rule, set env state, list env states
- [ ] Backend: evaluator respects per-environment rule activation

## Completion Record
<!-- Fill in when phase is set to Complete -->
- **Branch**: ``
- **Committed**: No
- **Pushed**: No
- **CI Checks**:
