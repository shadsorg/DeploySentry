# ApplicationsListPage Design

**Date:** 2026-03-30
**Scope:** New page listing all applications within a project, with card grid layout

## Summary

Add a dedicated ApplicationsListPage at `/orgs/:orgSlug/projects/:projectSlug/apps` showing all applications in a card grid (matching ProjectListPage pattern). Add a sidebar nav link and route.

## Page

- **File:** `web/src/pages/ApplicationsListPage.tsx`
- Uses existing `useApps(orgSlug, projectSlug)` hook
- Card grid with app name, slug, and description
- Each card links to `/orgs/:orgSlug/projects/:projectSlug/apps/:appSlug/deployments`
- Header with "Create App" button linking to `apps/new`
- Empty state with "Create Your First App" button
- Loading/error states matching ProjectListPage pattern

## Route

Add to `App.tsx` at the project level (line 62, before `apps/new`):
```
<Route path="apps" element={<ApplicationsListPage />} />
```

## Sidebar

Add "Applications" NavLink in the project-level section of `Sidebar.tsx`, before the "Project" section divider (so it sits right after the AppAccordion).

## No backend changes

The `GET /orgs/:orgSlug/projects/:projectSlug/apps` endpoint and `entitiesApi.listApps()` already exist.
