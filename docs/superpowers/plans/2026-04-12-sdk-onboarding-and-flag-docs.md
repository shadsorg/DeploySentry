# SDK Onboarding Script & Flag Management Docs Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship a curl-piped setup script that installs the SDK and writes an LLM integration prompt into CLAUDE.md, plus a new in-app `flag-management` docs page covering flag creation (UI/CLI/API), permanent vs temporary flags, targeting rules, and the flag lifecycle.

**Architecture:** A POSIX shell script at `scripts/setup-sdk.sh` detects the project language, installs the appropriate SDK package, and generates a language-aware CLAUDE.md section. A new markdown file at `web/src/docs/flag-management.md` is added to the docs manifest and cross-linked from the getting-started page.

**Tech Stack:** POSIX sh, markdown, Vite `?raw` imports (existing pattern)

**Spec:** `docs/superpowers/specs/2026-04-12-sdk-onboarding-and-flag-docs-design.md`

---

## Pre-flight notes

- The existing `scripts/install.sh` uses `#!/bin/sh` + `set -e` POSIX style — follow the same pattern.
- The docs manifest is at `web/src/docs/index.ts` — new entries go between `getting-started` and `sdks`.
- Markdown files live at `web/src/docs/*.md` and are imported via Vite's `?raw` suffix.
- The setup script is ~200 lines of shell. No tests (it's a script, not a library) — manual verification.

---

## Task 1: Setup script — argument parsing and language detection

**Files:**
- Create: `scripts/setup-sdk.sh`

- [ ] **Step 1: Create the script with argument parsing**

`scripts/setup-sdk.sh`:

```sh
#!/bin/sh
set -e

# DeploySentry SDK Setup Script
# Detects project language, installs the SDK, and writes CLAUDE.md
#
# Usage:
#   curl -sSL https://raw.githubusercontent.com/shadsorg/DeploySentry/main/scripts/setup-sdk.sh | sh -s -- --api-key <key>

DEPLOYSENTRY_API_KEY=""
DEPLOYSENTRY_ENV="production"
DEPLOYSENTRY_PROJECT=""
DEPLOYSENTRY_BASE_URL="https://api.deploysentry.io"

# --- Argument parsing ---

while [ $# -gt 0 ]; do
  case "$1" in
    --api-key)   DEPLOYSENTRY_API_KEY="$2"; shift 2 ;;
    --environment) DEPLOYSENTRY_ENV="$2"; shift 2 ;;
    --project)   DEPLOYSENTRY_PROJECT="$2"; shift 2 ;;
    --base-url)  DEPLOYSENTRY_BASE_URL="$2"; shift 2 ;;
    *)           echo "Unknown option: $1"; exit 1 ;;
  esac
done

if [ -z "$DEPLOYSENTRY_API_KEY" ]; then
  echo "Error: --api-key is required"
  echo "Usage: curl -sSL https://raw.githubusercontent.com/shadsorg/DeploySentry/main/scripts/setup-sdk.sh | sh -s -- --api-key <key>"
  exit 1
fi

if [ -z "$DEPLOYSENTRY_PROJECT" ]; then
  printf "Project slug: "
  read -r DEPLOYSENTRY_PROJECT
  if [ -z "$DEPLOYSENTRY_PROJECT" ]; then
    echo "Error: project slug is required"
    exit 1
  fi
fi

# --- Language detection ---

DETECTED_LANG=""

detect_language() {
  if [ -f "package.json" ]; then
    if grep -qE '"(react|next|@next/)"' package.json 2>/dev/null; then
      DETECTED_LANG="react"
    else
      DETECTED_LANG="node"
    fi
  elif [ -f "go.mod" ]; then
    DETECTED_LANG="go"
  elif [ -f "requirements.txt" ] || [ -f "pyproject.toml" ] || [ -f "setup.py" ]; then
    DETECTED_LANG="python"
  elif [ -f "pom.xml" ] || [ -f "build.gradle" ] || [ -f "build.gradle.kts" ]; then
    DETECTED_LANG="java"
  elif [ -f "Gemfile" ]; then
    DETECTED_LANG="ruby"
  elif [ -f "pubspec.yaml" ]; then
    DETECTED_LANG="flutter"
  fi
}

detect_language

if [ -z "$DETECTED_LANG" ]; then
  echo "Error: Could not detect project language."
  echo "Supported: Node.js (package.json), Go (go.mod), Python (requirements.txt/pyproject.toml),"
  echo "           Java (pom.xml/build.gradle), Ruby (Gemfile), Flutter (pubspec.yaml)"
  exit 1
fi

echo "Detected language: $DETECTED_LANG"
```

- [ ] **Step 2: Make it executable and verify arg parsing**

```bash
chmod +x scripts/setup-sdk.sh
scripts/setup-sdk.sh --api-key test-key --project myproj
```

Expected: prints "Detected language: go" (since the repo root has `go.mod`). Then test missing api-key:

```bash
scripts/setup-sdk.sh 2>&1 | head -3
```

Expected: "Error: --api-key is required"

- [ ] **Step 3: Commit**

```bash
git add scripts/setup-sdk.sh
git commit -m "feat: add setup-sdk.sh with argument parsing and language detection"
```

---

## Task 2: Setup script — SDK installation

**Files:**
- Modify: `scripts/setup-sdk.sh`

- [ ] **Step 1: Add the install_sdk function**

Append after the language detection block in `scripts/setup-sdk.sh`:

```sh
# --- SDK installation ---

install_sdk() {
  echo "Installing DeploySentry SDK for $DETECTED_LANG..."
  case "$DETECTED_LANG" in
    node)
      npm install @deploysentry/node
      ;;
    react)
      npm install @deploysentry/react
      ;;
    go)
      go get github.com/shadsorg/deploysentry-go
      ;;
    python)
      if [ -f "pyproject.toml" ]; then
        pip install deploysentry
      else
        pip install deploysentry
        if ! grep -q "deploysentry" requirements.txt 2>/dev/null; then
          echo "deploysentry" >> requirements.txt
        fi
      fi
      ;;
    java)
      echo ""
      echo "Add the following to your build file:"
      echo ""
      if [ -f "pom.xml" ]; then
        echo "  Maven (pom.xml):"
        echo "    <dependency>"
        echo "      <groupId>io.deploysentry</groupId>"
        echo "      <artifactId>deploysentry</artifactId>"
        echo "      <version>LATEST</version>"
        echo "    </dependency>"
      fi
      if [ -f "build.gradle" ] || [ -f "build.gradle.kts" ]; then
        echo "  Gradle:"
        echo "    implementation 'io.deploysentry:deploysentry:+'"
      fi
      echo ""
      ;;
    ruby)
      if ! grep -q "deploysentry" Gemfile 2>/dev/null; then
        echo "gem 'deploysentry'" >> Gemfile
      fi
      bundle install
      ;;
    flutter)
      flutter pub add deploysentry
      ;;
  esac
  echo "SDK installed."
}

install_sdk
```

- [ ] **Step 2: Test in a temp directory**

```bash
mkdir /tmp/test-setup && cd /tmp/test-setup
echo '{"name":"test","dependencies":{}}' > package.json
/Users/sgamel/git/DeploySentry/scripts/setup-sdk.sh --api-key test --project test 2>&1 | head -10
cd /Users/sgamel/git/DeploySentry && rm -rf /tmp/test-setup
```

Expected: detects "node", runs `npm install @deploysentry/node` (may fail if package doesn't exist on npm — that's OK, the script logic is correct).

- [ ] **Step 3: Commit**

```bash
git add scripts/setup-sdk.sh
git commit -m "feat: add SDK installation to setup-sdk.sh"
```

---

## Task 3: Setup script — CLAUDE.md generation

**Files:**
- Modify: `scripts/setup-sdk.sh`

This is the largest part of the script — the language-aware prompt templates.

- [ ] **Step 1: Add the CLAUDE.md generation function**

Append after `install_sdk` in `scripts/setup-sdk.sh`:

```sh
# --- CLAUDE.md generation ---

write_claude_md() {
  # Check if CLAUDE.md already has a DeploySentry section
  if [ -f "CLAUDE.md" ] && grep -q "## DeploySentry Feature Flags" CLAUDE.md; then
    echo "CLAUDE.md already contains DeploySentry section — skipping."
    return
  fi

  # Language-specific snippets
  case "$DETECTED_LANG" in
    node)
      LANG_LABEL="Node.js"
      INIT_SNIPPET="import { DeploySentryClient } from '@deploysentry/node';
const ds = new DeploySentryClient({
  apiKey: process.env.DS_API_KEY!,
  environment: '${DEPLOYSENTRY_ENV}',
  project: '${DEPLOYSENTRY_PROJECT}',
});
await ds.initialize();"
      WRONG_SNIPPET="if (await ds.boolValue('my-flag', false, ctx)) { newFn(); } else { oldFn(); }"
      REGISTER_SNIPPET="ds.register('createCart', createCartWithMembership, 'membership-lookup');
ds.register('createCart', createCart); // default — always last"
      DISPATCH_SNIPPET="const result = ds.dispatch('createCart', { user_id: user.id })(cart, user);"
      ;;
    react)
      LANG_LABEL="React"
      INIT_SNIPPET="// In app root
import { DeploySentryProvider } from '@deploysentry/react';

<DeploySentryProvider
  apiKey={process.env.DS_API_KEY!}
  baseURL=\"${DEPLOYSENTRY_BASE_URL}\"
  environment=\"${DEPLOYSENTRY_ENV}\"
  project=\"${DEPLOYSENTRY_PROJECT}\"
>
  <App />
</DeploySentryProvider>"
      WRONG_SNIPPET="const isOn = useFlag('my-flag', false);
if (isOn) { return <NewCheckout />; } else { return <OldCheckout />; }"
      REGISTER_SNIPPET="// At app init, outside components
ds.register('createCart', createCartWithMembership, 'membership-lookup');
ds.register('createCart', createCart); // default — always last"
      DISPATCH_SNIPPET="// In a component
const createCart = useDispatch<(items: CartItem[]) => Result>('createCart');
return <button onClick={() => createCart(items)}>Checkout</button>;"
      ;;
    go)
      LANG_LABEL="Go"
      INIT_SNIPPET="client := deploysentry.NewClient(
  deploysentry.WithAPIKey(os.Getenv(\"DS_API_KEY\")),
  deploysentry.WithEnvironment(\"${DEPLOYSENTRY_ENV}\"),
  deploysentry.WithProject(\"${DEPLOYSENTRY_PROJECT}\"),
)
client.Initialize(ctx)"
      WRONG_SNIPPET="if client.BoolValue(ctx, \"my-flag\", false) { newFn() } else { oldFn() }"
      REGISTER_SNIPPET="client.Register(\"createCart\", createCartWithMembership, \"membership-lookup\")
client.Register(\"createCart\", createCart) // default"
      DISPATCH_SNIPPET="fn := client.Dispatch(\"createCart\").(func(Cart, User) Result)
result := fn(cart, user)"
      ;;
    python)
      LANG_LABEL="Python"
      INIT_SNIPPET="from deploysentry import DeploySentryClient
ds = DeploySentryClient(
    api_key=os.environ[\"DS_API_KEY\"],
    environment=\"${DEPLOYSENTRY_ENV}\",
    project=\"${DEPLOYSENTRY_PROJECT}\",
)
ds.initialize()"
      WRONG_SNIPPET="if ds.bool_value(\"my-flag\", False, ctx): new_fn() else: old_fn()"
      REGISTER_SNIPPET="ds.register(\"create_cart\", create_cart_with_membership, flag_key=\"membership-lookup\")
ds.register(\"create_cart\", create_cart)  # default"
      DISPATCH_SNIPPET="result = ds.dispatch(\"create_cart\", ctx)(cart, user)"
      ;;
    java)
      LANG_LABEL="Java"
      INIT_SNIPPET="var ds = new DeploySentryClient(ClientOptions.builder()
    .apiKey(System.getenv(\"DS_API_KEY\"))
    .environment(\"${DEPLOYSENTRY_ENV}\")
    .project(\"${DEPLOYSENTRY_PROJECT}\")
    .build());
ds.initialize();"
      WRONG_SNIPPET="if (ds.boolValue(\"my-flag\", false, ctx)) { newFn(); } else { oldFn(); }"
      REGISTER_SNIPPET="ds.register(\"createCart\", () -> createCartWithMembership(cart, user), \"membership-lookup\");
ds.register(\"createCart\", () -> createCart(cart, user)); // default"
      DISPATCH_SNIPPET="var result = ds.<Result>dispatch(\"createCart\", ctx).get();"
      ;;
    ruby)
      LANG_LABEL="Ruby"
      INIT_SNIPPET="ds = DeploySentry::Client.new(
  api_key: ENV[\"DS_API_KEY\"],
  base_url: \"${DEPLOYSENTRY_BASE_URL}\",
  environment: \"${DEPLOYSENTRY_ENV}\",
  project: \"${DEPLOYSENTRY_PROJECT}\",
)
ds.initialize!"
      WRONG_SNIPPET="if ds.bool_value(\"my-flag\", default: false, context: ctx) then new_fn else old_fn end"
      REGISTER_SNIPPET="ds.register(\"create_cart\", method(:create_cart_with_membership), flag_key: \"membership-lookup\")
ds.register(\"create_cart\", method(:create_cart)) # default"
      DISPATCH_SNIPPET="result = ds.dispatch(\"create_cart\", context: ctx).call(cart, user)"
      ;;
    flutter)
      LANG_LABEL="Flutter / Dart"
      INIT_SNIPPET="final ds = DeploySentryClient(
  apiKey: const String.fromEnvironment('DS_API_KEY'),
  baseUrl: '${DEPLOYSENTRY_BASE_URL}',
  environment: '${DEPLOYSENTRY_ENV}',
  project: '${DEPLOYSENTRY_PROJECT}',
);
await ds.initialize();"
      WRONG_SNIPPET="if (await ds.boolValue('my-flag')) { newFn(); } else { oldFn(); }"
      REGISTER_SNIPPET="ds.register<Result Function(Cart, User)>('createCart', createCartWithMembership, flagKey: 'membership-lookup');
ds.register<Result Function(Cart, User)>('createCart', createCart); // default"
      DISPATCH_SNIPPET="final fn = ds.dispatch<Result Function(Cart, User)>('createCart');
final result = fn(cart, user);"
      ;;
  esac

  # Write the CLAUDE.md content
  cat >> CLAUDE.md << CLAUDEEOF

## DeploySentry Feature Flags

This project uses DeploySentry for feature flag management. Follow these rules when working with feature flags.

### Connection

- **API Key:** Stored in environment variable \`DS_API_KEY\`
- **Environment:** \`${DEPLOYSENTRY_ENV}\`
- **Project:** \`${DEPLOYSENTRY_PROJECT}\`
- **Base URL:** \`${DEPLOYSENTRY_BASE_URL}\`

### SDK Usage (${LANG_LABEL})

\`\`\`
${INIT_SNIPPET}
\`\`\`

### The Register/Dispatch Pattern (REQUIRED)

When implementing changes gated by a feature flag, NEVER use if/else branching:

\`\`\`
${WRONG_SNIPPET}
\`\`\`

Instead, use the register/dispatch pattern:

1. **Duplicate the function** you are changing. Keep the original, create the new version.
2. **Register both** against a shared operation name — flagged handler first, default last:

\`\`\`
${REGISTER_SNIPPET}
\`\`\`

3. **Dispatch at the call site** — the SDK picks the right function:

\`\`\`
${DISPATCH_SNIPPET}
\`\`\`

### Flag Categories and Lifecycle

When creating a flag, ask: **will this ever be retired?**

- **release** — **Temporary.** Ships with a deploy. Must have an expiration date. Retire after full rollout.
- **feature** — **Can be permanent.** Product toggles that different tenants may want on or off indefinitely. Mark \`is_permanent: true\`. These stay in the codebase.
- **experiment** — **Temporary.** A/B tests with a defined end date. Retire after analysis.
- **ops** — **Can be permanent.** Operational toggles (circuit breakers, maintenance mode). Mark permanent if needed.
- **permission** — **Typically permanent.** Role/entitlement gates tied to business logic, not a deploy cycle.

### Creating Flags

Create flags via CLI before writing code:

    deploysentry flags create <key> --type boolean --default false --description "What this flag controls"

For permanent flags:

    deploysentry flags create <key> --type boolean --default false --permanent

For temporary flags with expiration:

    deploysentry flags create <key> --type boolean --default false --expires 2026-09-01

Or via the dashboard at ${DEPLOYSENTRY_BASE_URL}

### Context Requirements

The evaluation context you pass to \`dispatch\` must include the attributes your targeting rules reference:

- \`user_id\` — for user targeting and percentage rollout
- \`session_id\` — for session-sticky rollouts
- Custom attributes (plan, region, role, etc.) — for attribute-based rules

### Retiring a Flag (temporary flags only)

When a temporary flag is fully rolled out (100%, stable):

1. Remove the \`register\` call for the flagged handler
2. Remove the \`register\` call for the default handler
3. Replace the \`dispatch\` call with a direct call to the winning function
4. Delete the losing function
5. Archive the flag: \`deploysentry flags archive <key>\`

Permanent flags (feature, ops, permission) stay registered and dispatched indefinitely. Do not retire them.
CLAUDEEOF

  echo "CLAUDE.md updated with DeploySentry integration prompt."
}

write_claude_md
```

- [ ] **Step 2: Add the summary output**

Append at the end of the script:

```sh
# --- Summary ---

echo ""
echo "✓ SDK installed (${DETECTED_LANG})"
echo "✓ CLAUDE.md updated with DeploySentry integration prompt"
echo ""
echo "Next steps:"
echo "  1. Create a flag:  deploysentry flags create my-flag --type boolean --default false"
echo "  2. Wire up register/dispatch in your code (see CLAUDE.md)"
echo "  3. Read the docs:  ${DEPLOYSENTRY_BASE_URL}/docs/flag-management"
echo ""
```

- [ ] **Step 3: Test the full script end-to-end**

```bash
mkdir /tmp/test-setup && cd /tmp/test-setup
echo '{"name":"test","dependencies":{}}' > package.json
/Users/sgamel/git/DeploySentry/scripts/setup-sdk.sh --api-key ds_test_123 --project my-app --environment staging
cat CLAUDE.md
```

Verify:
- CLAUDE.md exists and contains "## DeploySentry Feature Flags"
- The Node.js snippets are present (since we used package.json)
- Environment says "staging", project says "my-app"

Run again:
```bash
/Users/sgamel/git/DeploySentry/scripts/setup-sdk.sh --api-key ds_test_123 --project my-app
cat CLAUDE.md
```

Verify: "already contains DeploySentry section — skipping" is printed, file is unchanged.

```bash
cd /Users/sgamel/git/DeploySentry && rm -rf /tmp/test-setup
```

- [ ] **Step 4: Commit**

```bash
git add scripts/setup-sdk.sh
git commit -m "feat: add CLAUDE.md generation with language-aware LLM integration prompt"
```

---

## Task 4: Flag management docs page

**Files:**
- Create: `web/src/docs/flag-management.md`

- [ ] **Step 1: Write the flag management page**

`web/src/docs/flag-management.md`:

````markdown
# Flag Management

This page covers creating flags, configuring targeting rules, and managing the flag lifecycle.

## Creating Flags

You can create flags through the dashboard, the CLI, or the API.

### Dashboard

1. Navigate to your project's **Flags** page
2. Click **New Flag**
3. Fill in the fields:

| Field | Required | Description |
|---|---|---|
| Key | Yes | Unique identifier. Use lowercase with hyphens: `membership-lookup`, `dark-mode` |
| Name | Yes | Human-readable name |
| Description | No | What this flag controls and why it exists |
| Type | Yes | `boolean`, `string`, `integer`, or `json` |
| Category | Yes | `release`, `feature`, `experiment`, `ops`, or `permission` (see below) |
| Default value | Yes | The value returned when no targeting rules match |
| Owners | No | Teams or individuals responsible for this flag |
| Tags | No | Comma-separated labels for filtering: `checkout, membership` |
| Permanent | No | Check for long-lived flags; uncheck to set an expiration date |
| Expires at | Conditional | Required for non-permanent flags. When this flag should be retired |

### CLI

```bash
# Create a temporary release flag with expiration
deploysentry flags create membership-lookup \
  --type boolean \
  --default false \
  --description "Look up customer membership during cart creation" \
  --tag checkout --tag membership \
  --expires 2026-09-01

# Create a permanent feature flag
deploysentry flags create dark-mode \
  --type boolean \
  --default false \
  --description "Enable dark mode for the dashboard" \
  --permanent

# List flags
deploysentry flags list
deploysentry flags list --tag checkout

# Get flag details
deploysentry flags get membership-lookup

# Toggle a flag on in production
deploysentry flags toggle membership-lookup --on --env production

# Toggle a flag off
deploysentry flags toggle membership-lookup --off --env production
```

### API

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
    "description": "Look up customer membership during cart creation",
    "is_permanent": false,
    "expires_at": "2026-09-01T00:00:00Z",
    "tags": ["checkout", "membership"],
    "project_id": "YOUR_PROJECT_ID",
    "environment_id": "YOUR_ENVIRONMENT_ID"
  }'
