# SDK npm Publishing Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Phase**: Implementation

## Problem

The Node (`@dr-sentry/sdk`) and React (`@dr-sentry/react`) SDKs are installed into consumer projects via local `file:` dependencies that point at a sibling `DeploySentry/` checkout (`file:../../DeploySentry/sdk/node`, `file:../../DeploySentry/sdk/react`). This works on a dev machine where both repos live side by side, but breaks everywhere else:

1. **Railway / Render / any git-based deploy** — the build context is only the consumer repo. The relative `../../DeploySentry/...` path resolves outside the build context and `npm ci` creates a broken symlink, producing `Failed to resolve import "@dr-sentry/react"` at bundle time.
2. **Docker dev (without a bind-mount workaround)** — same problem: `/app/../..` inside the container doesn't contain the SDK source.
3. **Teammate / CI machines without a `DeploySentry` checkout** — `npm ci` fails immediately.

In addition, the React SDK currently emits CommonJS only (`module: commonjs` in `sdk/react/tsconfig.json`, single `main: dist/index.js` in `package.json`). Vite's dev server (esbuild pre-bundler) works around this by converting CJS → ESM, but `vite build` (Rollup) refuses CJS named imports and fails for downstream consumers.

## Goal

Publish `@dr-sentry/sdk` (Node) and `@dr-sentry/react` (browser) to the public npm registry with proper dual ESM/CJS output and an `exports` map. Replace `file:` dependencies in all consumers with semver ranges. Result: `vite build`, `npm ci` on Railway, and fresh teammate installs all work with no host-layout assumptions.

## Architecture

1. **Dual ESM/CJS output via two tsconfigs.** Each SDK gets `tsconfig.esm.json` (`module: ES2020`, `outDir: dist/esm`) and `tsconfig.cjs.json` (`module: commonjs`, `outDir: dist/cjs`). `npm run build` runs both.
2. **`exports` map in `package.json`** with `import` → ESM, `require` → CJS, and `types` → `.d.ts`. Keep `main`/`module`/`types` at the top level as legacy fallbacks for tools that don't understand `exports`.
3. **Scope: `@dr-sentry`** (public, unscoped publish). The scope is already reflected in the package names. If the user has not yet claimed it on npm, that is a prereq (one-time `npm access` setup).
4. **Publisher identity.** Publish under the user's existing npm account for v1.0.0. The `Identity & Provenance` plan (`2026-04-16-identity-and-provenance.md`) will later migrate ownership to a `crowdsoftapps` npm org — npm allows this without bumping versions. Not a blocker for this plan.
5. **Version floor: 1.0.0.** Both `package.json` files already declare `1.0.0`. First publish lands 1.0.0. Any small iteration during the publish process gets a patch bump.
6. **Consumers migrate immediately.** `jobmgr/backend/package.json` and `jobmgr/frontend/package.json` swap from `file:` to `^1.0.0`. The Docker bind-mount workaround added to `jobmgr/docker-compose.override.yml` gets reverted.
7. **Out of scope:** Flutter SDK (pub.dev, separate ecosystem — track as a follow-up), Python / Go / Java / Ruby SDKs (not breaking Railway today; publish when the first non-local consumer appears), CI-driven publishing (initial publishes are manual; automate in a follow-up once the workflow is stable).

## Tech Stack

TypeScript, `tsc` (dual builds), npm registry, Node.js 18+.

---

### Task 1: Add dual ESM/CJS build to `sdk/node`

**Files:**
- Create: `sdk/node/tsconfig.esm.json`
- Create: `sdk/node/tsconfig.cjs.json`
- Modify: `sdk/node/tsconfig.json` (becomes base config, no emit)
- Modify: `sdk/node/package.json` (scripts, `exports`, `main`, `module`, `types`)

- [ ] **Step 1: Split `sdk/node/tsconfig.json` into base + two output configs**

The existing `sdk/node/tsconfig.json` uses `module: Node16`, which is fine for ESM but doesn't emit CJS. Rework as:

`sdk/node/tsconfig.json` (base, no emit, extended by the others):

