# MCP Flag Routing Fix

**Phase**: Implementation

## Overview

`ds_list_flags` and `ds_create_flag` MCP tools return `404 page not found`
because they POST/GET against `/api/v1/orgs/{org}/projects/{project}/flags` —
a path the API never registered. The flag handler mounts at
`rg.Group("/flags")` (flat under `/api/v1`), unlike `apps`/`environments`
which are nested under `/orgs/.../projects/...`. The MCP author assumed the
nesting pattern was uniform.

There is also an underlying asymmetry in the API: `evaluateRequest.ProjectID`
is `string` and resolves slug-or-UUID via `resolveProjectID`, but
`createFlagRequest.ProjectID` is `uuid.UUID` and rejects slugs at JSON-bind
time. Fixing both gives the MCP tools a single consistent contract and matches
the slug-friendly behavior of `evaluate` and `listFlags`.

## Checklist

- [ ] Relax `createFlagRequest.ProjectID` to `string`; pass through
  `h.resolveProjectID(c, req.ProjectID)` in `internal/flags/handler.go`.
- [ ] Add a slug-based create test alongside the existing UUID test in
  `internal/flags/handler_test.go`.
- [ ] Fix `internal/mcp/tools_flags.go:handleListFlags` to call
  `GET /api/v1/flags?project_id={project_slug}`; remove the org from the URL
  (auth context already scopes by org).
- [ ] Fix `internal/mcp/tools_flags.go:handleCreateFlag` to call
  `POST /api/v1/flags` with `project_id` (slug) in the body.
- [ ] `go build ./...` and `go test ./internal/flags/... ./internal/mcp/...`.
- [ ] Rebuild `/Users/sgamel/bin/deploysentry` (out of scope for this PR;
  noted for the operator).

## Out of scope

- Adding nested `/orgs/{org}/projects/{project}/flags` routes. The flat shape
  matches how the CLI (`cmd/cli/flags.go:284`) and SDK already consume the
  endpoint; nesting flags would force changes across far more code than the
  MCP fix.
- `EnvironmentID` / `ApplicationID` continue to require UUIDs in
  `createFlagRequest`. The current MCP tool exposes neither, so the asymmetry
  doesn't bite us today.

## Completion Record

<!-- Fill in when phase is set to Complete -->
- **Branch**:
- **Committed**: No
- **Pushed**: No
- **CI Checks**:
