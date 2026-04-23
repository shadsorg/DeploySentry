package entities

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/deploysentry/deploysentry/internal/auth"
	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// AccessResolver checks whether a user has grant-based access to a resource.
type AccessResolver interface {
	ResolveAccess(ctx context.Context, userID uuid.UUID, orgRole string, projectID *uuid.UUID, applicationID *uuid.UUID) (*models.ResourcePermission, error)
}

// Handler provides HTTP endpoints for entity management (orgs, projects, apps).
type Handler struct {
	service EntityService
	rbac    *auth.RBACChecker
	access  AccessResolver
}

// NewHandler creates a new entity HTTP handler.
func NewHandler(service EntityService, rbac *auth.RBACChecker, access AccessResolver) *Handler {
	return &Handler{service: service, rbac: rbac, access: access}
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

		// Org-wide apps listing — aggregates across every project the caller can see.
		// Callers who want to enumerate apps without iterating projects can hit this
		// single endpoint (matches the pattern used by /orgs/:slug/status).
		orgs.GET("/:orgSlug/apps", h.listAppsByOrg)

		projects := orgs.Group("/:orgSlug/projects")
		{
			projects.POST("", auth.RequirePermission(h.rbac, auth.PermOrgManage), h.createProject)
			projects.GET("", h.listProjects)
			projects.GET("/:projectSlug", h.getProject)
			projects.PUT("/:projectSlug", auth.RequirePermission(h.rbac, auth.PermProjectManage), h.updateProject)
			projects.DELETE("/:projectSlug", auth.RequirePermission(h.rbac, auth.PermProjectManage), h.deleteProject)
			projects.DELETE("/:projectSlug/permanent", auth.RequirePermission(h.rbac, auth.PermOrgManage), h.hardDeleteProject)
			projects.POST("/:projectSlug/restore", auth.RequirePermission(h.rbac, auth.PermProjectManage), h.restoreProject)

			apps := projects.Group("/:projectSlug/apps")
			{
				apps.POST("", auth.RequirePermission(h.rbac, auth.PermProjectManage), h.createApp)
				apps.GET("", h.listApps)
				apps.GET("/:appSlug", h.getApp)
				apps.PUT("/:appSlug", auth.RequirePermission(h.rbac, auth.PermProjectManage), h.updateApp)
			apps.PUT("/:appSlug/monitoring-links", auth.RequirePermission(h.rbac, auth.PermProjectManage), h.updateAppMonitoringLinks)
				apps.DELETE("/:appSlug", auth.RequirePermission(h.rbac, auth.PermProjectManage), h.deleteApp)
				apps.DELETE("/:appSlug/permanent", auth.RequirePermission(h.rbac, auth.PermOrgManage), h.hardDeleteApp)
				apps.POST("/:appSlug/restore", auth.RequirePermission(h.rbac, auth.PermProjectManage), h.restoreApp)
				apps.GET("/:appSlug/environments", h.listEnvironments)
			}
		}
	}
}

// checkProjectAccess verifies the caller has grant-based access to the project.
// Returns false (and writes 404) when access is denied.
func (h *Handler) checkProjectAccess(c *gin.Context, projectID uuid.UUID) bool {
	uid, _ := c.Get("user_id")
	userID, _ := uid.(uuid.UUID)
	roleVal, _ := c.Get("role")
	orgRole := fmt.Sprintf("%v", roleVal)

	perm, err := h.access.ResolveAccess(c.Request.Context(), userID, orgRole, &projectID, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve access"})
		return false
	}
	if perm == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
		return false
	}
	return true
}

