#!/bin/sh
set -e

# DeploySentry SDK Setup Script
# Detects project language, installs the appropriate SDK, and writes an LLM
# integration prompt into CLAUDE.md

# ---------------------------------------------------------------------------
# Argument parsing
# ---------------------------------------------------------------------------

DS_API_KEY=""
DS_ENVIRONMENT="production"
DS_PROJECT=""
DS_BASE_URL="https://api.deploysentry.io"

usage() {
  echo "Usage: $0 --api-key <key> [--environment <env>] [--project <slug>] [--base-url <url>]" >&2
  exit 1
}

while [ $# -gt 0 ]; do
  case "$1" in
    --api-key)
      DS_API_KEY="$2"
      shift 2
      ;;
    --environment)
      DS_ENVIRONMENT="$2"
      shift 2
      ;;
    --project)
      DS_PROJECT="$2"
      shift 2
      ;;
    --base-url)
      DS_BASE_URL="$2"
      shift 2
      ;;
    --help|-h)
      usage
      ;;
    *)
      echo "Error: Unknown argument: $1" >&2
      usage
      ;;
  esac
done

if [ -z "$DS_API_KEY" ]; then
  echo "Error: --api-key is required" >&2
  usage
fi

if [ -z "$DS_PROJECT" ]; then
  printf "Enter project slug: "
  read -r DS_PROJECT
  if [ -z "$DS_PROJECT" ]; then
    echo "Error: project slug is required" >&2
    exit 1
  fi
fi

# ---------------------------------------------------------------------------
# Language detection
# ---------------------------------------------------------------------------

detect_language() {
  # Flutter — pubspec.yaml
  if [ -f "pubspec.yaml" ]; then
    echo "flutter"
    return
  fi

  # Java/Kotlin — pom.xml or build.gradle
  if [ -f "pom.xml" ] || [ -f "build.gradle" ] || [ -f "build.gradle.kts" ]; then
    echo "java"
    return
  fi

  # Ruby — Gemfile
  if [ -f "Gemfile" ]; then
    echo "ruby"
    return
  fi

  # Go — go.mod
  if [ -f "go.mod" ]; then
    echo "go"
    return
  fi

  # Python — requirements.txt, pyproject.toml, or setup.py
  if [ -f "requirements.txt" ] || [ -f "pyproject.toml" ] || [ -f "setup.py" ]; then
    echo "python"
    return
  fi

  # Node / React — package.json
  if [ -f "package.json" ]; then
    # Check for react or next in dependencies
    if grep -qE '"(react|next)"' package.json 2>/dev/null; then
      echo "react"
    else
      echo "node"
    fi
    return
  fi

  echo ""
}

LANG_DETECTED=$(detect_language)

if [ -z "$LANG_DETECTED" ]; then
  echo "Error: Could not detect project language." >&2
  echo "Supported languages: node, react, go, python, java, ruby, flutter" >&2
  echo "Make sure you are running this script from your project root directory." >&2
  exit 1
fi

echo "Detected language: $LANG_DETECTED"

# ---------------------------------------------------------------------------
# SDK installation
# ---------------------------------------------------------------------------

install_sdk() {
  case "$LANG_DETECTED" in
    node)
      echo "Installing @deploysentry/node..."
      npm install @deploysentry/node
      ;;
    react)
      echo "Installing @deploysentry/react..."
      npm install @deploysentry/react
      ;;
    go)
      echo "Installing deploysentry-go..."
      go get github.com/shadsorg/deploysentry-go
      ;;
    python)
      echo "Installing deploysentry (pip)..."
      pip install deploysentry
      if [ -f "requirements.txt" ]; then
        if ! grep -q "^deploysentry" requirements.txt 2>/dev/null; then
          echo "deploysentry" >> requirements.txt
          echo "Added deploysentry to requirements.txt"
        fi
      fi
      ;;
    java)
      echo ""
      echo "Java/Kotlin detected. Add the following dependency to your build file:"
      echo ""
      echo "Maven (pom.xml):"
      echo "  <dependency>"
      echo "    <groupId>io.deploysentry</groupId>"
      echo "    <artifactId>deploysentry-java</artifactId>"
      echo "    <version>LATEST</version>"
      echo "  </dependency>"
      echo ""
      echo "Gradle (build.gradle):"
      echo "  implementation 'io.deploysentry:deploysentry-java:LATEST'"
      echo ""
      ;;
    ruby)
      if [ -f "Gemfile" ]; then
        if ! grep -q "deploysentry" Gemfile 2>/dev/null; then
          echo "gem 'deploysentry'" >> Gemfile
          echo "Added deploysentry gem to Gemfile"
        fi
      fi
      echo "Running bundle install..."
      bundle install
      ;;
    flutter)
      echo "Adding deploysentry Flutter package..."
      flutter pub add deploysentry
      ;;
  esac
}

