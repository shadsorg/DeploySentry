// Package main is the entrypoint for the DeploySentry API server.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"encoding/json"

	"github.com/deploysentry/deploysentry/internal/agent/registry"
	"github.com/deploysentry/deploysentry/internal/analytics"
	"github.com/deploysentry/deploysentry/internal/auth"
	"github.com/deploysentry/deploysentry/internal/deploy"
	"github.com/deploysentry/deploysentry/internal/deploy/engine"
	"github.com/deploysentry/deploysentry/internal/entities"
	"github.com/deploysentry/deploysentry/internal/flags"
	"github.com/deploysentry/deploysentry/internal/grants"
	"github.com/deploysentry/deploysentry/internal/groups"
	"github.com/deploysentry/deploysentry/internal/health"
	githubint "github.com/deploysentry/deploysentry/internal/integrations/github"
	"github.com/deploysentry/deploysentry/internal/members"
	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/deploysentry/deploysentry/internal/notifications"
	"github.com/deploysentry/deploysentry/internal/platform/cache"
	"github.com/deploysentry/deploysentry/internal/platform/cache/flagcache"
	"github.com/deploysentry/deploysentry/internal/platform/config"
	"github.com/deploysentry/deploysentry/internal/platform/database"
	"github.com/deploysentry/deploysentry/internal/platform/database/postgres"
	"github.com/deploysentry/deploysentry/internal/platform/messaging"
	"github.com/deploysentry/deploysentry/internal/platform/gelf"
	"github.com/deploysentry/deploysentry/internal/platform/middleware"
	"github.com/deploysentry/deploysentry/internal/platform/metrics"
	"github.com/deploysentry/deploysentry/internal/ratings"
	"github.com/deploysentry/deploysentry/internal/releases"
	"github.com/deploysentry/deploysentry/internal/rollback"
	"github.com/deploysentry/deploysentry/internal/rollout"
	"github.com/deploysentry/deploysentry/internal/rollout/applicator"
	applicatorconfig "github.com/deploysentry/deploysentry/internal/rollout/applicator/config"
	applicatordeploy "github.com/deploysentry/deploysentry/internal/rollout/applicator/deploy"
	rolloutengine "github.com/deploysentry/deploysentry/internal/rollout/engine"
	"github.com/deploysentry/deploysentry/internal/settings"
	"github.com/deploysentry/deploysentry/internal/webhooks"
	"github.com/nats-io/nats.go/jetstream"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("fatal: %v", err)
	}
}

