package entities

import (
	"net/http"
	"strings"

	"github.com/deploysentry/deploysentry/internal/auth"
	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Handler provides HTTP endpoints for entity management (orgs, projects, apps).
type Handler struct {
	service EntityService
	rbac    *auth.RBACChecker
}

// NewHandler creates a new entity HTTP handler.
func NewHandler(service EntityService, rbac *auth.RBACChecker) *Handler {
	return &Handler{service: service, rbac: rbac}
}

// RegisterRoutes mounts all entity management routes on the given router group.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	orgs := rg.Group("/orgs")
	{
		orgs.POST("", h.createOrg)
		orgs.GET("", h.listOrgs)
		orgs.GET("/:orgSlug", h.getOrg)
		orgs.PUT("/:orgSlug", auth.RequirePermission(h.rbac, auth.PermOrgManage), h.updateOrg)

		envs := orgs.Group("/:orgSlug/environments")
		{
			envs.GET("", h.listOrgEnvironments)
			envs.POST("", auth.RequirePermission(h.rbac, auth.PermOrgManage), h.createEnvironment)
			envs.PUT("/:envSlug", auth.RequirePermission(h.rbac, auth.PermOrgManage), h.updateEnvironment)
			envs.DELETE("/:envSlug", auth.RequirePermission(h.rbac, auth.PermOrgManage), h.deleteEnvironment)
		}

		projects := orgs.Group("/:orgSlug/projects")
		{
			projects.POST("", auth.RequirePermission(h.rbac, auth.PermOrgManage), h.createProject)
			projects.GET("", h.listProjects)
			projects.GET("/:projectSlug", h.getProject)
			projects.PUT("/:projectSlug", auth.RequirePermission(h.rbac, auth.PermProjectManage), h.updateProject)

			apps := projects.Group("/:projectSlug/apps")
			{
				apps.POST("", auth.RequirePermission(h.rbac, auth.PermProjectManage), h.createApp)
				apps.GET("", h.listApps)
				apps.GET("/:appSlug", h.getApp)
				apps.PUT("/:appSlug", auth.RequirePermission(h.rbac, auth.PermProjectManage), h.updateApp)
				apps.GET("/:appSlug/environments", h.listEnvironments)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Org handlers
// ---------------------------------------------------------------------------

func (h *Handler) createOrg(c *gin.Context) {
	var req struct {
		Name string `json:"name" binding:"required"`
		Slug string `json:"slug" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	uid, _ := c.Get("user_id")
	userID, _ := uid.(uuid.UUID)

	org := &models.Organization{Name: req.Name, Slug: req.Slug}
	if err := h.service.CreateOrg(c.Request.Context(), org, userID); err != nil {
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") {
			c.JSON(http.StatusConflict, gin.H{"error": "slug already exists"})
			return
		}
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, org)
}

func (h *Handler) listOrgs(c *gin.Context) {
	uid, _ := c.Get("user_id")
	userID, _ := uid.(uuid.UUID)

	orgs, err := h.service.ListOrgsByUser(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"organizations": orgs})
}

func (h *Handler) getOrg(c *gin.Context) {
	org, err := h.service.GetOrgBySlug(c.Request.Context(), c.Param("orgSlug"))
	if err != nil || org == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "organization not found"})
		return
	}
	c.JSON(http.StatusOK, org)
}

func (h *Handler) updateOrg(c *gin.Context) {
	org, err := h.service.GetOrgBySlug(c.Request.Context(), c.Param("orgSlug"))
	if err != nil || org == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "organization not found"})
		return
	}

	var req struct {
		Name string `json:"name"`
		Plan string `json:"plan"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Name != "" {
		org.Name = req.Name
	}
	if req.Plan != "" {
		org.Plan = req.Plan
	}

	if err := h.service.UpdateOrg(c.Request.Context(), org); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, org)
}

// ---------------------------------------------------------------------------
// Project handlers
// ---------------------------------------------------------------------------

func (h *Handler) createProject(c *gin.Context) {
	org, err := h.service.GetOrgBySlug(c.Request.Context(), c.Param("orgSlug"))
	if err != nil || org == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "organization not found"})
		return
	}

	var req struct {
		Name        string `json:"name" binding:"required"`
		Slug        string `json:"slug" binding:"required"`
		Description string `json:"description"`
		RepoURL     string `json:"repo_url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	project := &models.Project{
		OrgID:       org.ID,
		Name:        req.Name,
		Slug:        req.Slug,
		Description: req.Description,
		RepoURL:     req.RepoURL,
	}
	if err := h.service.CreateProject(c.Request.Context(), project); err != nil {
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") {
			c.JSON(http.StatusConflict, gin.H{"error": "slug already exists"})
			return
		}
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, project)
}

func (h *Handler) listProjects(c *gin.Context) {
	org, err := h.service.GetOrgBySlug(c.Request.Context(), c.Param("orgSlug"))
	if err != nil || org == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "organization not found"})
		return
	}

	projects, err := h.service.ListProjectsByOrg(c.Request.Context(), org.ID, false)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"projects": projects})
}

func (h *Handler) getProject(c *gin.Context) {
	org, err := h.service.GetOrgBySlug(c.Request.Context(), c.Param("orgSlug"))
	if err != nil || org == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "organization not found"})
		return
	}

	project, err := h.service.GetProjectBySlug(c.Request.Context(), org.ID, c.Param("projectSlug"))
	if err != nil || project == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
		return
	}
	c.JSON(http.StatusOK, project)
}

func (h *Handler) updateProject(c *gin.Context) {
	org, err := h.service.GetOrgBySlug(c.Request.Context(), c.Param("orgSlug"))
	if err != nil || org == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "organization not found"})
		return
	}

	project, err := h.service.GetProjectBySlug(c.Request.Context(), org.ID, c.Param("projectSlug"))
	if err != nil || project == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
		return
	}

	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		RepoURL     string `json:"repo_url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Name != "" {
		project.Name = req.Name
	}
	if req.Description != "" {
		project.Description = req.Description
	}
	if req.RepoURL != "" {
		project.RepoURL = req.RepoURL
	}

	if err := h.service.UpdateProject(c.Request.Context(), project); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, project)
}