install_sdk || echo "Warning: SDK installation failed. You may need to install it manually. Continuing..."

# ---------------------------------------------------------------------------
# CLAUDE.md generation
# ---------------------------------------------------------------------------

write_claude_md() {
  if [ -f "CLAUDE.md" ] && grep -q "## DeploySentry Feature Flags" CLAUDE.md 2>/dev/null; then
    echo "CLAUDE.md already contains DeploySentry section — skipping."
    return
  fi

  # Build language-specific snippet variables.
  # Variables are assigned here and expanded into printf calls below.
  case "$LANG_DETECTED" in
    node)
      INIT_SNIPPET="import { DeploySentryClient } from '@deploysentry/node';

const ds = new DeploySentryClient({
  apiKey: process.env.DS_API_KEY!,
  environment: '${DS_ENVIRONMENT}',
  project: '${DS_PROJECT}',
  baseUrl: '${DS_BASE_URL}',
});
await ds.initialize();"

      WRONG_SNIPPET="// WRONG — direct boolean check, no register/dispatch
if (await ds.boolValue('my-flag', false, ctx)) {
  newFn();
} else {
  oldFn();
}"

      REGISTER_SNIPPET="// Register implementations (at startup)
ds.register('createCart', createCartWithMembership, 'membership-lookup');
ds.register('createCart', createCart); // default — always last"

      DISPATCH_SNIPPET="// Dispatch at call site
const result = ds.dispatch('createCart', { user_id: user.id })(cart, user);"
      ;;

    react)
      INIT_SNIPPET="// Wrap your app with DeploySentryProvider
import { DeploySentryProvider } from '@deploysentry/react';

<DeploySentryProvider
  apiKey={process.env.REACT_APP_DS_API_KEY}
  baseURL='${DS_BASE_URL}'
  environment='${DS_ENVIRONMENT}'
  project='${DS_PROJECT}'
>
  <App />
</DeploySentryProvider>"

      WRONG_SNIPPET="// WRONG — direct boolean check via hook, no register/dispatch
const isOn = useFlag('my-flag', false);
if (isOn) { ... } else { ... }"

      REGISTER_SNIPPET="// Register implementations at app init (outside components)
ds.register('createCart', createCartWithMembership, 'membership-lookup');
ds.register('createCart', createCart); // default — always last"

      DISPATCH_SNIPPET="// Dispatch via hook inside a component
const createCart = useDispatch<(items: CartItem[]) => Result>('createCart');"
      ;;

    go)
      INIT_SNIPPET='import "github.com/shadsorg/deploysentry-go"

client := deploysentry.NewClient(
    deploysentry.WithAPIKey(os.Getenv("DS_API_KEY")),
    deploysentry.WithEnvironment("'"${DS_ENVIRONMENT}"'"),
    deploysentry.WithProject("'"${DS_PROJECT}"'"),
    deploysentry.WithBaseURL("'"${DS_BASE_URL}"'"),
)
client.Initialize(ctx)'

      WRONG_SNIPPET='// WRONG — direct boolean check, no register/dispatch
if client.BoolValue(ctx, "my-flag", false) {
    newFn()
} else {
    oldFn()
}'

      REGISTER_SNIPPET='// Register implementations (at startup)
client.Register("createCart", createCartWithMembership, "membership-lookup")
client.Register("createCart", createCart) // default — always last'

      DISPATCH_SNIPPET='// Dispatch at call site
fn := client.Dispatch("createCart").(func(Cart, User) Result)
result := fn(cart, user)'
      ;;

    python)
      INIT_SNIPPET="from deploysentry import DeploySentryClient
import os

ds = DeploySentryClient(
    api_key=os.environ['DS_API_KEY'],
    environment='${DS_ENVIRONMENT}',
    project='${DS_PROJECT}',
    base_url='${DS_BASE_URL}',
)
ds.initialize()"

      WRONG_SNIPPET="# WRONG — direct boolean check, no register/dispatch
if ds.bool_value('my-flag', False, ctx):
    new_fn()
else:
    old_fn()"

      REGISTER_SNIPPET="# Register implementations (at startup)
ds.register('create_cart', create_cart_with_membership, flag_key='membership-lookup')
ds.register('create_cart', create_cart)  # default — always last"

      DISPATCH_SNIPPET="# Dispatch at call site
