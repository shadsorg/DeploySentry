# Getting Started with DeploySentry

A quick orientation for new users. This guide is a map — each step links to the authoritative detail elsewhere in the docs.

## Architecture at a Glance

DeploySentry is a Go API backed by PostgreSQL, Redis, and NATS JetStream, with a React web dashboard, Flutter mobile app, CLI, and client SDKs. The API serves REST endpoints for management and an SSE/gRPC streaming layer (the Sentinel server) that pushes flag updates to SDKs in real time.

```
Dashboard / Mobile / CLI / SDKs
            │
        HTTPS + SSE
            │
     DeploySentry API (Go/Gin)
      │        │       │
  PostgreSQL  Redis  NATS JetStream
                       │
                 Sentinel Server ──► SDK clients
```

For the full component diagram and stack details, see [README → Architecture](../README.md#architecture).

## Setup Steps

Each step below is a high-level checkpoint. Follow the linked section for the actual commands and options.

### 1. Bring up the backing services
Start PostgreSQL, Redis, and NATS locally, then run migrations against the `deploy` schema.
→ [README → Quick Start](../README.md#quick-start) · [DEVELOPMENT.md](./DEVELOPMENT.md)

### 2. Start the API and web dashboard
Run the API on `:8080` and the React dashboard on `:3001`.
→ [DEVELOPMENT.md](./DEVELOPMENT.md)

### 3. Create your organization
Register a user and create an org. Environments are defined at the org level and inherited by apps.
→ [README → Authentication](../README.md#authentication)

### 4. Create a project and application
Projects group related applications; applications are the unit flags and deployments target.
→ [README → Applications](../README.md#applications)

### 5. Invite members and assign roles
Add teammates at the org or project level. Roles: org (owner/admin/member/viewer), project (admin/developer/viewer).
→ [README → Member Management API](../README.md#member-management-api)

### 6. Generate an API key
Create an API key scoped to your project/environment for SDK and CLI use.
→ [README → Get an API Key](../README.md#2-get-an-api-key)

### 7. Create your first feature flag
Pick a category (`release`, `feature`, `experiment`, `ops`, `permission`). Release flags require an expiration date.
→ [README → Feature Flags](../README.md#feature-flags) · [README → Creating Flags](../README.md#creating-flags)

### 8. Integrate an SDK
Install the SDK for your language, initialize it with the API key, and call `isEnabled` / `getVariant`. SDKs for Go, Node, Python, Java, React, Flutter, and Ruby are available.
→ [SDK Onboarding](./sdk-onboarding.md) · [README → Integrating Into Your Project](../README.md#integrating-into-your-project)

### 9. Set up targeting rules
Define user/attribute-based rules and percentage rollouts. Changes stream to SDKs via SSE.
→ [README → Targeting Rules](../README.md#targeting-rules) · [README → SSE Streaming Protocol](../README.md#sse-streaming-protocol)

### 10. Monitor flags and deployments
Watch flag health, evaluation metrics, and deployment status from the dashboard. Configure webhooks, Slack, and observability exports.
→ [README → Monitoring Flags](../README.md#monitoring-flags) · [README → Webhooks & Slack](../README.md#webhook-notifications)

### 11. Deploy and enable automated rollbacks
Use the CLI to record deployments and releases; configure rollback rules tied to flag or health signals.
→ [README → Deployments](../README.md#deployments) · [README → Automated Rollbacks](../README.md#automated-rollbacks)

## Going to Production

Before running DeploySentry in a production environment, review the hardening checklist and the production-safety requirements (confirmation dialogs, rollback/history, managed secrets).
→ [PRODUCTION.md](./PRODUCTION.md)

## Bootstrap My App (LLM-Assisted Setup)

Have an AI coding assistant? Paste the bootstrap prompt into your project and it will automatically detect your SDKs, set up environment variables, create the flag registration file, and wire everything together.
→ [Bootstrap_My_App.md](./Bootstrap_My_App.md)

## Where to Go Next

- [Bootstrap_My_App.md](./Bootstrap_My_App.md) — LLM prompt to auto-integrate DeploySentry into any project
- [DEVELOPMENT.md](./DEVELOPMENT.md) — local dev loop, testing, schema conventions
- [sdk-onboarding.md](./sdk-onboarding.md) — per-language SDK walkthroughs
- [Current_Initiatives.md](./Current_Initiatives.md) — active work in flight
- [Feature_Flag_Engine_Improvements.md](./Feature_Flag_Engine_Improvements.md) — roadmap for the flag engine
