# CLI Self-Update — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Steps use checkbox syntax.

**Goal:** Add `deploysentry update` so users can check for and install newer CLI releases without leaving the terminal.

**Architecture:** Talks to the GitHub Releases REST API for `shadsorg/DeploySentry`, picks the right asset for the current platform, downloads to a temp file alongside the running binary, verifies with `--version`, and atomically replaces the running executable via `os.Rename`. Falls back gracefully on Windows (out of scope for v1; no asset is built).

**Tech Stack:** Go stdlib + `golang.org/x/mod/semver` for version comparison. No new third-party update libraries — the surface is small enough to own.

---

## File Structure

```
cmd/cli/
├── update.go                  # CREATE — `update` subcommand + check/download/install logic
├── update_test.go             # CREATE — unit tests with httptest mocks
└── root.go                    # MODIFY — register `updateCmd`; expose `version` accessor for tests
```

---

## Task 1: `update --check` subcommand + version comparison

**Files:**
- Create: `cmd/cli/update.go`
- Create: `cmd/cli/update_test.go`
- Modify: `cmd/cli/root.go` (add `rootCmd.AddCommand(updateCmd)`)

### Step 1: Add `golang.org/x/mod` if not already present

```bash
cd /Users/sgamel/git/DeploySentry/.worktrees/cli-self-update
grep "golang.org/x/mod" go.mod
```

If missing, run `go get golang.org/x/mod@latest` to add it.

### Step 2: Implement `cmd/cli/update.go` (just the check path; no download yet)

```go
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/mod/semver"
)

const (
	githubReleasesAPI = "https://api.github.com/repos/shadsorg/DeploySentry/releases/latest"
	assetPrefix       = "deploysentry"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Check for and install a newer CLI version",
	Long: `Check the project's GitHub releases for a newer CLI version.

By default this command checks, prompts, downloads, and replaces the running
binary. Use --check to only check (no download). Use --version to install a
specific version.

Examples:
  # Check whether a newer version is available
  deploysentry update --check

  # Update to the latest version (with prompt)
  deploysentry update

  # Update without prompting
  deploysentry update --yes

  # Pin to a specific version (also useful for rollback)
  deploysentry update --version v0.2.0`,
	RunE: runUpdate,
}

func init() {
	updateCmd.Flags().Bool("check", false, "only check; do not download or install")
	updateCmd.Flags().Bool("yes", false, "skip the confirmation prompt")
	updateCmd.Flags().String("version", "", "install a specific version tag (e.g. v0.2.0)")
}

// gitHubRelease is the trimmed shape we read from the GitHub releases API.
type gitHubRelease struct {
	TagName string                 `json:"tag_name"`
	Name    string                 `json:"name"`
	Assets  []gitHubReleaseAsset   `json:"assets"`
}

type gitHubReleaseAsset struct {
	Name        string `json:"name"`
	DownloadURL string `json:"browser_download_url"`
	Size        int64  `json:"size"`
}

// updateClient is the http.Client used for talking to GitHub. Override in tests.
var updateClient = &http.Client{Timeout: 30 * time.Second}

// updateAPI is the URL of the latest-release endpoint. Override in tests.
var updateAPI = githubReleasesAPI

func runUpdate(cmd *cobra.Command, _ []string) error {
	checkOnly, _ := cmd.Flags().GetBool("check")

	rel, err := fetchLatestRelease(cmd.Context())
	if err != nil {
		return err
	}

	current := normalizeVersion(version)
	latest := normalizeVersion(rel.TagName)

	cmp, err := compareVersions(current, latest)
	if err != nil {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Current version: %s\nLatest:          %s\n", current, latest)
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "(Cannot compare versions; refusing to auto-update from a non-semver build.)")
		return nil
	}

	switch {
	case cmp == 0:
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Already up to date (%s).\n", latest)
		return nil
	case cmp > 0:
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Local version (%s) is newer than the latest release (%s).\n", current, latest)
		return nil
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "A newer version is available: %s (current: %s)\n", latest, current)

	if checkOnly {
		return nil
	}

	// Download + install path lands in Task 2.
	return errors.New("install not yet implemented; use --check for now")
}

// fetchLatestRelease GETs the GitHub releases API and returns the parsed body.
func fetchLatestRelease(ctx interface {
	Done() <-chan struct{}
	Err() error
}) (*gitHubRelease, error) {
	req, err := http.NewRequest("GET", updateAPI, nil)
	if err != nil {
		return nil, fmt.Errorf("update: build request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := updateClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("update: fetch latest release: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("update: github returned %d: %s", resp.StatusCode, string(body))
	}
	var rel gitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, fmt.Errorf("update: decode release JSON: %w", err)
	}
	return &rel, nil
}

// normalizeVersion ensures a leading "v" so semver.Compare works on common
// shapes ("v0.2.0", "0.2.0", "dev"). Pure values are passed through unchanged.
func normalizeVersion(v string) string {
	v = strings.TrimSpace(v)
	if v == "" || v == "dev" {
		return v
	}
	if !strings.HasPrefix(v, "v") {
		return "v" + v
	}
	return v
}

// compareVersions returns -1/0/+1 like strings.Compare. Returns an error if
// either side isn't a valid semver (e.g. "dev").
func compareVersions(a, b string) (int, error) {
	if !semver.IsValid(a) || !semver.IsValid(b) {
		return 0, fmt.Errorf("not semver: %q vs %q", a, b)
	}
	return semver.Compare(a, b), nil
}

// pickAsset returns the asset matching the current GOOS/GOARCH from a
// release. Asset names follow `deploysentry-{linux|darwin}-{amd64|arm64}`.
func pickAsset(rel *gitHubRelease) (*gitHubReleaseAsset, error) {
	want := fmt.Sprintf("%s-%s-%s", assetPrefix, runtime.GOOS, runtime.GOARCH)
	for i, a := range rel.Assets {
		if a.Name == want {
			return &rel.Assets[i], nil
		}
	}
	return nil, fmt.Errorf("no release asset for %s/%s (looked for %q)", runtime.GOOS, runtime.GOARCH, want)
}
```

