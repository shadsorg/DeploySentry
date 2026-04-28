package auth

import (
	"net/http"
	"strconv"
	"time"

	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// APIKeyHandler provides HTTP endpoints for managing API keys.
type APIKeyHandler struct {
	service *APIKeyService
}

// NewAPIKeyHandler creates a new APIKeyHandler.
func NewAPIKeyHandler(service *APIKeyService) *APIKeyHandler {
	return &APIKeyHandler{service: service}
}

// RegisterRoutes mounts all API key management routes on the given router group.
func (h *APIKeyHandler) RegisterRoutes(rg *gin.RouterGroup) {
	apiKeys := rg.Group("/api-keys")
	{
		apiKeys.POST("", h.createAPIKey)
		apiKeys.GET("", h.listAPIKeys)
		apiKeys.GET("/:id", h.getAPIKey)
		apiKeys.DELETE("/:id", h.revokeAPIKey)
		apiKeys.POST("/:id/rotate", h.rotateAPIKey)
	}
}

// createAPIKeyRequest is the JSON body for creating a new API key.
type createAPIKeyRequest struct {
	Name           string               `json:"name" binding:"required"`
	ProjectID      *uuid.UUID           `json:"project_id"`
	ApplicationID  *uuid.UUID           `json:"application_id"`
	EnvironmentIDs []uuid.UUID          `json:"environment_ids"`
	Scopes         []models.APIKeyScope `json:"scopes" binding:"required"`
	AllowedCIDRs   []string             `json:"allowed_cidrs"`
	ExpiresAt      *time.Time           `json:"expires_at"`
}

func (h *APIKeyHandler) createAPIKey(c *gin.Context) {
	var req createAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	createdBy, ok := resolveAPIKeyManageActor(c)
	if !ok {
		return
	}

	orgIDValue, ok := c.Get("org_id")
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "organization context required"})
		return
	}

	orgIDStr, ok := orgIDValue.(string)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid organization identity"})
		return
	}

	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid organization id"})
		return
	}

	if req.EnvironmentIDs == nil {
		req.EnvironmentIDs = []uuid.UUID{}
	}

	result, err := h.service.GenerateKey(
		c.Request.Context(),
		orgID,
		req.ProjectID,
		req.ApplicationID,
		req.Name,
		req.Scopes,
		createdBy,
		req.EnvironmentIDs,
		req.AllowedCIDRs,
		req.ExpiresAt,
	)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"api_key":       result.APIKey,
		"plaintext_key": result.PlaintextKey,
	})
}

func (h *APIKeyHandler) listAPIKeys(c *gin.Context) {
	orgIDValue, ok := c.Get("org_id")
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "organization context required"})
		return
	}

	orgIDStr, ok := orgIDValue.(string)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid organization identity"})
		return
	}

	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid organization id"})
		return
	}

	var projectID *uuid.UUID
	if pidStr := c.Query("project_id"); pidStr != "" {
		pid, err := uuid.Parse(pidStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project_id"})
			return
		}
		projectID = &pid
	}

	var environmentID *uuid.UUID
	if eidStr := c.Query("environment_id"); eidStr != "" {
		eid, err := uuid.Parse(eidStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid environment_id"})
			return
		}
		environmentID = &eid
	}

	limit := 20
	offset := 0
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if o := c.Query("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	keys, err := h.service.ListKeys(c.Request.Context(), orgID, projectID, environmentID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list api keys"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"api_keys": keys})
}

func (h *APIKeyHandler) getAPIKey(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid api key id"})
		return
	}

	key, err := h.service.GetKey(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "api key not found"})
		return
	}

	c.JSON(http.StatusOK, key)
}

func (h *APIKeyHandler) revokeAPIKey(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid api key id"})
		return
	}

	if err := h.service.RevokeKey(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "revoked"})
}

func (h *APIKeyHandler) rotateAPIKey(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid api key id"})
		return
	}

	createdBy, ok := resolveAPIKeyManageActor(c)
	if !ok {
		return
	}

	result, err := h.service.RotateKey(c.Request.Context(), id, createdBy)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"api_key":       result.APIKey,
		"plaintext_key": result.PlaintextKey,
	})
}

// resolveAPIKeyManageActor confirms the caller is allowed to mutate API
// keys in the current org and returns the user UUID to stamp on new rows.
//
// Two valid shapes:
//   - Session/JWT auth: caller has PermAPIKeyManage on their RBAC role.
//     created_by = user_id from the token.
//   - API-key auth: the calling key carries the `apikey:manage` scope.
//     created_by = uuid.Nil (there is no human user). We still require an
//     org-scoped key so the new row inherits the right org context.
//
// On failure writes an actionable JSON body and returns ok=false so the
// caller just returns. 403 with a specific "requires apikey:manage" body
// instead of the legacy 401 "authentication required".
func resolveAPIKeyManageActor(c *gin.Context) (uuid.UUID, bool) {
	method, _ := c.Get("auth_method")
	switch method {
	case "api_key":
		scopesVal, _ := c.Get("api_key_scopes")
		scopes, _ := scopesVal.([]string)
		if !hasScope(scopes, "apikey:manage") && !hasScope(scopes, "admin") {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":          "insufficient scope",
				"required_scope": "apikey:manage",
				"scopes_on_key":  scopes,
			})
			return uuid.Nil, false
		}
		// Stamp the ancestor user on the new row so the api_keys.created_by
		// FK into users(id) stays valid. If the calling key itself is
		// orphaned (no created_by), tell the caller plainly — dropping
		// the FK is a separate data-model decision.
		if cbRaw, ok := c.Get("api_key_created_by"); ok {
			if cbStr, ok := cbRaw.(string); ok && cbStr != "" {
				if cb, err := uuid.Parse(cbStr); err == nil {
					return cb, true
				}
			}
		}
		c.AbortWithStatusJSON(http.StatusUnprocessableEntity, gin.H{
			"error": "the calling api key has no associated user on record (created_by is null); " +
				"mint a key from the dashboard (which stamps the creating user) and retry",
		})
		return uuid.Nil, false

	case "jwt", "":
		// Session auth must also hold the RBAC permission.
		roleVal, _ := c.Get("role")
		var role Role
		switch r := roleVal.(type) {
		case Role:
			role = r
		case string:
			role = Role(r)
		}
		rbac := NewRBACChecker()
		if !rbac.HasPermission(role, PermAPIKeyManage) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":               "insufficient permissions",
				"required_permission": string(PermAPIKeyManage),
				"role":                string(role),
			})
			return uuid.Nil, false
		}
		userIDVal, ok := c.Get("user_id")
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return uuid.Nil, false
		}
		userID, ok := userIDVal.(uuid.UUID)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid user identity"})
			return uuid.Nil, false
		}
		return userID, true
	}

	c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
	return uuid.Nil, false
}

func hasScope(scopes []string, want string) bool {
	for _, s := range scopes {
		if s == want {
			return true
		}
	}
	return false
}