// checkAppAccess verifies the caller has grant-based access to the application.
// Returns false (and writes 404) when access is denied.
func (h *Handler) checkAppAccess(c *gin.Context, projectID, appID uuid.UUID) bool {
	uid, _ := c.Get("user_id")
	userID, _ := uid.(uuid.UUID)
	roleVal, _ := c.Get("role")
	orgRole := fmt.Sprintf("%v", roleVal)

	perm, err := h.access.ResolveAccess(c.Request.Context(), userID, orgRole, &projectID, &appID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve access"})
		return false
	}
	if perm == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "application not found"})
		return false
	}
	return true
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
	// API-key authenticated callers have an org_id stamped in context by
	// the auth middleware, but no user_id. Resolve that single scoped org
	// directly so `orgs list` doesn't silently return [] for valid keys.
	if method, _ := c.Get("auth_method"); method == "api_key" {
		if orgIDRaw, exists := c.Get("org_id"); exists {
			orgIDStr, _ := orgIDRaw.(string)
			if orgIDStr != "" {
				if orgID, err := uuid.Parse(orgIDStr); err == nil {
					org, err := h.service.GetOrgByID(c.Request.Context(), orgID)
					if err != nil {
						c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
						return
					}
					if org == nil {
						c.JSON(http.StatusOK, gin.H{"organizations": []*models.Organization{}})
						return
					}
					c.JSON(http.StatusOK, gin.H{"organizations": []*models.Organization{org}})
					return
				}
			}
		}
		// API key with no org scoping falls through to the empty response
		// with a clear hint for diagnostics.
		c.JSON(http.StatusOK, gin.H{
			"organizations": []*models.Organization{},
			"hint":          "this API key is not scoped to an organization; mint a key under the target org",
		})
		return
	}

	uid, _ := c.Get("user_id")
	userID, _ := uid.(uuid.UUID)
	orgs, err := h.service.ListOrgsByUser(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if len(orgs) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"organizations": orgs,
			"hint":          "you are not a member of any organization; create one or ask an admin to invite you",
		})
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

	uid, _ := c.Get("user_id")
	userID, _ := uid.(uuid.UUID)
	roleVal, _ := c.Get("role")
	orgRole := fmt.Sprintf("%v", roleVal)

	includeDeleted := c.Query("include_deleted") == "true"
	projects, err := h.service.ListProjectsByOrg(c.Request.Context(), org.ID, includeDeleted, userID, orgRole)
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
	if !h.checkProjectAccess(c, project.ID) {
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
	if !h.checkProjectAccess(c, project.ID) {
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

func (h *Handler) deleteProject(c *gin.Context) {
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
	if !h.checkProjectAccess(c, project.ID) {
		return
	}
	result, err := h.service.DeleteProject(c.Request.Context(), org.ID, c.Param("projectSlug"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if len(result.ActiveFlags) > 0 {
		c.JSON(http.StatusConflict, gin.H{
			"error":        "project has flags with recent activity",
			"active_flags": result.ActiveFlags,
		})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handler) hardDeleteProject(c *gin.Context) {
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
	if !h.checkProjectAccess(c, project.ID) {
		return
	}
	err = h.service.HardDeleteProject(c.Request.Context(), org.ID, c.Param("projectSlug"))
	if err != nil {
		if strings.Contains(err.Error(), "eligible at") {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) restoreProject(c *gin.Context) {
	org, err := h.service.GetOrgBySlug(c.Request.Context(), c.Param("orgSlug"))
	if err != nil || org == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "organization not found"})
		return
	}
	// Resolve project for access check before restoring.
	proj, err := h.service.GetProjectBySlug(c.Request.Context(), org.ID, c.Param("projectSlug"))
	if err != nil || proj == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
		return
	}
	if !h.checkProjectAccess(c, proj.ID) {
		return
	}
	project, err := h.service.RestoreProject(c.Request.Context(), org.ID, c.Param("projectSlug"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
	if !h.checkProjectAccess(c, project.ID) {
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

	uid, _ := c.Get("user_id")
	userID, _ := uid.(uuid.UUID)
	roleVal, _ := c.Get("role")
	orgRole := fmt.Sprintf("%v", roleVal)

	includeDeleted := c.Query("include_deleted") == "true"
	apps, err := h.service.ListAppsByProject(c.Request.Context(), project.ID, includeDeleted, userID, orgRole)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"applications": apps})
}

// listAppsByOrg returns every application across every project in the org
// that the caller has visibility into. Saves MCP agents (and the CLI) from
// iterating projects × apps when they just want the full set.
//
// Each row in the response carries its parent project's id + slug + name
// so callers can render a grouped view without a second round-trip.
func (h *Handler) listAppsByOrg(c *gin.Context) {
	org, err := h.service.GetOrgBySlug(c.Request.Context(), c.Param("orgSlug"))
	if err != nil || org == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "organization not found"})
		return
	}

	uid, _ := c.Get("user_id")
	userID, _ := uid.(uuid.UUID)
	roleVal, _ := c.Get("role")
	orgRole := fmt.Sprintf("%v", roleVal)
	includeDeleted := c.Query("include_deleted") == "true"

	projects, err := h.service.ListProjectsByOrg(c.Request.Context(), org.ID, includeDeleted, userID, orgRole)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	type orgAppRow struct {
		*models.Application
		Project models.ProjectSummary `json:"project"`
	}
	rows := make([]orgAppRow, 0)
	for _, p := range projects {
		apps, err := h.service.ListAppsByProject(c.Request.Context(), p.ID, includeDeleted, userID, orgRole)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		for _, a := range apps {
			rows = append(rows, orgAppRow{
				Application: a,
				Project:     models.ProjectSummary{ID: p.ID, Slug: p.Slug, Name: p.Name},
			})
		}
	}
	c.JSON(http.StatusOK, gin.H{"applications": rows})
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
	if !h.checkAppAccess(c, project.ID, app.ID) {
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
	if !h.checkAppAccess(c, project.ID, app.ID) {
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

// updateAppMonitoringLinks replaces the monitoring_links array for an app.
func (h *Handler) updateAppMonitoringLinks(c *gin.Context) {
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
	if !h.checkAppAccess(c, project.ID, app.ID) {
		return
	}

	var req struct {
		MonitoringLinks []models.MonitoringLink `json:"monitoring_links"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	cleaned, err := h.service.UpdateAppMonitoringLinks(c.Request.Context(), app.ID, req.MonitoringLinks)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	app.MonitoringLinks = cleaned
	c.JSON(http.StatusOK, app)
}

func (h *Handler) deleteApp(c *gin.Context) {
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
	if !h.checkAppAccess(c, project.ID, app.ID) {
		return
	}
	result, err := h.service.DeleteApp(c.Request.Context(), project.ID, c.Param("appSlug"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if len(result.ActiveFlags) > 0 {
		c.JSON(http.StatusConflict, gin.H{
			"error":        "application has flags with recent activity",
			"active_flags": result.ActiveFlags,
		})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handler) hardDeleteApp(c *gin.Context) {
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
	if !h.checkAppAccess(c, project.ID, app.ID) {
		return
	}
	err = h.service.HardDeleteApp(c.Request.Context(), project.ID, c.Param("appSlug"))
	if err != nil {
		if strings.Contains(err.Error(), "eligible at") {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) restoreApp(c *gin.Context) {
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
	// Resolve app for access check before restoring.
	a, err := h.service.GetAppBySlug(c.Request.Context(), project.ID, c.Param("appSlug"))
	if err != nil || a == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "application not found"})
		return
	}
	if !h.checkAppAccess(c, project.ID, a.ID) {
		return
	}
	app, err := h.service.RestoreApp(c.Request.Context(), project.ID, c.Param("appSlug"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
	if !h.checkAppAccess(c, project.ID, app.ID) {
		return
	}

	environments, err := h.service.ListEnvironmentsByApp(c.Request.Context(), app.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"environments": environments})
}