```json
{
  "compilerOptions": {
    "target": "ES2022",
    "lib": ["ES2022"],
    "rootDir": "src",
    "declaration": true,
    "declarationMap": true,
    "sourceMap": true,
    "strict": true,
    "esModuleInterop": true,
    "skipLibCheck": true,
    "forceConsistentCasingInFileNames": true,
    "resolveJsonModule": true,
    "isolatedModules": true,
    "noUnusedLocals": true,
    "noUnusedParameters": true,
    "noImplicitReturns": true,
    "noFallthroughCasesInSwitch": true
  },
  "include": ["src"],
  "exclude": ["node_modules", "dist", "**/*.test.ts"]
}
```

`sdk/node/tsconfig.esm.json`:

```json
{
  "extends": "./tsconfig.json",
  "compilerOptions": {
    "module": "ES2020",
    "moduleResolution": "Bundler",
    "outDir": "dist/esm"
  }
}
```

`sdk/node/tsconfig.cjs.json`:

```json
{
  "extends": "./tsconfig.json",
  "compilerOptions": {
    "module": "commonjs",
    "moduleResolution": "node",
    "outDir": "dist/cjs"
  }
}
```

- [ ] **Step 2: Update `sdk/node/package.json` build scripts and entry points**

```json
{
  "name": "@dr-sentry/sdk",
  "version": "1.0.0",
  "main": "dist/cjs/index.js",
  "module": "dist/esm/index.js",
  "types": "dist/esm/index.d.ts",
  "exports": {
    ".": {
      "import": {
        "types": "./dist/esm/index.d.ts",
        "default": "./dist/esm/index.js"
      },
      "require": {
        "types": "./dist/cjs/index.d.ts",
        "default": "./dist/cjs/index.js"
      }
    }
  },
  "files": ["dist"],
  "scripts": {
    "build": "npm run build:esm && npm run build:cjs && npm run build:cjs-package-json",
    "build:esm": "tsc -p tsconfig.esm.json",
    "build:cjs": "tsc -p tsconfig.cjs.json",
    "build:cjs-package-json": "node -e \"require('fs').writeFileSync('dist/cjs/package.json', JSON.stringify({type:'commonjs'}))\"",
    "clean": "rm -rf dist",
    "prepublishOnly": "npm run clean && npm run build"
  }
}
```

The `build:cjs-package-json` step writes a tiny `dist/cjs/package.json` with `{"type":"commonjs"}` so Node treats the CJS output as CJS even if the root package becomes `"type": "module"` in the future.

- [ ] **Step 3: Verify dual build**

```bash
cd /Users/sgamel/git/DeploySentry/sdk/node
npm run clean
npm run build
ls dist/esm/index.js dist/cjs/index.js dist/esm/index.d.ts
```

Expected: all three files exist. No `tsc` errors.

- [ ] **Step 4: Smoke-test ESM import and CJS require**

```bash
cd /tmp && mkdir ds-smoke && cd ds-smoke && npm init -y
npm install /Users/sgamel/git/DeploySentry/sdk/node
node --input-type=module -e "import { DeploySentryClient } from '@dr-sentry/sdk'; console.log(typeof DeploySentryClient)"
node -e "const { DeploySentryClient } = require('@dr-sentry/sdk'); console.log(typeof DeploySentryClient)"
```

Expected: both print `function`. Cleanup: `cd .. && rm -rf ds-smoke`.

- [ ] **Step 5: Commit**

```bash
cd /Users/sgamel/git/DeploySentry
git add sdk/node/tsconfig.json sdk/node/tsconfig.esm.json sdk/node/tsconfig.cjs.json sdk/node/package.json
git commit -m "build(sdk/node): emit dual ESM+CJS with exports map"
```

---

### Task 2: Add dual ESM/CJS build to `sdk/react`

**Files:**
- Create: `sdk/react/tsconfig.esm.json`
- Create: `sdk/react/tsconfig.cjs.json`
- Modify: `sdk/react/tsconfig.json` (becomes base)
- Modify: `sdk/react/package.json`

- [ ] **Step 1: Split `sdk/react/tsconfig.json`**