```

## Flag Categories

Choose the category that matches the flag's purpose and expected lifecycle:

| Category | Lifetime | Use when |
|---|---|---|
| **release** | Temporary | Shipping new code dark. Must have an expiration date. Retire after rollout. |
| **feature** | Permanent or temporary | Product toggles. Some tenants want it, others don't. Mark permanent if it stays. |
| **experiment** | Temporary | A/B tests. Set an end date. Retire after analysis. |
| **ops** | Permanent or temporary | Circuit breakers, maintenance mode, rate limiters. Often permanent. |
| **permission** | Typically permanent | Role or entitlement gates. Tied to business rules, not deploy cycles. |

## Permanent vs Temporary Flags

When creating a flag, ask: **will this ever be fully rolled out and deleted?**

### Temporary flags (set an expiration)

- **Release flags** — code ships dark, the flag controls the rollout. Once at 100% and validated, retire the flag and the dead code path.
- **Experiment flags** — A/B tests with a defined analysis window. Retire after results are in.
- **Bug fix flags** — gate a fix for gradual rollout, retire once validated.

The [register/dispatch pattern](/docs/sdks#register--dispatch) makes retirement straightforward:
1. Remove the `register` calls for the flagged and default handlers
2. Replace the `dispatch` call with a direct call to the winning function
3. Delete the losing function
4. Archive the flag: `deploysentry flags archive <key>`

### Permanent flags (mark `is_permanent: true`)

- **Feature flags** — product toggles that some tenants want and others don't. These stay in the codebase indefinitely. The register/dispatch pattern keeps them organized — every handler is still registered in one place.
- **Ops flags** — circuit breakers, maintenance mode switches. Need to stay flippable at runtime forever.
- **Permission flags** — role or entitlement gates. Tied to business rules, not deploy cycles.

Permanent flags stay registered and dispatched indefinitely. Do not retire them.

CLI:
```bash
# Permanent
deploysentry flags create dark-mode --type boolean --default false --permanent