func run() error {
	// -------------------------------------------------------------------------
	// Load Configuration
	// -------------------------------------------------------------------------
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}

	log.Printf("starting deploysentry API server on %s:%d", cfg.Server.Host, cfg.Server.Port)

	// Production configuration validation
	if os.Getenv("DS_ENVIRONMENT") == "production" {
		if err := cfg.ValidateProduction(); err != nil {
			return fmt.Errorf("production configuration validation failed: %w", err)
		}
		log.Println("production configuration validation passed")
	}

	// -------------------------------------------------------------------------
	// Initialize Database
	// -------------------------------------------------------------------------
	ctx := context.Background()

	db, err := database.New(ctx, cfg.Database)
	if err != nil {
		return fmt.Errorf("initializing database: %w", err)
	}
	defer db.Close()
	log.Println("database connection established")

	// -------------------------------------------------------------------------
	// Initialize Redis
	// -------------------------------------------------------------------------
	rdb, err := cache.New(ctx, cfg.Redis)
	if err != nil {
		return fmt.Errorf("initializing redis: %w", err)
	}
	defer func() {
		if err := rdb.Close(); err != nil {
			log.Printf("error closing redis: %v", err)
		}
	}()
	log.Println("redis connection established")

	// -------------------------------------------------------------------------
	// Initialize NATS
	// -------------------------------------------------------------------------
	nc, err := messaging.New(ctx, cfg.NATS)
	if err != nil {
		return fmt.Errorf("initializing nats: %w", err)
	}
	defer func() {
		if err := nc.Close(); err != nil {
			log.Printf("error closing nats: %v", err)
		}
	}()
	log.Println("nats connection established")

	// -------------------------------------------------------------------------
	// Initialize Router
	// -------------------------------------------------------------------------
	if cfg.Log.Level == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()

	// Initialize GELF structured logging
	gelfClient, gelfErr := gelf.NewClient("deploysentry-api")
	if gelfErr != nil {
		log.Printf("warning: GELF logging disabled: %v", gelfErr)
		gelfClient = nil
	}
	defer func() {
		if gelfClient != nil {
			_ = gelfClient.Close()
		}
	}()

	// Core middleware for production readiness
	router.Use(middleware.RequestID())

	// Error handling and logging (replaces gin.Recovery() and gin.Logger())
	errorConfig := middleware.DefaultErrorHandlingConfig()
	if cfg.Log.Level == "debug" {
		errorConfig = middleware.DevelopmentErrorHandlingConfig()
	}
	loggingConfig := middleware.DefaultLoggingConfig()
	loggingConfig.LogLevel = cfg.Log.Level
	router.Use(middleware.ErrorHandler(errorConfig, gelfClient))
	router.Use(middleware.StructuredLogger(loggingConfig, gelfClient))

	// Security and performance middleware
	router.Use(middleware.RequestSizeLimit(middleware.DefaultRequestSizeConfig()))
	router.Use(middleware.SecurityHeaders(middleware.DefaultSecurityConfig()))
	router.Use(metrics.InstrumentHandler())

	// CORS must be at the router level so preflight OPTIONS requests
	// (which don't match any registered route) get handled before Gin's
	// default 404 response.
	router.Use(middleware.CORS(middleware.ProductionCORSConfig([]string{
		"https://www.dr-sentry.com",
		"https://dr-sentry.com",
		"http://localhost:3001",
		"http://localhost:3002", // e2e SDK dashboard instance
		"http://localhost:4310", // e2e React SDK harness (Vite preview)
		"http://localhost:8080",
	})))

	// Health check endpoint.
	router.GET("/health", func(c *gin.Context) {
		healthCtx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
		defer cancel()

		checks := map[string]string{
			"database": "ok",
			"redis":    "ok",
			"nats":     "ok",
		}

		status := http.StatusOK

		if err := db.Health(healthCtx); err != nil {
			checks["database"] = fmt.Sprintf("error: %v", err)
			status = http.StatusServiceUnavailable
		}

		if err := rdb.Health(healthCtx); err != nil {
			checks["redis"] = fmt.Sprintf("error: %v", err)
			status = http.StatusServiceUnavailable
		}

		if err := nc.Health(); err != nil {
			checks["nats"] = fmt.Sprintf("error: %v", err)
			status = http.StatusServiceUnavailable
		}

		c.JSON(status, gin.H{
			"status":  statusText(status),
			"checks":  checks,
			"version": version(),
		})
	})

	// Readiness probe (lightweight, no dependency checks).
	router.GET("/ready", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})

	// CLI install script — served at /install.sh for `curl | sh` installs.
	router.GET("/install.sh", func(c *gin.Context) {
		c.Header("Content-Type", "text/plain; charset=utf-8")
		c.File("scripts/install.sh")
	})

	// Prometheus metrics endpoint (no authentication required for scraping).
	router.GET("/metrics", metrics.Handler())

	// -------------------------------------------------------------------------
	// Repositories
	// -------------------------------------------------------------------------
	userRepo := postgres.NewUserRepository(db.Pool)
	apiKeyRepo := postgres.NewAPIKeyRepository(db.Pool)
	auditRepo := postgres.NewAuditLogRepository(db.Pool)
	flagRepo := postgres.NewFlagRepository(db.Pool)
	deployRepo := postgres.NewDeployRepository(db.Pool)
	releaseRepo := postgres.NewReleaseRepository(db.Pool)
	webhookRepo := postgres.NewWebhookRepository(db.Pool)
	ratingRepo := postgres.NewRatingRepository(db.Pool)
	entityRepo := postgres.NewEntityRepository(db.Pool)
	envRepo := entities.NewEnvironmentRepository(db.Pool)
	settingRepo := postgres.NewSettingRepository(db.Pool)
	memberRepo := postgres.NewMemberRepository(db.Pool)
	groupRepo := postgres.NewGroupRepository(db.Pool)
	grantRepo := postgres.NewGrantRepository(db.Pool)
	agentRepo := postgres.NewAgentRepository(db.Pool)

	// -------------------------------------------------------------------------
	// Services
	// -------------------------------------------------------------------------
	flagCache := flagcache.NewFlagCache(rdb)
	flagService := flags.NewFlagService(flagRepo, flagCache, nc)
	deployService := deploy.NewDeployService(deployRepo, nc)
	releaseService := releases.NewReleaseServiceWithPublisher(releaseRepo, nc)
	apiKeyService := auth.NewAPIKeyService(apiKeyRepo)
	rbacChecker := auth.NewRBACChecker()
	analyticsService := analytics.NewService(db.Pool, rdb.Client)
	webhookService := webhooks.NewService(webhookRepo, nc, []byte(cfg.Security.EncryptionKey))
	ratingService := ratings.NewRatingService(ratingRepo)
	entityService := entities.NewEntityService(entityRepo, envRepo, flagRepo)
	settingService := settings.NewSettingService(settingRepo)
	memberService := members.NewService(memberRepo)
	groupService := groups.NewService(groupRepo)
	grantService := grants.NewService(grantRepo)
	agentService := registry.NewService(agentRepo)

	// -------------------------------------------------------------------------
	// Notifications
	// -------------------------------------------------------------------------
	notificationService := notifications.NewNotificationService()

	if cfg.Notifications.Slack.Enabled {
		slackChannel := notifications.NewSlackChannel(notifications.SlackConfig{
			WebhookURL: cfg.Notifications.Slack.WebhookURL,
			Channel:    cfg.Notifications.Slack.Channel,
			Username:   cfg.Notifications.Slack.Username,
		})
		notificationService.RegisterChannel(slackChannel)
		log.Println("slack notification channel enabled")
	}

	if cfg.Notifications.Email.Enabled {
		emailChannel := notifications.NewEmailChannel(notifications.EmailConfig{
			SMTPHost:    cfg.Notifications.Email.SMTPHost,
			SMTPPort:    cfg.Notifications.Email.SMTPPort,
			Username:    cfg.Notifications.Email.Username,
			Password:    cfg.Notifications.Email.Password,
			FromAddress: cfg.Notifications.Email.FromEmail,
			FromName:    cfg.Notifications.Email.FromName,
			UseHTML:     true,
		})
		notificationService.RegisterChannel(emailChannel)
		log.Println("email notification channel enabled")
	}

	if cfg.Notifications.PagerDuty.Enabled {
		pdChannel := notifications.NewPagerDutyChannel(notifications.PagerDutyConfig{
			RoutingKey: cfg.Notifications.PagerDuty.RoutingKey,
		})
		notificationService.RegisterChannel(pdChannel)
		log.Println("pagerduty notification channel enabled")
	}

	// -------------------------------------------------------------------------
	// Phase Engine (canary rollout)
	// -------------------------------------------------------------------------
	phaseEngine := engine.New(deployRepo, nc, nil, nil)

	// Start event subscriber to bridge NATS events to notifications
	eventSubscriber := notifications.NewEventSubscriber(nc, notificationService)
	go func() {
		if err := eventSubscriber.Start(ctx, notifications.DefaultSubscriberConfig()); err != nil {
			log.Printf("warning: notification subscriber failed to start: %v", err)
		}
	}()

	// Start the phase engine in the background. It subscribes to
	// deployments.deployment.created and drives canary phases.
	engineSubscriber := &natsEngineSubscriber{nats: nc, ctx: ctx}
	go func() {
		if err := phaseEngine.Start(ctx, engineSubscriber); err != nil && err != context.Canceled {
			log.Printf("warning: phase engine stopped: %v", err)
		}
	}()

	prefStore := notifications.NewInMemoryPreferenceStore()

	// -------------------------------------------------------------------------
	// Middleware
	// -------------------------------------------------------------------------
	rateLimitConfig := middleware.DefaultRateLimitConfig()
	// The hermetic e2e stack runs many requests in tight bursts (login,
	// seed, list, toggle) and trips the default 100 req/min limiter,
	// which then causes the React dashboard to drop the user's session.
	// Allow scaling the limit via DS_RATE_LIMIT_PER_MINUTE for tests.
	if v := os.Getenv("DS_RATE_LIMIT_PER_MINUTE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			rateLimitConfig.RequestsPerWindow = n
		}
	}
	rateLimiter := middleware.NewRateLimiter(rdb.Client, rateLimitConfig)
	apiKeyValidator := &apiKeyValidatorAdapter{service: apiKeyService}
	authMiddleware := auth.NewAuthMiddleware(cfg.Auth.JWTSecret, apiKeyValidator, envRepo)

	// -------------------------------------------------------------------------
	// Routes
	// -------------------------------------------------------------------------

	// Authenticated API routes.
	api := router.Group("/api/v1")
	api.Use(rateLimiter.Middleware())
	api.Use(authMiddleware.RequireAuth())
	orgRoleLookup := postgres.NewOrgRoleLookup(db.Pool)
	api.Use(auth.ResolveOrgRole(orgRoleLookup))

	releases.NewHandler(releaseService).RegisterRoutes(api)
	analytics.NewHandler(analyticsService).RegisterRoutes(api)
	webhooks.NewHandler(webhookService).RegisterRoutes(api)
	ratings.NewHandler(ratingService, rbacChecker).RegisterRoutes(api)
	auth.NewUserHandler(userRepo).RegisterRoutes(api)
	auth.NewAPIKeyHandler(apiKeyService).RegisterRoutes(api)
	auth.NewAuditHandler(auditRepo).RegisterRoutes(api)
	entities.NewHandler(entityService, rbacChecker, grantService).RegisterRoutes(api)
	settings.NewHandler(settingService, rbacChecker).RegisterRoutes(api)
	members.NewHandler(memberService, entityService, rbacChecker).RegisterRoutes(api)
	groups.NewHandler(groupService, entityService, rbacChecker).RegisterRoutes(api)
	grants.NewHandler(grantService, entityService, rbacChecker).RegisterRoutes(api)
	notifications.NewPreferencesHandler(prefStore, notificationService, rbacChecker).RegisterRoutes(api)
	registry.NewHandler(agentService).RegisterRoutes(api, rbacChecker)

	// Rollout control plane: strategies, defaults, onboarding policies.
	strategyRepo := postgres.NewStrategyRepo(db.Pool)
	strategyDefRepo := postgres.NewStrategyDefaultsRepo(db.Pool)
	rolloutPolicyRepo := postgres.NewRolloutPolicyRepo(db.Pool)
	rolloutScopeResolver := &rolloutScopeAdapter{entities: entityRepo}
	strategySvc := rollout.NewStrategyService(strategyRepo, nil)
	strategyDefaultSvc := rollout.NewStrategyDefaultService(strategyDefRepo)
	rolloutPolicySvc := rollout.NewRolloutPolicyService(rolloutPolicyRepo)
	rollout.NewHandler(
		strategySvc,
		strategyDefaultSvc,
		rolloutPolicySvc,
		rolloutScopeResolver,
	).RegisterRoutes(api)

	// ---- Plan 2: Rollout execution (engine + service + handler) ----
	rolloutRepo := postgres.NewRolloutRepo(db.Pool)
	rolloutPhaseRepo := postgres.NewRolloutPhaseRepo(db.Pool)
	rolloutEventRepo := postgres.NewRolloutEventRepo(db.Pool)

	trafficSetter := &deployTrafficSetter{svc: deployService}
	deployApp := applicatordeploy.NewApplicator(trafficSetter, &noopHealthReader{})
	configApp := applicatorconfig.NewApplicator(&flagRuleUpdater{svc: flagService})
	routerApp := applicator.NewRouter(deployApp, configApp)

	engineRepos := &rolloutEngineRepoAdapter{
		rollouts: rolloutRepo,
		phases:   rolloutPhaseRepo,
		events:   rolloutEventRepo,
	}

	rolloutExecEngine := rolloutengine.New(engineRepos, routerApp, nc, rolloutengine.EngineOptions{
		PollInterval: 2 * time.Second,
	})

	rolloutExecSvc := rollout.NewRolloutService(rolloutRepo, rolloutPhaseRepo, rolloutEventRepo, nc)
	rollout.NewRolloutHandler(rolloutExecSvc).RegisterRoutes(api)

	rolloutAttacher := rollout.NewAttacher(strategySvc, strategyDefaultSvc, rolloutPolicySvc, rolloutExecSvc)
	deployHandlerAdapter := &deployRolloutAttacherAdapter{attacher: rolloutAttacher, entities: entityRepo}

	// Register the deploy handler with the rollout attacher.
	deploy.NewHandlerWithRollouts(deployService, webhookService, analyticsService, phaseEngine, deployHandlerAdapter).RegisterRoutes(api, rbacChecker)

	// Now that rolloutAttacher is ready, wire the flag handler with rollout support.
	flagRolloutAttacher := &flagRolloutAttacherAdapter{
		attacher: rolloutAttacher,
		flagSvc:  flagService,
		entities: entityRepo,
	}
	flagHandler := flags.NewHandlerWithRollouts(flagService, rbacChecker, webhookService, analyticsService, entityRepo, envRepo, auditRepo, flagRolloutAttacher)
	flagHandler.SetRatingService(ratingService)
	flagHandler.RegisterRoutes(api)
	flagHandler.RegisterSegmentRoutes(api)

	// Wire rollout lookup into the deploy phase engine so it skips canary phases
	// when a rollout engine is already managing traffic for a deployment.
	deployEngineGuard := &deployEngineRolloutGuard{rollouts: rolloutRepo}
	phaseEngine.SetRolloutLookup(deployEngineGuard)

	// Subscribe to rollout.created events and drive the rollout engine.
	go func() {
		if err := (&natsEngineSubscriber{nats: nc, ctx: ctx}).Subscribe(
			"rollouts.rollout.created",
			func(msg []byte) {
				var payload struct {
					RolloutID string `json:"rollout_id"`
				}
				if err := json.Unmarshal(msg, &payload); err != nil {
					log.Printf("rollout engine: bad payload: %v", err)
					return
				}
				id, err := uuid.Parse(payload.RolloutID)
				if err != nil {
					return
				}
				if err := rolloutExecEngine.DriveRollout(ctx, id); err != nil {
					log.Printf("rollout engine: drive error rollout_id=%s: %v", id, err)
				}
			},
		); err != nil && err != context.Canceled {
			log.Printf("warning: rollout engine subscriber failed: %v", err)
		}
	}()

	// Feature lifecycle scheduler: emits flag.scheduled_for_removal.due when
	// scheduled_removal_at passes. Runs in-process on every API instance; the
	// marker column (scheduled_removal_fired_at) keeps firings idempotent.
	lifecycleScheduler := flags.NewLifecycleScheduler(flagService, webhookService, time.Minute)
	go lifecycleScheduler.Run(ctx)

	// Rollback handler: manual rollback triggers and rollback history.
	rollbackExecutor := &deployServiceRollbackExecutor{service: deployService}
	rollbackController := rollback.NewRollbackController(
		rollbackExecutor,
		rollback.NewImmediateRollbackStrategy(),
		0.95,           // healthThreshold
		2*time.Minute,  // evaluationWindow
	)
	rollback.NewHandler(rollbackController).RegisterRoutes(api)

	// Public routes (no auth required).
	public := router.Group("/api/v1")
	auth.NewLoginHandler(userRepo, cfg.Auth).RegisterRoutes(public)

	// GitHub webhook integration (public, verified by signature).
	if cfg.GitHub.WebhookSecret != "" || cfg.GitHub.AutoDeploy {
		ghAppID, _ := uuid.Parse(cfg.GitHub.DefaultProjectID)
		ghEnvID, _ := uuid.Parse(cfg.GitHub.DefaultEnvironmentID)
		ghHandler := githubint.NewHandler(githubint.Config{
			WebhookSecret:        cfg.GitHub.WebhookSecret,
			DefaultApplicationID: ghAppID,
			DefaultEnvironmentID: ghEnvID,
			DefaultStrategy:      cfg.GitHub.DefaultStrategy,
			AutoDeploy:           cfg.GitHub.AutoDeploy,
			DeployBranches:       cfg.GitHub.DeployBranches,
		}, deployService)
		ghHandler.RegisterRoutes(public)
		log.Println("github webhook integration enabled")
	}

	// -------------------------------------------------------------------------
	// Debug: print all registered routes at startup
	// -------------------------------------------------------------------------
	for _, route := range router.Routes() {
		log.Printf("ROUTE: %-6s %s", route.Method, route.Path)
	}

	// GELF startup confirmation
	if gelfClient != nil {
		gelfClient.Info("deploysentry-api started")
	}

	// -------------------------------------------------------------------------
	// Start HTTP Server
	// -------------------------------------------------------------------------
	srv := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  2 * cfg.Server.ReadTimeout,
	}

	// Channel to capture server errors.
	serverErrors := make(chan error, 1)

	go func() {
		log.Printf("API server listening on %s", srv.Addr)
		serverErrors <- srv.ListenAndServe()
	}()

	// -------------------------------------------------------------------------
	// Graceful Shutdown
	// -------------------------------------------------------------------------
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("server error: %w", err)
		}
	case sig := <-quit:
		log.Printf("received signal %v, initiating graceful shutdown", sig)

		shutdownCtx, shutdownCancel := context.WithTimeout(ctx, cfg.Server.ShutdownTimeout)
		defer shutdownCancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			// Force close if graceful shutdown times out.
			_ = srv.Close()
			return fmt.Errorf("graceful shutdown failed: %w", err)
		}
	}

	log.Println("server stopped")
	return nil
}

