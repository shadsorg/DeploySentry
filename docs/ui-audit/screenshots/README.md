# No-Mockup Screenshots — DeploySentry UI

These screenshots capture the current state of every page that **does not** have
a corresponding `newscreens/*.html` mockup. They were produced by
`web/e2e/ui/no-mockup-screenshots.spec.ts` against the Vite dev server with
`setupMockApi` fixtures.

## Naming convention

Each file is named so it maps 1:1 to a React component and an implied mockup
filename. When you (or a designer) produce a mockup, **save it in
`newscreens/`** using the same stem. For example:

- Screenshot `rollout-detail.png` → when ready, drop `rollout-detail.html` in
  `newscreens/` and the audit pairs up automatically.

## Inventory

| Screenshot | Component | Route | Implied mockup filename |
|---|---|---|---|
| `landing.png` | `LandingPage.tsx` | `/` | `landing.html` |
| `login.png` | `LoginPage.tsx` | `/login` | `login.html` |
| `register.png` | `RegisterPage.tsx` | `/register` | `register.html` |
| `create-org.png` | `CreateOrgPage.tsx` | `/orgs/new` | `create-org.html` |
| `create-project.png` | `CreateProjectPage.tsx` | `/orgs/:org/projects/new` | `create-project.html` |
| `create-app.png` | `CreateAppPage.tsx` | `…/projects/:proj/apps/new` | `create-app.html` |
| `project-list.png` | `ProjectListPage.tsx` | `/orgs/:org/projects` | `project-list.html` |
| `project-shell.png` | `ProjectPage.tsx` + `ProjectAppsTab.tsx` | `/orgs/:org/projects/:proj` | `project-shell.html` |
| `flag-create.png` | `FlagCreatePage.tsx` | `…/flags/new` | `flag-create.html` |
| `releases-list.png` | `ReleasesPage.tsx` | `…/apps/:app/releases` | `releases-list.html` |
| `release-detail.png` | `ReleaseDetailPage.tsx` | `…/releases/:id` | `release-detail.html` |
| `deployment-detail.png` | `DeploymentDetailPage.tsx` | `…/deployments/:id` | `deployment-detail.html` |
| `rollout-detail.png` | `RolloutDetailPage.tsx` | `/orgs/:org/rollouts/:id` | `rollout-detail.html` |
| `rollout-group-detail.png` | `RolloutGroupDetailPage.tsx` | `/orgs/:org/rollout-groups/:id` | `rollout-group-detail.html` |
| `settings-app.png` | `SettingsPage.tsx` (level=app) | `…/apps/:app/settings` | `settings-app.html` |

## Edge note

`newscreens/project-applications.html` exists but is a **0-byte file**, so
`ProjectAppsTab.tsx` is effectively a no-mockup page too — captured here as
part of `project-shell.png` (the route renders `ProjectPage` with the apps
tab as its default child).

## Known artifact

The headless browser runs offline, so `fonts.googleapis.com` (Material Symbols
+ Manrope) is blocked. In the screenshots, icon glyphs render as their literal
names (e.g. `security`, `layers`, `dynamic_feed`) and headings fall back to
the system sans-serif. Real browsers show glyphs and Manrope normally — this
does not represent a bug.

## Regenerating

```bash
cd web
npx playwright install chromium   # first time only
PWBROWSERS_PATH=/opt/pw-browsers \
  npx playwright test e2e/ui/no-mockup-screenshots.spec.ts --project=ui --reporter=list
```

Output lands back in this directory.

## Conventions for the mockups you add

- Viewport: 1440×900 (matches the Playwright screenshot viewport).
- Dark Sentry palette per `DESIGN.MD` — indigo `#6366f1`, emerald `#10b981`,
  surface `#0b1326`, surface-container `#1b2339`, border `#334155`,
  text `#e2e8f0` / variant `#94a3b8`.
- Manrope 800 for headings (`-0.025em` tracking), Inter for body.
- 8px border-radius default; glass panels at `rgba(11,19,38,0.6)` with
  `backdrop-filter: blur(20px)`.