Base `sdk/react/tsconfig.json`:

```json
{
  "compilerOptions": {
    "target": "ES2020",
    "lib": ["ES2020", "DOM", "DOM.Iterable"],
    "jsx": "react-jsx",
    "rootDir": "src",
    "declaration": true,
    "declarationMap": true,
    "sourceMap": true,
    "strict": true,
    "esModuleInterop": true,
    "skipLibCheck": true,
    "forceConsistentCasingInFileNames": true,
    "resolveJsonModule": true,
    "isolatedModules": true
  },
  "include": ["src"],
  "exclude": ["node_modules", "dist", "**/*.test.ts", "**/*.test.tsx"]
}
```

`sdk/react/tsconfig.esm.json`:

```json
{
  "extends": "./tsconfig.json",
  "compilerOptions": {
    "module": "ES2020",
    "moduleResolution": "Bundler",
    "outDir": "dist/esm"
  }
}
```

`sdk/react/tsconfig.cjs.json`:

```json
{
  "extends": "./tsconfig.json",
  "compilerOptions": {
    "module": "commonjs",
    "moduleResolution": "node",
    "outDir": "dist/cjs"
  }
}
```

- [ ] **Step 2: Update `sdk/react/package.json`**

```json
{
  "name": "@dr-sentry/react",
  "version": "1.0.0",
  "main": "dist/cjs/index.js",
  "module": "dist/esm/index.js",
  "types": "dist/esm/index.d.ts",
  "exports": {
    ".": {
      "import": {
        "types": "./dist/esm/index.d.ts",
        "default": "./dist/esm/index.js"
      },
      "require": {
        "types": "./dist/cjs/index.d.ts",
        "default": "./dist/cjs/index.js"
      }
    }
  },
  "files": ["dist"],
  "scripts": {
    "build": "npm run build:esm && npm run build:cjs && npm run build:cjs-package-json",
    "build:esm": "tsc -p tsconfig.esm.json",
    "build:cjs": "tsc -p tsconfig.cjs.json",
    "build:cjs-package-json": "node -e \"require('fs').writeFileSync('dist/cjs/package.json', JSON.stringify({type:'commonjs'}))\"",
    "clean": "rm -rf dist",
    "prepublishOnly": "npm run clean && npm run build"
  }
}
```

Keep the existing `peerDependencies.react` and `devDependencies` blocks untouched.

- [ ] **Step 3: Verify dual build**

```bash
cd /Users/sgamel/git/DeploySentry/sdk/react
npm run clean
npm run build
ls dist/esm/index.js dist/cjs/index.js dist/esm/index.d.ts
```

Expected: all three files exist. No `tsc` errors.

- [ ] **Step 4: Smoke-test bundler resolution**

The React SDK exports `DeploySentryClient` (framework-agnostic browser client used by Vue consumers too). Verify a bundler picks the ESM path:

```bash
cd /tmp && mkdir ds-react-smoke && cd ds-react-smoke && npm init -y
npm pkg set type=module
npm install /Users/sgamel/git/DeploySentry/sdk/react
node --input-type=module -e "import { DeploySentryClient } from '@dr-sentry/react'; console.log(typeof DeploySentryClient)"
```

Expected: prints `function`. Cleanup: `cd .. && rm -rf ds-react-smoke`.

- [ ] **Step 5: Commit**

```bash
cd /Users/sgamel/git/DeploySentry
git add sdk/react/tsconfig.json sdk/react/tsconfig.esm.json sdk/react/tsconfig.cjs.json sdk/react/package.json
git commit -m "build(sdk/react): emit dual ESM+CJS with exports map"
```

---

### Task 3: Run existing SDK tests against the new build layout

**Files:**
- Possibly: `sdk/node/jest.config.js`, `sdk/react/jest.config.js` (only if tests break)

- [ ] **Step 1: Run Node SDK tests**

```bash
cd /Users/sgamel/git/DeploySentry/sdk/node && npm test
```

Expected: all tests pass. If Jest complains about the new tsconfig layout, pin `ts-jest` to use the base `tsconfig.json` (which still has full strict settings but no `outDir`) by adding to `jest.config.js`:

