# 08 — Notification Service

## Core Service (`internal/notifications/service.go`)
- [ ] Event subscription (listen to NATS JetStream events)
- [x] Notification routing (which events go to which channels)
- [x] Notification template rendering
- [ ] Delivery retry with exponential backoff
- [ ] Notification preferences per user/project

## Slack Integration (`internal/notifications/slack.go`)
- [x] Slack webhook integration
- [ ] Slack Bot API integration (optional)
- [x] Rich message formatting (blocks, attachments)
- [x] Channel configuration per project/environment
- [ ] Event-specific message templates:
  - [x] Deployment started
  - [ ] Deployment phase completed
  - [x] Deployment succeeded
  - [x] Deployment failed
  - [ ] Rollback initiated
  - [x] Rollback completed
  - [x] Health alert triggered
  - [ ] Health alert resolved
  - [x] Feature flag toggled

## Webhook Integration (`internal/notifications/webhook.go`)
- [ ] Webhook endpoint management (CRUD)
- [x] HMAC signature verification (shared secret)
- [x] Webhook event filtering (subscribe to specific event types)
- [x] Delivery tracking and retry:
  - [x] Retry up to 3 times with exponential backoff
  - [x] Record response status and body
  - [ ] Mark delivery as pending/delivered/failed
- [x] Webhook event payload format (RFC-compliant):
  - [x] `id`, `type`, `created_at`, `project_id`, `data`
- [ ] Supported event types:
  - [ ] `deployment.created`, `deployment.phase.completed`, `deployment.completed`, `deployment.failed`
  - [ ] `deployment.rollback.initiated`, `deployment.rollback.completed`
  - [ ] `flag.created`, `flag.updated`, `flag.toggled`, `flag.archived`
  - [ ] `release.created`, `release.promoted`, `release.health.degraded`
  - [ ] `health.alert.triggered`, `health.alert.resolved`

## Email Integration (`internal/notifications/email.go`)
- [x] Email provider integration (SMTP / SES)
- [ ] HTML email templates
- [ ] Email notification preferences
- [ ] Digest mode (batch notifications into periodic summaries)

## PagerDuty Integration
- [ ] Bi-directional integration:
  - [ ] Receive incident signals from PagerDuty
  - [ ] Trigger PagerDuty incidents on critical events
- [ ] Incident creation on health degradation
- [ ] Auto-resolve on recovery
