package ratings

import (
	"net/http"
	"strconv"
	"time"

	"github.com/deploysentry/deploysentry/internal/auth"
	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Handler provides HTTP endpoints for flag ratings and error tracking.
type Handler struct {
	service RatingService
	rbac    *auth.RBACChecker
}

// NewHandler creates a new ratings HTTP handler.
func NewHandler(service RatingService, rbac *auth.RBACChecker) *Handler {
	return &Handler{service: service, rbac: rbac}
}

// RegisterRoutes mounts all rating and error tracking routes on the given router group.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	flags := rg.Group("/flags")
	{
		flags.POST("/errors/report", auth.RequirePermission(h.rbac, auth.PermFlagUpdate), h.reportErrors)

		flag := flags.Group("/:id")
		{
			flag.POST("/ratings", auth.RequirePermission(h.rbac, auth.PermFlagUpdate), h.upsertRating)
			flag.GET("/ratings", auth.RequirePermission(h.rbac, auth.PermFlagRead), h.listRatings)
			flag.GET("/ratings/summary", auth.RequirePermission(h.rbac, auth.PermFlagRead), h.getRatingSummary)
			flag.DELETE("/ratings", auth.RequirePermission(h.rbac, auth.PermFlagUpdate), h.deleteRating)

			flag.GET("/errors/summary", auth.RequirePermission(h.rbac, auth.PermFlagRead), h.getErrorSummary)
			flag.GET("/errors/by-org", auth.RequirePermission(h.rbac, auth.PermOrgManage), h.getErrorsByOrg)
		}
	}
}

// ---------------------------------------------------------------------------
// Request types
// ---------------------------------------------------------------------------

type upsertRatingRequest struct {
	Rating  int16  `json:"rating" binding:"required"`
	Comment string `json:"comment"`
}

type reportErrorsRequest struct {
	ProjectID     uuid.UUID        `json:"project_id" binding:"required"`
	EnvironmentID uuid.UUID        `json:"environment_id" binding:"required"`
	OrgID         uuid.UUID        `json:"org_id" binding:"required"`
	Stats         []errorStatEntry `json:"stats" binding:"required"`
}

type errorStatEntry struct {
	FlagKey     string `json:"flag_key" binding:"required"`
	Evaluations int64  `json:"evaluations" binding:"required"`
	Errors      int64  `json:"errors"`
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

// getContextIDs extracts user_id and org_id from the gin context.
// user_id is set as uuid.UUID by JWT middleware; org_id is set as string.
func getContextIDs(c *gin.Context) (userID uuid.UUID, orgID uuid.UUID, ok bool) {
	uid, exists := c.Get("user_id")
	if !exists {
		return uuid.Nil, uuid.Nil, false
	}
	userID, _ = uid.(uuid.UUID)

	oid, err := uuid.Parse(c.GetString("org_id"))
	if err != nil {
		return uuid.Nil, uuid.Nil, false
	}
	return userID, oid, true
}

// requireRatingsEnabled checks the org's ratings toggle.
func (h *Handler) requireRatingsEnabled(c *gin.Context) (uuid.UUID, bool) {
	_, orgID, ok := getContextIDs(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing user/org context"})
		return uuid.Nil, false
	}
	enabled, err := h.service.IsRatingsEnabled(c.Request.Context(), orgID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check ratings setting"})
		return uuid.Nil, false
	}
	if !enabled {
		c.JSON(http.StatusForbidden, gin.H{"error": "ratings are not enabled for this organization"})
		return uuid.Nil, false
	}
	return orgID, true
}

func (h *Handler) upsertRating(c *gin.Context) {
	flagID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid flag ID"})
		return
	}

	userID, orgID, ok := getContextIDs(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing user/org context"})
		return
	}

	enabled, err := h.service.IsRatingsEnabled(c.Request.Context(), orgID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check ratings setting"})
		return
	}
	if !enabled {
		c.JSON(http.StatusForbidden, gin.H{"error": "ratings are not enabled for this organization"})
		return
	}

	var req upsertRatingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	rating := &models.FlagRating{
		FlagID:  flagID,
		UserID:  userID,
		OrgID:   orgID,
		Rating:  req.Rating,
		Comment: req.Comment,
	}

	if err := h.service.UpsertRating(c.Request.Context(), rating); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rating)
}

func (h *Handler) listRatings(c *gin.Context) {
	flagID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid flag ID"})
		return
	}
	if _, ok := h.requireRatingsEnabled(c); !ok {
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	ratings, err := h.service.ListRatings(c.Request.Context(), flagID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, ratings)
}

func (h *Handler) getRatingSummary(c *gin.Context) {
	flagID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid flag ID"})
		return
	}
	if _, ok := h.requireRatingsEnabled(c); !ok {
		return
	}

	summary, err := h.service.GetRatingSummary(c.Request.Context(), flagID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, summary)
}

func (h *Handler) deleteRating(c *gin.Context) {
	flagID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid flag ID"})
		return
	}
	if _, ok := h.requireRatingsEnabled(c); !ok {
		return
	}

	userID, _, ok := getContextIDs(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing user context"})
		return
	}

	if err := h.service.DeleteRating(c.Request.Context(), flagID, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) reportErrors(c *gin.Context) {
	var req reportErrorsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate that the request org_id matches the authenticated caller's org.
	_, authedOrgID, ok := getContextIDs(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing user/org context"})
		return
	}
	if req.OrgID != authedOrgID {
		c.JSON(http.StatusForbidden, gin.H{"error": "org_id does not match authenticated organization"})
		return
	}

	entries := make([]ErrorReportEntry, len(req.Stats))
	for i, s := range req.Stats {
		entries[i] = ErrorReportEntry{FlagKey: s.FlagKey, Evaluations: s.Evaluations, Errors: s.Errors}
	}

	if err := h.service.ReportErrors(c.Request.Context(), req.ProjectID, entries, req.EnvironmentID, req.OrgID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"status": "accepted"})
}

func (h *Handler) getErrorSummary(c *gin.Context) {
	flagID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid flag ID"})
		return
	}

	periodStr := c.DefaultQuery("period", "7d")
	period := parsePeriod(periodStr)

	summary, err := h.service.GetErrorSummary(c.Request.Context(), flagID, period)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, summary)
}

func (h *Handler) getErrorsByOrg(c *gin.Context) {
	flagID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid flag ID"})
		return
	}

	periodStr := c.DefaultQuery("period", "7d")
	period := parsePeriod(periodStr)

	breakdown, err := h.service.GetErrorsByOrg(c.Request.Context(), flagID, period)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, breakdown)
}

func parsePeriod(s string) time.Duration {
	switch s {
	case "24h":
		return 24 * time.Hour
	case "7d":
		return 7 * 24 * time.Hour
	case "30d":
		return 30 * 24 * time.Hour
	default:
		return 7 * 24 * time.Hour
	}
}