// ---------------------------------------------------------------------------
// App handlers
// ---------------------------------------------------------------------------

func (h *Handler) createApp(c *gin.Context) {
	org, err := h.service.GetOrgBySlug(c.Request.Context(), c.Param("orgSlug"))
	if err != nil || org == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "organization not found"})
		return
	}

	project, err := h.service.GetProjectBySlug(c.Request.Context(), org.ID, c.Param("projectSlug"))
	if err != nil || project == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
		return
	}

	var req struct {
		Name        string `json:"name" binding:"required"`
		Slug        string `json:"slug" binding:"required"`
		Description string `json:"description"`
		RepoURL     string `json:"repo_url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	app := &models.Application{
		ProjectID:   project.ID,
		Name:        req.Name,
		Slug:        req.Slug,
		Description: req.Description,
		RepoURL:     req.RepoURL,
	}
	if err := h.service.CreateApp(c.Request.Context(), app); err != nil {
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") {
			c.JSON(http.StatusConflict, gin.H{"error": "slug already exists"})
			return
		}
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, app)
}

func (h *Handler) listApps(c *gin.Context) {
	org, err := h.service.GetOrgBySlug(c.Request.Context(), c.Param("orgSlug"))
	if err != nil || org == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "organization not found"})
		return
	}

	project, err := h.service.GetProjectBySlug(c.Request.Context(), org.ID, c.Param("projectSlug"))
	if err != nil || project == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
		return
	}

	apps, err := h.service.ListAppsByProject(c.Request.Context(), project.ID, false)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"applications": apps})
}

func (h *Handler) getApp(c *gin.Context) {
	org, err := h.service.GetOrgBySlug(c.Request.Context(), c.Param("orgSlug"))
	if err != nil || org == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "organization not found"})
		return
	}

	project, err := h.service.GetProjectBySlug(c.Request.Context(), org.ID, c.Param("projectSlug"))
	if err != nil || project == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
		return
	}

	app, err := h.service.GetAppBySlug(c.Request.Context(), project.ID, c.Param("appSlug"))
	if err != nil || app == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "application not found"})
		return
	}
	c.JSON(http.StatusOK, app)
}

func (h *Handler) updateApp(c *gin.Context) {
	org, err := h.service.GetOrgBySlug(c.Request.Context(), c.Param("orgSlug"))
	if err != nil || org == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "organization not found"})
		return
	}

	project, err := h.service.GetProjectBySlug(c.Request.Context(), org.ID, c.Param("projectSlug"))
	if err != nil || project == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
		return
	}

	app, err := h.service.GetAppBySlug(c.Request.Context(), project.ID, c.Param("appSlug"))
	if err != nil || app == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "application not found"})
		return
	}

	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		RepoURL     string `json:"repo_url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Name != "" {
		app.Name = req.Name
	}
	if req.Description != "" {
		app.Description = req.Description
	}
	if req.RepoURL != "" {
		app.RepoURL = req.RepoURL
	}

	if err := h.service.UpdateApp(c.Request.Context(), app); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, app)
}

