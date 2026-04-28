package orgstate

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/shadsorg/deploysentry/internal/auth"
	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Handler exposes the org-level status + deploy-history HTTP endpoints.
type Handler struct {
	status      *StatusService
	deployments *DeploymentsService
}

func NewHandler(s *StatusService, d *DeploymentsService) *Handler {
	return &Handler{status: s, deployments: d}
}

// RegisterRoutes mounts the org-scoped read endpoints. RBAC: PermDeployRead.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup, rbac *auth.RBACChecker) {
	grp := rg.Group("/orgs/:orgSlug")
	grp.GET("/status", mw(rbac, auth.PermDeployRead), h.getStatus)
	grp.GET("/deployments", mw(rbac, auth.PermDeployRead), h.listDeployments)
}

func mw(rbac *auth.RBACChecker, perm auth.Permission) gin.HandlerFunc {
	if rbac == nil {
		return func(c *gin.Context) { c.Next() }
	}
	return auth.RequirePermission(rbac, perm)
}

func (h *Handler) getStatus(c *gin.Context) {
	orgSlug := c.Param("orgSlug")
	userID := userIDFrom(c)
	orgRole := roleFrom(c)

	resp, err := h.status.Resolve(c.Request.Context(), orgSlug, userID, orgRole)
	if err != nil {
		if errors.Is(err, ErrOrgNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "organization not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) listDeployments(c *gin.Context) {
	orgSlug := c.Param("orgSlug")
	userID := userIDFrom(c)
	orgRole := roleFrom(c)

	p := ListParams{Cursor: c.Query("cursor")}
	if v := c.Query("project_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project_id"})
			return
		}
		p.ProjectID = &id
	}
	if v := c.Query("application_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid application_id"})
			return
		}
		p.ApplicationID = &id
	}
	if v := c.Query("environment_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid environment_id"})
			return
		}
		p.EnvironmentID = &id
	}
	if v := c.Query("status"); v != "" {
		st := models.DeployStatus(v)
		p.Status = &st
	}
	if v := c.Query("mode"); v != "" {
		m := models.DeployMode(v)
		p.Mode = &m
	}
	if v := c.Query("from"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "from must be RFC3339"})
			return
		}
		p.From = &t
	}
	if v := c.Query("to"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "to must be RFC3339"})
			return
		}
		p.To = &t
	}
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			p.Limit = n
		}
	}

	resp, err := h.deployments.List(c.Request.Context(), orgSlug, userID, orgRole, p)
	if err != nil {
		if errors.Is(err, ErrOrgNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "organization not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

func userIDFrom(c *gin.Context) uuid.UUID {
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

func roleFrom(c *gin.Context) string {
	if v, ok := c.Get("role"); ok {
		switch r := v.(type) {
		case string:
			return r
		case auth.Role:
			return string(r)
		}
	}
	return ""
}
