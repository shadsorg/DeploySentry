package deploysentry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"runtime/debug"
	"time"
)

// HealthReport is returned by a user-supplied health provider.
type HealthReport struct {
	State  string   `json:"state"` // healthy | degraded | unhealthy | unknown
	Score  *float64 `json:"score,omitempty"`
	Reason string   `json:"reason,omitempty"`
}

// versionEnvChain is probed in order; first non-empty wins.
var versionEnvChain = []string{
	"APP_VERSION",
	"GIT_SHA",
	"GIT_COMMIT",
	"SOURCE_COMMIT",
	"RAILWAY_GIT_COMMIT_SHA",
	"RENDER_GIT_COMMIT",
	"VERCEL_GIT_COMMIT_SHA",
	"HEROKU_SLUG_COMMIT",
}

const (
	defaultStatusInterval = 30 * time.Second
	statusMinBackoff      = 1 * time.Second
	statusMaxBackoff      = 5 * time.Minute
)

// resolveVersion picks the reported version from: explicit → env vars →
// build info → "unknown". Exposed for testing.
func resolveVersion(explicit string) string {
	if explicit != "" {
		return explicit
	}
	for _, name := range versionEnvChain {
		if v := os.Getenv(name); v != "" {
			return v
		}
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "unknown"
}

// statusReporter is owned by Client and fires periodic /status POSTs.
type statusReporter struct {
	client *Client
}

func newStatusReporter(c *Client) *statusReporter {
	return &statusReporter{client: c}
}

// run blocks until ctx is cancelled, firing reports on the configured cadence.
func (r *statusReporter) run(ctx context.Context) {
	// Initial report on start.
	r.report(ctx)

	interval := r.client.statusInterval
	if interval < 0 {
		interval = 0
	}
	if interval == 0 && r.client.statusInterval == 0 {
		// startup-only mode
		return
	}
	if interval == 0 {
		interval = defaultStatusInterval
	}

	backoff := time.Duration(0)
	timer := time.NewTimer(interval)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			if err := r.reportOnce(ctx); err != nil {
				r.client.logger.Printf("deploysentry: status report error: %v", err)
				if backoff == 0 {
					backoff = statusMinBackoff
				} else {
					backoff *= 2
					if backoff > statusMaxBackoff {
						backoff = statusMaxBackoff
					}
				}
				timer.Reset(backoff)
			} else {
				backoff = 0
				timer.Reset(interval)
			}
		}
	}
}

// report is the best-effort wrapper used during run() for non-test paths.
func (r *statusReporter) report(ctx context.Context) {
	if err := r.reportOnce(ctx); err != nil {
		r.client.logger.Printf("deploysentry: status report error: %v", err)
	}
}

// reportOnce sends exactly one POST. Exposed for tests and explicit triggers.
func (r *statusReporter) reportOnce(ctx context.Context) error {
	version := resolveVersion(r.client.statusVersion)
	health := HealthReport{State: "healthy"}
	if r.client.healthProvider != nil {
		got, err := r.client.healthProvider()
		if err != nil {
			health = HealthReport{State: "unknown", Reason: err.Error()}
		} else {
			health = got
		}
	}

	body := map[string]interface{}{
		"version": version,
		"health":  health.State,
	}
	if health.Score != nil {
		body["health_score"] = *health.Score
	}
	if health.Reason != "" {
		body["health_reason"] = health.Reason
	}
	if r.client.statusCommitSHA != "" {
		body["commit_sha"] = r.client.statusCommitSHA
	}
	if r.client.statusDeploySlot != "" {
		body["deploy_slot"] = r.client.statusDeploySlot
	}
	if len(r.client.statusTags) > 0 {
		body["tags"] = r.client.statusTags
	}

	buf, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal status payload: %w", err)
	}

	endpoint := fmt.Sprintf("%s/api/v1/applications/%s/status", r.client.baseURL, url.PathEscape(r.client.applicationID))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(buf))
	if err != nil {
		return fmt.Errorf("build status request: %w", err)
	}
	r.client.setAuthHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.client.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("post status: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("status report returned %s", resp.Status)
	}
	return nil
}
