// Package deploy implements the deploy-target Applicator: bridges rollout phase
// progression to the existing deploy service's traffic-percent update path and
// the existing health monitor.
package deploy

import (
	"context"
	"errors"
	"math"

	"github.com/deploysentry/deploysentry/internal/health"
	"github.com/deploysentry/deploysentry/internal/models"
	"github.com/deploysentry/deploysentry/internal/rollout/applicator"
	"github.com/google/uuid"
)

// TrafficSetter is the subset of the deploy service the applicator depends on.
type TrafficSetter interface {
	SetTrafficPercent(ctx context.Context, deploymentID uuid.UUID, pct int) error
}

// HealthReader is the subset of the health monitor the applicator depends on.
type HealthReader interface {
	GetHealth(deploymentID uuid.UUID) (*health.DeploymentHealth, error)
}

// Applicator implements applicator.Applicator for deploy targets.
type Applicator struct {
	traffic TrafficSetter
	health  HealthReader
}

// NewApplicator builds a deploy applicator.
func NewApplicator(traffic TrafficSetter, healthReader HealthReader) *Applicator {
	return &Applicator{traffic: traffic, health: healthReader}
}

var _ applicator.Applicator = (*Applicator)(nil)

// ErrMissingDeploymentID is returned when a deploy rollout lacks a deployment_id.
var ErrMissingDeploymentID = errors.New("deploy rollout missing deployment_id in target_ref")

func (a *Applicator) deploymentID(ro *models.Rollout) (uuid.UUID, error) {
	if ro.TargetRef.DeploymentID == nil {
		return uuid.Nil, ErrMissingDeploymentID
	}
	return uuid.Parse(*ro.TargetRef.DeploymentID)
}

// Apply sets traffic % on the target deployment.
func (a *Applicator) Apply(ctx context.Context, ro *models.Rollout, step models.Step) error {
	depID, err := a.deploymentID(ro)
	if err != nil {
		return err
	}
	pct := int(math.Round(step.Percent))
	return a.traffic.SetTrafficPercent(ctx, depID, pct)
}

// Revert sets traffic back to 0 on the target deployment.
func (a *Applicator) Revert(ctx context.Context, ro *models.Rollout) error {
	depID, err := a.deploymentID(ro)
	if err != nil {
		return err
	}
	return a.traffic.SetTrafficPercent(ctx, depID, 0)
}

// CurrentSignal reads the health monitor and maps into a normalized HealthScore.
// Note: health.DeploymentHealth only exposes Overall (overall score) and Healthy;
// it does not carry ErrorRate, LatencyP99, LatencyP50, or RequestRate fields,
// so those are left as zero in the returned HealthScore.
func (a *Applicator) CurrentSignal(ctx context.Context, ro *models.Rollout, _ *models.SignalSource) (applicator.HealthScore, error) {
	depID, err := a.deploymentID(ro)
	if err != nil {
		return applicator.HealthScore{}, err
	}
	h, err := a.health.GetHealth(depID)
	if err != nil {
		return applicator.HealthScore{}, err
	}
	return applicator.HealthScore{
		Score: h.Overall,
	}, nil
}