// statusText returns a human-readable status string for an HTTP status code.
func statusText(code int) string {
	if code == http.StatusOK {
		return "healthy"
	}
	return "unhealthy"
}

// version returns the build version. In production, this would be set via
// ldflags at build time.
func version() string {
	v := os.Getenv("DS_VERSION")
	if v == "" {
		return "dev"
	}
	return v
}

// natsEngineSubscriber adapts *messaging.NATS to the engine.MessageSubscriber
// interface. The engine's Subscribe signature is:
//
//	Subscribe(subject string, handler func(msg []byte)) error
//
// whereas the NATS wrapper exposes a JetStream-based Subscribe that requires a
// stream name, durable consumer name, and a jetstream.Msg handler. This adapter
// bridges the two by ensuring the DEPLOYSENTRY stream exists and creating a
// durable consumer named "engine-<sanitized-subject>".
type natsEngineSubscriber struct {
	nats *messaging.NATS
	ctx  context.Context
}

func (s *natsEngineSubscriber) Subscribe(subject string, handler func(msg []byte)) error {
	// Ensure the stream covers the subject. The stream may already exist from
	// the notification subscriber; CreateOrUpdateStream is idempotent.
	_, err := s.nats.EnsureStream(s.ctx, jetstream.StreamConfig{
		Name:     "DEPLOYSENTRY",
		Subjects: []string{"deployments.>", "flags.>", "releases.>", "health.>"},
	})
	if err != nil {
		return fmt.Errorf("natsEngineSubscriber: ensure stream: %w", err)
	}

	consumerName := "engine-" + sanitizeNATSConsumerName(subject)
	_, err = s.nats.Subscribe(s.ctx, "DEPLOYSENTRY", consumerName, subject, func(msg jetstream.Msg) {
		handler(msg.Data())
		_ = msg.Ack()
	})
	return err
}

