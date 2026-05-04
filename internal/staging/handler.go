package staging

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/shadsorg/deploysentry/internal/models"
)

// StagingEnabledSettingKey is the settings key that controls whether the
// staging layer is active for an org.
const StagingEnabledSettingKey = "staged-changes-enabled"

// settingResolver is the minimal interface the Handler needs from the settings
// package to check the staged-changes-enabled org-level setting.
type settingResolver interface {
	Resolve(ctx context.Context, key string, orgID, projectID, appID, envID *uuid.UUID) (*models.Setting, error)
}

// Handler exposes the /deploy-changes HTTP endpoints.
type Handler struct {
	svc      *Service
	settings settingResolver
}

// NewHandler constructs the HTTP handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// NewHandlerWithSettings constructs the HTTP handler with settings support for
// the GET /staging endpoint.
func NewHandlerWithSettings(svc *Service, settings settingResolver) *Handler {
	return &Handler{svc: svc, settings: settings}
}

// RegisterRoutes mounts the staging routes under the org-scoped router group.
// Caller is responsible for installing the standard auth middleware ahead of
// this registration.
//
// Routes:
//   GET    /api/v1/orgs/:orgSlug/staging                      — server-side enablement check
//   GET    /api/v1/orgs/:orgSlug/deploy-changes               — list user's pending
//   POST   /api/v1/orgs/:orgSlug/deploy-changes/stage         — upsert one staged row (Phase A test seam)
//   POST   /api/v1/orgs/:orgSlug/deploy-changes/commit        — deploy selected rows
//   DELETE /api/v1/orgs/:orgSlug/deploy-changes/:id           — discard one
//   DELETE /api/v1/orgs/:orgSlug/deploy-changes               — discard all
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/staging", h.getEnabled)
	rg.GET("/deploy-changes", h.list)
	rg.POST("/deploy-changes/stage", h.stage)
	rg.POST("/deploy-changes/commit", h.commit)
	rg.DELETE("/deploy-changes/:id", h.discardOne)
	rg.DELETE("/deploy-changes", h.discardAll)
}

// getEnabled returns {"enabled": bool} for the org — reads the
// staged-changes-enabled org-level setting. Gracefully defaults to false on
// any error so the UI never breaks if settings are absent.
func (h *Handler) getEnabled(c *gin.Context) {
	if h.settings == nil {
		c.JSON(http.StatusOK, gin.H{"enabled": false})
		return
	}
	orgID, ok := orgIDFromContext(c)
	if !ok {
		c.JSON(http.StatusOK, gin.H{"enabled": false})
		return
	}
	setting, err := h.settings.Resolve(c.Request.Context(), StagingEnabledSettingKey, &orgID, nil, nil, nil)
	if err != nil || setting == nil {
		c.JSON(http.StatusOK, gin.H{"enabled": false})
		return
	}
	// Value is JSONB; expect "true" or "false".
	var enabled bool
	if jsonErr := json.Unmarshal(setting.Value, &enabled); jsonErr != nil {
		c.JSON(http.StatusOK, gin.H{"enabled": false})
		return
	}
	c.JSON(http.StatusOK, gin.H{"enabled": enabled})
}

func (h *Handler) list(c *gin.Context) {
	userID, orgID, ok := identity(c)
	if !ok {
		return
	}
	rows, err := h.svc.ListForUser(c.Request.Context(), userID, orgID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"changes": rows, "count": len(rows)})
}

// stageRequest mirrors the shape of models.StagedChange but only the fields
// callers can set (ID/timestamps are server-managed, user/org are derived
// from the auth context).
type stageRequest struct {
	ResourceType  string          `json:"resource_type" binding:"required"`
	ResourceID    *uuid.UUID      `json:"resource_id,omitempty"`
	ProvisionalID *uuid.UUID      `json:"provisional_id,omitempty"`
	Action        string          `json:"action" binding:"required"`
	FieldPath     string          `json:"field_path,omitempty"`
	OldValue      json.RawMessage `json:"old_value,omitempty"`
	NewValue      json.RawMessage `json:"new_value,omitempty"`
}

func (h *Handler) stage(c *gin.Context) {
	userID, orgID, ok := identity(c)
	if !ok {
		return
	}

	var req stageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.ResourceID == nil && req.ProvisionalID == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "either resource_id or provisional_id is required"})
		return
	}

	row := &models.StagedChange{
		UserID:        userID,
		OrgID:         orgID,
		ResourceType:  req.ResourceType,
		ResourceID:    req.ResourceID,
		ProvisionalID: req.ProvisionalID,
		Action:        req.Action,
		FieldPath:     req.FieldPath,
		OldValue:      req.OldValue,
		NewValue:      req.NewValue,
	}
	if err := h.svc.Stage(c.Request.Context(), row); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, row)
}

type commitRequest struct {
	IDs []uuid.UUID `json:"ids" binding:"required"`
}

func (h *Handler) commit(c *gin.Context) {
	userID, orgID, ok := identity(c)
	if !ok {
		return
	}
	var req commitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	res, err := h.svc.Commit(c.Request.Context(), userID, orgID, userID, req.IDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if res.FailedID != nil {
		c.JSON(http.StatusUnprocessableEntity, res)
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *Handler) discardOne(c *gin.Context) {
	userID, orgID, ok := identity(c)
	if !ok {
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := h.svc.DiscardOne(c.Request.Context(), userID, orgID, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"discarded": id})
}

func (h *Handler) discardAll(c *gin.Context) {
	userID, orgID, ok := identity(c)
	if !ok {
		return
	}
	n, err := h.svc.DiscardAll(c.Request.Context(), userID, orgID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"discarded": n})
}

// orgIDFromContext extracts only the org_id set by the ResolveOrgRole middleware.
func orgIDFromContext(c *gin.Context) (uuid.UUID, bool) {
	orgIDVal, ok := c.Get("org_id")
	if !ok {
		return uuid.Nil, false
	}
	switch v := orgIDVal.(type) {
	case uuid.UUID:
		return v, true
	case string:
		parsed, err := uuid.Parse(v)
		if err != nil {
			return uuid.Nil, false
		}
		return parsed, true
	default:
		return uuid.Nil, false
	}
}

// identity extracts the (user_id, org_id) pair the auth + org-resolver
// middleware put on the Gin context. Both must be present for staging
// operations: staging is per-user and per-org by definition.
func identity(c *gin.Context) (uuid.UUID, uuid.UUID, bool) {
	uidVal, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user context required"})
		return uuid.Nil, uuid.Nil, false
	}
	uid, ok := uidVal.(uuid.UUID)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user identity"})
		return uuid.Nil, uuid.Nil, false
	}
	orgIDVal, ok := c.Get("org_id")
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "organization context required"})
		return uuid.Nil, uuid.Nil, false
	}
	// org_id may be set as either string or uuid.UUID depending on the
	// upstream middleware — handle both.
	var orgID uuid.UUID
	switch v := orgIDVal.(type) {
	case uuid.UUID:
		orgID = v
	case string:
		parsed, err := uuid.Parse(v)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid organization id"})
			return uuid.Nil, uuid.Nil, false
		}
		orgID = parsed
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid organization id"})
		return uuid.Nil, uuid.Nil, false
	}
	return uid, orgID, true
}

// ErrNothingStaged is returned when an operation requires a staged row that
// doesn't exist. Used by future callers; staying exported so tests can
// errors.Is against it.
var ErrNothingStaged = errors.New("staging: no staged change for given id")