# Temporary with expiration
deploysentry flags create new-checkout --type boolean --default false --expires 2026-09-01
```

## Targeting Rules

Targeting rules control which users see which flag value. Add rules via the dashboard (Flag Detail → Targeting Rules tab), the CLI, or the API.

### Percentage rollout

Roll out to a percentage of users. The `user_id` in your evaluation context is the bucketing key.

```bash
deploysentry flags update membership-lookup \
  --add-rule '{"rule_type": "percentage", "percentage": 10}'
```

### User targeting

Enable for specific user IDs.

```bash
deploysentry flags update membership-lookup \
  --add-rule '{"rule_type": "user_target", "target_values": ["user-123", "user-456"]}'
```

### Attribute rules

Enable based on a context attribute. Supported operators: `eq`, `neq`, `contains`, `starts_with`, `ends_with`, `in`, `not_in`, `gt`, `gte`, `lt`, `lte`.

```bash
# Enable for users on the "pro" plan
deploysentry flags update membership-lookup \
  --add-rule '{"rule_type": "attribute", "attribute": "plan", "operator": "eq", "value": "pro"}'
```

### Segment rules

Enable for a pre-defined user segment.

```bash
deploysentry flags update membership-lookup \
  --add-rule '{"rule_type": "segment", "segment_id": "SEGMENT_UUID"}'
