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
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/deploysentry/deploysentry/internal/auth"
	"github.com/deploysentry/deploysentry/internal/deploy"
	"github.com/deploysentry/deploysentry/internal/flags"
	"github.com/deploysentry/deploysentry/internal/platform/cache"
	"github.com/deploysentry/deploysentry/internal/platform/cache/flagcache"
	"github.com/deploysentry/deploysentry/internal/platform/config"
	"github.com/deploysentry/deploysentry/internal/platform/database"
	"github.com/deploysentry/deploysentry/internal/platform/database/postgres"
	"github.com/deploysentry/deploysentry/internal/platform/messaging"
	"github.com/deploysentry/deploysentry/internal/platform/middleware"
	"github.com/deploysentry/deploysentry/internal/releases"
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
	router.Use(gin.Recovery())
	router.Use(gin.Logger())

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

	// -------------------------------------------------------------------------
	// Repositories
	// -------------------------------------------------------------------------
	userRepo := postgres.NewUserRepository(db.Pool)
	apiKeyRepo := postgres.NewAPIKeyRepository(db.Pool)
	auditRepo := postgres.NewAuditLogRepository(db.Pool)
	flagRepo := postgres.NewFlagRepository(db.Pool)
	deployRepo := postgres.NewDeployRepository(db.Pool)
	releaseRepo := postgres.NewReleaseRepository(db.Pool)

	// -------------------------------------------------------------------------
	// Services
	// -------------------------------------------------------------------------
	flagCache := flagcache.NewFlagCache(rdb)
	flagService := flags.NewFlagService(flagRepo, flagCache, nc)
	deployService := deploy.NewDeployService(deployRepo, nc)
	releaseService := releases.NewReleaseServiceWithPublisher(releaseRepo, nc)
	apiKeyService := auth.NewAPIKeyService(apiKeyRepo)
	rbacChecker := auth.NewRBACChecker()

	// -------------------------------------------------------------------------
	// Middleware
	// -------------------------------------------------------------------------
	apiKeyValidator := &apiKeyValidatorAdapter{service: apiKeyService}
	authMiddleware := auth.NewAuthMiddleware(cfg.Auth.JWTSecret, apiKeyValidator)
	corsMiddleware := middleware.CORS(middleware.DefaultCORSConfig())
	rateLimiter := middleware.NewRateLimiter(rdb.Client, middleware.DefaultRateLimitConfig())

	// -------------------------------------------------------------------------
	// Routes
	// -------------------------------------------------------------------------

	// Authenticated API routes.
	api := router.Group("/api/v1")
	api.Use(corsMiddleware)
	api.Use(rateLimiter.Middleware())
	api.Use(authMiddleware.RequireAuth())

	flags.NewHandler(flagService, rbacChecker).RegisterRoutes(api)
	deploy.NewHandler(deployService).RegisterRoutes(api, rbacChecker)
	releases.NewHandler(releaseService).RegisterRoutes(api)
	auth.NewUserHandler(userRepo).RegisterRoutes(api)
	auth.NewAPIKeyHandler(apiKeyService).RegisterRoutes(api)
	auth.NewAuditHandler(auditRepo).RegisterRoutes(api)

	// Public routes (no auth required).
	public := router.Group("/api/v1")
	public.Use(corsMiddleware)
	auth.NewLoginHandler(userRepo, cfg.Auth).RegisterRoutes(public)

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

	var projectID uuid.UUID
	if apiKey.ProjectID != nil {
		projectID = *apiKey.ProjectID
	}

	scopes := make([]string, len(apiKey.Scopes))
	for i, s := range apiKey.Scopes {
		scopes[i] = string(s)
	}

	return &auth.APIKeyInfo{
		OrgID:     apiKey.OrgID,
		ProjectID: projectID,
		Scopes:    scopes,
	}, nil
}
