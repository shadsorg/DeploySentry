# 08 — Notification Service

## Core Service (`internal/notifications/service.go`)
- [x] Event subscription (listen to NATS JetStream events)
- [x] Notification routing (which events go to which channels)
- [x] Notification template rendering
- [x] Delivery retry with exponential backoff
- [x] Notification preferences per user/project

## Slack Integration (`internal/notifications/slack.go`)
- [x] Slack webhook integration
- [ ] Slack Bot API integration (optional)
- [x] Rich message formatting (blocks, attachments)
- [x] Channel configuration per project/environment
- [x] Event-specific message templates:
  - [x] Deployment started
  - [x] Deployment phase completed
  - [x] Deployment succeeded
  - [x] Deployment failed
  - [x] Rollback initiated
  - [x] Rollback completed
  - [x] Health alert triggered
  - [x] Health alert resolved
  - [x] Feature flag toggled

## Webhook Integration (`internal/notifications/webhook.go`)
- [x] Webhook endpoint management (CRUD)
- [x] HMAC signature verification (shared secret)
- [x] Webhook event filtering (subscribe to specific event types)
- [x] Delivery tracking and retry:
  - [x] Retry up to 3 times with exponential backoff
  - [x] Record response status and body
  - [x] Mark delivery as pending/delivered/failed
- [x] Webhook event payload format (RFC-compliant):
  - [x] `id`, `type`, `created_at`, `project_id`, `data`
- [x] Supported event types:
  - [x] `deployment.created`, `deployment.phase.completed`, `deployment.completed`, `deployment.failed`
  - [x] `deployment.rollback.initiated`, `deployment.rollback.completed`
  - [x] `flag.created`, `flag.updated`, `flag.toggled`, `flag.archived`
  - [x] `release.created`, `release.promoted`, `release.health.degraded`
  - [x] `health.alert.triggered`, `health.alert.resolved`

## Email Integration (`internal/notifications/email.go`)
- [x] Email provider integration (SMTP / SES)
- [x] HTML email templates
- [x] Email notification preferences
- [x] Digest mode (batch notifications into periodic summaries)

## PagerDuty Integration
- [x] Bi-directional integration:
  - [x] Receive incident signals from PagerDuty
  - [x] Trigger PagerDuty incidents on critical events
- [x] Incident creation on health degradation
- [x] Auto-resolve on recovery
