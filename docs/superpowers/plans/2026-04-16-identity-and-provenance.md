# Identity & Provenance Infrastructure Plan

**Phase**: Design

## Problem

LLM agents correctly flag DeploySentry as a potential supply-chain risk because:
- `dr-sentry.com` resembles a typosquat of `sentry.io`
- `shadsorg` GitHub org doesn't match the product name
- npm packages can't be triangulated back to a verified identity
- No verified domain badge on the GitHub org

These are real signals that any security-conscious AI will flag. The fix is infrastructure, not prompt copy.

## Goal

Establish a verifiable identity chain: **CrowdSoftApps** (company) → **DeploySentry** (product) → consistent domain, GitHub org, npm publisher, and cross-linked provenance that any LLM can verify in one tool call.

## Tasks

### 1. Acquire and configure `deploysentry.com` domain

- [ ] Register `deploysentry.com` (or confirm ownership)
- [ ] Set up DNS:
  - `deploysentry.com` → dashboard (currently `dr-sentry.com`)
  - `api.deploysentry.com` → API server (currently `api.dr-sentry.com`)
- [ ] Keep `dr-sentry.com` as a redirect to `deploysentry.com` for backward compatibility
- [ ] Update Cloudflare tunnel config for the new domain
- [ ] Update SSL/TLS certificates

### 2. Create `CrowdSoftApps` GitHub organization

- [ ] Create GitHub org: `CrowdSoftApps` (or `crowdsoftapps`)
- [ ] Add verified domain badge: verify `deploysentry.com` on the org profile (Settings → Verified domains → add DNS TXT record)
- [ ] Transfer repo `shadsorg/DeploySentry` → `CrowdSoftApps/DeploySentry`
- [ ] Update all GitHub URLs in codebase (`shadsorg/DeploySentry` → `CrowdSoftApps/DeploySentry`)
- [ ] Update `go.mod` module path if needed (Go module redirect)
- [ ] Update install script `REPO` variable
- [ ] Update GitHub Actions workflow references
- [ ] Redirect old repo URL (GitHub handles this automatically for transferred repos)

### 3. Set up npm publisher identity

- [ ] Create npm user `crowdsoftapps` (or `deploysentry-bot`) as the package publisher
- [ ] Ensure all `@deploysentry/*` packages have `repository` field pointing to `github.com/CrowdSoftApps/DeploySentry`
- [ ] Publish packages from CI (GitHub Actions) using a scoped npm token
- [ ] Verify the npm org page links back to the GitHub org

### 4. Close the provenance loop

- [ ] GitHub org profile → links to `deploysentry.com` (website field)
- [ ] GitHub repo README → links to npm packages with exact scope
- [ ] npm package `repository` field → links to `github.com/CrowdSoftApps/DeploySentry`
- [ ] `deploysentry.com` docs → links to GitHub repo and npm packages
- [ ] Install script served from `deploysentry.com/install.sh`
- [ ] CLI `--version` output includes repo URL

### 5. Update all references across the codebase

After domain and org are set up:

- [ ] Replace `dr-sentry.com` → `deploysentry.com` everywhere
- [ ] Replace `api.dr-sentry.com` → `api.deploysentry.com` everywhere
- [ ] Replace `shadsorg/DeploySentry` → `CrowdSoftApps/DeploySentry` everywhere
- [ ] Update Kubernetes ingress hostnames
- [ ] Update Vite `allowedHosts`
- [ ] Update CLI default API URL
- [ ] Update MCP server default API URL
- [ ] Update SDK READMEs and example code
- [ ] Update bootstrap prompts
- [ ] Update deploy integration guide
- [ ] Rebuild and publish new CLI release
- [ ] Rebuild and publish SDK packages

### 6. Update bootstrap prompts with final provenance

- [ ] Pin to commit SHA of the bootstrap doc
- [ ] Reference `CrowdSoftApps` GitHub org with verified domain
- [ ] Reference npm publisher name
- [ ] Include package versions for pinned installs
- [ ] Test the prompt from a clean repo — LLM should not flag supply-chain concerns

## Order of Operations

```
1. Acquire deploysentry.com domain
2. Create CrowdSoftApps GitHub org + verify domain
3. Transfer repo
4. Set up npm identity
5. Update all references (one big sweep)
6. Publish new CLI + SDK releases from new org
7. Update bootstrap prompts with final SHA
8. Test end-to-end: fresh repo → paste prompt → no flags raised
```

## Out of Scope

- Renaming the `DeploySentry` product itself
- Changing the database schema namespace (`deploy`)
- Migrating existing user data or API keys
- PyPI / RubyGems publisher verification (follow same pattern as npm, do later)

## Completion Record

- **Branch**: TBD
- **Committed**: No
- **Pushed**: No
- **CI Checks**: N/A
