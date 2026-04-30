# Build Status Ingestion — Deferred Follow-ups

**Status**: Design
**Date**: 2026-04-29
**Parent (archived)**: [`../../archives/2026-04-23-build-status-and-deploy-autocomplete-design.md`](../../archives/2026-04-23-build-status-and-deploy-autocomplete-design.md)

## Context

All four runtime phases of Build Status Ingestion landed on main and the smoke runner shipped. The parent initiative is closed. These items were explicitly deferred at merge time and are tracked here so they don't get lost.

## Tasks

- [ ] **Handler unit tests** for `/applications/:id/artifacts`, `/applications/:id/versions`, and the workflow_run upsert handler. Coverage exists at the repo level only.
- [ ] **Combobox Vitest** for the reusable `<Combobox>` component on the New Deployment modal — keyboard nav, async loading state, no-results state.
- [ ] **Playwright smoke** for the full deploy-create flow with autocomplete + build pill rendered on `OrgStatusPage`.
- [ ] **MCP `deploy_create` tool update** so it accepts the same artifact/version slugs the UI now offers.
- [ ] **Stale-build sweep** — periodic job that marks `latest_build` rows as `stale` when no `workflow_run` has come through for a configured TTL (default: 7d). Avoids `OrgStatusPage` showing months-old `BuildPill` icons.

## Done when

All five boxes ticked. None of these block customers; they exist to harden the surface that already shipped.
