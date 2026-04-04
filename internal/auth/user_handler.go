package auth

import (
	"context"
	"net/http"
	"strconv"

	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// UserRepository defines the persistence interface for user operations.
type UserRepository interface {
	// CreateUser persists a new user record.
	CreateUser(ctx context.Context, user *models.User) error

	// GetUser retrieves a user by their ID.
	GetUser(ctx context.Context, id uuid.UUID) (*models.User, error)

	// GetUserByEmail retrieves a user by their email address.
	GetUserByEmail(ctx context.Context, email string) (*models.User, error)

	// UpdateUser persists changes to an existing user.
	UpdateUser(ctx context.Context, user *models.User) error

	// DeleteUser soft-deletes a user.
	DeleteUser(ctx context.Context, id uuid.UUID) error

	// ListOrgMembers returns members of an organization.
	ListOrgMembers(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*models.OrgMember, error)

	// GetOrgMember retrieves an organization membership record.
	GetOrgMember(ctx context.Context, orgID, userID uuid.UUID) (*models.OrgMember, error)

	// CreateOrgMember adds a user to an organization.
	CreateOrgMember(ctx context.Context, member *models.OrgMember) error

	// UpdateOrgMember updates a user's role within an organization.
	UpdateOrgMember(ctx context.Context, member *models.OrgMember) error

	// DeleteOrgMember removes a user from an organization.
	DeleteOrgMember(ctx context.Context, orgID, userID uuid.UUID) error

	// ListProjectMembers returns members of a project.
	ListProjectMembers(ctx context.Context, projectID uuid.UUID, limit, offset int) ([]*models.ProjectMember, error)

	// GetProjectMember retrieves a project membership record.
	GetProjectMember(ctx context.Context, projectID, userID uuid.UUID) (*models.ProjectMember, error)

	// CreateProjectMember adds a user to a project.
	CreateProjectMember(ctx context.Context, member *models.ProjectMember) error

	// UpdateProjectMember updates a user's role within a project.
	UpdateProjectMember(ctx context.Context, member *models.ProjectMember) error

	// DeleteProjectMember removes a user from a project.
	DeleteProjectMember(ctx context.Context, projectID, userID uuid.UUID) error
}

// UserHandler provides HTTP endpoints for managing users and memberships.
type UserHandler struct {
	repo UserRepository
}

// NewUserHandler creates a new UserHandler.
func NewUserHandler(repo UserRepository) *UserHandler {
	return &UserHandler{repo: repo}
}

// RegisterRoutes mounts all user management routes on the given router group.
func (h *UserHandler) RegisterRoutes(rg *gin.RouterGroup) {
	users := rg.Group("/users")
	{
		users.GET("/me", h.getCurrentUser)
		users.PUT("/me", h.updateProfile)
		users.GET("/:id", h.getUser)
		users.DELETE("/:id", h.deleteUser)
	}

	// Member routes moved to internal/members/handler.go
}

// ---------------------------------------------------------------------------
// User profile endpoints
// ---------------------------------------------------------------------------

func (h *UserHandler) getCurrentUser(c *gin.Context) {
	userIDValue, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	userID, ok := userIDValue.(uuid.UUID)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user identity"})
		return
	}

	user, err := h.repo.GetUser(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	c.JSON(http.StatusOK, user)
}

func (h *UserHandler) getUser(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	user, err := h.repo.GetUser(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	c.JSON(http.StatusOK, user)
}

// updateProfileRequest is the JSON body for updating a user's profile.
type updateProfileRequest struct {
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
}

func (h *UserHandler) updateProfile(c *gin.Context) {
	userIDValue, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	userID, ok := userIDValue.(uuid.UUID)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user identity"})
		return
	}

	user, err := h.repo.GetUser(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	var req updateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Name != "" {
		user.Name = req.Name
	}
	if req.AvatarURL != "" {
		user.AvatarURL = req.AvatarURL
	}

	if err := h.repo.UpdateUser(c.Request.Context(), user); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, user)
}

func (h *UserHandler) deleteUser(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	if err := h.repo.DeleteUser(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

// ---------------------------------------------------------------------------
// Organization membership endpoints
// ---------------------------------------------------------------------------

// func (h *UserHandler) listOrgMembers(c *gin.Context) {
// 	orgID, err := uuid.Parse(c.Param("org_id"))
// 	if err != nil {
// 		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org_id"})
// 		return
// 	}
//
// 	limit, offset := parsePagination(c)
//
// 	members, err := h.repo.ListOrgMembers(c.Request.Context(), orgID, limit, offset)
// 	if err != nil {
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list org members"})
// 		return
// 	}
//
// 	c.JSON(http.StatusOK, gin.H{"members": members})
// }

// // inviteOrgMemberRequest is the JSON body for inviting a user to an organization.
// type inviteOrgMemberRequest struct {
// 	Email string        `json:"email" binding:"required"`
// 	Role  models.OrgRole `json:"role" binding:"required"`
// }
//
// func (h *UserHandler) inviteOrgMember(c *gin.Context) {
// 	orgID, err := uuid.Parse(c.Param("org_id"))
// 	if err != nil {
// 		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org_id"})
// 		return
// 	}
//
// 	var req inviteOrgMemberRequest
// 	if err := c.ShouldBindJSON(&req); err != nil {
// 		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
// 		return
// 	}
//
// 	if !models.ValidRole(req.Role) {
// 		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid role"})
// 		return
// 	}
//
// 	// Look up the user by email.
// 	user, err := h.repo.GetUserByEmail(c.Request.Context(), req.Email)
// 	if err != nil {
// 		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
// 		return
// 	}
//
// 	inviterIDValue, _ := c.Get("user_id")
// 	inviterID, ok := inviterIDValue.(uuid.UUID)
// 	if !ok {
// 		inviterID = uuid.Nil
// 	}
//
// 	member := &models.OrgMember{
// 		ID:        uuid.New(),
// 		OrgID:     orgID,
// 		UserID:    user.ID,
// 		Role:      req.Role,
// 		InvitedBy: inviterID,
// 	}
//
// 	if err := member.Validate(); err != nil {
// 		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
// 		return
// 	}
//
// 	if err := h.repo.CreateOrgMember(c.Request.Context(), member); err != nil {
// 		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
// 		return
// 	}
//
// 	c.JSON(http.StatusCreated, member)
// }
//
// changeOrgRoleRequest is the JSON body for changing a user's organization role.
// type changeOrgRoleRequest struct {
// 	Role models.OrgRole `json:"role" binding:"required"`
// }
//
// func (h *UserHandler) changeOrgRole(c *gin.Context) {
// 	orgID, err := uuid.Parse(c.Param("org_id"))
// 	if err != nil {
// 		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org_id"})
// 		return
// 	}
//
// 	userID, err := uuid.Parse(c.Param("user_id"))
// 	if err != nil {
// 		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user_id"})
// 		return
// 	}
//
// 	var req changeOrgRoleRequest
// 	if err := c.ShouldBindJSON(&req); err != nil {
// 		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
// 		return
// 	}
//
// 	if !models.ValidRole(req.Role) {
// 		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid role"})
// 		return
// 	}
//
// 	member, err := h.repo.GetOrgMember(c.Request.Context(), orgID, userID)
// 	if err != nil {
// 		c.JSON(http.StatusNotFound, gin.H{"error": "membership not found"})
// 		return
// 	}
//
// 	member.Role = req.Role
// 	if err := h.repo.UpdateOrgMember(c.Request.Context(), member); err != nil {
// 		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
// 		return
// 	}
//
// 	c.JSON(http.StatusOK, member)
// }

// func (h *UserHandler) removeOrgMember(c *gin.Context) {
// 	orgID, err := uuid.Parse(c.Param("org_id"))
// 	if err != nil {
// 		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org_id"})
// 		return
// 	}
//
// 	userID, err := uuid.Parse(c.Param("user_id"))
// 	if err != nil {
// 		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user_id"})
// 		return
// 	}
//
// 	if err := h.repo.DeleteOrgMember(c.Request.Context(), orgID, userID); err != nil {
// 		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
// 		return
// 	}
//
// 	c.JSON(http.StatusOK, gin.H{"status": "removed"})
// }
//
// // ---------------------------------------------------------------------------
// // Project membership endpoints
// // ---------------------------------------------------------------------------

// func (h *UserHandler) listProjectMembers(c *gin.Context) {
// 	projectID, err := uuid.Parse(c.Param("project_id"))
// 	if err != nil {
// 		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project_id"})
// 		return
// 	}
//
// 	limit, offset := parsePagination(c)
//
// 	members, err := h.repo.ListProjectMembers(c.Request.Context(), projectID, limit, offset)
// 	if err != nil {
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list project members"})
// 		return
// 	}
//
// 	c.JSON(http.StatusOK, gin.H{"members": members})
// }
//
// addProjectMemberRequest is the JSON body for adding a user to a project.
// type addProjectMemberRequest struct {
// 	UserID uuid.UUID         `json:"user_id" binding:"required"`
// 	Role   models.ProjectRole `json:"role" binding:"required"`
// }
//
// func (h *UserHandler) addProjectMember(c *gin.Context) {
// 	projectID, err := uuid.Parse(c.Param("project_id"))
// 	if err != nil {
// 		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project_id"})
// 		return
// 	}
//
// 	var req addProjectMemberRequest
// 	if err := c.ShouldBindJSON(&req); err != nil {
// 		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
// 		return
// 	}
//
// 	member := &models.ProjectMember{
// 		ID:        uuid.New(),
// 		ProjectID: projectID,
// 		UserID:    req.UserID,
// 		Role:      req.Role,
// 	}
//
// 	if err := h.repo.CreateProjectMember(c.Request.Context(), member); err != nil {
// 		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
// 		return
// 	}
//
// 	c.JSON(http.StatusCreated, member)
// }
//
// changeProjectRoleRequest is the JSON body for changing a user's project role.
// type changeProjectRoleRequest struct {
// 	Role models.ProjectRole `json:"role" binding:"required"`
// }
//
// func (h *UserHandler) changeProjectRole(c *gin.Context) {
// 	projectID, err := uuid.Parse(c.Param("project_id"))
// 	if err != nil {
// 		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project_id"})
// 		return
// 	}
//
// 	userID, err := uuid.Parse(c.Param("user_id"))
// 	if err != nil {
// 		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user_id"})
// 		return
// 	}
//
// 	var req changeProjectRoleRequest
// 	if err := c.ShouldBindJSON(&req); err != nil {
// 		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
// 		return
// 	}
//
// 	member, err := h.repo.GetProjectMember(c.Request.Context(), projectID, userID)
// 	if err != nil {
// 		c.JSON(http.StatusNotFound, gin.H{"error": "membership not found"})
// 		return
// 	}
//
// 	member.Role = req.Role
// 	if err := h.repo.UpdateProjectMember(c.Request.Context(), member); err != nil {
// 		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
// 		return
// 	}
//
// 	c.JSON(http.StatusOK, member)
// }

// func (h *UserHandler) removeProjectMember(c *gin.Context) {
// 	projectID, err := uuid.Parse(c.Param("project_id"))
// 	if err != nil {
// 		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project_id"})
// 		return
// 	}
//
// 	userID, err := uuid.Parse(c.Param("user_id"))
// 	if err != nil {
// 		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user_id"})
// 		return
// 	}
//
// 	if err := h.repo.DeleteProjectMember(c.Request.Context(), projectID, userID); err != nil {
// 		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
// 		return
// 	}
//
// 	c.JSON(http.StatusOK, gin.H{"status": "removed"})
// }
//
// // ---------------------------------------------------------------------------
// // Helpers
// // ---------------------------------------------------------------------------
//
// parsePagination extracts limit and offset query parameters with defaults.
func parsePagination(c *gin.Context) (int, int) {
	limit := 20
	offset := 0
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if limit > 100 {
		limit = 100
	}
	if o := c.Query("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}
	return limit, offset
}
