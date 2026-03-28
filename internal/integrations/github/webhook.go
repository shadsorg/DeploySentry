// Package github provides a GitHub webhook receiver that auto-creates
// DeploySentry deployments from GitHub push and release events.
package github

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/deploysentry/deploysentry/internal/deploy"
	"github.com/deploysentry/deploysentry/internal/models"
)

// Config holds GitHub webhook integration configuration.
type Config struct {
	// WebhookSecret is used to verify webhook signatures from GitHub.
	WebhookSecret string

	// DefaultProjectID is the project to create deployments under when not specified.
	DefaultProjectID uuid.UUID

	// DefaultEnvironmentID is the environment to deploy to when not specified.
	DefaultEnvironmentID uuid.UUID

	// DefaultStrategy is the deployment strategy to use (canary, blue-green, rolling).
	DefaultStrategy string

	// AutoDeploy enables automatic deployment creation on push to main.
	AutoDeploy bool

	// DeployBranches is the list of branches that trigger auto-deployments.
	DeployBranches []string
}

// Handler provides HTTP endpoints for receiving GitHub webhooks.
type Handler struct {
	config  Config
	service deploy.DeployService
}

// NewHandler creates a new GitHub webhook handler.
func NewHandler(config Config, service deploy.DeployService) *Handler {
	if config.DefaultStrategy == "" {
		config.DefaultStrategy = "rolling"
	}
	if len(config.DeployBranches) == 0 {
		config.DeployBranches = []string{"main", "master"}
	}
	return &Handler{config: config, service: service}
}

// RegisterRoutes mounts GitHub webhook routes on the given router group.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/integrations/github/webhook", h.handleWebhook)
}

func (h *Handler) handleWebhook(c *gin.Context) {
	// Read body
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
		return
	}

	// Verify signature if secret is configured
	if h.config.WebhookSecret != "" {
		signature := c.GetHeader("X-Hub-Signature-256")
		if !verifySignature(body, signature, h.config.WebhookSecret) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid webhook signature"})
			return
		}
	}

	// Route by event type
	eventType := c.GetHeader("X-GitHub-Event")
	switch eventType {
	case "push":
		h.handlePush(c, body)
	case "release":
		h.handleRelease(c, body)
	case "ping":
		c.JSON(http.StatusOK, gin.H{"status": "pong"})
	default:
		c.JSON(http.StatusOK, gin.H{"status": "ignored", "event": eventType})
	}
}

// pushEvent is a subset of the GitHub push webhook payload.
type pushEvent struct {
	Ref        string `json:"ref"`
	After      string `json:"after"`
	Repository struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
	Pusher struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	} `json:"pusher"`
	HeadCommit struct {
		Message string `json:"message"`
	} `json:"head_commit"`
}

func (h *Handler) handlePush(c *gin.Context, body []byte) {
	var event pushEvent
	if err := json.Unmarshal(body, &event); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid push payload"})
		return
	}

	// Extract branch name from ref (refs/heads/main -> main)
	branch := strings.TrimPrefix(event.Ref, "refs/heads/")

	// Check if this branch triggers auto-deployment
	if !h.config.AutoDeploy || !h.isDeployBranch(branch) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "skipped",
			"reason":  "branch not configured for auto-deploy",
			"branch":  branch,
		})
		return
	}

	// Create deployment
	shortSHA := event.After
	if len(shortSHA) > 7 {
		shortSHA = shortSHA[:7]
	}

	d := &models.Deployment{
		ID:            uuid.New(),
		ProjectID:     h.config.DefaultProjectID,
		EnvironmentID: h.config.DefaultEnvironmentID,
		Version:       shortSHA,
		Strategy:      models.DeployStrategyType(h.config.DefaultStrategy),
		Status:        models.DeployStatusPending,
		CreatedAt:     time.Now(),
	}

	if err := h.service.CreateDeployment(c.Request.Context(), d); err != nil {
		log.Printf("[github-webhook] failed to create deployment: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create deployment"})
		return
	}

	log.Printf("[github-webhook] auto-deployment created: %s (branch: %s, commit: %s)", d.ID, branch, shortSHA)

	c.JSON(http.StatusCreated, gin.H{
		"status":        "deployment_created",
		"deployment_id": d.ID,
		"version":       shortSHA,
		"branch":        branch,
		"repository":    event.Repository.FullName,
	})
}

// releaseEvent is a subset of the GitHub release webhook payload.
type releaseEvent struct {
	Action  string `json:"action"`
	Release struct {
		TagName    string `json:"tag_name"`
		Name       string `json:"name"`
		Prerelease bool   `json:"prerelease"`
		Draft      bool   `json:"draft"`
	} `json:"release"`
	Repository struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
}

func (h *Handler) handleRelease(c *gin.Context, body []byte) {
	var event releaseEvent
	if err := json.Unmarshal(body, &event); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid release payload"})
		return
	}

	// Only process published releases
	if event.Action != "published" || event.Release.Draft {
		c.JSON(http.StatusOK, gin.H{
			"status": "skipped",
			"reason": fmt.Sprintf("release action '%s' or draft release", event.Action),
		})
		return
	}

	// Skip pre-releases unless auto-deploy is enabled
	if event.Release.Prerelease && !h.config.AutoDeploy {
		c.JSON(http.StatusOK, gin.H{
			"status": "skipped",
			"reason": "pre-release ignored without auto-deploy",
		})
		return
	}

	// Create deployment from release tag
	d := &models.Deployment{
		ID:            uuid.New(),
		ProjectID:     h.config.DefaultProjectID,
		EnvironmentID: h.config.DefaultEnvironmentID,
		Version:       event.Release.TagName,
		Strategy:      models.DeployStrategyType(h.config.DefaultStrategy),
		Status:        models.DeployStatusPending,
		CreatedAt:     time.Now(),
	}

	ctx := context.Background()
	if err := h.service.CreateDeployment(ctx, d); err != nil {
		log.Printf("[github-webhook] failed to create deployment from release: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create deployment"})
		return
	}

	log.Printf("[github-webhook] release deployment created: %s (tag: %s)", d.ID, event.Release.TagName)

	c.JSON(http.StatusCreated, gin.H{
		"status":        "deployment_created",
		"deployment_id": d.ID,
		"version":       event.Release.TagName,
		"release":       event.Release.Name,
		"repository":    event.Repository.FullName,
	})
}

func (h *Handler) isDeployBranch(branch string) bool {
	for _, b := range h.config.DeployBranches {
		if b == branch {
			return true
		}
	}
	return false
}

// verifySignature checks the GitHub webhook HMAC-SHA256 signature.
func verifySignature(payload []byte, signature string, secret string) bool {
	if !strings.HasPrefix(signature, "sha256=") {
		return false
	}

	sig, err := hex.DecodeString(strings.TrimPrefix(signature, "sha256="))
	if err != nil {
		return false
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	expected := mac.Sum(nil)

	return hmac.Equal(sig, expected)
}