// sanitizeNATSConsumerName converts a NATS subject into a valid durable
// consumer name by replacing dots and wildcards with dashes.
func sanitizeNATSConsumerName(subject string) string {
	result := make([]byte, 0, len(subject))
	for i := 0; i < len(subject); i++ {
		switch subject[i] {
		case '.', '>', '*':
			result = append(result, '-')
		default:
			result = append(result, subject[i])
		}
	}
	return string(result)
}

// deployServiceRollbackExecutor adapts *deploy.DeployService to the
// rollback.RollbackExecutor interface. It delegates to RollbackDeployment,
// which handles state transition and event publishing, and ignores the
// strategy parameter since the service manages its own rollback logic.
type deployServiceRollbackExecutor struct {
	service deploy.DeployService
}

func (a *deployServiceRollbackExecutor) Execute(ctx context.Context, deploymentID uuid.UUID, _ rollback.RollbackStrategy) error {
	return a.service.RollbackDeployment(ctx, deploymentID)
}

// rolloutScopeAdapter bridges rollout.ScopeResolver to the existing entity
// repository. It reads path params set by gin.
type rolloutScopeAdapter struct {
	entities entities.EntityRepository
}

func (a *rolloutScopeAdapter) ResolveOrg(c *gin.Context) (uuid.UUID, error) {
	org, err := a.entities.GetOrgBySlug(c.Request.Context(), c.Param("orgSlug"))
	if err != nil {
		return uuid.Nil, err
	}
	return org.ID, nil
}

