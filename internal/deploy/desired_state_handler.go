package deploy

import (
	"net/http"

	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// DesiredStateResponse describes the desired runtime state for a deployment,
// including the active phase and previous deployment info if available.
type DesiredStateResponse struct {
	DeploymentID       uuid.UUID                 `json:"deployment_id"`
	ApplicationID      uuid.UUID                 `json:"application_id"`
	EnvironmentID      uuid.UUID                 `json:"environment_id"`
	Artifact           string                    `json:"artifact"`
	Version            string                    `json:"version"`
	CommitSHA          string                    `json:"commit_sha,omitempty"`
	Strategy           models.DeployStrategyType `json:"strategy"`
	Status             models.DeployStatus       `json:"status"`
	DesiredTrafficPct  int                       `json:"desired_traffic_percent"`
	CurrentPhase       *PhaseInfo                `json:"current_phase,omitempty"`
	PreviousDeployment *PrevDeploymentInfo       `json:"previous_deployment,omitempty"`
}

// PhaseInfo summarises the currently active deployment phase.
type PhaseInfo struct {
	Name        string `json:"name"`
	SortOrder   int    `json:"sort_order"`
	StartedAt   string `json:"started_at,omitempty"`
	DurationSec int    `json:"duration_secs"`
	AutoPromote bool   `json:"auto_promote"`
}

// PrevDeploymentInfo identifies the deployment that preceded the current one.
type PrevDeploymentInfo struct {
	DeploymentID uuid.UUID `json:"deployment_id"`
	Artifact     string    `json:"artifact"`
	Version      string    `json:"version"`
}

// getDesiredState handles GET /deployments/:id/desired-state.
// It returns the full desired-state view for a single deployment.
func (h *Handler) getDesiredState(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid deployment id"})
		return
	}

	d, err := h.service.GetDeployment(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "deployment not found"})
		return
	}

	resp := h.buildDesiredStateResponse(c, d)
	c.JSON(http.StatusOK, resp)
}

// getAppDesiredState handles GET /applications/:app_id/desired-state.
// It returns desired-state views for all active deployments of an application.
func (h *Handler) getAppDesiredState(c *gin.Context) {
	applicationID, err := uuid.Parse(c.Param("app_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid application id"})
		return
	}

	deployments, err := h.service.GetActiveDeployments(c.Request.Context(), applicationID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get active deployments"})
		return
	}

	responses := make([]DesiredStateResponse, 0, len(deployments))
	for _, d := range deployments {
		responses = append(responses, h.buildDesiredStateResponse(c, d))
	}

	c.JSON(http.StatusOK, responses)
}

// buildDesiredStateResponse constructs a DesiredStateResponse for a deployment,
// fetching the active phase and previous deployment info as needed.
func (h *Handler) buildDesiredStateResponse(c *gin.Context, d *models.Deployment) DesiredStateResponse {
	resp := DesiredStateResponse{
		DeploymentID:      d.ID,
		ApplicationID:     d.ApplicationID,
		EnvironmentID:     d.EnvironmentID,
		Artifact:          d.Artifact,
		Version:           d.Version,
		CommitSHA:         d.CommitSHA,
		Strategy:          d.Strategy,
		Status:            d.Status,
		DesiredTrafficPct: d.TrafficPercent,
	}

	// Attempt to attach active phase info (best-effort; never fails the response).
	if phases, err := h.service.ListPhases(c.Request.Context(), d.ID); err == nil {
		for _, p := range phases {
			if p.Status == models.DeploymentPhaseStatusActive {
				pi := &PhaseInfo{
					Name:        p.Name,
					SortOrder:   p.SortOrder,
					DurationSec: p.Duration,
					AutoPromote: p.AutoPromote,
				}
				if p.StartedAt != nil {
					pi.StartedAt = p.StartedAt.UTC().Format("2006-01-02T15:04:05Z")
				}
				resp.CurrentPhase = pi
				break
			}
		}
	}

	// Attach previous deployment info if a predecessor is recorded.
	if d.PreviousDeploymentID != nil {
		if prev, err := h.service.GetDeployment(c.Request.Context(), *d.PreviousDeploymentID); err == nil {
			resp.PreviousDeployment = &PrevDeploymentInfo{
				DeploymentID: prev.ID,
				Artifact:     prev.Artifact,
				Version:      prev.Version,
			}
		}
	}

	return resp
}
