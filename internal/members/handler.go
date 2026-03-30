// internal/members/handler.go
package members

import (
	"errors"
	"net/http"

	"github.com/deploysentry/deploysentry/internal/auth"
	"github.com/deploysentry/deploysentry/internal/entities"
	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Handler provides HTTP endpoints for member management.
type Handler struct {
	service  Service
	entities entities.EntityService
	rbac     *auth.RBACChecker
}

// NewHandler creates a new members HTTP handler.
func NewHandler(service Service, entityService entities.EntityService, rbac *auth.RBACChecker) *Handler {
	return &Handler{service: service, entities: entityService, rbac: rbac}
}

// RegisterRoutes mounts member management routes.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	// Org members: /orgs/:orgSlug/members
	orgMembers := rg.Group("/orgs/:orgSlug/members")
	{
		orgMembers.GET("", auth.RequirePermission(h.rbac, auth.PermOrgManage), h.listOrgMembers)
		orgMembers.POST("", auth.RequirePermission(h.rbac, auth.PermOrgManage), h.addOrgMember)
		orgMembers.PUT("/:userId", auth.RequirePermission(h.rbac, auth.PermOrgManage), h.updateOrgMemberRole)
		orgMembers.DELETE("/:userId", auth.RequirePermission(h.rbac, auth.PermOrgManage), h.removeOrgMember)
	}

	// Project members: /orgs/:orgSlug/projects/:projectSlug/members
	projectMembers := rg.Group("/orgs/:orgSlug/projects/:projectSlug/members")
	{
		projectMembers.GET("", auth.RequirePermission(h.rbac, auth.PermProjectManage), h.listProjectMembers)
		projectMembers.POST("", auth.RequirePermission(h.rbac, auth.PermProjectManage), h.addProjectMember)
		projectMembers.PUT("/:userId", auth.RequirePermission(h.rbac, auth.PermProjectManage), h.updateProjectMemberRole)
		projectMembers.DELETE("/:userId", auth.RequirePermission(h.rbac, auth.PermProjectManage), h.removeProjectMember)
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

// resolveProjectID looks up the project by org slug + project slug.
func (h *Handler) resolveProjectID(c *gin.Context) (uuid.UUID, bool) {
	org, err := h.entities.GetOrgBySlug(c.Request.Context(), c.Param("orgSlug"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "organization not found"})
		return uuid.Nil, false
	}
	project, err := h.entities.GetProjectBySlug(c.Request.Context(), org.ID, c.Param("projectSlug"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
		return uuid.Nil, false
	}
	return project.ID, true
}

// ---------------------------------------------------------------------------
// Org member handlers
// ---------------------------------------------------------------------------

func (h *Handler) listOrgMembers(c *gin.Context) {
	orgID, ok := h.resolveOrgID(c)
	if !ok {
		return
	}
	rows, err := h.service.ListOrgMembers(c.Request.Context(), orgID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list members"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"members": rows})
}

type addMemberRequest struct {
	Email string `json:"email" binding:"required"`
	Role  string `json:"role" binding:"required"`
}

func (h *Handler) addOrgMember(c *gin.Context) {
	orgID, ok := h.resolveOrgID(c)
	if !ok {
		return
	}

	var req addMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	uid, _ := c.Get("user_id")
	addedBy, _ := uid.(uuid.UUID)

	row, err := h.service.AddOrgMember(c.Request.Context(), orgID, req.Email, models.OrgRole(req.Role), addedBy)
	if err != nil {
		switch {
		case errors.Is(err, ErrUserNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found with that email"})
		case errors.Is(err, ErrAlreadyMember):
			c.JSON(http.StatusConflict, gin.H{"error": "user is already a member"})
		case errors.Is(err, ErrInvalidRole):
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid role"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to add member"})
		}
		return
	}
	c.JSON(http.StatusCreated, gin.H{"member": row})
}

type updateRoleRequest struct {
	Role string `json:"role" binding:"required"`
}

func (h *Handler) updateOrgMemberRole(c *gin.Context) {
	orgID, ok := h.resolveOrgID(c)
	if !ok {
		return
	}

	userID, err := uuid.Parse(c.Param("userId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})
		return
	}

	var req updateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.UpdateOrgMemberRole(c.Request.Context(), orgID, userID, models.OrgRole(req.Role)); err != nil {
		switch {
		case errors.Is(err, ErrInvalidRole):
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid role"})
		case errors.Is(err, ErrOwnerAssign):
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		case errors.Is(err, ErrLastOwner):
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		case errors.Is(err, ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "member not found"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update role"})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

func (h *Handler) removeOrgMember(c *gin.Context) {
	orgID, ok := h.resolveOrgID(c)
	if !ok {
		return
	}

	userID, err := uuid.Parse(c.Param("userId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})
		return
	}

	if err := h.service.RemoveOrgMember(c.Request.Context(), orgID, userID); err != nil {
		switch {
		case errors.Is(err, ErrLastOwner):
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		case errors.Is(err, ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "member not found"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to remove member"})
		}
		return
	}
	c.Status(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// Project member handlers
// ---------------------------------------------------------------------------

func (h *Handler) listProjectMembers(c *gin.Context) {
	projectID, ok := h.resolveProjectID(c)
	if !ok {
		return
	}
	rows, err := h.service.ListProjectMembers(c.Request.Context(), projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list members"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"members": rows})
}

func (h *Handler) addProjectMember(c *gin.Context) {
	projectID, ok := h.resolveProjectID(c)
	if !ok {
		return
	}

	var req addMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	uid, _ := c.Get("user_id")
	addedBy, _ := uid.(uuid.UUID)

	row, err := h.service.AddProjectMember(c.Request.Context(), projectID, req.Email, models.ProjectRole(req.Role), addedBy)
	if err != nil {
		switch {
		case errors.Is(err, ErrUserNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found with that email"})
		case errors.Is(err, ErrAlreadyMember):
			c.JSON(http.StatusConflict, gin.H{"error": "user is already a member"})
		case errors.Is(err, ErrInvalidRole):
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid role"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to add member"})
		}
		return
	}
	c.JSON(http.StatusCreated, gin.H{"member": row})
}

func (h *Handler) updateProjectMemberRole(c *gin.Context) {
	projectID, ok := h.resolveProjectID(c)
	if !ok {
		return
	}

	userID, err := uuid.Parse(c.Param("userId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})
		return
	}

	var req updateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.UpdateProjectMemberRole(c.Request.Context(), projectID, userID, models.ProjectRole(req.Role)); err != nil {
		switch {
		case errors.Is(err, ErrInvalidRole):
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid role"})
		case errors.Is(err, ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "member not found"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update role"})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

func (h *Handler) removeProjectMember(c *gin.Context) {
	projectID, ok := h.resolveProjectID(c)
	if !ok {
		return
	}

	userID, err := uuid.Parse(c.Param("userId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})
		return
	}

	if err := h.service.RemoveProjectMember(c.Request.Context(), projectID, userID); err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "member not found"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to remove member"})
		}
		return
	}
	c.Status(http.StatusNoContent)
}