func (a *rolloutScopeAdapter) ResolveProject(c *gin.Context) (uuid.UUID, uuid.UUID, error) {
	orgID, err := a.ResolveOrg(c)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	proj, err := a.entities.GetProjectBySlug(c.Request.Context(), orgID, c.Param("projectSlug"))
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	return orgID, proj.ID, nil
}

func (a *rolloutScopeAdapter) ResolveApp(c *gin.Context) (uuid.UUID, uuid.UUID, uuid.UUID, error) {
	orgID, projID, err := a.ResolveProject(c)
	if err != nil {
		return uuid.Nil, uuid.Nil, uuid.Nil, err
	}
	app, err := a.entities.GetAppBySlug(c.Request.Context(), projID, c.Param("appSlug"))
	if err != nil {
		return uuid.Nil, uuid.Nil, uuid.Nil, err
	}
	return orgID, projID, app.ID, nil
}

// apiKeyValidatorAdapter adapts *auth.APIKeyService to the auth.APIKeyValidator
// interface expected by AuthMiddleware. It bridges ValidateKey (which returns
// *models.APIKey) to ValidateAPIKey (which returns *auth.APIKeyInfo).
type apiKeyValidatorAdapter struct {
	service *auth.APIKeyService
}

func (a *apiKeyValidatorAdapter) ValidateAPIKey(ctx context.Context, key string) (*auth.APIKeyInfo, error) {
	apiKey, err := a.service.ValidateKey(ctx, key)
	if err != nil {
		return nil, err
	}

	scopes := make([]string, len(apiKey.Scopes))
	for i, s := range apiKey.Scopes {
		scopes[i] = string(s)
	}

	orgID := apiKey.OrgID
	return &auth.APIKeyInfo{
		OrgID:          &orgID,
		ProjectID:      apiKey.ProjectID,
		ApplicationID:  apiKey.ApplicationID,
		EnvironmentIDs: apiKey.EnvironmentIDs,
		Scopes:         scopes,
		AllowedCIDRs:   apiKey.AllowedCIDRs,
	}, nil
}