### Step 3: Register the command in `cmd/cli/root.go`

In the `init()` function (or wherever subcommands are added to `rootCmd`), append:

```go
rootCmd.AddCommand(updateCmd)
```

### Step 4: Write tests

`cmd/cli/update_test.go`:

```go
package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

// fakeReleaseServer returns an httptest server that responds to the
// `/releases/latest` URL with the given release JSON.
func fakeReleaseServer(t *testing.T, rel gitHubRelease) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(rel)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func swapUpdateAPI(t *testing.T, url string) {
	t.Helper()
	prev := updateAPI
	updateAPI = url
	t.Cleanup(func() { updateAPI = prev })
}

func swapVersion(t *testing.T, v string) {
	t.Helper()
	prev := version
	version = v
	t.Cleanup(func() { version = prev })
}

func TestUpdateCheck_NewerAvailable(t *testing.T) {
	srv := fakeReleaseServer(t, gitHubRelease{
		TagName: "v0.2.0",
		Assets: []gitHubReleaseAsset{
			{Name: "deploysentry-darwin-arm64", DownloadURL: "https://example/asset"},
		},
	})
	swapUpdateAPI(t, srv.URL)
	swapVersion(t, "v0.1.0")

	stdout, _, err := runCmd(t, rootCmd, "update", "--check")
	require.NoError(t, err)
	require.Contains(t, stdout, "newer version is available")
	require.Contains(t, stdout, "v0.2.0")
}

func TestUpdateCheck_AlreadyUpToDate(t *testing.T) {
	srv := fakeReleaseServer(t, gitHubRelease{TagName: "v0.2.0"})
	swapUpdateAPI(t, srv.URL)
	swapVersion(t, "v0.2.0")

	stdout, _, err := runCmd(t, rootCmd, "update", "--check")
	require.NoError(t, err)
	require.Contains(t, stdout, "Already up to date")
}

func TestUpdateCheck_LocalIsNewer(t *testing.T) {
	srv := fakeReleaseServer(t, gitHubRelease{TagName: "v0.1.0"})
	swapUpdateAPI(t, srv.URL)
	swapVersion(t, "v0.5.0")

	stdout, _, err := runCmd(t, rootCmd, "update", "--check")
	require.NoError(t, err)
	require.Contains(t, stdout, "newer than the latest release")
}

func TestUpdateCheck_DevBuildAcceptsCheck(t *testing.T) {
	srv := fakeReleaseServer(t, gitHubRelease{TagName: "v0.2.0"})
	swapUpdateAPI(t, srv.URL)
	swapVersion(t, "dev")

	stdout, _, err := runCmd(t, rootCmd, "update", "--check")
	require.NoError(t, err)
	require.Contains(t, stdout, "non-semver build")
}

func TestUpdateCheck_NetworkError(t *testing.T) {
	swapUpdateAPI(t, "http://127.0.0.1:1") // closed port
	swapVersion(t, "v0.1.0")

	_, _, err := runCmd(t, rootCmd, "update", "--check")
	require.Error(t, err)
}

func TestPickAsset_PicksCurrentPlatform(t *testing.T) {
	rel := gitHubRelease{Assets: []gitHubReleaseAsset{
		{Name: "deploysentry-linux-amd64", DownloadURL: "https://x/linux"},
		{Name: "deploysentry-linux-arm64", DownloadURL: "https://x/linuxarm"},
		{Name: "deploysentry-darwin-amd64", DownloadURL: "https://x/macintel"},
		{Name: "deploysentry-darwin-arm64", DownloadURL: "https://x/macarm"},
	}}
	asset, err := pickAsset(&rel)
	require.NoError(t, err)
	require.Equal(t, "deploysentry-"+runtime.GOOS+"-"+runtime.GOARCH, asset.Name)
}

func TestPickAsset_NoMatchingAsset(t *testing.T) {
	rel := gitHubRelease{Assets: []gitHubReleaseAsset{
		{Name: "deploysentry-windows-amd64"},
	}}
	_, err := pickAsset(&rel)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no release asset for")
}
```

