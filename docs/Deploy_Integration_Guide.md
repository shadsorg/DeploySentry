# Deploy Integration Guide: GitHub Actions

**Phase**: Complete

## Overview

This guide covers how to connect a GitHub repository to DeploySentry so that deployments are automatically created, monitored, and (optionally) rolled back when you push or release code. It covers setup on three sides: DeploySentry, GitHub, and your repository.

---

## How It Works

```
 Your Repository                  GitHub                      DeploySentry
 ┌─────────────┐          ┌──────────────────┐         ┌──────────────────────┐
 │ git push     │────────► │ GitHub Actions   │         │                      │
 │ main branch  │          │ builds + deploys │         │   ds-sentry.com      │
 └─────────────┘          │ to your infra    │         │                      │
                          └────────┬─────────┘         │  ┌────────────────┐  │
                                   │                    │  │ Phase Engine   │  │
                                   │ POST /deployments  │  │ (drives canary │  │
                                   ├───────────────────►│  │  phases auto-  │  │
                                   │                    │  │  matically)    │  │
                                   │                    │  └───────┬────────┘  │
                                   │                    │          │           │
                          ┌────────┴─────────┐         │  ┌───────▼────────┐  │
                          │ GitHub Webhook   │◄────────│  │ Health Monitor │  │
                          │ (status checks)  │         │  │ (checks your   │  │
                          └──────────────────┘         │  │  app health)   │  │
                                                       │  └───────┬────────┘  │
                                                       │          │           │
                                                       │  ┌───────▼────────┐  │
                                                       │  │ Rollback Ctrl  │  │
                                                       │  │ (auto-rollback │  │
                                                       │  │  if unhealthy) │  │
                                                       │  └────────────────┘  │
                                                       │                      │
                                                       │  Notifications ──►   │
                                                       │  Slack / Email /     │
                                                       │  PagerDuty / Webhook │
                                                       └──────────────────────┘
```

**The deployment model**: DeploySentry does not perform the actual deployment to your infrastructure — your CI/CD pipeline (GitHub Actions) does that. DeploySentry tracks the deployment lifecycle: it records what was deployed, monitors its health, drives canary traffic phases, and triggers rollbacks if something goes wrong.

Think of it as a deployment control plane sitting alongside your existing CI/CD.

---

## Prerequisites

Before starting, you need:

- A DeploySentry account at ds-sentry.com
- An organization, project, and application set up in DeploySentry
- At least one environment configured (e.g., `production`, `staging`)
- A GitHub repository with Actions enabled

---

## Step 1: DeploySentry Setup

### 1.1 Create an API Key

In the DeploySentry dashboard:

1. Navigate to your project
2. Go to **Settings > API Keys**
3. Click **Create API Key**
4. Give it a name like `github-actions-deploy`
5. Select scopes: `flags:read`, `deploys:read`, `deploys:write`
6. Copy the key (starts with `ds_live_` or `ds_test_`)

Save this key — you'll add it to GitHub secrets in Step 2.

### 1.2 Note Your IDs

You'll need these values for the GitHub Action. Find them in the dashboard or via the API:

| Value | Where to find it |
|-------|-----------------|
| Application ID | Dashboard > Project > Application, or `GET /api/v1/orgs/:org/projects/:project/apps/:app` |
| Environment ID | Dashboard > Org Settings > Environments, or `GET /api/v1/orgs/:org/environments` |

### 1.3 Configure a Webhook (Optional)

If you want DeploySentry to post deployment status back to a Slack channel or other service:

1. Go to **Org Settings > Webhooks**
2. Click **Create Webhook**
3. Enter the target URL
4. Select events: `deployment.created`, `deployment.completed`, `deployment.failed`, `deployment.rolled_back`
5. Save — DeploySentry will deliver signed payloads to this URL on each event

### 1.4 Configure Health Monitoring (Optional)

If you want automated rollbacks based on health signals:

1. Go to **Project Settings**
2. Configure a health check integration (Prometheus, Datadog, Sentry, or a custom HTTP endpoint)
3. Set the health threshold (default: 95% — deployments roll back if health drops below this)

---

## Step 2: GitHub Setup

### 2.1 Add Repository Secrets

In your GitHub repository:

1. Go to **Settings > Secrets and variables > Actions**
2. Add these secrets:

| Secret Name | Value |
|-------------|-------|
| `DS_API_KEY` | Your DeploySentry API key (`ds_live_...`) |
| `DS_APP_ID` | Your DeploySentry Application ID (UUID) |
| `DS_ENV_ID` | Your DeploySentry Environment ID (UUID) |

Optionally:

| Secret Name | Value | Default |
|-------------|-------|---------|
| `DS_API_URL` | DeploySentry API URL | `https://ds-sentry.com` |
| `DS_STRATEGY` | Deployment strategy | `rolling` |

### 2.2 Choose Your Integration Pattern

There are two ways to connect GitHub to DeploySentry:

**Pattern A: GitHub Action step (recommended)** — Your workflow calls the DeploySentry API after your deployment step completes. You control exactly when the deployment is recorded.

**Pattern B: GitHub Webhook (automatic)** — DeploySentry listens for push/release events from GitHub and creates deployments automatically. Less configuration in the repo, but less control over timing.

Most teams use **Pattern A** because it lets you record the deployment at the right moment — after the build succeeds and the deploy to infrastructure completes.

---

## Step 3: Repository Setup

### Pattern A: GitHub Action Step

Add a deployment notification step to your existing workflow. This goes **after** your build and deploy steps.

#### Basic Setup (Rolling Deploy)

```yaml
# .github/workflows/deploy.yml
name: Deploy

on:
  push:
    branches: [main]

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      # ── Your existing build + deploy steps ──
      - name: Build
        run: |
          # your build commands here
          docker build -t myapp:${{ github.sha }} .

      - name: Deploy to infrastructure
        run: |
          # your deploy commands here (kubectl, aws, etc.)
          kubectl set image deployment/myapp myapp=myapp:${{ github.sha }}

      # ── Record the deployment in DeploySentry ──
      - name: Record deployment
        if: success()
        env:
          DS_API_KEY: ${{ secrets.DS_API_KEY }}
          DS_API_URL: ${{ secrets.DS_API_URL || 'https://ds-sentry.com' }}
          DS_APP_ID: ${{ secrets.DS_APP_ID }}
          DS_ENV_ID: ${{ secrets.DS_ENV_ID }}
        run: |
          DEPLOY_RESPONSE=$(curl -sf -X POST "${DS_API_URL}/api/v1/deployments" \
            -H "Authorization: ApiKey ${DS_API_KEY}" \
            -H "Content-Type: application/json" \
            -d "{
              \"application_id\": \"${DS_APP_ID}\",
              \"environment_id\": \"${DS_ENV_ID}\",
              \"strategy\": \"rolling\",
              \"artifact\": \"${{ github.repository }}\",
              \"version\": \"${{ github.sha }}\",
              \"commit_sha\": \"${{ github.sha }}\",
              \"description\": \"Deployed from GitHub Actions (${GITHUB_REF_NAME})\"
            }")

          DEPLOY_ID=$(echo "$DEPLOY_RESPONSE" | jq -r '.deployment.id // .id')
          echo "DEPLOY_ID=${DEPLOY_ID}" >> $GITHUB_ENV
          echo "Deployment created: ${DEPLOY_ID}"
```

#### Canary Deploy with Monitoring

For canary deployments, DeploySentry drives the traffic phases automatically. Your workflow creates the deployment, and the Phase Engine takes over:

