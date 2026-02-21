# 14 — Mobile App (Flutter)

## Project Setup
- [ ] Flutter project initialization (`flutter create`) in `mobile/` directory
- [ ] `pubspec.yaml` with dependencies (dio, provider, go_router, flutter_secure_storage)
- [ ] Dart analysis options and linting rules
- [ ] Flavor configuration for dev / staging / prod environments
- [ ] App icons and splash screen (iOS + Android)
- [ ] CI workflow for Flutter (analyze, test, build APK + IPA)

## Authentication & Session

### OAuth Login
- [ ] OAuth 2.0 login screen with provider selection (GitHub, Google)
- [ ] Browser-based OAuth redirect flow (`url_launcher` + deep link callback)
- [ ] JWT token storage in secure storage (`flutter_secure_storage`)
- [ ] Token refresh on 401 response (automatic retry via dio interceptor)
- [ ] Session persistence across app restarts
- [ ] Logout with token invalidation and local storage clearing

### RBAC Integration
- [ ] Fetch user roles and permissions on login (`GET /api/v1/auth/me`)
- [ ] Role-aware navigation — hide/disable screens based on role
- [ ] Read-only mode for `project:viewer` role (disable action buttons)
- [ ] Environment-scoped controls for `environment:deployer` role
- [ ] Permission checks before destructive actions (deploy, rollback, flag toggle)

## App Shell & Navigation

### Navigation Structure
- [ ] Bottom navigation bar: Dashboard, Deployments, Flags, Releases, Settings
- [ ] `go_router` setup with named routes and deep linking
- [ ] Organization switcher (multi-org support)
- [ ] Project selector with persistent last-selected project
- [ ] Pull-to-refresh on all list screens

### Theme & Design
- [ ] Material 3 design system with DeploySentry brand colors
- [ ] Light and dark theme support
- [ ] Responsive layout (phone + tablet)
- [ ] Status color system (green/yellow/red for health, deployment states)
- [ ] Skeleton loading states for all data screens

## API Client

### HTTP Layer
- [ ] Dio HTTP client with base URL configuration per environment
- [ ] Auth interceptor (attach JWT, handle 401 refresh)
- [ ] Request/response logging in debug mode
- [ ] Retry interceptor with exponential backoff for network failures
- [ ] API error model mapping (RFC 7807 Problem Details)
- [ ] Connectivity monitoring (online/offline detection)

### Offline Support
- [ ] Local cache for last-fetched data (SQLite or Hive)
- [ ] Serve cached data when offline with "offline" indicator
- [ ] Queue write actions (toggle, promote) for replay when back online
- [ ] Cache invalidation on successful fresh fetch

## Dashboard (Home Screen)
- [ ] Active deployments summary card (count, in-progress states)
- [ ] Deployment success rate metric (last 7 days)
- [ ] Feature flag summary card (total flags, recently toggled)
- [ ] Release health overview (healthy / degraded / rolled back counts)
- [ ] Recent activity feed (last 10 events across deploys, flags, releases)
- [ ] Quick action buttons: create deployment, create flag

## Deployments

### Deployment List
- [ ] Paginated deployment list with cursor-based pagination
- [ ] Filter by environment (dev, staging, prod)
- [ ] Filter by status (pending, in_progress, completed, failed, rolling_back)
- [ ] Filter by strategy (canary, blue_green, rolling)
- [ ] Search by release version or deployment ID
- [ ] Color-coded status badges

### Deployment Detail
- [ ] Deployment header: release version, environment, strategy, initiator
- [ ] Phase timeline visualization (stepper widget showing completed/active/pending phases)
- [ ] Traffic percentage indicator per phase (progress bar)
- [ ] Health score gauge (circular indicator, color-coded)
- [ ] Real-time status updates via SSE (`GET /api/v1/deployments/:id/stream`)
- [ ] Phase history with timestamps and health snapshots
- [ ] Metadata / config display (JSONB viewer)

### Deployment Actions (RBAC-gated)
- [ ] Create deployment form: select release, environment, strategy, config
- [ ] Promote to next phase (with confirmation dialog)
- [ ] Pause active deployment (with confirmation dialog)
- [ ] Resume paused deployment
- [ ] Trigger rollback (with confirmation dialog and reason input)

## Feature Flags

