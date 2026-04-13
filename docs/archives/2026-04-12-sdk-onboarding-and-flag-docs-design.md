# SDK Onboarding Script, LLM Integration Prompt & Flag Management Docs — Design Spec

**Date:** 2026-04-12
**Status:** Approved (brainstorming complete, ready for implementation plan)
**Related plans:** TBD (writing-plans next)

## Overview

Three deliverables that together give a developer a one-command path from "I want to use DeploySentry" to "my LLM knows how to use it and I know how to create flags":

1. **`scripts/setup-sdk.sh`** — a curl-piped shell script that detects the project language, installs the SDK, and writes an LLM integration prompt into `CLAUDE.md`.
2. **LLM integration prompt template** — the text the script writes into `CLAUDE.md`, teaching an LLM the register/dispatch pattern, flag categories, context requirements, and retirement workflow. Language-aware (the script fills in SDK-specific snippets).
3. **`flag-management.md`** — a new in-app docs page covering flag creation (UI, CLI, API), permanent vs temporary flags, all 6 targeting rule types with examples, and the flag lifecycle.

## Goals

- A developer can go from zero to "SDK installed, LLM briefed, first flag created" in under 5 minutes.
- LLMs working in the codebase know the register/dispatch pattern without the developer having to explain it every session.
- Flag creation guidance is complete and reachable from inside the app — not scattered across READMEs.

## Non-goals

- No API-served personalized setup endpoint (follow-up).
- No changes to the SDK code itself (already implemented).
- No changes to the dashboard UI or CLI (they already support flag creation).
- No new CLI subcommand for the prompt (the shell script handles it).

## Decisions locked in brainstorming

| # | Decision | Choice |
|---|---|---|
| 1 | Script hosting | Static shell script at `scripts/setup-sdk.sh`, curled from GitHub raw URL |
| 2 | Flag docs structure | One new page `flag-management.md` in the in-app docs |
| 3 | Permanent vs temporary | Explicit guidance in both CLAUDE.md prompt and docs page |

---

## Deliverable 1: Setup Script (`scripts/setup-sdk.sh`)

### Invocation

```bash
curl -sSL https://raw.githubusercontent.com/shadsorg/DeploySentry/main/scripts/setup-sdk.sh | sh -s -- --api-key ds_live_abc123
```

### Arguments

| Arg | Required | Default | Description |
|---|---|---|---|
| `--api-key <key>` | Yes | — | DeploySentry API key |
| `--environment <env>` | No | `production` | Environment name |
| `--project <slug>` | No | prompted | Project slug |
| `--base-url <url>` | No | `https://api.deploysentry.io` | API base URL |

### Behavior (in order)

1. **Parse arguments.** Validate `--api-key` is present. If `--project` is missing, prompt interactively.

2. **Detect language** by scanning the current directory:
   - `package.json` exists → **Node**. If `package.json` contains `react` or `next` in dependencies → **React** variant.
   - `go.mod` exists → **Go**
   - `requirements.txt` or `pyproject.toml` or `setup.py` exists → **Python**
   - `pom.xml` or `build.gradle` exists → **Java**
   - `Gemfile` exists → **Ruby**
   - `pubspec.yaml` exists → **Flutter**
   - Multiple matches → prompt user to choose.
   - No match → print error with list of supported languages and exit.