// ---------------------------------------------------------------------------
// Org environment handlers
// ---------------------------------------------------------------------------

func (h *Handler) listOrgEnvironments(c *gin.Context) {
	org, err := h.service.GetOrgBySlug(c.Request.Context(), c.Param("orgSlug"))
	if err != nil || org == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "organization not found"})
		return
	}

	envs, err := h.service.ListEnvironments(c.Request.Context(), org.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"environments": envs})
}

func (h *Handler) createEnvironment(c *gin.Context) {
	org, err := h.service.GetOrgBySlug(c.Request.Context(), c.Param("orgSlug"))
	if err != nil || org == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "organization not found"})
		return
	}

	var req struct {
		Name         string `json:"name" binding:"required"`
		Slug         string `json:"slug" binding:"required"`
		IsProduction bool   `json:"is_production"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	env := &OrgEnvironment{
		OrgID:        org.ID,
		Name:         req.Name,
		Slug:         req.Slug,
		IsProduction: req.IsProduction,
	}
	if err := h.service.CreateEnvironment(c.Request.Context(), env); err != nil {
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") {
			c.JSON(http.StatusConflict, gin.H{"error": "slug already exists"})
			return
		}
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, env)
}

func (h *Handler) updateEnvironment(c *gin.Context) {
	org, err := h.service.GetOrgBySlug(c.Request.Context(), c.Param("orgSlug"))
	if err != nil || org == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "organization not found"})
		return
	}

	env, err := h.service.GetEnvironmentBySlug(c.Request.Context(), org.ID, c.Param("envSlug"))
	if err != nil || env == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "environment not found"})
		return
	}

	var req struct {
		Name         string `json:"name"`
		Slug         string `json:"slug"`
		IsProduction *bool  `json:"is_production"`
		SortOrder    *int   `json:"sort_order"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Name != "" {
		env.Name = req.Name
	}
	if req.Slug != "" {
		env.Slug = req.Slug
	}
	if req.IsProduction != nil {
		env.IsProduction = *req.IsProduction
	}
	if req.SortOrder != nil {
		env.SortOrder = *req.SortOrder
	}

	if err := h.service.UpdateEnvironment(c.Request.Context(), env); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, env)
}

func (h *Handler) deleteEnvironment(c *gin.Context) {
	org, err := h.service.GetOrgBySlug(c.Request.Context(), c.Param("orgSlug"))
	if err != nil || org == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "organization not found"})
		return
	}

	env, err := h.service.GetEnvironmentBySlug(c.Request.Context(), org.ID, c.Param("envSlug"))
	if err != nil || env == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "environment not found"})
		return
	}

	if err := h.service.DeleteEnvironment(c.Request.Context(), env.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

// ---------------------------------------------------------------------------
// App-scoped Environment handlers
// ---------------------------------------------------------------------------

func (h *Handler) listEnvironments(c *gin.Context) {
	org, err := h.service.GetOrgBySlug(c.Request.Context(), c.Param("orgSlug"))
	if err != nil || org == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "organization not found"})
		return
	}

	project, err := h.service.GetProjectBySlug(c.Request.Context(), org.ID, c.Param("projectSlug"))
	if err != nil || project == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
		return
	}

	app, err := h.service.GetAppBySlug(c.Request.Context(), project.ID, c.Param("appSlug"))
	if err != nil || app == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "application not found"})
		return
	}

	environments, err := h.service.ListEnvironmentsByApp(c.Request.Context(), app.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"environments": environments})
}
