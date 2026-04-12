# Getting Started

DeploySentry decouples deployment from release. This guide walks you from a fresh install to your first feature flag in production.

## Install

Self-host with Docker Compose:

```bash
git clone https://github.com/shadsorg/DeploySentry.git
cd DeploySentry
make dev-up
make migrate-up
make run-api
```

The API listens on `:8080` and the dashboard on `:3001`.

## Create your organization

1. Open http://localhost:3001
2. Sign up for an account
3. Create your first organization

## Create a project and application

Inside your organization, create a project and an application. Projects group related work; applications are the deployable units inside a project.

## Create your first flag

From the project's Flags page, click **New Flag**. Pick a category (release, feature, experiment, ops, or permission) and define a key.

## Wire up an SDK

Pick the SDK that matches your stack (see [SDKs](/docs/sdks)) and follow the init pattern. The minimum is one API key and the flag key you just created.
