# UI Features

A tour of every page in the DeploySentry dashboard.

## Projects

Lists every project in the current organization. Click a project to open its applications, flags, and analytics.

## Applications

Each project contains one or more applications — the deployable units. Each app has its own deployments, releases, and flags.

## Feature Flags

The flag list shows every flag for the current scope (project or app). Each flag has a key, category (release/feature/experiment/ops/permission), targeting rules, and rollout status.

## Flag Detail

Edit targeting rules, view evaluation history, and toggle the flag per environment.

## Deployments

Lists deployments for the current application. Each deployment links to its release record and source commit.

## Releases

Releases are independent of deployments. A release is a flag rollout — opening the gate to a cohort.

## Members

Manage organization members and their roles (owner, admin, member, viewer).

## API Keys

Create and revoke API keys scoped to the current organization.

## Settings

Hierarchical settings: org > project > app > environment. Lower levels inherit from higher levels unless overridden.

## Analytics

Per-flag evaluation counts, error rates, and rollout health.

## SDKs

Quickstart snippets for every SDK, prefilled with your API key.
