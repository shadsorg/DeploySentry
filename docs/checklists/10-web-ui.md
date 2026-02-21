# 10 — Web UI (React Dashboard)

## Project Setup
- [x] React + TypeScript project initialization
- [x] `package.json` with dependencies
- [x] `tsconfig.json` configuration
- [x] Build configuration (Vite or webpack)
- [x] ESLint + Prettier configuration
- [ ] Tailwind CSS or styled-components setup

## Application Shell
- [ ] App layout with sidebar navigation
- [ ] Header with user profile, org switcher
- [ ] Authentication flow (OAuth redirect)
- [ ] Protected route wrapper
- [ ] API client with auth token management
- [ ] Error boundary components
- [ ] Loading states / skeleton screens

## Pages

### Dashboard (Home)
- [ ] Deployment overview: active deploys, success rate, mean time to deploy
- [ ] Recent deployments list
- [ ] Feature flag usage summary
- [ ] Release health summary
- [ ] Quick actions (create deploy, create flag)

### Deployments Page
- [ ] Deployments list with filtering (environment, status, strategy)
- [ ] Deployment detail view:
  - [ ] Phase timeline visualization
  - [ ] Traffic percentage indicator
  - [ ] Health score gauge
  - [ ] Action buttons: promote, pause, resume, rollback
- [ ] Live deployment status updates (SSE/WebSocket)
- [ ] Deployment creation wizard

### Feature Flags Page
- [ ] Flags list with search, filter by tags/status
- [ ] Flag detail view:
  - [ ] Toggle switch
  - [ ] Targeting rules editor
  - [ ] Evaluation log / recent evaluations
  - [ ] Flag history (audit trail)
- [ ] Flag creation form
- [ ] Targeting rule builder UI:
  - [ ] Percentage slider
  - [ ] User ID list input
  - [ ] Attribute condition builder
  - [ ] Segment selector
  - [ ] Schedule picker

### Releases Page
- [ ] Release timeline visualization
- [ ] Release list with environment status badges
- [ ] Release detail view:
  - [ ] Commit info, changelog
  - [ ] Environment progression (dev → staging → prod)
  - [ ] Health score per environment
  - [ ] Promote action button

### Settings Pages
- [ ] Organization settings
- [ ] Project settings
- [ ] Environment configuration
- [ ] Webhook management
- [ ] API key management
- [ ] Team / member management (invite, roles)
- [ ] Notification preferences

## State Management
- [ ] Decide: Redux Toolkit / Zustand / React Query only (Open question #5)
- [ ] API hooks for data fetching
- [ ] Optimistic updates for toggles and actions
- [ ] Real-time update subscriptions

## Components Library
- [ ] Button, Input, Select, Toggle components
- [ ] Modal / Dialog
- [ ] Table with sorting and pagination
- [ ] Toast / notification system
- [ ] Status badges (deployment status, flag status)
- [ ] Health score gauge/indicator
- [ ] Timeline / stepper component
- [ ] Code/JSON viewer
- [ ] Confirmation dialogs for destructive actions

## Key Dashboards (Grafana-style)
- [ ] Deployment overview: active deploys, success rate, mean time to deploy
- [ ] Feature flag usage: evaluation volume, flag age, stale flags
- [ ] Release health: per-release health scores, rollback frequency
- [ ] System health: API latency, error rates, resource utilization
