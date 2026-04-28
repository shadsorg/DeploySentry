package rollout

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/shadsorg/deploysentry/internal/entities"
	"github.com/shadsorg/deploysentry/internal/models"
	"github.com/google/uuid"
)

// DeploymentLookup is the subset of deploy.DeployService used during
// rollout-list enrichment.
type DeploymentLookup interface {
	GetDeployment(ctx context.Context, id uuid.UUID) (*models.Deployment, error)
}

// ApplicationLookup fetches the parent app + project for a deployment or flag.
type ApplicationLookup interface {
	GetAppByID(ctx context.Context, id uuid.UUID) (*models.Application, error)
}

// ProjectLookup resolves a project UUID to its slug + name.
type ProjectLookup interface {
	GetProjectByID(ctx context.Context, id uuid.UUID) (*models.Project, error)
}

// EnvironmentLookup resolves an env UUID to its slug + name.
type EnvironmentLookup interface {
	GetByID(ctx context.Context, id uuid.UUID) (*entities.OrgEnvironment, error)
}

// FlagLookup fetches a feature flag + its parent application for a rule-based rollout.
// Method names mirror the existing flag service / repo surface so no wrapper is needed.
type FlagLookup interface {
	GetRule(ctx context.Context, id uuid.UUID) (*models.TargetingRule, error)
	GetFlag(ctx context.Context, id uuid.UUID) (*models.FeatureFlag, error)
}

// Enricher attaches human-readable target summaries to bare Rollout rows.
// Every lookup is best-effort — a missing row (target deleted after the
// rollout was created) leaves fields empty so the UI can render
// "(unknown)".
type Enricher struct {
	deploys DeploymentLookup
	apps    ApplicationLookup
	projects ProjectLookup
	envs    EnvironmentLookup
	flags   FlagLookup
}

// NewEnricher constructs an Enricher. All arguments are optional; any nil
// dep causes the corresponding target type to fall through to a
// minimally-decorated summary.
func NewEnricher(d DeploymentLookup, a ApplicationLookup, p ProjectLookup, e EnvironmentLookup, f FlagLookup) *Enricher {
	return &Enricher{deploys: d, apps: a, projects: p, envs: e, flags: f}
}

// Enrich batches target lookups and returns one RolloutWithTarget per
// input row, in the same order.
func (en *Enricher) Enrich(ctx context.Context, rows []*models.Rollout) []*models.RolloutWithTarget {
	now := time.Now().UTC()
	out := make([]*models.RolloutWithTarget, 0, len(rows))
	for _, r := range rows {
		display := en.targetFor(ctx, r)
		age := int64(now.Sub(r.CreatedAt).Seconds())
		out = append(out, &models.RolloutWithTarget{
			Rollout:    r,
			Target:     display,
			AgeSeconds: age,
		})
	}
	return out
}

func (en *Enricher) targetFor(ctx context.Context, r *models.Rollout) models.RolloutTargetDisplay {
	switch r.TargetType {
	case models.TargetTypeDeploy:
		return en.displayForDeploy(ctx, r)
	case models.TargetTypeConfig:
		return en.displayForConfig(ctx, r)
	}
	return models.RolloutTargetDisplay{Kind: string(r.TargetType), Summary: "(unknown target type)"}
}

func (en *Enricher) displayForDeploy(ctx context.Context, r *models.Rollout) models.RolloutTargetDisplay {
	d := models.RolloutTargetDisplay{Kind: "deploy"}
	if r.TargetRef.DeploymentID == nil {
		d.Summary = "deploy · (missing id)"
		return d
	}
	depID, err := uuid.Parse(*r.TargetRef.DeploymentID)
	if err != nil {
		d.Summary = "deploy · (invalid id)"
		return d
	}
	if en.deploys == nil {
		d.Summary = "deploy · " + shortID(*r.TargetRef.DeploymentID)
		return d
	}
	dep, err := en.deploys.GetDeployment(ctx, depID)
	if err != nil || dep == nil {
		d.Summary = "deploy · " + shortID(*r.TargetRef.DeploymentID) + " (deleted?)"
		return d
	}
	d.Version = dep.Version
	if en.apps != nil {
		if app, aerr := en.apps.GetAppByID(ctx, dep.ApplicationID); aerr == nil && app != nil {
			d.ApplicationSlug = app.Slug
			d.ApplicationName = app.Name
			if en.projects != nil {
				if p, perr := en.projects.GetProjectByID(ctx, app.ProjectID); perr == nil && p != nil {
					d.ProjectSlug = p.Slug
				}
			}
		}
	}
	if en.envs != nil {
		if env, eerr := en.envs.GetByID(ctx, dep.EnvironmentID); eerr == nil && env != nil {
			d.EnvironmentSlug = env.Slug
		}
	}
	d.Summary = composeDeploySummary(d)
	return d
}

func (en *Enricher) displayForConfig(ctx context.Context, r *models.Rollout) models.RolloutTargetDisplay {
	d := models.RolloutTargetDisplay{Kind: "config"}
	if r.TargetRef.FlagKey != nil {
		d.FlagKey = *r.TargetRef.FlagKey
	}
	if r.TargetRef.Env != nil {
		d.EnvironmentSlug = *r.TargetRef.Env
	}
	if r.TargetRef.RuleID != nil && en.flags != nil {
		if rid, err := uuid.Parse(*r.TargetRef.RuleID); err == nil {
			if rule, rerr := en.flags.GetRule(ctx, rid); rerr == nil && rule != nil {
				if flag, ferr := en.flags.GetFlag(ctx, rule.FlagID); ferr == nil && flag != nil {
					d.FlagKey = flag.Key
					if en.apps != nil && flag.ApplicationID != nil {
						if app, aerr := en.apps.GetAppByID(ctx, *flag.ApplicationID); aerr == nil && app != nil {
							d.ApplicationSlug = app.Slug
							d.ApplicationName = app.Name
						}
					}
				}
			}
		}
	}
	d.Summary = composeConfigSummary(d, r.TargetRef)
	return d
}

func composeDeploySummary(d models.RolloutTargetDisplay) string {
	parts := []string{}
	if d.ApplicationSlug != "" {
		parts = append(parts, d.ApplicationSlug)
	}
	if d.Version != "" {
		parts = append(parts, shortID(d.Version))
	}
	if d.EnvironmentSlug != "" {
		parts = append(parts, "["+d.EnvironmentSlug+"]")
	}
	if len(parts) == 0 {
		return "deploy · (unknown)"
	}
	return strings.Join(parts, " · ")
}

func composeConfigSummary(d models.RolloutTargetDisplay, ref models.RolloutTargetRef) string {
	parts := []string{}
	if d.ApplicationSlug != "" {
		parts = append(parts, d.ApplicationSlug)
	}
	if d.FlagKey != "" {
		parts = append(parts, "flag "+d.FlagKey)
	}
	if d.EnvironmentSlug != "" {
		parts = append(parts, "["+d.EnvironmentSlug+"]")
	}
	if ref.PreviousPercentage != nil {
		parts = append(parts, fmt.Sprintf("(from %d%%)", *ref.PreviousPercentage))
	}
	if len(parts) == 0 {
		return "config · (unknown)"
	}
	return strings.Join(parts, " · ")
}

func shortID(s string) string {
	if len(s) <= 8 {
		return s
	}
	return s[:8]
}
