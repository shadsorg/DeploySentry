package currentstate

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/shadsorg/deploysentry/internal/auth"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Handler exposes the read-side current-state endpoint.
type Handler struct {
	svc *Service
}

// NewHandler constructs a Handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes mounts the route with RBAC = PermDeployRead.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup, rbac *auth.RBACChecker) {
	grp := rg.Group("/applications")
	grp.GET("/:app_id/environments/:env_id/current-state", mw(rbac, auth.PermDeployRead), h.getCurrentState)
}

func mw(rbac *auth.RBACChecker, perm auth.Permission) gin.HandlerFunc {
	if rbac == nil {
		return func(c *gin.Context) { c.Next() }
	}
	return auth.RequirePermission(rbac, perm)
}

func (h *Handler) getCurrentState(c *gin.Context) {
	appID, err := uuid.Parse(c.Param("app_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid app_id"})
		return
	}
	envID, err := uuid.Parse(c.Param("env_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid env_id"})
		return
	}

	limit := 0
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}

	resp, err := h.svc.Resolve(c.Request.Context(), appID, envID, limit)
	if err != nil {
		if errors.Is(err, ErrEnvNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "environment not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}