### Flag List
- [ ] Paginated flag list with search by key or name
- [ ] Filter by status (enabled, disabled, archived)
- [ ] Filter by flag type (boolean, string, number, JSON)
- [ ] Filter by tags
- [ ] Inline toggle switch per flag (with optimistic update)
- [ ] Color-coded type badges

### Flag Detail
- [ ] Flag header: key, name, type, enabled state, created by
- [ ] Large toggle switch for enable/disable
- [ ] Default value display
- [ ] Targeting rules list with rule type icons (percentage, user, attribute, segment, schedule)
- [ ] Evaluation log — recent evaluations with context and result
- [ ] Audit history — who changed what, when

### Flag Management (RBAC-gated)
- [ ] Create flag form: key, name, type, default value, tags
- [ ] Edit flag: update name, description, default value, tags
- [ ] Targeting rule editor:
  - [ ] Percentage rollout slider
  - [ ] User ID include/exclude list input
  - [ ] Attribute condition builder (field, operator, value)
  - [ ] Segment selector (dropdown from existing segments)
  - [ ] Schedule picker (start/end datetime)
- [ ] Add / remove / reorder targeting rules
- [ ] Archive flag (with confirmation)

## Releases

### Release List
- [ ] Paginated release list with environment status badges
- [ ] Filter by status (building, deploying, healthy, degraded, rolled_back)
- [ ] Search by version or commit SHA
- [ ] Environment progression indicators (dev → staging → prod)

### Release Detail
- [ ] Release header: version, commit SHA, branch, created by
- [ ] Changelog display
- [ ] Environment cards showing: status, deployed time, health score
- [ ] Promote to next environment button (RBAC-gated, with confirmation)
- [ ] Associated deployments list (link to deployment detail)

## Settings

### Organization Settings
- [ ] Organization name and slug display
- [ ] Team member list with roles
- [ ] Invite member (if org:admin or org:owner)
- [ ] Change member role (if org:admin or org:owner)
- [ ] Remove member (if org:owner)

### Project Settings
- [ ] Project name, description, repo URL
- [ ] Environment list with configuration
- [ ] API key list (masked, with copy prefix)
- [ ] Webhook endpoint list

### User Profile
- [ ] Display name, email, avatar
- [ ] Connected OAuth providers
- [ ] Active sessions
- [ ] Notification preferences (push notification toggles)

## Push Notifications
- [ ] Firebase Cloud Messaging (FCM) integration for Android
- [ ] Apple Push Notification service (APNs) integration for iOS
- [ ] Notification permission request flow
- [ ] Device token registration with backend (`POST /api/v1/devices`)
- [ ] Notification categories:
  - [ ] Deployment started / completed / failed
  - [ ] Deployment rollback triggered
  - [ ] Health alert (degraded release)
  - [ ] Flag toggled by another user
- [ ] Notification tap handling (deep link to relevant screen)
- [ ] In-app notification center (bell icon with badge count)
- [ ] Notification preferences per category (enable/disable)

## Real-Time Updates
- [ ] SSE client for deployment status streaming
- [ ] SSE client for flag change streaming
- [ ] Automatic reconnection with exponential backoff on disconnect
- [ ] Background SSE connection management (pause on app background, resume on foreground)
- [ ] Visual indicator when receiving live updates

## Testing
- [ ] Unit tests for all state management / providers (`flutter_test`)
- [ ] Unit tests for API client and response parsing
- [ ] Unit tests for RBAC permission logic
- [ ] Widget tests for key screens (dashboard, flag detail, deployment detail)
- [ ] Widget tests for form validation (create flag, create deployment)
- [ ] Integration tests for login flow (`integration_test`)
- [ ] Integration tests for deployment lifecycle (create → promote → complete)
- [ ] Integration tests for flag CRUD (create → toggle → edit rules → archive)
- [ ] Golden tests for critical UI components
- [ ] Mock API responses for offline / error state testing

## App Store Preparation
- [ ] App Store metadata (title, description, screenshots, keywords)
- [ ] Google Play Store listing
- [ ] Privacy policy page
- [ ] App signing configuration (iOS certificates, Android keystore)
- [ ] Fastlane setup for automated builds and uploads
- [ ] Beta testing distribution (TestFlight for iOS, internal track for Android)