```yaml
# .github/workflows/deploy-canary.yml
name: Canary Deploy

on:
  push:
    branches: [main]

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Build and push image
        run: |
          docker build -t myapp:${{ github.sha }} .
          docker push myapp:${{ github.sha }}

      # Deploy the canary instance (1% traffic initially)
      - name: Deploy canary
        run: |
          kubectl apply -f k8s/canary.yaml

      # Record in DeploySentry — Phase Engine takes over from here
      - name: Create canary deployment
        env:
          DS_API_KEY: ${{ secrets.DS_API_KEY }}
          DS_API_URL: ${{ secrets.DS_API_URL || 'https://ds-sentry.com' }}
          DS_APP_ID: ${{ secrets.DS_APP_ID }}
          DS_ENV_ID: ${{ secrets.DS_ENV_ID }}
        run: |
          DEPLOY_RESPONSE=$(curl -sf -X POST "${DS_API_URL}/api/v1/deployments" \
            -H "Authorization: ApiKey ${DS_API_KEY}" \
            -H "Content-Type: application/json" \
            -d "{
              \"application_id\": \"${DS_APP_ID}\",
              \"environment_id\": \"${DS_ENV_ID}\",
              \"strategy\": \"canary\",
              \"artifact\": \"myapp:${{ github.sha }}\",
              \"version\": \"${{ github.sha }}\",
              \"commit_sha\": \"${{ github.sha }}\"
            }")

          DEPLOY_ID=$(echo "$DEPLOY_RESPONSE" | jq -r '.deployment.id // .id')
          echo "Canary deployment created: ${DEPLOY_ID}"
          echo "DeploySentry will drive phases: 1% → 5% → 25% → 50% → 100%"
          echo "Monitor at: ${DS_API_URL}/deployments/${DEPLOY_ID}"
```

After creation, DeploySentry's Phase Engine automatically:
1. Transitions the deployment to `running`
2. Drives through canary phases (1% → 5% → 25% → 50% → 100%)
3. Checks health at each phase boundary
4. Rolls back if health drops below threshold
5. Completes the deployment when all phases pass

#### Wait for Deployment Completion (Optional)

If you want your workflow to wait for DeploySentry to finish monitoring:

```yaml
      - name: Wait for deployment completion
        timeout-minutes: 30
        env:
          DS_API_KEY: ${{ secrets.DS_API_KEY }}
          DS_API_URL: ${{ secrets.DS_API_URL || 'https://ds-sentry.com' }}
        run: |
          echo "Waiting for deployment ${DEPLOY_ID} to complete..."
          while true; do
            STATUS=$(curl -sf "${DS_API_URL}/api/v1/deployments/${DEPLOY_ID}" \
              -H "Authorization: ApiKey ${DS_API_KEY}" \
              | jq -r '.deployment.status // .status')

            echo "Status: ${STATUS}"

            case "$STATUS" in
              completed)
                echo "Deployment completed successfully"
                exit 0
                ;;
              failed|rolled_back|cancelled)
                echo "Deployment ${STATUS}"
                exit 1
                ;;
            esac

            sleep 30
          done
```

#### With the DeploySentry CLI

If you prefer the CLI over raw API calls:

```yaml
      - name: Install DeploySentry CLI
        run: curl -fsSL https://ds-sentry.com/install.sh | sh

      - name: Create deployment
        env:
          DEPLOYSENTRY_API_KEY: ${{ secrets.DS_API_KEY }}
          DEPLOYSENTRY_URL: ${{ secrets.DS_API_URL || 'https://ds-sentry.com' }}
        run: |
          deploysentry deploy create \
            --release "${{ github.sha }}" \
            --env production \
            --strategy canary \
            --description "Deploy from ${{ github.ref_name }}"

      - name: Watch deployment
        env:
          DEPLOYSENTRY_API_KEY: ${{ secrets.DS_API_KEY }}
        run: |
          deploysentry deploy status --watch
```

---

### Pattern B: GitHub Webhook (Automatic)

With this pattern, GitHub sends push/release events directly to DeploySentry, which creates deployments automatically.

#### Setup on DeploySentry