```

### Schedule rules

Enable between two timestamps.

```bash
deploysentry flags update membership-lookup \
  --add-rule '{"rule_type": "schedule", "start_time": "2026-06-01T00:00:00Z", "end_time": "2026-07-01T00:00:00Z"}'
```

### Compound rules

Combine multiple conditions with AND or OR.

```bash
# Enable for pro users in the US
deploysentry flags update membership-lookup \
  --add-rule '{
    "rule_type": "compound",
    "combine_op": "AND",
    "conditions": [
      {"attribute": "plan", "operator": "eq", "value": "pro"},
      {"attribute": "region", "operator": "eq", "value": "us"}
    ]
  }'
```

## Wiring Flags to Code

After creating a flag, wire it into your code using the [register/dispatch pattern](/docs/sdks#register--dispatch):

1. **Create the flag** in the dashboard or CLI
2. **Register handlers** in your code — the flag key in `register('op', handler, 'flag-key')` must match the key you created
3. **Configure targeting rules** to control who sees what
4. **Dispatch** at call sites — the SDK evaluates the flag and returns the right function

See [SDKs](/docs/sdks) for language-specific examples.

## Flag Lifecycle

### Create

Define the flag with a key, type, category, and default value. Set permanent or add an expiration.

### Target

Add targeting rules to control rollout: percentage, user targeting, attributes, segments, schedules, or compound rules.

### Roll out

Gradually increase the percentage or expand targeting. Monitor metrics and errors at each stage.

### Observe

Use the Analytics page to watch per-flag evaluation counts, error rates, and rollout health.

### Retire (temporary flags only)

When a temporary flag is fully rolled out and stable:
1. Remove the `register` calls for both handlers
2. Replace the `dispatch` call with a direct call to the winning function
3. Delete the losing function
4. Archive: `deploysentry flags archive <key>`

Permanent flags skip this step — they stay in the codebase.
````

- [ ] **Step 2: Verify the markdown renders correctly**

Visually scan the file for broken code fences, unclosed tables, and formatting issues. All code blocks should use triple backticks with a language hint.

- [ ] **Step 3: Commit**

```bash
git add web/src/docs/flag-management.md
git commit -m "docs(web): add flag-management page covering creation, targeting, and lifecycle"
```

---

## Task 5: Wire flag-management into the docs manifest

**Files:**
- Modify: `web/src/docs/index.ts`
- Modify: `web/src/docs/getting-started.md`

- [ ] **Step 1: Add flag-management to the manifest**

In `web/src/docs/index.ts`, add the import and manifest entry. The new entry goes between `getting-started` and `sdks`:

```ts
import gettingStarted from './getting-started.md?raw';
import flagManagement from './flag-management.md?raw';
import sdks from './sdks.md?raw';
import cli from './cli.md?raw';
import uiFeatures from './ui-features.md?raw';

