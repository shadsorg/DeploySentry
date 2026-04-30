package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// recordedRequest captures one inbound HTTP call so tests can assert on shape.
type recordedRequest struct {
	Method string
	Path   string         // includes query string
	Body   map[string]any // parsed JSON body (nil for GET / empty body)
}

// mockServer is a tiny httptest wrapper that records every request and
// dispatches based on (METHOD, path-prefix) to a stub function returning
// (status, body).
type mockServer struct {
	t        *testing.T
	srv      *httptest.Server
	requests []recordedRequest
	routes   []routeStub
}

type routeStub struct {
	Method  string
	Match   func(path string) bool
	Respond func(req recordedRequest) (status int, body any)
}

func newMockServer(t *testing.T) *mockServer {
	t.Helper()
	m := &mockServer{t: t}
	m.srv = httptest.NewServer(http.HandlerFunc(m.handle))
	t.Cleanup(m.srv.Close)
	return m
}

func (m *mockServer) URL() string { return m.srv.URL }

func (m *mockServer) on(method string, match func(string) bool, respond func(recordedRequest) (int, any)) {
	m.routes = append(m.routes, routeStub{Method: method, Match: match, Respond: respond})
}

// onPathFunc stubs an exact-method, prefix-matched path and lets the test
// inspect the request and return a dynamic response (e.g., echo back posted
// fields). `pathPrefix` is matched against URL.Path *before* the query string.
func (m *mockServer) onPathFunc(method, pathPrefix string, fn func(recordedRequest) (int, any)) {
	m.on(method, func(p string) bool { return strings.HasPrefix(p, pathPrefix) }, fn)
}

func (m *mockServer) handle(w http.ResponseWriter, r *http.Request) {
	rec := recordedRequest{Method: r.Method, Path: r.URL.RequestURI()}
	if r.Body != nil {
		raw, _ := io.ReadAll(r.Body)
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &rec.Body)
		}
	}
	m.requests = append(m.requests, rec)

	for _, route := range m.routes {
		if route.Method != r.Method {
			continue
		}
		if !route.Match(r.URL.Path) {
			continue
		}
		status, body := route.Respond(rec)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(body)
		return
	}
	// Unmatched: return 599 so the test fails loudly with a descriptive body.
	m.t.Errorf("unexpected request %s %s", r.Method, r.URL.RequestURI())
	w.WriteHeader(599)
	_, _ = w.Write([]byte(`{"error":"unstubbed route"}`))
}

// setTestConfig points the CLI's viper config at the mock server. Call this
// from each test before invoking a command.
//
// Note: keys are `api_url` and `api_key` (underscored) — these are what
// clientFromConfig reads in deploy.go. ensureValidToken() will fail in tests
// (no credentials file) so the api_key fallback path is what authenticates.
func setTestConfig(t *testing.T, baseURL, apiKey, org, project, env string) {
	t.Helper()
	viper.Reset()
	viper.Set("api_url", baseURL)
	viper.Set("api_key", apiKey)
	viper.Set("org", org)
	viper.Set("project", project)
	if env != "" {
		viper.Set("env", env)
	}
	t.Cleanup(viper.Reset)
}

// runCmd executes a cobra command tree with the given argv.  Returns
// captured stdout, stderr, and the command's RunE error.
func runCmd(t *testing.T, root *cobra.Command, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	var outBuf, errBuf bytes.Buffer
	root.SetOut(&outBuf)
	root.SetErr(&errBuf)
	root.SetArgs(args)
	err = root.Execute()
	return outBuf.String(), errBuf.String(), err
}