### Step 5: Run tests + verify

```bash
go test ./cmd/cli/ -run TestUpdate -v -count=1
go test ./cmd/cli/ -run TestPickAsset -v -count=1
go test ./cmd/cli/ -v -count=1
go vet ./cmd/cli/...
go build ./...
```

Expected: all pass; full suite gains 6 new tests (was 75, now 81).

### Step 6: Commit

```bash
git add cmd/cli/update.go cmd/cli/update_test.go cmd/cli/root.go go.mod go.sum
git commit -m "feat(cli): add 'update --check' that compares local vs latest GitHub release"
```

---

## Task 2: Download + atomic replace

**Files:**
- Modify: `cmd/cli/update.go` — replace the placeholder error with real install logic
- Modify: `cmd/cli/update_test.go` — add download/install tests

### Step 1: Add the install logic to `update.go`

Replace the `errors.New("install not yet implemented...")` line in `runUpdate` with a real install path. Add three new functions:

```go
// installRelease downloads the asset, verifies it runs, and atomically
// replaces the running binary.
func installRelease(cmd *cobra.Command, asset *gitHubReleaseAsset) error {
	stdout := cmd.OutOrStdout()

	// Resolve the running binary's path.
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("update: locate running binary: %w", err)
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return fmt.Errorf("update: resolve binary symlink: %w", err)
	}

	// Download the asset to a temp file in the SAME directory as the running
	// binary, so os.Rename below is atomic.
	dir := filepath.Dir(exe)
	tmp, err := os.CreateTemp(dir, ".deploysentry-update-*")
	if err != nil {
		return fmt.Errorf("update: create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
	}

	_, _ = fmt.Fprintf(stdout, "Downloading %s (%d bytes)…\n", asset.Name, asset.Size)
	if err := downloadAsset(asset.DownloadURL, tmp); err != nil {
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("update: close temp file: %w", err)
	}
	if err := os.Chmod(tmpPath, 0o755); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("update: chmod temp file: %w", err)
	}

	// Smoke-test the new binary with --version. This catches a corrupted
	// download or a totally wrong asset before we replace the running binary.
	out, err := exec.Command(tmpPath, "--version").CombinedOutput()
	if err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("update: downloaded binary failed --version: %w (output: %s)", err, string(out))
	}

	// Atomic replace. On Unix this works even while the binary is executing.
	if err := os.Rename(tmpPath, exe); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("update: replace running binary: %w", err)
	}

	_, _ = fmt.Fprintf(stdout, "Updated to %s. Re-run `deploysentry --version` to confirm.\n", asset.Name)
	return nil
}

// downloadAsset streams the asset bytes into the given writer.
var downloadAssetFunc = func(url string, w io.Writer) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("update: build download request: %w", err)
	}
	resp, err := updateClient.Do(req)
	if err != nil {
		return fmt.Errorf("update: download asset: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("update: download asset: status %d", resp.StatusCode)
	}
	if _, err := io.Copy(w, resp.Body); err != nil {
		return fmt.Errorf("update: copy asset bytes: %w", err)
	}
	return nil
}

func downloadAsset(url string, w io.Writer) error { return downloadAssetFunc(url, w) }

// confirmInstall prompts the user. Returns true if they accept. Skipped if --yes.
func confirmInstall(cmd *cobra.Command, latest string) (bool, error) {
	if yes, _ := cmd.Flags().GetBool("yes"); yes {
		return true, nil
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Install %s? [y/N] ", latest)
	var answer string
	if _, err := fmt.Fscanln(cmd.InOrStdin(), &answer); err != nil {
		return false, nil // EOF / no input → treat as "no"
	}
	answer = strings.TrimSpace(strings.ToLower(answer))
	return answer == "y" || answer == "yes", nil
}
```