export type DocEntry = {
  slug: string;
  title: string;
  source: string;
};

export const docsManifest: readonly DocEntry[] = [
  { slug: 'getting-started',  title: 'Getting Started',  source: gettingStarted },
  { slug: 'flag-management',  title: 'Flag Management',  source: flagManagement },
  { slug: 'sdks',             title: 'SDKs',             source: sdks },
  { slug: 'cli',              title: 'CLI',              source: cli },
  { slug: 'ui-features',      title: 'UI Features',      source: uiFeatures },
] as const;

export function findDoc(slug: string): DocEntry | undefined {
  return docsManifest.find((d) => d.slug === slug);
}
```

- [ ] **Step 2: Add a cross-link from getting-started.md**

In `web/src/docs/getting-started.md`, replace the "Create your first flag" section (lines 29-31):

```markdown
## Create your first flag

From the project's Flags page, click **New Flag**. Pick a category (release, feature, experiment, ops, or permission) and define a key.

For detailed guidance on flag creation, targeting rules, permanent vs temporary flags, and the full lifecycle, see [Flag Management](/docs/flag-management).
```

- [ ] **Step 3: Run lint and dev server**

```bash
cd web
npm run lint
npm run dev
```

Visit `http://localhost:3001/docs/flag-management` and verify:
- The page renders with all sections
- Code blocks are syntax-highlighted
- Internal links (`/docs/sdks#register--dispatch`) navigate correctly
- The getting-started page links to flag-management

