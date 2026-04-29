# Mobile PWA — Phase 5: Flags Writes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Steps use checkbox syntax. Each task ends with a verification step that includes `npm run lint` (eslint) — never commit a step without lint clean.

**Goal:** Promote `FlagDetailPage` from read-only to read+write. Add an env enable toggle, per-env default value editing, per-rule per-env enable toggle, a rule edit sheet for non-compound rule types, and rule reorder. No flag create/archive (still desktop-only). No compound rule editing (still desktop-only).

**Architecture:** Optimistic UI everywhere — apply the change locally, fire the PUT, revert on failure with a small inline error toast. No queueing; no offline modal yet (Phase 6 owns that). The mutation surface is small enough that all writes go through 3 endpoints:

1. `PUT /flags/:id/environments/:envId` — `{ enabled?, value? }` (env toggle + default value edit)
2. `PUT /flags/:id/rules/:ruleId/environments/:envId` — `{ enabled }` (per-rule per-env toggle)
3. `PUT /flags/:id/rules/:ruleId` — `Partial<TargetingRule>` (rule edit sheet, rule reorder via repeated calls)

There is **no bulk reorder endpoint** in the backend. Reorder = N parallel `updateRule` calls, one per rule, each sending the rule's new `priority`. This matches what the desktop `web/src/pages/FlagDetailPage.tsx` does.

**Tech Stack:** Same as Phases 2–4. No new dependencies.

**Scope reminder (per spec):**
- IN: env toggle, env default-value edit (boolean/string/number; JSON deep-links to desktop), per-rule per-env toggle, rule edit sheet for `percentage` / `user_target` / `attribute` / `segment` / `schedule`, rule reorder via up/down arrows.
- OUT: rule create, rule delete, compound rule edit (still "edit on desktop"), flag create, flag archive, offline write blocking (Phase 6).

**RBAC:** Backend enforces `flags:write` / `project:developer`. Client treats a 403 from any mutation the same way as a network failure for now: revert + toast. A "Read-only" badge gating writes-when-viewer is deferred until the API exposes the user's effective project role to the PWA without an extra call (Phase 6 polish or later).

---

## File Structure

```
mobile-pwa/
└── src/
    ├── api.ts                                    # MODIFY — add flagEnvStateApi.set, flagsApi.setRuleEnvState, flagsApi.updateRule
    ├── components/
    │   ├── ToggleSwitch.tsx                      # CREATE — reusable accessible toggle (size sm/md)
    │   ├── ToggleSwitch.test.tsx
    │   ├── RuleEditSheet.tsx                     # CREATE — full-screen modal sheet for rule edit
    │   └── RuleEditSheet.test.tsx
    ├── pages/
    │   ├── FlagDetailPage.tsx                    # MODIFY — wire env toggle, value edit, rule toggle, rule edit sheet, reorder
    │   └── FlagDetailPage.test.tsx               # MODIFY — append write-path tests
    ├── lib/
    │   ├── ruleSummary.ts                        # CREATE — extract the existing summary helper from FlagDetailPage so RuleEditSheet can preview the result
    │   └── ruleSummary.test.ts
    ├── e2e/smoke.spec.ts                         # MODIFY — extend the flags smoke to cover an env toggle PUT
    └── styles/
        └── mobile.css                            # MODIFY — append toggle + sheet styles
```

---

## Task 1: API surface — flagEnvStateApi.set + flagsApi.setRuleEnvState + flagsApi.updateRule (TDD)

**Files:**
- Modify: `mobile-pwa/src/api.ts`
- Modify: `mobile-pwa/src/api.test.ts`

The PWA api currently has these read-only entries; add the writes.