Add these imports at the top of `update.go`: `os`, `os/exec`, `path/filepath`.

Replace the placeholder error line in `runUpdate` with:

```go
asset, err := pickAsset(rel)
if err != nil {
	return err
}

ok, err := confirmInstall(cmd, latest)
if err != nil {
	return err
}
if !ok {
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Cancelled.")
	return nil
}

return installRelease(cmd, asset)
```

### Step 2: Handle `--version <tag>`

If `--version vX.Y.Z` is provided, fetch THAT release instead of `/latest`. Add a helper:

```go
func fetchReleaseByTag(tag string) (*gitHubRelease, error) {
	url := strings.Replace(updateAPI, "/releases/latest", "/releases/tags/"+tag, 1)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("update: build request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := updateClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("update: fetch release %q: %w", tag, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("update: release %q not found", tag)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("update: github returned %d: %s", resp.StatusCode, string(body))
	}
	var rel gitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, fmt.Errorf("update: decode release JSON: %w", err)
	}
	return &rel, nil
}
```

In `runUpdate`, before the `fetchLatestRelease` call:

```go
explicitVersion, _ := cmd.Flags().GetString("version")

var rel *gitHubRelease
if explicitVersion != "" {
	rel, err = fetchReleaseByTag(normalizeVersion(explicitVersion))
} else {
	rel, err = fetchLatestRelease(cmd.Context())
}
if err != nil {
	return err
}
```

When `--version` is set, skip the "compare current vs latest" output (just always install).

### Step 3: Add tests

Append to `update_test.go`:

```go
func TestUpdateInstall_FullFlow(t *testing.T) {
	// Stub the GitHub /releases/latest response.
	relSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(gitHubRelease{
			TagName: "v0.2.0",
			Assets: []gitHubReleaseAsset{
				{
					Name:        "deploysentry-" + runtime.GOOS + "-" + runtime.GOARCH,
					DownloadURL: "/asset",
					Size:        100,
				},
			},
		})
	}))
	t.Cleanup(relSrv.Close)
	swapUpdateAPI(t, relSrv.URL)
	swapVersion(t, "v0.1.0")

	// Stub the asset download to return a tiny shell script that prints
	// "v0.2.0" so the smoke-check can run it.
	prevDL := downloadAssetFunc
	downloadAssetFunc = func(url string, w io.Writer) error {
		// Write a real exec'able payload: a shell script.
		_, err := io.WriteString(w, "#!/bin/sh\necho v0.2.0\n")
		return err
	}
	t.Cleanup(func() { downloadAssetFunc = prevDL })

	// Run from a temp dir so we don't replace the test binary itself.
	// Override os.Executable via a small seam: we can't easily intercept it,
	// so we instead rely on the test running on a Unix host and accept that
	// the test will skip on Windows (which we don't ship for anyway).
	if runtime.GOOS == "windows" {
		t.Skip("self-update install not supported on Windows in v1")
	}

	// Create a fake "current binary" in a temp dir.
	tmpDir := t.TempDir()
	fakeBinary := filepath.Join(tmpDir, "deploysentry-fake")
	require.NoError(t, os.WriteFile(fakeBinary, []byte("#!/bin/sh\necho v0.1.0\n"), 0o755))

	// Override os.Executable for the test.
	prevExe := osExecutable
	osExecutable = func() (string, error) { return fakeBinary, nil }
	t.Cleanup(func() { osExecutable = prevExe })

	stdout, _, err := runCmd(t, rootCmd, "update", "--yes")
	require.NoError(t, err)
	require.Contains(t, stdout, "Updated to")

	// Verify the binary was replaced by reading its first line.
	data, err := os.ReadFile(fakeBinary)
	require.NoError(t, err)
	require.Contains(t, string(data), "v0.2.0")
}

func TestUpdate_PinnedVersion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Contains(t, r.URL.Path, "/releases/tags/v0.1.5")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(gitHubRelease{
			TagName: "v0.1.5",
			Assets: []gitHubReleaseAsset{
				{Name: "deploysentry-" + runtime.GOOS + "-" + runtime.GOARCH, DownloadURL: "/asset", Size: 100},
			},
		})
	}))
	t.Cleanup(srv.Close)
	swapUpdateAPI(t, srv.URL+"/repos/shadsorg/DeploySentry/releases/latest")
	swapVersion(t, "v0.5.0")

	prevDL := downloadAssetFunc
	downloadAssetFunc = func(url string, w io.Writer) error {
		_, err := io.WriteString(w, "#!/bin/sh\necho v0.1.5\n")
		return err
	}
	t.Cleanup(func() { downloadAssetFunc = prevDL })

	if runtime.GOOS == "windows" {
		t.Skip("self-update install not supported on Windows in v1")
	}

	tmpDir := t.TempDir()
	fakeBinary := filepath.Join(tmpDir, "deploysentry-fake")
	require.NoError(t, os.WriteFile(fakeBinary, []byte("#!/bin/sh\necho v0.5.0\n"), 0o755))
	prevExe := osExecutable
	osExecutable = func() (string, error) { return fakeBinary, nil }
	t.Cleanup(func() { osExecutable = prevExe })

	stdout, _, err := runCmd(t, rootCmd, "update", "--yes", "--version", "v0.1.5")
	require.NoError(t, err)
	require.Contains(t, stdout, "Updated to")
}

func TestUpdate_NoMatchingAsset(t *testing.T) {
	srv := fakeReleaseServer(t, gitHubRelease{
		TagName: "v0.2.0",
		Assets: []gitHubReleaseAsset{
			{Name: "deploysentry-some-other-platform", DownloadURL: "/x"},
		},
	})
	swapUpdateAPI(t, srv.URL)
	swapVersion(t, "v0.1.0")

	_, _, err := runCmd(t, rootCmd, "update", "--yes")
	require.Error(t, err)
	require.Contains(t, err.Error(), "no release asset for")
}
```