Stop dev server.

- [ ] **Step 4: Commit**

```bash
git add web/src/docs/index.ts web/src/docs/getting-started.md
git commit -m "docs(web): wire flag-management into docs manifest and cross-link from getting-started"
```

---

## Task 6: Final verification and completion

**Files:**
- Modify: `docs/Current_Initiatives.md`

- [ ] **Step 1: Test the setup script one more time**

```bash
mkdir /tmp/test-final && cd /tmp/test-final
echo '{"name":"test","dependencies":{"react":"^18"}}' > package.json
/Users/sgamel/git/DeploySentry/scripts/setup-sdk.sh --api-key ds_test --project demo --environment staging --base-url http://localhost:8080
echo "---REACT CHECK---"
grep "React" CLAUDE.md | head -3
echo "---PERMANENT CHECK---"
grep "permanent" CLAUDE.md | head -3
echo "---IDEMPOTENT CHECK---"
/Users/sgamel/git/DeploySentry/scripts/setup-sdk.sh --api-key ds_test --project demo 2>&1 | grep "skipping"
cd /Users/sgamel/git/DeploySentry && rm -rf /tmp/test-final
```

Expected:
- React snippets present (detected from package.json react dependency)
- "permanent" mentioned in the flag categories section
- Second run prints "already contains DeploySentry section — skipping"

