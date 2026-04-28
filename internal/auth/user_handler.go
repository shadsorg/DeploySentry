package auth

import (
	"context"
	"net/http"
	"strconv"

	"github.com/shadsorg/deploysentry/internal/models"
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

	// Only allow querying own profile to prevent enumeration
	userIDValue, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	requestorID, ok := userIDValue.(uuid.UUID)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user context"})
		return
	}

	if id != requestorID {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
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
// Helpers
// ---------------------------------------------------------------------------

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
