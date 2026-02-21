# 08 — Notification Service

## Core Service (`internal/notifications/service.go`)
- [ ] Event subscription (listen to NATS JetStream events)
- [ ] Notification routing (which events go to which channels)
- [ ] Notification template rendering
- [ ] Delivery retry with exponential backoff
- [ ] Notification preferences per user/project

## Slack Integration (`internal/notifications/slack.go`)
- [ ] Slack webhook integration
- [ ] Slack Bot API integration (optional)
- [ ] Rich message formatting (blocks, attachments)
- [ ] Channel configuration per project/environment
- [ ] Event-specific message templates:
  - [ ] Deployment started
  - [ ] Deployment phase completed
  - [ ] Deployment succeeded
  - [ ] Deployment failed
  - [ ] Rollback initiated
  - [ ] Rollback completed
  - [ ] Health alert triggered
  - [ ] Health alert resolved
  - [ ] Feature flag toggled

## Webhook Integration (`internal/notifications/webhook.go`)
- [ ] Webhook endpoint management (CRUD)
- [ ] HMAC signature verification (shared secret)
- [ ] Webhook event filtering (subscribe to specific event types)
- [ ] Delivery tracking and retry:
  - [ ] Retry up to 3 times with exponential backoff
  - [ ] Record response status and body
  - [ ] Mark delivery as pending/delivered/failed
- [ ] Webhook event payload format (RFC-compliant):
  - [ ] `id`, `type`, `created_at`, `project_id`, `data`
- [ ] Supported event types:
  - [ ] `deployment.created`, `deployment.phase.completed`, `deployment.completed`, `deployment.failed`
  - [ ] `deployment.rollback.initiated`, `deployment.rollback.completed`
  - [ ] `flag.created`, `flag.updated`, `flag.toggled`, `flag.archived`
  - [ ] `release.created`, `release.promoted`, `release.health.degraded`
  - [ ] `health.alert.triggered`, `health.alert.resolved`

## Email Integration (`internal/notifications/email.go`)
- [ ] Email provider integration (SMTP / SES)
- [ ] HTML email templates
- [ ] Email notification preferences
- [ ] Digest mode (batch notifications into periodic summaries)

## PagerDuty Integration
- [ ] Bi-directional integration:
  - [ ] Receive incident signals from PagerDuty
  - [ ] Trigger PagerDuty incidents on critical events
- [ ] Incident creation on health degradation
- [ ] Auto-resolve on recovery