result = ds.dispatch('create_cart', ctx)(cart, user)"
      ;;

    java)
      INIT_SNIPPET="import io.deploysentry.DeploySentryClient;
import io.deploysentry.ClientOptions;

var ds = new DeploySentryClient(
    ClientOptions.builder()
        .apiKey(System.getenv(\"DS_API_KEY\"))
        .environment(\"${DS_ENVIRONMENT}\")
        .project(\"${DS_PROJECT}\")
        .baseUrl(\"${DS_BASE_URL}\")
        .build()
);
ds.initialize();"

      WRONG_SNIPPET='// WRONG — direct boolean check, no register/dispatch
if (ds.boolValue("my-flag", false, ctx)) {
    newFn();
} else {
    oldFn();
}'

      REGISTER_SNIPPET='// Register implementations (at startup)
ds.register("createCart", () -> createCartWithMembership(cart, user), "membership-lookup");
ds.register("createCart", () -> createCart(cart, user)); // default — always last'

      DISPATCH_SNIPPET='// Dispatch at call site
var result = ds.<Result>dispatch("createCart", ctx).get();'
      ;;

    ruby)
      INIT_SNIPPET="require 'deploysentry'

ds = DeploySentry::Client.new(
  api_key: ENV['DS_API_KEY'],
  environment: '${DS_ENVIRONMENT}',
  project: '${DS_PROJECT}',
  base_url: '${DS_BASE_URL}',
)
ds.initialize!"

      WRONG_SNIPPET="# WRONG — direct boolean check, no register/dispatch
if ds.bool_value('my-flag', default: false, context: ctx)
  new_fn
else
  old_fn
end"

      REGISTER_SNIPPET="# Register implementations (at startup)
ds.register('create_cart', method(:create_cart_with_membership), flag_key: 'membership-lookup')
ds.register('create_cart', method(:create_cart)) # default — always last"

      DISPATCH_SNIPPET="# Dispatch at call site
result = ds.dispatch('create_cart', context: ctx).call(cart, user)"
      ;;

    flutter)
      INIT_SNIPPET="import 'package:deploysentry/deploysentry.dart';

final ds = DeploySentryClient(
  apiKey: const String.fromEnvironment('DS_API_KEY'),
  baseUrl: '${DS_BASE_URL}',
  environment: '${DS_ENVIRONMENT}',
  project: '${DS_PROJECT}',
);
await ds.initialize();"

      WRONG_SNIPPET="// WRONG — direct boolean check, no register/dispatch
if (await ds.boolValue('my-flag')) {
  newFn();
} else {
  oldFn();
}"

      REGISTER_SNIPPET="// Register implementations (at startup)
ds.register<Result Function(Cart, User)>('createCart', createCartWithMembership, flagKey: 'membership-lookup');
ds.register<Result Function(Cart, User)>('createCart', createCart); // default — always last"

      DISPATCH_SNIPPET="// Dispatch at call site
final fn = ds.dispatch<Result Function(Cart, User)>('createCart');
final result = fn(cart, user);"
      ;;
  esac

  # Write the section using printf so special chars are handled safely.
  # Literal backticks in markdown are written as shell '\`' escapes.
  printf '\n## DeploySentry Feature Flags\n' >> CLAUDE.md

  printf '\n### Connection\n' >> CLAUDE.md
  printf '\n| Setting     | Value |\n' >> CLAUDE.md
  printf '|-------------|-------|\n' >> CLAUDE.md
  printf '| API Key     | `DS_API_KEY` env var |\n' >> CLAUDE.md
  printf '| Environment | %s |\n' "$DS_ENVIRONMENT" >> CLAUDE.md
  printf '| Project     | %s |\n' "$DS_PROJECT" >> CLAUDE.md
  printf '| Base URL    | %s |\n' "$DS_BASE_URL" >> CLAUDE.md

  printf '\n### Initialization\n' >> CLAUDE.md
  printf '\n```\n' >> CLAUDE.md
  printf '%s\n' "$INIT_SNIPPET" >> CLAUDE.md
  printf '```\n' >> CLAUDE.md

  printf '\n### Register / Dispatch Pattern (REQUIRED)\n' >> CLAUDE.md
  printf '\n**NEVER** evaluate a flag as a plain boolean to branch between old and new code:\n' >> CLAUDE.md
  printf '\n```\n' >> CLAUDE.md
  printf '%s\n' "$WRONG_SNIPPET" >> CLAUDE.md
  printf '```\n' >> CLAUDE.md
  printf '\n**ALWAYS** use register + dispatch so the flag engine selects the right implementation:\n' >> CLAUDE.md
  printf '\n```\n' >> CLAUDE.md
  printf '%s\n' "$REGISTER_SNIPPET" >> CLAUDE.md
  printf '\n%s\n' "$DISPATCH_SNIPPET" >> CLAUDE.md
  printf '```\n' >> CLAUDE.md
  printf '\nRegister all variants at startup (one register call per implementation). The last\n' >> CLAUDE.md
  printf 'register call with no flag key is the default fallback. Dispatch at each call site.\n' >> CLAUDE.md

  printf '\n### Flag Categories\n' >> CLAUDE.md
  printf '\n| Category   | Lifecycle            | Notes                                     |\n' >> CLAUDE.md
  printf '|------------|----------------------|-------------------------------------------|\n' >> CLAUDE.md
  printf '| release    | Temporary            | Remove after rollout is complete          |\n' >> CLAUDE.md
  printf '| feature    | Can be permanent     | Permanent if it controls a toggle-able UX |\n' >> CLAUDE.md
  printf '| experiment | Temporary            | Remove after experiment concludes         |\n' >> CLAUDE.md
  printf '| ops        | Can be permanent     | Permanent for operational controls        |\n' >> CLAUDE.md
  printf '| permission | Typically permanent  | Gates access by role/plan/attribute       |\n' >> CLAUDE.md
  printf '%s\n' '' '- **Temporary flags**: retire after the rollout or experiment is done (see Retiring a Flag).' >> CLAUDE.md
  printf '%s\n' '- **Permanent flags**: keep indefinitely; do NOT retire them.' >> CLAUDE.md

  printf '\n### Creating Flags\n' >> CLAUDE.md
  printf '\n```sh\n' >> CLAUDE.md
  printf '# Permanent flag (feature toggle, permission gate, ops control)\n' >> CLAUDE.md
  printf 'deploysentry flags create \\\n' >> CLAUDE.md
  printf '  --project %s \\\n' "$DS_PROJECT" >> CLAUDE.md
  printf '  --key my-feature \\\n' >> CLAUDE.md
  printf '  --category feature \\\n' >> CLAUDE.md
  printf '  --is-permanent\n' >> CLAUDE.md
  printf '\n# Temporary flag (release rollout, experiment)\n' >> CLAUDE.md
  printf 'deploysentry flags create \\\n' >> CLAUDE.md
  printf '  --project %s \\\n' "$DS_PROJECT" >> CLAUDE.md
  printf '  --key my-release \\\n' >> CLAUDE.md
  printf '  --category release \\\n' >> CLAUDE.md
  printf '  --expires-at 2026-12-31\n' >> CLAUDE.md
  printf '```\n' >> CLAUDE.md

  printf '\n### Context Requirements\n' >> CLAUDE.md
  printf '\nEvery dispatch call must supply context so targeting rules can be evaluated:\n' >> CLAUDE.md
  printf '%s\n' '' '- `user_id` -- unique user identifier (required)' >> CLAUDE.md
  printf '%s\n' '- `session_id` -- current session identifier (recommended)' >> CLAUDE.md
  printf '%s\n' '- Custom attributes -- any key/value pairs used in targeting rules (e.g. `plan`, `country`)' >> CLAUDE.md

  printf '\n### Retiring a Flag (temporary flags only)\n' >> CLAUDE.md
  printf '\nPermanent flags (`is_permanent = true`) are never retired.\n' >> CLAUDE.md
  printf '\nFor temporary flags, follow these steps once the rollout or experiment is complete:\n' >> CLAUDE.md
  printf '%s\n' '' '1. Remove all register calls for the flag key.' >> CLAUDE.md
  printf '%s\n' '2. Remove all dispatch call sites -- replace with a direct call to the winning implementation.' >> CLAUDE.md
  printf '%s\n' '3. Delete or archive the losing implementation.' >> CLAUDE.md
  printf '%s\n' "4. Delete the flag via CLI: \`deploysentry flags delete --project $DS_PROJECT --key <flag-key>\`" >> CLAUDE.md
  printf '%s\n' '5. Remove any targeting rules or segments associated with the flag.' >> CLAUDE.md

  echo "CLAUDE.md updated with DeploySentry integration prompt."
}

write_claude_md

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------

echo ""
echo "✓ SDK installed (${LANG_DETECTED})"
echo "✓ CLAUDE.md updated with DeploySentry integration prompt"
echo ""
echo "Next steps:"
echo "  1. Create a flag:  deploysentry flags create --project ${DS_PROJECT} --key my-flag --category release --expires-at 2026-12-31"
echo "  2. Wire up register/dispatch in your application startup and call sites."
echo "  3. Read the docs:  ${DS_BASE_URL}/docs"
