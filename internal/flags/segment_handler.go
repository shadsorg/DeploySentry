package flags

import (
	"net/http"
	"time"

	"github.com/deploysentry/deploysentry/internal/auth"
	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// createSegmentRequest is the JSON body for creating a new segment.
type createSegmentRequest struct {
	ProjectID   uuid.UUID                    `json:"project_id" binding:"required"`
	Key         string                       `json:"key" binding:"required"`
	Name        string                       `json:"name" binding:"required"`
	Description string                       `json:"description"`
	CombineOp   string                       `json:"combine_op"`
	Conditions  []createSegmentConditionReq  `json:"conditions"`
}

// createSegmentConditionReq is a single condition within a create segment request.
type createSegmentConditionReq struct {
	Attribute string `json:"attribute" binding:"required"`
	Operator  string `json:"operator" binding:"required"`
	Value     string `json:"value"`
	Priority  int    `json:"priority"`
}

// updateSegmentRequest is the JSON body for updating an existing segment.
type updateSegmentRequest struct {
	Name        string                       `json:"name"`
	Description string                       `json:"description"`
	CombineOp   string                       `json:"combine_op"`
	Conditions  []createSegmentConditionReq  `json:"conditions"`
}

// RegisterSegmentRoutes mounts the segment CRUD API routes on the given router group.
func (h *Handler) RegisterSegmentRoutes(rg *gin.RouterGroup) {
	segments := rg.Group("/segments")
	{
		segments.POST("", auth.RequirePermission(h.rbac, auth.PermFlagCreate), h.createSegment)
		segments.GET("", auth.RequirePermission(h.rbac, auth.PermFlagRead), h.listSegments)
		segments.GET("/:segmentID", auth.RequirePermission(h.rbac, auth.PermFlagRead), h.getSegment)
		segments.PUT("/:segmentID", auth.RequirePermission(h.rbac, auth.PermFlagUpdate), h.updateSegment)
		segments.DELETE("/:segmentID", auth.RequirePermission(h.rbac, auth.PermFlagArchive), h.deleteSegment)
	}
}

func (h *Handler) createSegment(c *gin.Context) {
	var req createSegmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	combineOp := req.CombineOp
	if combineOp == "" {
		combineOp = "AND"
	}

	now := time.Now().UTC()
	segment := &models.Segment{
		ID:          uuid.New(),
		ProjectID:   req.ProjectID,
		Key:         req.Key,
		Name:        req.Name,
		Description: req.Description,
		CombineOp:   combineOp,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	for _, condReq := range req.Conditions {
		segment.Conditions = append(segment.Conditions, models.SegmentCondition{
			ID:        uuid.New(),
			SegmentID: segment.ID,
			Attribute: condReq.Attribute,
			Operator:  condReq.Operator,
			Value:     condReq.Value,
			Priority:  condReq.Priority,
			CreatedAt: now,
		})
	}

	if err := h.service.CreateSegment(c.Request.Context(), segment); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, segment)
}

func (h *Handler) listSegments(c *gin.Context) {
	projectIDStr := c.Query("project_id")
	if projectIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "project_id query parameter is required"})
		return
	}

	projectID, err := uuid.Parse(projectIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project_id"})
		return
	}

	segments, err := h.service.ListSegments(c.Request.Context(), projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list segments"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"segments": segments})
}

func (h *Handler) getSegment(c *gin.Context) {
	id, err := uuid.Parse(c.Param("segmentID"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid segment id"})
		return
	}

	segment, err := h.service.GetSegment(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "segment not found"})
		return
	}

	c.JSON(http.StatusOK, segment)
}

func (h *Handler) updateSegment(c *gin.Context) {
	id, err := uuid.Parse(c.Param("segmentID"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid segment id"})
		return
	}

	var req updateSegmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	segment, err := h.service.GetSegment(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "segment not found"})
		return
	}

	if req.Name != "" {
		segment.Name = req.Name
	}
	if req.Description != "" {
		segment.Description = req.Description
	}
	if req.CombineOp != "" {
		segment.CombineOp = req.CombineOp
	}
	if req.Conditions != nil {
		now := time.Now().UTC()
		segment.Conditions = make([]models.SegmentCondition, 0, len(req.Conditions))
		for _, condReq := range req.Conditions {
			segment.Conditions = append(segment.Conditions, models.SegmentCondition{
				ID:        uuid.New(),
				SegmentID: segment.ID,
				Attribute: condReq.Attribute,
				Operator:  condReq.Operator,
				Value:     condReq.Value,
				Priority:  condReq.Priority,
				CreatedAt: now,
			})
		}
	}
	segment.UpdatedAt = time.Now().UTC()

	if err := h.service.UpdateSegment(c.Request.Context(), segment); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, segment)
}

func (h *Handler) deleteSegment(c *gin.Context) {
	id, err := uuid.Parse(c.Param("segmentID"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid segment id"})
		return
	}

	if err := h.service.DeleteSegment(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete segment"})
		return
	}

	c.Status(http.StatusNoContent)
}
