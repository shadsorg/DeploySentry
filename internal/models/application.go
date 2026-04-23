package models

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

// MonitoringLinkMaxCount is the per-application cap on the number of
// configurable monitoring links.
const MonitoringLinkMaxCount = 10

// MonitoringLinkMaxLabelLen is the per-link label length cap (characters).
const MonitoringLinkMaxLabelLen = 60

// AllowedMonitoringIcons is the curated icon set the dashboard renders. An
// entry with `icon` outside this set (or empty) falls back to a favicon or
// plain text label.
var AllowedMonitoringIcons = []string{
	"github", "datadog", "newrelic", "grafana", "pagerduty", "sentry",
	"slack", "loki", "prometheus", "cloudwatch", "custom",
}

// MonitoringLink is one configurable "jump to my observability / runbook /
// tool" link attached to an application.
type MonitoringLink struct {
	Label string `json:"label"`
	URL   string `json:"url"`
	Icon  string `json:"icon,omitempty"`
}

// Application represents a deployable application within a project.
type Application struct {
	ID              uuid.UUID        `json:"id"`
	ProjectID       uuid.UUID        `json:"project_id"`
	Name            string           `json:"name"`
	Slug            string           `json:"slug"`
	Description     string           `json:"description,omitempty"`
	RepoURL         string           `json:"repo_url,omitempty"`
	MonitoringLinks []MonitoringLink `json:"monitoring_links"`
	CreatedBy       *uuid.UUID       `json:"created_by,omitempty"`
	CreatedAt       time.Time        `json:"created_at"`
	UpdatedAt       time.Time        `json:"updated_at"`
	DeletedAt       *time.Time       `json:"deleted_at,omitempty"`
}

// Validate checks that the Application has all required fields populated.
func (a *Application) Validate() error {
	if a.ProjectID == uuid.Nil {
		return errors.New("project_id is required")
	}
	if a.Name == "" {
		return errors.New("name is required")
	}
	if a.Slug == "" {
		return errors.New("slug is required")
	}
	return nil
}

// ValidateMonitoringLinks enforces count + per-link shape rules. Returns
// the trimmed/normalized slice on success.
func ValidateMonitoringLinks(links []MonitoringLink) ([]MonitoringLink, error) {
	if len(links) > MonitoringLinkMaxCount {
		return nil, fmt.Errorf("at most %d monitoring links per application (got %d)", MonitoringLinkMaxCount, len(links))
	}
	out := make([]MonitoringLink, 0, len(links))
	for i, l := range links {
		label := strings.TrimSpace(l.Label)
		if label == "" {
			return nil, fmt.Errorf("monitoring_links[%d]: label is required", i)
		}
		if len(label) > MonitoringLinkMaxLabelLen {
			return nil, fmt.Errorf("monitoring_links[%d]: label exceeds %d chars", i, MonitoringLinkMaxLabelLen)
		}
		raw := strings.TrimSpace(l.URL)
		if raw == "" {
			return nil, fmt.Errorf("monitoring_links[%d]: url is required", i)
		}
		u, err := url.Parse(raw)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return nil, fmt.Errorf("monitoring_links[%d]: url must be http(s) with a host", i)
		}
		icon := strings.TrimSpace(l.Icon)
		if icon != "" && !iconAllowed(icon) {
			return nil, fmt.Errorf("monitoring_links[%d]: icon %q is not in the allowed set", i, icon)
		}
		out = append(out, MonitoringLink{Label: label, URL: raw, Icon: icon})
	}
	return out, nil
}

func iconAllowed(icon string) bool {
	for _, a := range AllowedMonitoringIcons {
		if a == icon {
			return true
		}
	}
	return false
}