```js
globals: { 'ts-jest': { tsconfig: 'tsconfig.json' } }
```

- [ ] **Step 2: Run React SDK tests**

```bash
cd /Users/sgamel/git/DeploySentry/sdk/react && npm test
```

Expected: all tests pass. Apply the same `ts-jest` pin if needed.

- [ ] **Step 3: Commit (only if jest configs changed)**

```bash
git add sdk/node/jest.config.js sdk/react/jest.config.js
git commit -m "test(sdk): pin ts-jest to base tsconfig after dual-build split"
```

---

### Task 4: Claim the `@dr-sentry` npm scope

**Prereq:** user must have an npm account and be logged in locally (`npm whoami` returns a username).

- [ ] **Step 1: Verify npm login**

```bash
npm whoami
```

Expected: prints the username. If not, run `npm login` (interactive — user enters credentials in the terminal). Surface the need via `!npm login` so it runs in the host shell.

- [ ] **Step 2: Check scope availability**

```bash
npm view @dr-sentry/sdk 2>&1 | head -5
npm view @dr-sentry/react 2>&1 | head -5
```

Expected: both return `npm error code E404`. If either returns a real package, stop — someone else owns the scope and the plan needs to pivot to a different scope (e.g. `@crowdsoftapps/deploysentry-sdk`). Flag to the user.

- [ ] **Step 3: No action needed to "claim" the scope**

Publishing any package with a scoped name implicitly registers the scope to the publisher's account for unscoped-public publishes. Proceed to Task 5.

---

### Task 5: Publish `@dr-sentry/sdk` to npm

- [ ] **Step 1: Dry-run publish and inspect the tarball**

```bash
cd /Users/sgamel/git/DeploySentry/sdk/node
npm publish --dry-run --access public
```

Expected output lists `dist/esm/*`, `dist/cjs/*`, `package.json`, `README.md`, `LICENSE` (if present). Confirm no `src/`, no `node_modules/`, no test fixtures.

- [ ] **Step 2: Publish**

```bash
cd /Users/sgamel/git/DeploySentry/sdk/node
npm publish --access public
```

`--access public` is required for the first publish of a scoped package (npm defaults scoped packages to private). Expected: `+ @dr-sentry/sdk@1.0.0`.

- [ ] **Step 3: Verify the package is resolvable**

```bash
npm view @dr-sentry/sdk version
npm view @dr-sentry/sdk exports
```

Expected: `1.0.0` and the `exports` map.

---

### Task 6: Publish `@dr-sentry/react` to npm

- [ ] **Step 1: Dry-run**

```bash
cd /Users/sgamel/git/DeploySentry/sdk/react
npm publish --dry-run --access public
```

Confirm the tarball contains `dist/esm/*` and `dist/cjs/*` only.

- [ ] **Step 2: Publish**

```bash
cd /Users/sgamel/git/DeploySentry/sdk/react
npm publish --access public
```

Expected: `+ @dr-sentry/react@1.0.0`.

- [ ] **Step 3: Verify**

```bash
npm view @dr-sentry/react version
```

Expected: `1.0.0`.

---

### Task 7: Migrate `jobmgr` consumers off `file:` deps

**Files (in the `jobmgr` repo):**
- Modify: `backend/package.json`
- Modify: `frontend/package.json`
- Modify: `docker-compose.override.yml` (remove the DeploySentry bind mounts added on 2026-04-16)

- [ ] **Step 1: Swap backend dep**

```bash
cd /Users/sgamel/git/jobmgr/backend
npm install @dr-sentry/sdk@^1.0.0
```

This rewrites `package.json` and `package-lock.json` to point at the published package.

- [ ] **Step 2: Swap frontend dep**

```bash
cd /Users/sgamel/git/jobmgr/frontend
npm install @dr-sentry/react@^1.0.0
```

- [ ] **Step 3: Revert the Docker bind-mount workaround**

In `jobmgr/docker-compose.override.yml`, remove the two `${HOME}/git/DeploySentry:/DeploySentry:ro` lines (one under `backend`, one under `frontend`). The bind mount is no longer needed.