- [ ] **Step 2: Run web lint + tests**

```bash
cd web
npm run lint
npm test
```

Expected: lint clean, all tests pass.

- [ ] **Step 3: Update Current Initiatives**

Add to `docs/Current_Initiatives.md` completed section:

```markdown
| SDK Onboarding & Flag Docs | [Plan](./superpowers/plans/2026-04-12-sdk-onboarding-and-flag-docs.md) / [Spec](./superpowers/specs/2026-04-12-sdk-onboarding-and-flag-docs-design.md) | Setup script + CLAUDE.md prompt + flag-management docs |
```

- [ ] **Step 4: Commit**

```bash
git add docs/Current_Initiatives.md
git commit -m "docs: mark SDK onboarding and flag management docs complete"
```

---

## Self-review notes

**Spec coverage check:**
- Setup script with arg parsing: Task 1 ✓
- Language detection (all 7 languages + React variant): Task 1 ✓
- SDK installation per language: Task 2 ✓
- CLAUDE.md generation with language-aware snippets: Task 3 ✓
- Idempotent CLAUDE.md writes: Task 3 ✓
- Summary output with next steps: Task 3 ✓
- POSIX sh compatible: Tasks 1-3 (no bash-isms) ✓
- LLM prompt template — connection, init, register/dispatch, wrong example: Task 3 ✓
- Flag categories with permanent/temporary lifecycle guidance: Task 3 (CLAUDE.md) + Task 4 (docs) ✓
- Context requirements in prompt: Task 3 ✓
- Retirement workflow (5 steps, temporary only): Task 3 + Task 4 ✓
- Permanent flags guidance: Task 3 + Task 4 ✓
- Flag-management.md — creating flags (UI/CLI/API): Task 4 ✓
- Flag-management.md — all 6 targeting rule types with CLI examples: Task 4 ✓
- Flag-management.md — permanent vs temporary section: Task 4 ✓
- Flag-management.md — flag lifecycle: Task 4 ✓
- Flag-management.md — wiring to register/dispatch: Task 4 ✓
- Docs manifest update: Task 5 ✓
- Cross-link from getting-started: Task 5 ✓
- All 7 language snippet templates (Node, React, Go, Python, Java, Ruby, Flutter): Task 3 ✓

**Placeholder scan:** No TBD, TODO, or "fill in later" found.

**Type/name consistency:**
- `DEPLOYSENTRY_API_KEY`, `DEPLOYSENTRY_ENV`, `DEPLOYSENTRY_PROJECT`, `DEPLOYSENTRY_BASE_URL` — consistent across all script functions ✓
- `DETECTED_LANG` values: node, react, go, python, java, ruby, flutter — consistent between detect, install, and CLAUDE.md functions ✓
- `flag-management` slug — used in manifest (Task 5), getting-started link (Task 5), and summary output (Task 3) ✓
