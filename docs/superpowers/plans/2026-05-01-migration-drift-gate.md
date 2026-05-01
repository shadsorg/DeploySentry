# Migration Drift Gate

**Phase**: Design

## Overview

Today's smoke-test of the MCP flag tools surfaced a class of failure that was
indistinguishable from a code bug: every read against `feature_flags` returned
500 / 404 because the running binary's `flagSelectCols` referenced
`delete_after` (added by migration `060_add_flag_delete_after`), but the local
DB was at migration `59`. The error message was generic ("failed to list
flags") and the cause — schema drift between code and DB — took an hour of
investigation to isolate.

We need automation that fails loudly and early when the migrations on disk
are ahead of the DB the running binary is talking to.

## Approaches (to brainstorm before writing a plan)

1. **Boot-time check.** On API startup, compare `max(version) FROM
   schema_migrations` against the highest migration number bundled in the
   binary (`migrations/*.up.sql`). Refuse to start (or log a loud warning
   that includes the gap) when they diverge. Cheap; covers the runtime
   failure mode directly. Question: warn vs. refuse — refuse is safer but
   blocks anyone running `make run-api` without `make migrate-up` first.
2. **CI gate on PRs that add migrations.** When a PR touches
   `migrations/*.up.sql`, require a check that runs `make migrate-up`
   against an ephemeral Postgres before the test job. Catches the
   forward case (new migration that breaks reads) but not the local-dev
   forgot-to-migrate case.
3. **`make run-api` runs `migrate-up` first.** Simplest local fix, but
   it's an opt-in target — operators in production wouldn't get it.
4. **Health endpoint surfaces drift.** `/health` reports the migration
   gap, monitoring catches it. Reactive rather than preventive.

Approaches 1 and 2 are complementary: 1 protects the runtime, 2 protects
the CI feedback loop. 3 is a 5-line ergonomics fix; 4 is for the
production case.

## Open questions

- For approach 1: do we ship a `MIGRATIONS_FS embed.FS` so the binary
  knows the highest migration it was built against, or do we re-read
  `migrations/` from disk at startup? Embed is more robust (binary stays
  truthful even if the working tree changes underneath it).
- Should the gate also detect the *backward* case (DB is ahead of code,
  e.g. running an older binary against a newer DB)? That can also fail
  reads if the older binary's columns were dropped.
- Where does the `dirty` flag in `schema_migrations` factor in? A dirty
  state means a migration crashed mid-apply — already a hard error, but
  worth refusing to start under that condition specifically.

## Checklist

- [ ] Brainstorm with the team / pick approach (1 + 2 + 3 likely).
- [ ] Spec the boot-time check: embed FS, compare-versions helper,
  refuse-or-warn policy, dirty-state behavior.
- [ ] Implement boot-time check in `cmd/api/main.go` startup path.
- [ ] Add `make run-api` dependency on `migrate-up` (or print a
  loud warning when out of sync).
- [ ] Add a CI step that runs `make migrate-up` against a temporary
  Postgres on every PR that touches `migrations/`.
- [ ] Document the developer story in `docs/DEVELOPMENT.md`: "if you
  pulled main and reads are 500ing, run `make migrate-up`".

## Trigger context

Surfaced during PR #88 smoke-test on 2026-05-01:

- Local DB at migration 59 (last touched 2026-04-30).
- Code at migration 61 after `cbaea3e` (PR #86, staged_changes Phase A)
  and PR #80 (flag hard-delete + retention, migration 060).
- API server bounced to a freshly built binary; first read after bounce
  500'd. Spent ~1h investigating before hitting `\d feature_flags`
  showing no `delete_after` column.

## Out of scope

- Production deployment migration ordering (that's a separate concern
  about whether to gate cutover on migration completion).
- Cross-process coordination of long-running migrations.

## Completion Record

<!-- Fill in when phase is set to Complete -->
- **Branch**:
- **Committed**: No
- **Pushed**: No
- **CI Checks**:
