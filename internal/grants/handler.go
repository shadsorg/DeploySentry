package grants

import (
	"errors"
	"net/http"

	"github.com/shadsorg/deploysentry/internal/auth"
	"github.com/shadsorg/deploysentry/internal/entities"
	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Handler provides HTTP endpoints for resource grant management.
type Handler struct {
	service  Service
	entities entities.EntityService
	rbac     *auth.RBACChecker
}

// NewHandler creates a new grants HTTP handler.
func NewHandler(service Service, entityService entities.EntityService, rbac *auth.RBACChecker) *Handler {
	return &Handler{service: service, entities: entityService, rbac: rbac}
}

// RegisterRoutes mounts grant management routes.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	// Project grants
	pg := rg.Group("/orgs/:orgSlug/projects/:projectSlug/grants")
	pg.Use(auth.RequirePermission(h.rbac, auth.PermOrgManage))
	pg.GET("", h.listProjectGrants)
	pg.POST("", h.createProjectGrant)
	pg.DELETE("/:grantId", h.deleteGrant)

	// App grants
	ag := rg.Group("/orgs/:orgSlug/projects/:projectSlug/apps/:appSlug/grants")
	ag.Use(auth.RequirePermission(h.rbac, auth.PermOrgManage))
	ag.GET("", h.listAppGrants)
	ag.POST("", h.createAppGrant)
	ag.DELETE("/:grantId", h.deleteGrant)
}

type createGrantRequest struct {
	UserID     *string `json:"user_id"`
	GroupID    *string `json:"group_id"`
	Permission string  `json:"permission" binding:"required"`
}

// ---------------------------------------------------------------------------
// Resolve helpers
// ---------------------------------------------------------------------------

func (h *Handler) resolveOrg(c *gin.Context) (*models.Organization, bool) {
	org, err := h.entities.GetOrgBySlug(c.Request.Context(), c.Param("orgSlug"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "organization not found"})
		return nil, false
	}
	return org, true
}

func (h *Handler) resolveProject(c *gin.Context, orgID uuid.UUID) (*models.Project, bool) {
	project, err := h.entities.GetProjectBySlug(c.Request.Context(), orgID, c.Param("projectSlug"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
		return nil, false
	}
	return project, true
}

func (h *Handler) resolveApp(c *gin.Context, projectID uuid.UUID) (*models.Application, bool) {
	app, err := h.entities.GetAppBySlug(c.Request.Context(), projectID, c.Param("appSlug"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "application not found"})
		return nil, false
	}
	return app, true
}

// ---------------------------------------------------------------------------
// Project grant handlers
// ---------------------------------------------------------------------------

func (h *Handler) listProjectGrants(c *gin.Context) {
	org, ok := h.resolveOrg(c)
	if !ok {
		return
	}
	project, ok := h.resolveProject(c, org.ID)
	if !ok {
		return
	}
	rows, err := h.service.ListByProject(c.Request.Context(), project.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list grants"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"grants": rows})
}

func (h *Handler) createProjectGrant(c *gin.Context) {
	org, ok := h.resolveOrg(c)
	if !ok {
		return
	}
	project, ok := h.resolveProject(c, org.ID)
	if !ok {
		return
	}
	h.createGrant(c, org.ID, &project.ID, nil)
}

// ---------------------------------------------------------------------------
// App grant handlers
// ---------------------------------------------------------------------------

func (h *Handler) listAppGrants(c *gin.Context) {
	org, ok := h.resolveOrg(c)
	if !ok {
		return
	}
	project, ok := h.resolveProject(c, org.ID)
	if !ok {
		return
	}
	app, ok := h.resolveApp(c, project.ID)
	if !ok {
		return
	}
	rows, err := h.service.ListByApp(c.Request.Context(), app.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list grants"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"grants": rows})
}

func (h *Handler) createAppGrant(c *gin.Context) {
	org, ok := h.resolveOrg(c)
	if !ok {
		return
	}
	project, ok := h.resolveProject(c, org.ID)
	if !ok {
		return
	}
	app, ok := h.resolveApp(c, project.ID)
	if !ok {
		return
	}
	h.createGrant(c, org.ID, nil, &app.ID)
}

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

func (h *Handler) createGrant(c *gin.Context, orgID uuid.UUID, projectID *uuid.UUID, appID *uuid.UUID) {
	var req createGrantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate permission value.
	if req.Permission != "read" && req.Permission != "write" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "permission must be 'read' or 'write'"})
		return
	}

	// Exactly one of user_id or group_id.
	if (req.UserID == nil) == (req.GroupID == nil) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "exactly one of user_id or group_id is required"})
		return
	}

	grant := &models.ResourceGrant{
		OrgID:         orgID,
		ProjectID:     projectID,
		ApplicationID: appID,
		Permission:    models.ResourcePermission(req.Permission),
	}

	if req.UserID != nil {
		uid, err := uuid.Parse(*req.UserID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user_id"})
			return
		}
		grant.UserID = &uid
	}
	if req.GroupID != nil {
		gid, err := uuid.Parse(*req.GroupID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid group_id"})
			return
		}
		grant.GroupID = &gid
	}

	// Extract granted_by from context.
	if uid, exists := c.Get("user_id"); exists {
		if userID, ok := uid.(uuid.UUID); ok {
			grant.GrantedBy = &userID
		}
	}

	created, err := h.service.Create(c.Request.Context(), grant)
	if err != nil {
		switch {
		case errors.Is(err, ErrDuplicate):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		return
	}
	c.JSON(http.StatusCreated, gin.H{"grant": created})
}

func (h *Handler) deleteGrant(c *gin.Context) {
	grantID, err := uuid.Parse(c.Param("grantId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid grant ID"})
		return
	}
	if err := h.service.Delete(c.Request.Context(), grantID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete grant"})
		return
	}
	c.Status(http.StatusNoContent)
}