// ---------------------------------------------------------------------------
// Plan 2 adapter types
// ---------------------------------------------------------------------------

// deployTrafficSetter bridges deploy.DeployService to applicatordeploy.TrafficSetter.
type deployTrafficSetter struct {
	svc deploy.DeployService
}

func (t *deployTrafficSetter) SetTrafficPercent(ctx context.Context, depID uuid.UUID, pct int) error {
	return t.svc.SetTrafficPercent(ctx, depID, pct)
}

// rolloutEngineRepoAdapter composes three postgres repos into rolloutengine.RepoSet.
type rolloutEngineRepoAdapter struct {
	rollouts *postgres.RolloutRepo
	phases   *postgres.RolloutPhaseRepo
	events   *postgres.RolloutEventRepo
}

func (a *rolloutEngineRepoAdapter) GetRollout(ctx context.Context, id uuid.UUID) (*models.Rollout, error) {
	return a.rollouts.Get(ctx, id)
}
func (a *rolloutEngineRepoAdapter) UpdateRolloutStatus(ctx context.Context, id uuid.UUID, st models.RolloutStatus, reason *string) error {
	return a.rollouts.UpdateStatus(ctx, id, st, reason)
}
func (a *rolloutEngineRepoAdapter) UpdateRolloutPhasePointer(ctx context.Context, id uuid.UUID, idx int, startedAt, lastHealthy *time.Time) error {
	return a.rollouts.UpdatePhasePointer(ctx, id, idx, startedAt, lastHealthy)
}
func (a *rolloutEngineRepoAdapter) MarkRolloutCompleted(ctx context.Context, id uuid.UUID) error {
	return a.rollouts.MarkCompleted(ctx, id)
}
func (a *rolloutEngineRepoAdapter) BulkInsertPhases(ctx context.Context, phases []*models.RolloutPhase) error {
	return a.phases.BulkInsert(ctx, phases)
}
func (a *rolloutEngineRepoAdapter) ListPhases(ctx context.Context, rid uuid.UUID) ([]*models.RolloutPhase, error) {
	return a.phases.ListByRollout(ctx, rid)
}
func (a *rolloutEngineRepoAdapter) UpdatePhaseStatus(ctx context.Context, id uuid.UUID, st models.PhaseStatus, ea, xa *time.Time, ap, hs *float64, notes string) error {
	return a.phases.UpdateStatus(ctx, id, st, ea, xa, ap, hs, notes)
}
func (a *rolloutEngineRepoAdapter) InsertEvent(ctx context.Context, e *models.RolloutEvent) error {
	return a.events.Insert(ctx, e)
}