- [ ] **Step 1 (test first):** Append tests covering:
  - `flagEnvStateApi.set(flagId, envId, { enabled: true })` issues `PUT /flags/<flagId>/environments/<envId>` with the JSON body, returns the updated state.
  - `flagEnvStateApi.set(flagId, envId, { value: 'v2' })` (separate call shape).
  - `flagEnvStateApi.set(flagId, envId, { enabled: true, value: 'true' })` (combined update).
  - `flagsApi.setRuleEnvState(flagId, ruleId, envId, { enabled: false })` issues `PUT /flags/<flagId>/rules/<ruleId>/environments/<envId>`.
  - `flagsApi.updateRule(flagId, ruleId, { priority: 2 })` issues `PUT /flags/<flagId>/rules/<ruleId>`.
  - `flagsApi.updateRule(flagId, ruleId, { attribute: 'plan', operator: 'eq', value: 'enterprise' })` (attribute rule edit shape).
  - Auth header attached on each.

- [ ] **Step 2:** Implement:

```ts
// inside flagsApi
updateRule: (flagId: string, ruleId: string, patch: Partial<TargetingRule>) =>
  request<TargetingRule>(`/flags/${flagId}/rules/${ruleId}`, {
    method: 'PUT',
    body: JSON.stringify(patch),
  }),
setRuleEnvState: (
  flagId: string,
  ruleId: string,
  envId: string,
  patch: { enabled: boolean },
) =>
  request<RuleEnvironmentState>(
    `/flags/${flagId}/rules/${ruleId}/environments/${envId}`,
    { method: 'PUT', body: JSON.stringify(patch) },
  ),

// inside flagEnvStateApi
set: (
  flagId: string,
  envId: string,
  patch: { enabled?: boolean; value?: unknown },
) =>
  request<FlagEnvironmentState>(`/flags/${flagId}/environments/${envId}`, {
    method: 'PUT',
    body: JSON.stringify(patch),
  }),
```

- [ ] **Step 3:** Verify: `npm run test --silent` green, `npx tsc --noEmit` clean, `npm run lint` clean.

---

## Task 2: ToggleSwitch component (TDD)

**Files:**
- Create: `mobile-pwa/src/components/ToggleSwitch.tsx`
- Create: `mobile-pwa/src/components/ToggleSwitch.test.tsx`
- Modify: `mobile-pwa/src/styles/mobile.css` (append toggle styles)

A small, accessible toggle reused everywhere we toggle write state in this phase. Single source of truth for toggle styling and a11y.

- [ ] **Step 1 (test first):**
  - Renders an `<input type="checkbox" role="switch">` with the `aria-label` prop.
  - Reflects `checked` prop.
  - Calls `onChange(next)` with the new boolean when clicked.
  - When `disabled`, click does NOT fire `onChange`.
  - When `loading`, click does NOT fire `onChange` AND a "loading" data attribute is present (for spinner CSS).

- [ ] **Step 2:** Component shape:

```tsx
type Size = 'sm' | 'md';
export function ToggleSwitch({
  checked,
  onChange,
  ariaLabel,
  size = 'md',
  disabled = false,
  loading = false,
}: {
  checked: boolean;
  onChange: (next: boolean) => void;
  ariaLabel: string;
  size?: Size;
  disabled?: boolean;
  loading?: boolean;
}) {
  return (
    <label className="m-toggle" data-size={size} data-loading={loading || undefined}>
      <input
        type="checkbox"
        role="switch"
        aria-label={ariaLabel}
        checked={checked}
        disabled={disabled || loading}
        onChange={(e) => onChange(e.target.checked)}
      />
      <span className="m-toggle-track" aria-hidden="true" />
    </label>
  );
}
```

- [ ] **Step 3:** Append CSS — track + thumb, `data-size="sm"` smaller variant, `data-loading` overlays a spinner. Touch target stays ≥ 44px even at sm size (extend the hit area via padding; visual track can be smaller).

- [ ] **Step 4:** Verify: vitest green, lint clean, tsc clean.

---

## Task 3: Extract ruleSummary helper (TDD)

**Files:**
- Create: `mobile-pwa/src/lib/ruleSummary.ts`
- Create: `mobile-pwa/src/lib/ruleSummary.test.ts`
- Modify: `mobile-pwa/src/pages/FlagDetailPage.tsx` (replace inline summary with import)

The Phase 4 `FlagDetailPage` has a one-line rule summary inline. Phase 5 needs the same logic from `RuleEditSheet` (so the sheet can show a live preview as the user types). Extract to a helper.