1. Go to **Org Settings** in the dashboard
2. Navigate to the GitHub integration section
3. Configure:
   - **Webhook Secret**: Generate a strong secret (you'll add this to GitHub)
   - **Default Strategy**: `rolling`, `canary`, or `blue_green`
   - **Deploy Branches**: `main`, `master` (or your deploy branch)
   - **Auto Deploy**: Enable to create deployments on every push

#### Setup on GitHub

1. Go to your repository **Settings > Webhooks**
2. Click **Add webhook**
3. Configure:
   - **Payload URL**: `https://ds-sentry.com/api/v1/integrations/github/webhook`
   - **Content type**: `application/json`
   - **Secret**: The webhook secret from DeploySentry
   - **Events**: Select "Pushes" and "Releases"

#### How It Works

When you push to a configured branch:

1. GitHub sends a `push` event to DeploySentry
2. DeploySentry verifies the HMAC-SHA256 signature
3. A deployment is created with:
   - **Version**: The commit SHA
   - **Artifact**: The repository full name (e.g., `org/repo`)
   - **Strategy**: Your configured default
4. The Phase Engine takes over (for canary deployments)

When you publish a release:

1. GitHub sends a `release` event
2. DeploySentry creates a deployment with the release tag as the version
3. Pre-releases are handled based on your Auto Deploy setting

---

## Deployment Strategies

### Rolling (Default)

Replaces instances in batches. Simplest strategy — good for most services.

```
Phase 1: Replace batch 1 (1 of 3 instances) ──► health check ──► continue
Phase 2: Replace batch 2 (2 of 3 instances) ──► health check ──► continue
Phase 3: Replace batch 3 (3 of 3 instances) ──► health check ──► complete
```

Configuration defaults: batch size 1, 30s delay between batches, max 1 unavailable.

### Canary

Routes a small percentage of traffic to the new version, increasing gradually. Best for critical services where you need confidence before full rollout.

```
Phase 1:   1% traffic for  5 min ──► health check ──► auto-promote
Phase 2:   5% traffic for  5 min ──► health check ──► auto-promote
Phase 3:  25% traffic for 10 min ──► health check ──► auto-promote
Phase 4:  50% traffic for 10 min ──► health check ──► auto-promote
Phase 5: 100% traffic             ──► complete
```

If health drops below 95% at any phase, the deployment is automatically rolled back to 0% traffic.

Phases can also be manual gates — the Phase Engine pauses and waits for you to advance via the dashboard or API (`POST /deployments/:id/advance`).

### Blue-Green

Deploys to a standby environment, verifies health, then switches all traffic at once. Best for zero-downtime requirements.

```
Deploy to green ──► warmup (2 min) ──► health check ──► switch all traffic ──► complete
                                            │
                                            └──► unhealthy ──► rollback (switch back to blue)
```

---

## Monitoring & Automated Rollback

### How Health Monitoring Works

Once a deployment is created, DeploySentry's health monitor periodically checks your application:

1. **Health checks run** on a configurable interval (default: every 30 seconds)
2. **Multiple sources** can feed health data: Prometheus, Datadog, Sentry, or a custom HTTP endpoint
3. **Scores are aggregated** into a single 0.0–1.0 health score
4. The **Rollback Controller** watches this score

### Automatic Rollback Triggers

| Trigger | Default Threshold | Evaluation Window |
|---------|-------------------|-------------------|
| Error rate | > 5% | 2 minutes sustained |
| P99 latency | > 2 seconds | 2 minutes sustained |
| Health score | < 95% | 2 minutes sustained |

If any trigger fires for the full evaluation window, the rollback controller:

1. Transitions the deployment to `rolled_back`
2. Sets traffic to 0% on the new version
3. Creates a `RollbackRecord` with `automatic: true`
4. Publishes a `deployment.rolled_back` event
5. Notifies all configured channels (Slack, email, PagerDuty)
6. Enters a 5-minute cooldown before allowing another rollback

### Manual Rollback

From the dashboard, CLI, or API:

```bash
# CLI
deploysentry deploy rollback <deployment-id> --reason "Elevated error rates"

# API
curl -X POST https://ds-sentry.com/api/v1/deployments/<id>/rollback \
  -H "Authorization: ApiKey <key>" \
  -H "Content-Type: application/json" \
  -d '{"reason": "Elevated error rates"}'
```

---

## Deployment Lifecycle Events

DeploySentry publishes events at each state transition. These drive notifications, webhooks, and the dashboard UI.

| Event | When |
|-------|------|
| `deployment.created` | Deployment recorded |
| `deployment.started` | Phase Engine begins driving phases |
| `deployment.phase.completed` | A canary phase passed health checks |
| `deployment.completed` | All phases passed, deployment is live |
| `deployment.paused` | User paused the deployment |
| `deployment.resumed` | User resumed a paused deployment |
| `deployment.failed` | Deployment encountered an error |
| `deployment.rolled_back` | Automatic or manual rollback triggered |
| `deployment.cancelled` | User cancelled a pending deployment |
| `health.degraded` | Health score dropped below threshold |
| `health.alert.triggered` | Health alert fired |
| `health.alert.resolved` | Health recovered |

---

## Notifications

Configure notification channels in **Org Settings > Notifications**:

| Channel | Setup |
|---------|-------|
| **Slack** | Provide a Slack webhook URL. Events post as formatted messages with deployment details. |
| **Email** | Configure SMTP or provide email addresses. Sends on deployment completion, failure, and rollback. |
| **PagerDuty** | Provide a PagerDuty routing key. Creates incidents on rollback and health degradation. |
| **Webhooks** | Provide any HTTP endpoint. Payloads are signed with HMAC-SHA256 (`X-DeploySentry-Signature` header). |

---

## API Quick Reference

All endpoints require `Authorization: ApiKey <key>` header.

### Create a Deployment

```
POST /api/v1/deployments
```

```json
{
  "application_id": "uuid",
  "environment_id": "uuid",
  "strategy": "rolling | canary | blue_green",
  "artifact": "docker-image:tag or repo name",
  "version": "v1.2.3 or commit SHA",
  "commit_sha": "full 40-char SHA",
  "description": "optional human-readable note"
}
```

### Check Deployment Status

```
GET /api/v1/deployments/:id
```

Returns the full deployment object including `status`, `traffic_percent`, `started_at`, `completed_at`.

### List Deployments

```
GET /api/v1/deployments?app_id=<uuid>&limit=20
```

### Promote / Advance

```
POST /api/v1/deployments/:id/promote    # Promote to 100% traffic
POST /api/v1/deployments/:id/advance    # Advance past a manual canary gate
```

### Pause / Resume

```
POST /api/v1/deployments/:id/pause
POST /api/v1/deployments/:id/resume
```

### Rollback

```
POST /api/v1/deployments/:id/rollback
{"reason": "optional reason string"}
```

### Desired State (for controllers/agents)

```
GET /api/v1/deployments/:id/desired-state
GET /api/v1/applications/:app_id/desired-state
```

Returns what should be running — artifact, version, traffic percent, current phase. Useful for custom controllers that reconcile actual vs. desired state.

---

## Complete Example: Full Workflow

Here's a complete `.github/workflows/deploy.yml` that builds, deploys, records the deployment, and waits for DeploySentry to confirm health:

```yaml
name: Deploy to Production

on:
  push:
    branches: [main]

env:
  DS_API_URL: https://ds-sentry.com

jobs:
  build:
    runs-on: ubuntu-latest
    outputs:
      image_tag: ${{ steps.build.outputs.tag }}
    steps:
      - uses: actions/checkout@v4

      - name: Build and push Docker image
        id: build
        run: |
          TAG="${{ github.sha }}"
          docker build -t myapp:${TAG} .
          docker push myapp:${TAG}
          echo "tag=${TAG}" >> $GITHUB_OUTPUT

  deploy:
    needs: build
    runs-on: ubuntu-latest
    steps:
      - name: Deploy to Kubernetes
        run: |
          kubectl set image deployment/myapp \
            myapp=myapp:${{ needs.build.outputs.image_tag }}
          kubectl rollout status deployment/myapp --timeout=120s

      - name: Record deployment in DeploySentry
        id: ds_deploy
        env:
          DS_API_KEY: ${{ secrets.DS_API_KEY }}
          DS_APP_ID: ${{ secrets.DS_APP_ID }}
          DS_ENV_ID: ${{ secrets.DS_ENV_ID }}
        run: |
          RESPONSE=$(curl -sf -X POST "${DS_API_URL}/api/v1/deployments" \
            -H "Authorization: ApiKey ${DS_API_KEY}" \
            -H "Content-Type: application/json" \
            -d "{
              \"application_id\": \"${DS_APP_ID}\",
              \"environment_id\": \"${DS_ENV_ID}\",
              \"strategy\": \"canary\",
              \"artifact\": \"myapp:${{ needs.build.outputs.image_tag }}\",
              \"version\": \"${{ github.sha }}\",
              \"commit_sha\": \"${{ github.sha }}\",
              \"description\": \"${{ github.event.head_commit.message }}\"
            }")

          DEPLOY_ID=$(echo "$RESPONSE" | jq -r '.deployment.id // .id')
          echo "deploy_id=${DEPLOY_ID}" >> $GITHUB_OUTPUT
          echo "::notice::Deployment ${DEPLOY_ID} created. Canary phases starting."

      - name: Wait for canary to complete
        timeout-minutes: 45
        env:
          DS_API_KEY: ${{ secrets.DS_API_KEY }}
          DEPLOY_ID: ${{ steps.ds_deploy.outputs.deploy_id }}
        run: |
          while true; do
            RESULT=$(curl -sf "${DS_API_URL}/api/v1/deployments/${DEPLOY_ID}" \
              -H "Authorization: ApiKey ${DS_API_KEY}")

            STATUS=$(echo "$RESULT" | jq -r '.deployment.status // .status')
            TRAFFIC=$(echo "$RESULT" | jq -r '.deployment.traffic_percent // .traffic_percent')

            echo "$(date '+%H:%M:%S') Status: ${STATUS} | Traffic: ${TRAFFIC}%"

            case "$STATUS" in
              completed)
                echo "::notice::Deployment completed successfully at 100% traffic"
                exit 0
                ;;
              failed|rolled_back|cancelled)
                echo "::error::Deployment ${STATUS}"
                exit 1
                ;;
            esac

            sleep 30
          done
```

---

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| `401 Unauthorized` | Invalid or missing API key | Check `DS_API_KEY` secret. Key must have `deploys:write` scope. |
| `404 Not Found` on create | Invalid application or environment ID | Verify `DS_APP_ID` and `DS_ENV_ID` match UUIDs in the dashboard |
| Deployment stuck in `pending` | Phase Engine not running or NATS disconnected | Check DeploySentry server health at `/health` |
| No rollback despite errors | Health monitoring not configured | Set up a health check integration in project settings |
| Webhook not received | Signature mismatch | Verify the webhook secret matches between GitHub and DeploySentry |
| `409 Conflict` | Active deployment already exists for this app+env | Wait for the current deployment to complete, or cancel it first |

## Checklist

- [x] Integration architecture diagram
- [x] DeploySentry setup (API key, IDs, webhooks, health monitoring)
- [x] GitHub setup (secrets, webhook)
- [x] Pattern A: GitHub Action step with full workflow examples
- [x] Pattern B: GitHub Webhook automatic mode
- [x] All three deployment strategies explained
- [x] Health monitoring and automated rollback model
- [x] Event lifecycle reference
- [x] Notification channels
- [x] Full API quick reference
- [x] Complete end-to-end workflow example
- [x] Troubleshooting table

## Completion Record

- **Branch**: `feature/groups-and-resource-authorization`
- **Committed**: No (pending review)
- **Pushed**: No
- **CI Checks**: N/A (documentation only)