// deployRolloutAttacherAdapter satisfies deploy.RolloutAttacher. It converts
// the deploy-side RolloutAttachRequest into a rollout AttachIntent and calls
// through to the rollout Attacher.
type deployRolloutAttacherAdapter struct {
	attacher *rollout.Attacher
	entities entities.EntityRepository
}

func (a *deployRolloutAttacherAdapter) AttachFromDeployRequest(ctx context.Context, d *models.Deployment, req *deploy.RolloutAttachRequest, actor uuid.UUID) error {
	// The entity repo exposes only GetAppBySlug and GetProjectBySlug (by slug, not ID).
	// The Deployment carries ApplicationID and EnvironmentID. Since we can't look up by
	// ID with the current repo, we pass nil ancestors — the attacher resolves by scope
	// leaf (app) and will fall back to app-scoped or global defaults/policies.
	intent := &rollout.AttachIntent{
		StrategyID:   req.StrategyID,
		StrategyName: req.StrategyName,
		Overrides:    req.Overrides,
		ReleaseID:    req.ReleaseID,
		Leaf:         rollout.ScopeRef{Type: models.ScopeApp, ID: d.ApplicationID},
	}
	return a.attacher.AttachDeploy(ctx, d, intent, actor)
}

// deployEngineRolloutGuard satisfies engine.RolloutLookup (Plan 2 T12).
type deployEngineRolloutGuard struct {
	rollouts *postgres.RolloutRepo
}