- [ ] **Step 1 (test first):** Tests for each rule type matching the spec table:
  - `percentage` → `{percentage}% rollout`
  - `user_target` → `{n} user IDs` (count of `user_ids`)
  - `attribute` → `{attribute} {operator} {value}` truncated at 40 chars with `…`
  - `segment` → `segment: {segment_id}`
  - `schedule` → `{start_time} – {end_time}`
  - `compound` → `compound (edit on desktop)`
  - Unknown / undefined `rule_type` → fallback to `{attribute} {operator} {value}` (legacy attribute rule shape with no rule_type field).

- [ ] **Step 2:** Implement `ruleSummary(rule: TargetingRule): string`. Pure function, no React.

- [ ] **Step 3:** Replace the inline `summarizeRule` (or whatever it's named) inside `FlagDetailPage.tsx` with an import. Run the existing FlagDetailPage tests and confirm they still pass — extraction must be behavior-preserving.

- [ ] **Step 4:** Verify: vitest green, lint clean, tsc clean.

---

## Task 4: Wire env toggle + per-env default value edit in FlagDetailPage (TDD)

**Files:**
- Modify: `mobile-pwa/src/pages/FlagDetailPage.tsx`
- Modify: `mobile-pwa/src/pages/FlagDetailPage.test.tsx`

Replace the read-only "On"/"Off" pill in each env section header with `<ToggleSwitch>`. Inside the expanded section, replace the current "default value" text with an editable control:
- `flag.flag_type === 'boolean'` → small select (`true` / `false`).
- `string` / `number` → `<input>` with `inputMode="text"` (or `numeric` for number) and `enterKeyHint="done"`. Commits on blur AND on Enter.
- `json` → render the value preview with a "Edit JSON on desktop →" deep-link. NO inline editor.

When a user reads the env state but it has never been written (no row in `envStates`), default `checked` is `false` and default value is `flag.default_value`. After a successful PUT, the local state updates with the returned row.

### Optimistic update pattern

Lift this helper inside the page (or extract to `lib/optimistic.ts` if you prefer):

```ts
async function commitEnvState(
  flagId: string,
  envId: string,
  patch: { enabled?: boolean; value?: unknown },
  apply: (next: Partial<FlagEnvironmentState>) => void,
  revert: (prev: Partial<FlagEnvironmentState>) => void,
  prev: Partial<FlagEnvironmentState>,
) {
  apply(patch);
  try {
    const res = await flagEnvStateApi.set(flagId, envId, patch);
    apply(res); // sync with server response
  } catch (err) {
    revert(prev);
    throw err;
  }
}
```

Surface the error inline near the env header — small red text "couldn't save: <message>" that fades after 4 seconds.

- [ ] **Step 1 (test first):** Tests:
  - Tapping the env toggle calls `flagEnvStateApi.set(flagId, envId, { enabled: true })`. Initially-disabled env, after click → "On" visible.
  - Tapping it back to off calls with `{ enabled: false }`.
  - Editing the default value (boolean select) calls `flagEnvStateApi.set(flagId, envId, { value: 'true' })`.
  - Editing the default value (string input) commits on blur with the typed value.
  - Editing the default value (string input) commits on Enter keypress.
  - On a 500 response, the toggle reverts to its previous state and an error message renders.
  - JSON flag type renders a deep-link, NOT an input.

- [ ] **Step 2:** Implement. Don't auto-expand sections after a toggle — the toggle lives in the header, separate from the expand chevron.

- [ ] **Step 3:** Verify: vitest green, lint clean, tsc clean.

---

## Task 5: Wire per-rule per-env toggle (TDD)

**Files:**
- Modify: `mobile-pwa/src/pages/FlagDetailPage.tsx`
- Modify: `mobile-pwa/src/pages/FlagDetailPage.test.tsx`

Inside each expanded env section, the rule list currently shows ONLY rules whose `RuleEnvironmentState.enabled === true` for that env. Phase 5 changes this: show ALL rules, each with a toggle reflecting its per-env state. Disabled rules render with reduced opacity.

- [ ] **Step 1 (test first):** Tests:
  - In an expanded env section, a rule with `RuleEnvironmentState.enabled === false` for this env is NOW visible (was hidden in Phase 4).
  - Toggling the rule toggle calls `flagsApi.setRuleEnvState(flagId, ruleId, envId, { enabled: next })`.
  - On failure, the toggle reverts.

- [ ] **Step 2:** Implement. The Phase 4 filter (`ruleStates.find(...).enabled === true`) becomes a styling cue (`data-rule-disabled="true"`) instead of a visibility filter.

- [ ] **Step 3:** Verify: vitest green, lint clean, tsc clean.

---

## Task 6: RuleEditSheet — full-screen modal for non-compound rules (TDD)

**Files:**
- Create: `mobile-pwa/src/components/RuleEditSheet.tsx`
- Create: `mobile-pwa/src/components/RuleEditSheet.test.tsx`
- Modify: `mobile-pwa/src/styles/mobile.css` (append sheet styles)

A full-screen iOS-style modal opened from a rule's "Edit" button. Renders different fields per rule type. Compound rules NEVER open this — instead they show "Edit on desktop →".

### Props

```ts
interface RuleEditSheetProps {
  rule: TargetingRule;
  flagId: string;
  open: boolean;
  onClose: () => void;
  onSaved: (updatedRule: TargetingRule) => void;
}
```

### Field map per rule type

| Type | Editable fields |
|------|-----------------|
| `percentage` | `percentage` slider 0–100 + numeric input mirroring it |
| `user_target` | `user_ids` chip-style editor (typed comma or Enter adds; tap chip to remove) |
| `attribute` | `attribute` text input, `operator` `<select>` (eq, neq, contains, starts_with, ends_with, in, gt, gte, lt, lte), `value` text input |
| `segment` | `segment_id` text input (no segment picker — segments aren't fanned in here; user types/pastes id; future polish: list segments) |
| `schedule` | `start_time` + `end_time` `<input type="datetime-local">` |

### UX

- Header: "Edit rule" title + close `✕` chevron + "Save" button (disabled while saving or while form has no changes).
- Body: rule-type-specific fields.
- Footer (or below body, sticky): live-preview text using `ruleSummary({ ...rule, ...edits })`.
- Save → `flagsApi.updateRule(flagId, rule.id, patch)` → on success `onSaved(updated)` then `onClose()`.
- Cancel / `✕` / hardware back → discard local edits; `onClose()`.
- Save errors render inline above the Save button.

- [ ] **Step 1 (test first):** Tests (one per rule type minimum):
  - `percentage`: typing into the numeric input updates the slider; tapping Save calls `flagsApi.updateRule(flagId, ruleId, { percentage: <n> })`.
  - `user_target`: typing "alice,bob" + Enter creates 2 chips; tapping a chip removes it; Save calls `updateRule(..., { user_ids: ['alice', 'bob'] })` (or whichever still exists after remove).
  - `attribute`: changing operator + value calls `updateRule` with the new fields.
  - `segment`: typing a segment id and saving calls `updateRule(..., { segment_id })`.
  - `schedule`: setting start + end calls `updateRule(..., { start_time, end_time })`.
  - Cancel / close discards edits and `onSaved` is NOT called.
  - On a 500 response, the sheet stays open, error renders, no `onSaved`.

- [ ] **Step 2:** Implement. Use a single `<dialog>`-like portal-less full-screen `<div>` overlay (no portal needed — the sheet renders inline at the bottom of `FlagDetailPage`).

- [ ] **Step 3:** Append CSS for `.m-sheet`, `.m-sheet-header`, `.m-sheet-body`, `.m-sheet-footer`. Use `position: fixed; inset: 0;` and `env(safe-area-inset-bottom)` padding.

- [ ] **Step 4:** Verify: vitest green, lint clean, tsc clean.

---

## Task 7: Wire RuleEditSheet + rule reorder into FlagDetailPage (TDD)

**Files:**
- Modify: `mobile-pwa/src/pages/FlagDetailPage.tsx`
- Modify: `mobile-pwa/src/pages/FlagDetailPage.test.tsx`

### A. RuleEditSheet wiring

Each rule row inside an expanded env section gets an "Edit" button (compound rules show "Edit on desktop →" deep-link instead). Tap "Edit" → set `editingRuleId` state → render `<RuleEditSheet open ...>` → on save, replace the rule in local state with the returned one.

### B. Rule reorder (up/down arrows)

Add small `↑` / `↓` buttons next to each rule (NOT in the env section — at the top of the page in a new "Rule order" panel that lists all rules globally, since rules are flag-level not env-level). The reorder UX:
- The page already fetches `rules` ordered by priority ascending.
- Up arrow on rule N swaps its priority with rule N-1; down arrow with N+1.
- A swap fires TWO `flagsApi.updateRule` calls in parallel (`Promise.all`).
- On any failure, revert both rules to their pre-swap priorities.
- "Save order" is implicit — every arrow tap commits.

- [ ] **Step 1 (test first):** Tests:
  - Tapping "Edit" on a non-compound rule opens the sheet (sheet visible).
  - Saving the sheet updates the rule in the rendered list (e.g., the summary text changes).
  - Compound rules show "Edit on desktop →" link, NOT an Edit button.
  - Tapping `↑` on rule 2 (priority 2) swaps with rule 1 (priority 1): two `updateRule` calls fire with the new priorities.
  - Tapping `↓` on the last rule does nothing (button disabled or hidden).
  - Tapping `↑` on the first rule does nothing.

- [ ] **Step 2:** Implement. Place the "Rule order" panel above the env sections, collapsed by default with a count badge ("3 rules"). When expanded, list the rules with arrows + summary. This keeps env sections focused on per-env state.

- [ ] **Step 3:** Verify: vitest green, lint clean, tsc clean.

---

## Task 8: Playwright smoke + initiatives + final verify

**Files:**
- Modify: `mobile-pwa/e2e/smoke.spec.ts`
- Modify: `docs/Current_Initiatives.md`

- [ ] **Step 1:** Extend the existing flags smoke case OR add a new case: log in, navigate into a flag, tap the env toggle, assert the PUT was issued (intercept via `page.route` and assert URL + method). Don't depend on backend round-trip — mock the PUT to return 200.

- [ ] **Step 2:** Update `docs/Current_Initiatives.md`: bump Mobile PWA row to reflect Phase 4 merged (PR #61) and Phase 5 in flight on `feature/mobile-pwa-phase5`. Update the "Last updated" date at the top.

- [ ] **Step 3:** Final verification from `mobile-pwa/`:
  - `npm run lint --silent`
  - `npx tsc --noEmit`
  - `npm run test --silent` — target: 130+ tests, baseline 124.
  - `npm run build --silent` — succeeds; report SW precache count + JS bundle size.
  - Skip `npm run test:e2e` if port 3002 is held (note in PR body).

- [ ] **Step 4:** Push branch, open PR with body matching the Phase 4 PR style. Include lint+test results in a verification table. Note pre-existing CI Lint failures.

---

## Success criteria

- All 8 tasks committed in order on `feature/mobile-pwa-phase5`.
- `npm run lint` clean **before every commit** (this was raised by the user — do not skip it).
- `npx tsc --noEmit` clean.
- Vitest green; new tests added for every new mutation path (target ≥ 25 added: ToggleSwitch ~5, ruleSummary ~7, RuleEditSheet ~6, FlagDetailPage write tests ~7).
- Build succeeds; SW precache regenerates.
- Playwright smoke green when run in CI.
- `docs/Current_Initiatives.md` updated.
- PR opened with detailed body matching the Phase 4 PR style.

## Out of scope (Phase 6 deferrals)

- Offline write blocked modal (`navigator.onLine === false` detection + UX).
- SW stale-while-revalidate cache strategy for reads.
- Stale data badge.
- SW update banner.
- Read-only badge for `viewer`-role users — needs a project-role API surface that doesn't currently exist; deferred.