### Step 4: Add the `osExecutable` test seam in `update.go`

Replace `os.Executable()` calls with a package-level var:

```go
var osExecutable = os.Executable
```

…and use `osExecutable()` everywhere instead of `os.Executable()` directly. This lets tests override it.

### Step 5: Run + verify

```bash
go test ./cmd/cli/ -run TestUpdate -v -count=1
go test ./cmd/cli/ -v -count=1
go vet ./cmd/cli/...
go build ./...
```

Expected: 84 tests total (was 81 after Task 1, +3 new). All pass on macOS/Linux; the 2 new install tests skip on Windows.

### Step 6: Commit

```bash
git add cmd/cli/update.go cmd/cli/update_test.go
git commit -m "feat(cli): finish 'update' with download + atomic replace + smoke check"
```

---

## Task 3: Initiatives + push + PR

### Step 1: Update `docs/Current_Initiatives.md`

Add a new row near the bottom:

```
| CLI Self-Update | Implementation | [Plan](./superpowers/plans/2026-04-27-cli-self-update.md) | New `deploysentry update` subcommand: checks GitHub releases, picks the right asset for GOOS/GOARCH, downloads to a temp file in the binary's directory, smoke-tests via `--version`, atomically replaces the running binary via `os.Rename`. Supports `--check`, `--yes`, `--version <tag>` for pinning/rollback. Unix-only in v1 (no Windows asset shipped anyway). 9 new tests. |
```

Bump `> Last updated:` to `> Last updated: 2026-04-27`.

### Step 2: Final verification

```bash
go vet ./cmd/cli/...
go test ./cmd/cli/ -v -count=1
go build ./...
```

Expected: 84 tests pass, vet clean, build clean.

### Step 3: Commit + push

```bash
git add docs/Current_Initiatives.md
git commit -m "docs: track CLI self-update initiative"
git push -u origin feature/cli-self-update
```

---

## Success criteria

- `deploysentry update --check` prints "newer version available" / "up to date" / "local newer than latest" correctly.
- `deploysentry update` (and `--yes`) downloads the matching asset, smoke-tests it, and replaces the running binary on macOS + Linux.
- `deploysentry update --version v0.1.5` pins to a specific release.
- 9 new tests; full suite 84 passing.
- Windows is documented as out of scope for v1; no test failures from it.

## Out of scope

- Auto-check at startup (deferred — adds telemetry-shaped concerns).
- Checksum verification (deferred — GitHub releases don't include a manifest yet).
- Windows install path (no asset built; `os.Rename` of a running .exe is not atomic on Windows).
- Rollback to "previous version" sugar (`update --rollback`) — `update --version <tag>` covers this manually.
- Background self-update without prompting via cron / systemd hooks.
