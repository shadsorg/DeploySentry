package appstatus

import (
	"net/http"

	"github.com/deploysentry/deploysentry/internal/auth"
	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Handler exposes the POST /applications/:app_id/status endpoint.
type Handler struct {
	svc *Service
}

// NewHandler constructs an application-status handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes mounts the handler. RBAC is applied with PermStatusWrite.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup, rbac *auth.RBACChecker) {
	grp := rg.Group("/applications")
	grp.POST("/:app_id/status", mw(rbac, auth.PermStatusWrite), h.reportStatus)
}

func mw(rbac *auth.RBACChecker, perm auth.Permission) gin.HandlerFunc {
	if rbac == nil {
		return func(c *gin.Context) { c.Next() }
	}
	return auth.RequirePermission(rbac, perm)
}

func (h *Handler) reportStatus(c *gin.Context) {
	appID, err := uuid.Parse(c.Param("app_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid app_id"})
		return
	}

	var payload models.ReportStatusPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	envID, rerr := resolveEnvironmentID(c, appID)
	if rerr != nil {
		c.JSON(rerr.status, gin.H{"error": rerr.msg})
		return
	}

	createdBy := actorFromContext(c)
	status, sErr := h.svc.Report(c.Request.Context(), ReportInput{
		ApplicationID: appID,
		EnvironmentID: envID,
		Payload:       payload,
		Source:        "app-push",
		CreatedBy:     createdBy,
	})
	if sErr != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": sErr.Error()})
		return
	}

	c.JSON(http.StatusCreated, status)
}

// resolveError pairs an HTTP status with a client-facing message.
type resolveError struct {
	status int
	msg    string
}

// resolveEnvironmentID determines which environment a status report targets.
//
// API-key auth:  the key must be scoped to a single environment and to the
// requested application.
//
// Session/JWT auth:  the environment must be supplied in the body as
// ?environment_id= (query) or `environment_id` JSON field. Primarily a
// testing convenience; production flows use scoped API keys.
func resolveEnvironmentID(c *gin.Context, appID uuid.UUID) (uuid.UUID, *resolveError) {
	if method, _ := c.Get("auth_method"); method == "api_key" {
		if keyAppIDVal, ok := c.Get("api_key_app_id"); ok {
			if keyAppIDStr, ok := keyAppIDVal.(string); ok && keyAppIDStr != "" {
				keyAppID, err := uuid.Parse(keyAppIDStr)
				if err != nil || keyAppID != appID {
					return uuid.Nil, &resolveError{http.StatusForbidden, "api key is not scoped to this application"}
				}
			}
		}
		envsVal, ok := c.Get("api_key_environment_ids")
		if !ok {
			return uuid.Nil, &resolveError{http.StatusBadRequest, "status:write api key must be scoped to a single environment"}
		}
		envs, ok := envsVal.([]string)
		if !ok || len(envs) == 0 {
			return uuid.Nil, &resolveError{http.StatusBadRequest, "status:write api key must be scoped to a single environment"}
		}
		if len(envs) != 1 {
			return uuid.Nil, &resolveError{http.StatusBadRequest, "status:write api key must be scoped to exactly one environment"}
		}
		envID, err := uuid.Parse(envs[0])
		if err != nil {
			return uuid.Nil, &resolveError{http.StatusInternalServerError, "invalid environment id on api key"}
		}
		return envID, nil
	}

	// Non-api-key path: accept environment_id from the query string.
	if envStr := c.Query("environment_id"); envStr != "" {
		envID, err := uuid.Parse(envStr)
		if err != nil {
			return uuid.Nil, &resolveError{http.StatusBadRequest, "invalid environment_id"}
		}
		return envID, nil
	}
	return uuid.Nil, &resolveError{http.StatusBadRequest, "environment_id is required"}
}

func actorFromContext(c *gin.Context) uuid.UUID {
	if v, ok := c.Get("user_id"); ok {
		if id, ok := v.(uuid.UUID); ok {
			return id
		}
		if s, ok := v.(string); ok {
			if id, err := uuid.Parse(s); err == nil {
				return id
			}
		}
	}
	return uuid.Nil
}
