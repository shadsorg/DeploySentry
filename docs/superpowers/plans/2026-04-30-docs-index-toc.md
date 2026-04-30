# DocsPage Index — Topic TOC

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task.

**Status**: Implementation
**Date**: 2026-04-30
**Origin**: UI audit §20
**Audit**: [`../../ui-audit/MOCKUP_DISPARITIES.md`](../../ui-audit/MOCKUP_DISPARITIES.md)
**Estimated total**: ~1 hour

**Goal**: when a user hits `/docs` (no slug), show a topic-organized table of contents instead of an empty / first-doc-only render. The mockup proposed a marketing landing — product confirmed this is internal product documentation, so a clean TOC matching the existing Markdown article set is the right shape.

**Out of scope**: marketing copy, hero panels, "Managed Infrastructure / Self-Host" comparison cards from the mockup. Those belong on the public site, not the in-app docs surface.

---

### Task 1: Build the TOC component

**Files:**
- Create: `web/src/components/docs/DocsIndex.tsx`
- Modify: `web/src/pages/DocsPage.tsx` (route `/docs` with no slug renders `DocsIndex` instead of fallback)
- Modify: `web/src/docs/index.ts` (already enumerates docs; add `category` + `summary` to each entry)

- [ ] Define a `DocCategory` enum: `Getting Started`, `Deploys`, `Feature Flags`, `Rollouts`, `Integrations`, `Reference`, `Operations`.
- [ ] In `web/src/docs/index.ts`, add `{ category, summary }` to each existing doc entry. Categories from the file list:
  - `Getting_Started.md` → Getting Started
  - `Bootstrap_My_App.md` → Getting Started
  - `Deploy_Integration_Guide.md` → Deploys
  - `Deploy_Monitoring_Setup.md` → Deploys
  - `Feature_Flag_Engine_Improvements.md` → Feature Flags
  - `Feature_Lifecycle.md` → Feature Flags
  - `Rollout_Strategies.md` → Rollouts
  - `Traffic_Management_Guide.md` → Rollouts
  - `sdk-onboarding.md` → Integrations
  - `DEVELOPMENT.md` → Reference
  - `PRODUCTION.md` → Operations
- [ ] `DocsIndex.tsx` renders sections per category with a card per doc: title, one-line summary, "Read →" link.
- [ ] Search box at the top filters across title + summary as the user types (client-side, no backend).

### Task 2: Wire empty-slug route

- [ ] In `App.tsx` (or wherever routes are defined), the `/docs` route with no slug renders `DocsIndex`. The `/docs/:slug` route keeps rendering the existing single-article view.
- [ ] Sidebar "Docs" link → `/docs` (currently goes to first article — verify and update if needed).

### Task 3: Verify

- [ ] `npm run lint --max-warnings 0` passes.
- [ ] `npm run build` clean.
- [ ] Visual: click "Docs" in sidebar → see the TOC, not a single article.
- [ ] Update `docs/ui-audit/MOCKUP_DISPARITIES.md` to mark §20 as ✅ shipped.

## PR

Title: `feat(web): docs index page (UI audit §20)`. Body links to this plan and notes the marketing-landing direction was rejected by product in favor of an internal TOC.