func (g *deployEngineRolloutGuard) HasActiveRolloutForDeployment(ctx context.Context, id uuid.UUID) (bool, error) {
	ro, err := g.rollouts.GetActiveByDeployment(ctx, id)
	if err != nil {
		if errors.Is(err, postgres.ErrRolloutNotFound) {
			return false, nil
		}
		return false, err
	}
	return ro != nil, nil
}

// noopHealthReader is used when no HealthMonitor is wired — returns a constant
// healthy signal so the rollout engine advances on time alone (matching legacy
// engine behavior that also runs without health data today).
type noopHealthReader struct{}

func (noopHealthReader) GetHealth(_ uuid.UUID) (*health.DeploymentHealth, error) {
	return &health.DeploymentHealth{Overall: 1.0, Healthy: true}, nil
}

// flagRuleUpdater adapts flags.FlagService to applicatorconfig.RuleUpdater.
// Implements UpdateRulePercentage by reading the rule, mutating its
// Percentage field, and calling UpdateRule.
type flagRuleUpdater struct{ svc flags.FlagService }

func (u *flagRuleUpdater) UpdateRulePercentage(ctx context.Context, ruleID uuid.UUID, pct int) error {
	rule, err := u.svc.GetRule(ctx, ruleID)
	if err != nil {
		return err
	}
	p := pct
	rule.Percentage = &p
	return u.svc.UpdateRule(ctx, rule)
}

// flagRolloutAttacherAdapter satisfies flags.RolloutAttacher by delegating to
// rollout.Attacher.AttachConfig with scope context resolved from the rule's flag.
type flagRolloutAttacherAdapter struct {
	attacher *rollout.Attacher
	flagSvc  flags.FlagService
	entities entities.EntityRepository
}

func (a *flagRolloutAttacherAdapter) AttachFromRuleRequest(ctx context.Context, rule *models.TargetingRule, prev int, req *flags.RolloutAttachRequest, actor uuid.UUID) error {
	flag, err := a.flagSvc.GetFlag(ctx, rule.FlagID)
	if err != nil {
		return err
	}
	// Build the scope leaf from the flag. Ancestor IDs (project, org) are not
	// directly available via ID-based lookups in the current entity repository, so
	// we pass nil — AttachConfig tolerates partial ancestry and falls back to
	// app/project-scoped or global defaults/policies.
	var leaf rollout.ScopeRef
	if flag.ApplicationID != nil {
		leaf = rollout.ScopeRef{Type: models.ScopeApp, ID: *flag.ApplicationID}
	} else {
		leaf = rollout.ScopeRef{Type: models.ScopeProject, ID: flag.ProjectID}
	}

	intent := &rollout.AttachIntent{
		StrategyID:   req.StrategyID,
		StrategyName: req.StrategyName,
		Overrides:    req.Overrides,
		ReleaseID:    req.ReleaseID,
		Leaf:         leaf,
	}
	if err := a.attacher.AttachConfig(ctx, rule.ID, prev, intent, actor); err != nil {
		if errors.Is(err, rollout.ErrAlreadyActiveOnTarget) {
			return flags.ErrRolloutInProgress
		}
		return err
	}
	return nil
}