- [ ] **Step 4: Rebuild Docker volumes and verify**

```bash
cd /Users/sgamel/git/jobmgr
docker compose down
docker volume rm jobmgr_frontend_node_modules jobmgr_backend_node_modules
docker compose up -d
docker compose logs -f frontend
```

Expected: `vite` serves `/` with no `Failed to resolve import "@dr-sentry/react"` error.

- [ ] **Step 5: Verify production bundle builds**

```bash
cd /Users/sgamel/git/jobmgr/frontend
npm run build
```

Expected: Vite production build completes successfully (fixes bootstrap plan follow-up #2).

- [ ] **Step 6: Commit in `jobmgr`**

```bash
cd /Users/sgamel/git/jobmgr
git add backend/package.json backend/package-lock.json frontend/package.json frontend/package-lock.json docker-compose.override.yml
git commit -m "feat(deploysentry): consume published @dr-sentry/* packages

Replaces file: deps with ^1.0.0 from npm. Removes the Docker bind-mount
workaround that was only needed while the SDKs shipped as local file
refs. Vite production build now works (previously failed on CJS named
imports from the React SDK)."
```

- [ ] **Step 7: Update `jobmgr` bootstrap plan follow-ups**

In `jobmgr/docs/superpowers/plans/2026-04-16-deploysentry-bootstrap.md`, mark follow-ups #1 and #2 resolved (SDKs now on npm; Vite production build passes). Move the plan to `docs/archives/` if follow-up #3 and #4 are also addressed, otherwise leave it active.

---

### Task 8: Commit + push DeploySentry repo

- [ ] **Step 1: Push to origin**

```bash
cd /Users/sgamel/git/DeploySentry
git push origin main
```

Expected: build-config commits land on `main`. No failed CI.

- [ ] **Step 2: Tag the release**

```bash
git tag sdk-node-v1.0.0 -m "First npm publish of @dr-sentry/sdk"
git tag sdk-react-v1.0.0 -m "First npm publish of @dr-sentry/react"
git push origin sdk-node-v1.0.0 sdk-react-v1.0.0
```

These tags let future releases reference the exact commit state that was published.

---

### Task 9: Move this plan to archives

- [ ] **Step 1: Set phase to Complete**

Edit the frontmatter at the top of this file: `**Phase**: Complete`. Fill in the Completion Record with the branch, commit status, push status, and CI result.

- [ ] **Step 2: Archive**

```bash
cd /Users/sgamel/git/DeploySentry
git mv docs/superpowers/plans/2026-04-16-sdk-npm-publishing.md docs/archives/
```

- [ ] **Step 3: Update `docs/Current_Initiatives.md`**

Remove the `SDK npm Publishing` row from the Active table. Commit.

```bash
git add docs/Current_Initiatives.md docs/archives/2026-04-16-sdk-npm-publishing.md
git commit -m "docs: archive completed SDK npm publishing plan"
git push origin main
```

---

## Follow-ups (not part of this plan)

1. **Flutter SDK → pub.dev.** `sdk/flutter` is consumed by `jobmgr/mobile` via a local path dep. Publishing to pub.dev requires a verified publisher. Track separately.
2. **CI-driven publishing.** Set up a GitHub Actions workflow that publishes on tag push (`sdk-node-v*`, `sdk-react-v*`) using an `NPM_TOKEN` secret. Removes the manual `npm publish` step.
3. **Ownership transfer to CrowdSoftApps.** Once the `Identity & Provenance` plan creates the `crowdsoftapps` npm org, transfer `@dr-sentry/*` ownership (no version bump required). Update the `repository` field in each `package.json` to the final GitHub URL.
4. **Go / Python / Ruby / Java SDKs.** Publish via their ecosystems (pkg.go.dev is automatic; PyPI / RubyGems / Maven Central require the same "dual output + publish" treatment when the first non-local consumer appears).

---

## Completion Record
<!-- Filled on completion -->
- **Branch**:
- **Committed**: No
- **Pushed**: No
- **CI Checks**: N/A
