package deploysentry

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestResolveVersion_ExplicitWins(t *testing.T) {
	os.Setenv("APP_VERSION", "from-env")
	defer os.Unsetenv("APP_VERSION")

	if v := resolveVersion("1.2.3"); v != "1.2.3" {
		t.Fatalf("resolveVersion override = %q; want 1.2.3", v)
	}
}

func TestResolveVersion_EnvVarFallback(t *testing.T) {
	for _, k := range versionEnvChain {
		os.Unsetenv(k)
	}
	os.Setenv("GIT_SHA", "abc123")
	defer os.Unsetenv("GIT_SHA")

	if v := resolveVersion(""); v != "abc123" {
		t.Fatalf("resolveVersion env = %q; want abc123", v)
	}
}

func TestResolveVersion_UnknownFallback(t *testing.T) {
	for _, k := range versionEnvChain {
		os.Unsetenv(k)
	}
	v := resolveVersion("")
	// build info may or may not populate; accept either "unknown" or a real semver.
	if v == "" {
		t.Fatalf("resolveVersion returned empty")
	}
}

func TestReportOnce_PostsToCorrectURL(t *testing.T) {
	var captured struct {
		path    string
		method  string
		auth    string
		body    map[string]interface{}
		contentType string
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured.path = r.URL.Path
		captured.method = r.Method
		captured.auth = r.Header.Get("Authorization")
		captured.contentType = r.Header.Get("Content-Type")
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &captured.body)
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client := NewClient(
		WithAPIKey("test-key"),
		WithBaseURL(server.URL),
		WithApplicationID("f47ac10b-58cc-4372-a567-0e02b2c3d479"),
		WithReportStatus(true),
		WithReportStatusVersion("1.4.2"),
		WithReportStatusCommitSHA("abc123"),
		WithReportStatusDeploySlot("canary"),
		WithReportStatusTags(map[string]string{"region": "us-east"}),
	)
	reporter := newStatusReporter(client)
	if err := reporter.reportOnce(context.Background()); err != nil {
		t.Fatalf("reportOnce: %v", err)
	}

	if captured.method != http.MethodPost {
		t.Errorf("method = %q; want POST", captured.method)
	}
	wantPath := "/api/v1/applications/f47ac10b-58cc-4372-a567-0e02b2c3d479/status"
	if captured.path != wantPath {
		t.Errorf("path = %q; want %q", captured.path, wantPath)
	}
	if !strings.HasPrefix(captured.auth, "ApiKey ") {
		t.Errorf("auth header = %q; want ApiKey-prefixed", captured.auth)
	}
	if captured.contentType != "application/json" {
		t.Errorf("content-type = %q", captured.contentType)
	}
	if captured.body["version"] != "1.4.2" {
		t.Errorf("version = %v; want 1.4.2", captured.body["version"])
	}
	if captured.body["health"] != "healthy" {
		t.Errorf("health = %v; want healthy", captured.body["health"])
	}
	if captured.body["commit_sha"] != "abc123" {
		t.Errorf("commit_sha = %v", captured.body["commit_sha"])
	}
	if captured.body["deploy_slot"] != "canary" {
		t.Errorf("deploy_slot = %v", captured.body["deploy_slot"])
	}
	tags, ok := captured.body["tags"].(map[string]interface{})
	if !ok || tags["region"] != "us-east" {
		t.Errorf("tags = %v", captured.body["tags"])
	}
}

func TestReportOnce_HealthProviderIsCalled(t *testing.T) {
	var got map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &got)
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	score := 0.8
	client := NewClient(
		WithAPIKey("k"),
		WithBaseURL(server.URL),
		WithApplicationID("app"),
		WithReportStatus(true),
		WithReportStatusVersion("1"),
		WithHealthProvider(func() (HealthReport, error) {
			return HealthReport{State: "degraded", Score: &score, Reason: "db slow"}, nil
		}),
	)
	if err := newStatusReporter(client).reportOnce(context.Background()); err != nil {
		t.Fatalf("reportOnce: %v", err)
	}
	if got["health"] != "degraded" {
		t.Errorf("health = %v; want degraded", got["health"])
	}
	if got["health_score"] != 0.8 {
		t.Errorf("health_score = %v", got["health_score"])
	}
	if got["health_reason"] != "db slow" {
		t.Errorf("health_reason = %v", got["health_reason"])
	}
}

func TestReportOnce_ReturnsErrorOnServerFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(
		WithAPIKey("k"),
		WithBaseURL(server.URL),
		WithApplicationID("app"),
		WithReportStatusVersion("1"),
	)
	err := newStatusReporter(client).reportOnce(context.Background())
	if err == nil {
		t.Fatalf("expected error on 500")
	}
}

func TestReportOnce_HealthProviderErrorSurfacesAsUnknown(t *testing.T) {
	var got map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &got)
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client := NewClient(
		WithAPIKey("k"),
		WithBaseURL(server.URL),
		WithApplicationID("app"),
		WithReportStatusVersion("1"),
		WithHealthProvider(func() (HealthReport, error) {
			return HealthReport{}, errAssertTest("boom")
		}),
	)
	if err := newStatusReporter(client).reportOnce(context.Background()); err != nil {
		t.Fatalf("reportOnce: %v", err)
	}
	if got["health"] != "unknown" {
		t.Errorf("health = %v; want unknown", got["health"])
	}
}

type errAssertTest string

func (e errAssertTest) Error() string { return string(e) }

func TestStatusReporter_StartupOnlyMode(t *testing.T) {
	hits := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client := NewClient(
		WithAPIKey("k"),
		WithBaseURL(server.URL),
		WithApplicationID("app"),
		WithReportStatus(true),
		WithReportStatusInterval(0),
		WithReportStatusVersion("1"),
	)
	reporter := newStatusReporter(client)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		reporter.run(ctx)
		close(done)
	}()

	// startup-only run should complete nearly immediately.
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("reporter did not exit in startup-only mode")
	}
	cancel()
	if hits != 1 {
		t.Errorf("hits = %d; want 1", hits)
	}
}
