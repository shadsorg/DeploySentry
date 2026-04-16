// internal/groups/handler.go
package groups

import (
	"errors"
	"net/http"

	"github.com/deploysentry/deploysentry/internal/auth"
	"github.com/deploysentry/deploysentry/internal/entities"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Handler provides HTTP endpoints for group management.
type Handler struct {
	service  Service
	entities entities.EntityService
	rbac     *auth.RBACChecker
}

// NewHandler creates a new groups HTTP handler.
func NewHandler(service Service, entityService entities.EntityService, rbac *auth.RBACChecker) *Handler {
	return &Handler{service: service, entities: entityService, rbac: rbac}
}

// RegisterRoutes mounts group management routes.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/orgs/:orgSlug/groups")
	{
		g.GET("", h.listGroups)
		g.POST("", auth.RequirePermission(h.rbac, auth.PermGroupManage), h.createGroup)
		g.GET("/:groupSlug", h.getGroup)
		g.PUT("/:groupSlug", auth.RequirePermission(h.rbac, auth.PermGroupManage), h.updateGroup)
		g.DELETE("/:groupSlug", auth.RequirePermission(h.rbac, auth.PermGroupManage), h.deleteGroup)
		g.GET("/:groupSlug/members", h.listGroupMembers)
		g.POST("/:groupSlug/members", auth.RequirePermission(h.rbac, auth.PermGroupManage), h.addGroupMember)
		g.DELETE("/:groupSlug/members/:userId", auth.RequirePermission(h.rbac, auth.PermGroupManage), h.removeGroupMember)
	}
}

// resolveOrgID looks up the org by slug and returns its ID.
func (h *Handler) resolveOrgID(c *gin.Context) (uuid.UUID, bool) {
	org, err := h.entities.GetOrgBySlug(c.Request.Context(), c.Param("orgSlug"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "organization not found"})
		return uuid.Nil, false
	}
	return org.ID, true
}

// ---------------------------------------------------------------------------
// Request types
// ---------------------------------------------------------------------------

type createGroupRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

type updateGroupRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

type addGroupMemberRequest struct {
	UserID string `json:"user_id" binding:"required"`
}

// ---------------------------------------------------------------------------
// Group handlers
// ---------------------------------------------------------------------------

func (h *Handler) listGroups(c *gin.Context) {
	orgID, ok := h.resolveOrgID(c)
	if !ok {
		return
	}
	rows, err := h.service.ListByOrg(c.Request.Context(), orgID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list groups"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"groups": rows})
}

func (h *Handler) createGroup(c *gin.Context) {
	orgID, ok := h.resolveOrgID(c)
	if !ok {
		return
	}

	var req createGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	uid, _ := c.Get("user_id")
	createdBy, _ := uid.(uuid.UUID)

	group, err := h.service.Create(c.Request.Context(), orgID, req.Name, req.Description, createdBy)
	if err != nil {
		switch {
		case errors.Is(err, ErrSlugTaken):
			c.JSON(http.StatusConflict, gin.H{"error": "group slug already exists in this organization"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create group"})
		}
		return
	}
	c.JSON(http.StatusCreated, gin.H{"group": group})
}

func (h *Handler) getGroup(c *gin.Context) {
	orgID, ok := h.resolveOrgID(c)
	if !ok {
		return
	}

	group, err := h.service.GetBySlug(c.Request.Context(), orgID, c.Param("groupSlug"))
	if err != nil {
		switch {
		case errors.Is(err, ErrGroupNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "group not found"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get group"})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"group": group})
}

func (h *Handler) updateGroup(c *gin.Context) {
	orgID, ok := h.resolveOrgID(c)
	if !ok {
		return
	}

	var req updateGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	group, err := h.service.Update(c.Request.Context(), orgID, c.Param("groupSlug"), req.Name, req.Description)
	if err != nil {
		switch {
		case errors.Is(err, ErrGroupNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "group not found"})
		case errors.Is(err, ErrSlugTaken):
			c.JSON(http.StatusConflict, gin.H{"error": "group slug already exists in this organization"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update group"})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"group": group})
}

func (h *Handler) deleteGroup(c *gin.Context) {
	orgID, ok := h.resolveOrgID(c)
	if !ok {
		return
	}

	if err := h.service.Delete(c.Request.Context(), orgID, c.Param("groupSlug")); err != nil {
		switch {
		case errors.Is(err, ErrGroupNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "group not found"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete group"})
		}
		return
	}
	c.Status(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// Group member handlers
// ---------------------------------------------------------------------------

func (h *Handler) listGroupMembers(c *gin.Context) {
	orgID, ok := h.resolveOrgID(c)
	if !ok {
		return
	}

	rows, err := h.service.ListMembers(c.Request.Context(), orgID, c.Param("groupSlug"))
	if err != nil {
		switch {
		case errors.Is(err, ErrGroupNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "group not found"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list group members"})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"members": rows})
}

func (h *Handler) addGroupMember(c *gin.Context) {
	orgID, ok := h.resolveOrgID(c)
	if !ok {
		return
	}

	var req addGroupMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user_id"})
		return
	}

	if err := h.service.AddMember(c.Request.Context(), orgID, c.Param("groupSlug"), userID); err != nil {
		switch {
		case errors.Is(err, ErrGroupNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "group not found"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to add group member"})
		}
		return
	}
	c.JSON(http.StatusCreated, gin.H{"status": "added"})
}

func (h *Handler) removeGroupMember(c *gin.Context) {
	orgID, ok := h.resolveOrgID(c)
	if !ok {
		return
	}

	userID, err := uuid.Parse(c.Param("userId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})
		return
	}

	if err := h.service.RemoveMember(c.Request.Context(), orgID, c.Param("groupSlug"), userID); err != nil {
		switch {
		case errors.Is(err, ErrGroupNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "group not found"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to remove group member"})
		}
		return
	}
	c.Status(http.StatusNoContent)
}