3. **Install the SDK** using the detected package manager:
   - Node: `npm install @deploysentry/node`
   - React: `npm install @deploysentry/react`
   - Go: `go get github.com/shadsorg/deploysentry-go`
   - Python: `pip install deploysentry`
   - Java: print Maven/Gradle snippet (can't auto-install)
   - Ruby: add to Gemfile + `bundle install`
   - Flutter: `flutter pub add deploysentry`

4. **Write CLAUDE.md.** If the file doesn't exist, create it with the full prompt. If it exists, check for an existing `## DeploySentry` section — skip if found (idempotent), otherwise append the section.

5. **Print next steps:**
   ```
   ✓ SDK installed (@deploysentry/node)
   ✓ CLAUDE.md updated with DeploySentry integration prompt

   Next:
   1. Create a flag:  deploysentry flags create my-flag --type boolean --default false
   2. Wire up register/dispatch in your code (see CLAUDE.md for the pattern)
   3. Read the docs: http://localhost:3001/docs/flag-management
   ```

### Constraints

- POSIX sh compatible — no bash-isms (`[[`, arrays, etc.). Works on macOS sh, Ubuntu dash, Alpine ash.
- No destructive operations — never overwrites existing CLAUDE.md content.
- Idempotent — running twice doesn't duplicate the SDK install or CLAUDE.md section.
- ~200 lines estimated.

---

## Deliverable 2: LLM Integration Prompt (CLAUDE.md content)

This is the text the setup script writes. It's a template with `{placeholders}` the script fills in based on detected language and provided arguments.

### Template

```markdown
## DeploySentry Feature Flags

This project uses DeploySentry for feature flag management. Follow these rules when working with feature flags.

### Connection

- **API Key:** Stored in environment variable `DS_API_KEY`
- **Environment:** `{environment}`
- **Project:** `{project}`
- **Base URL:** `{baseUrl}`

### SDK Usage ({language})

{language_init_snippet}

### The Register/Dispatch Pattern (REQUIRED)

When implementing changes gated by a feature flag, NEVER use if/else branching:

❌ WRONG — scattered, hard to find, impossible to clean up:
{language_wrong_example}

Instead, use the register/dispatch pattern:

1. **Duplicate the function** you're changing. Keep the original, create the new version.
2. **Register both** against a shared operation name — flagged handler first, default last:

{language_register_snippet}

3. **Dispatch at the call site** — the SDK picks the right function:

{language_dispatch_snippet}

### Flag Categories and Lifecycle

When creating a flag, ask: **will this ever be retired?**

- **release** — **Temporary.** Ships with a deploy. Must have an expiration date. Retire after full rollout. Example: shipping a new checkout flow dark.
- **feature** — **Can be permanent.** Product toggles that different tenants or users may want on or off indefinitely. Mark `is_permanent: true`. These stay in the codebase — the register/dispatch pattern keeps them clean. Example: dark mode, advanced reporting.
- **experiment** — **Temporary.** A/B tests with a defined end date. Retire after analysis. Example: testing two onboarding flows.
- **ops** — **Can be permanent.** Operational toggles like circuit breakers and maintenance mode. Mark permanent if they need to stay. Example: kill switch for an external API.
- **permission** — **Typically permanent.** Role and entitlement gates tied to business logic, not a deploy cycle. Example: premium-only features.

### Creating Flags

Create flags via CLI before writing code:

    deploysentry flags create <key> --type boolean --default false --description "What this flag controls"

For permanent flags:

    deploysentry flags create <key> --type boolean --default false --permanent

For temporary flags with expiration:

    deploysentry flags create <key> --type boolean --default false --expires 2026-09-01

Or via the dashboard at {baseUrl}

### Context Requirements

The evaluation context you pass to `dispatch` must include the attributes your targeting rules reference. Common attributes:

- `user_id` — for user targeting and percentage rollout
- `session_id` — for session-sticky rollouts
- Custom attributes (plan, region, role, etc.) — for attribute-based rules

### Retiring a Flag (temporary flags only)

When a temporary flag is fully rolled out (100%, stable):

1. Remove the `register` call for the flagged handler
2. Remove the `register` call for the default handler
3. Replace the `dispatch` call with a direct call to the winning function
4. Delete the losing function
5. Archive the flag: `deploysentry flags archive <key>`

Permanent flags (feature, ops, permission) stay registered and dispatched indefinitely. Do not retire them.
```

### Language-specific snippets

The script holds a template per language. Each template provides:
- `{language_init_snippet}` — SDK initialization code
- `{language_wrong_example}` — the if/else anti-pattern
- `{language_register_snippet}` — register call example
- `{language_dispatch_snippet}` — dispatch call example

**Node example:**
```ts
// Init
import { DeploySentryClient } from '@deploysentry/node';
const ds = new DeploySentryClient({
  apiKey: process.env.DS_API_KEY!,
  environment: '{environment}',
  project: '{project}',
});
await ds.initialize();

// ❌ Wrong
if (await ds.boolValue('membership-lookup', false, ctx)) {
  createCartWithMembership(cart, user);
} else {
  createCart(cart, user);
}

// Register
ds.register('createCart', createCartWithMembership, 'membership-lookup');
ds.register('createCart', createCart); // default — always last

// Dispatch
const result = ds.dispatch('createCart', { user_id: user.id })(cart, user);
```

**Go example:**
```go
// Init
client := deploysentry.NewClient(
  deploysentry.WithAPIKey(os.Getenv("DS_API_KEY")),
  deploysentry.WithEnvironment("{environment}"),
  deploysentry.WithProject("{project}"),
)
client.Initialize(ctx)

// ❌ Wrong
if client.BoolValue(ctx, "membership-lookup", false) {
  createCartWithMembership(cart, user)
} else {
  createCart(cart, user)
}

// Register
client.Register("createCart", createCartWithMembership, "membership-lookup")
client.Register("createCart", createCart) // default

// Dispatch
fn := client.Dispatch("createCart").(func(Cart, User) Result)
result := fn(cart, user)
```

**Python example:**
```python
# Init
from deploysentry import DeploySentryClient
ds = DeploySentryClient(
    api_key=os.environ["DS_API_KEY"],
    environment="{environment}",
    project="{project}",
)
ds.initialize()

# ❌ Wrong
if ds.bool_value("membership-lookup", False, ctx):
    create_cart_with_membership(cart, user)
else:
    create_cart(cart, user)

# Register
ds.register("create_cart", create_cart_with_membership, flag_key="membership-lookup")
ds.register("create_cart", create_cart)  # default

# Dispatch
result = ds.dispatch("create_cart", ctx)(cart, user)
```

**Java example:**
```java
// Init
var ds = new DeploySentryClient(ClientOptions.builder()
    .apiKey(System.getenv("DS_API_KEY"))
    .environment("{environment}")
    .project("{project}")
    .build());
ds.initialize();

// ❌ Wrong
if (ds.boolValue("membership-lookup", false, ctx)) {
    createCartWithMembership(cart, user);
} else {
    createCart(cart, user);
}

// Register
ds.register("createCart", () -> createCartWithMembership(cart, user), "membership-lookup");
ds.register("createCart", () -> createCart(cart, user)); // default

// Dispatch
var result = ds.<Result>dispatch("createCart", ctx).get();
```

**Ruby example:**
```ruby
# Init
ds = DeploySentry::Client.new(
  api_key: ENV["DS_API_KEY"],
  base_url: "{baseUrl}",
  environment: "{environment}",
  project: "{project}",
)
ds.initialize!

# ❌ Wrong
if ds.bool_value("membership-lookup", default: false, context: ctx)
  create_cart_with_membership(cart, user)
else
  create_cart(cart, user)
end

# Register
ds.register("create_cart", method(:create_cart_with_membership), flag_key: "membership-lookup")
ds.register("create_cart", method(:create_cart)) # default

# Dispatch
result = ds.dispatch("create_cart", context: ctx).call(cart, user)
```

**React example:**
```tsx
// Init (in app root)
import { DeploySentryProvider } from '@deploysentry/react';

<DeploySentryProvider
  apiKey={process.env.DS_API_KEY!}
  baseURL="{baseUrl}"
  environment="{environment}"
  project="{project}"
>
  <App />
</DeploySentryProvider>

// ❌ Wrong (in component)
const isOn = useFlag('membership-lookup', false);
if (isOn) { return <NewCheckout />; } else { return <OldCheckout />; }

// Register (at app init, outside component)
ds.register('createCart', createCartWithMembership, 'membership-lookup');
ds.register('createCart', createCart);

// Dispatch (in component via hook)
const createCart = useDispatch<(items: CartItem[]) => Result>('createCart');
return <button onClick={() => createCart(items)}>Checkout</button>;
```

**Flutter example:**
```dart
// Init
final ds = DeploySentryClient(
  apiKey: const String.fromEnvironment('DS_API_KEY'),
  baseUrl: '{baseUrl}',
  environment: '{environment}',
  project: '{project}',
);
await ds.initialize();

// ❌ Wrong
if (await ds.boolValue('membership-lookup')) {
  createCartWithMembership(cart, user);
} else {
  createCart(cart, user);
}

// Register
ds.register<Result Function(Cart, User)>('createCart', createCartWithMembership, flagKey: 'membership-lookup');
ds.register<Result Function(Cart, User)>('createCart', createCart); // default

// Dispatch
final fn = ds.dispatch<Result Function(Cart, User)>('createCart');
final result = fn(cart, user);
```

---

## Deliverable 3: Flag Management Docs Page (`flag-management.md`)

**New file:** `web/src/docs/flag-management.md`
**Added to manifest:** `web/src/docs/index.ts` — inserted between `getting-started` and `sdks`.
**Cross-link:** `getting-started.md` updated to link to this page.

### Sections

#### 1. Creating Flags

Three methods with complete walkthroughs:

**Dashboard UI:**
- Navigate to project → Flags → New Flag
- Field reference: key (naming conventions: lowercase, hyphens, e.g. `membership-lookup`), name, description, type (boolean/string/integer/json), category, default value, owners, tags
- Permanent checkbox: check for long-lived flags; uncheck to set an expiration date
- Category guidance: one sentence per category (same as CLAUDE.md prompt)

**CLI:**
```bash
# Temporary release flag with expiration
deploysentry flags create membership-lookup \
  --type boolean \
  --default false \
  --description "Look up customer membership during cart creation" \
  --tag checkout --tag membership \
  --expires 2026-09-01

# Permanent feature flag
deploysentry flags create dark-mode \
  --type boolean \
  --default false \
  --description "Enable dark mode for the dashboard" \
  --permanent

# List, get, toggle
deploysentry flags list --tag checkout
deploysentry flags get membership-lookup
deploysentry flags toggle membership-lookup --on --env production
```

**API:**
```bash
curl -X POST https://api.deploysentry.io/api/v1/flags \
  -H "Authorization: Bearer $DS_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "key": "membership-lookup",
    "name": "Membership Lookup",
    "flag_type": "boolean",
    "category": "release",
    "default_value": "false",
    "is_permanent": false,
    "expires_at": "2026-09-01T00:00:00Z",
    "project_id": "...",
    "environment_id": "..."
  }'
```

#### 2. Permanent vs Temporary Flags

Explicit guidance:

**Temporary flags** (set an expiration):
- **release** — code ships dark, flag controls the rollout. Once at 100% and validated, retire the flag and the dead code path. The register/dispatch pattern makes retirement a 5-step process (remove registrations, delete dead function, archive flag).
- **experiment** — A/B test with a defined analysis window. Retire after results are in.
- Bug fix flags — gate a fix for gradual rollout, retire once validated.

**Permanent flags** (mark `is_permanent: true`):
- **feature** — product toggles that some tenants want and others don't. These stay in the codebase. The register/dispatch pattern keeps them organized — every handler is still registered in one place.
- **ops** — circuit breakers, maintenance mode, rate limiters. Need to stay flippable at runtime forever.
- **permission** — role or entitlement gates. Tied to business rules, not deploy cycles.

Rule of thumb: *"If you can imagine a date when this flag will be fully rolled out and deleted, it's temporary. If different users will always see different behavior, it's permanent."*

#### 3. Targeting Rules

One subsection per rule type with a concrete example:

- **Percentage rollout** — roll out to N% of users. The `user_id` in context is the bucketing key. CLI: `--add-rule '{"rule_type":"percentage","percentage":10}'`
- **User targeting** — enable for specific user IDs. CLI: `--add-rule '{"rule_type":"user_target","target_values":["u123","u456"]}'`
- **Attribute rules** — enable based on a context attribute. Operators: eq, neq, contains, starts_with, ends_with, in, not_in, gt, gte, lt, lte. CLI: `--add-rule '{"rule_type":"attribute","attribute":"plan","operator":"eq","value":"pro"}'`
- **Segment rules** — enable for a pre-defined segment. CLI: `--add-rule '{"rule_type":"segment","segment_id":"..."}'`
- **Schedule rules** — enable between two timestamps. CLI: `--add-rule '{"rule_type":"schedule","start_time":"...","end_time":"..."}'`
- **Compound rules** — multiple conditions with AND/OR. CLI: `--add-rule '{"rule_type":"compound","combine_op":"AND","conditions":[{"attribute":"plan","operator":"eq","value":"pro"},{"attribute":"region","operator":"eq","value":"us"}]}'`

#### 4. Wiring to Register/Dispatch

Short section: create the flag first, then register handlers in code, then configure targeting. The flag key in `register('op', handler, 'flag-key')` must match the key you created. Links to `sdks.md#register--dispatch` for full language examples.

#### 5. Flag Lifecycle

Linear flow: **Create → Target → Roll out → Observe → Retire (if temporary)**

One paragraph per step. Retirement links to the CLAUDE.md prompt's 5-step retirement checklist. Permanent flags skip the retirement step — they stay registered and dispatched indefinitely.

Mentions `deploysentry flags archive <key>` for cleanup.

---

## Files created or modified

| File | Action | Description |
|---|---|---|
| `scripts/setup-sdk.sh` | Create | Onboarding shell script |
| `web/src/docs/flag-management.md` | Create | Flag management docs page |
| `web/src/docs/index.ts` | Modify | Add `flag-management` to manifest |
| `web/src/docs/getting-started.md` | Modify | Add link to flag-management page |

## Testing

- **Script:** Manual test — run in a temp directory with a `package.json`, verify SDK installs and CLAUDE.md is written correctly. Run again, verify idempotent (no duplicate section).
- **Docs page:** `npm run dev` → navigate to `/docs/flag-management`, verify all sections render, code blocks highlight, internal links work.
- **CLAUDE.md content:** paste the generated output into a fresh Claude Code session and ask it to "create a feature flag for dark mode using register/dispatch" — verify it follows the pattern correctly.

## Follow-up items

- API-served personalized setup endpoint (`GET /api/v1/setup?token=...`) that embeds the user's API key and project slug.
- `deploysentry integration prompt` CLI subcommand that prints the CLAUDE.md content to stdout.
