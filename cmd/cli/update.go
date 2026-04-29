package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
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
	rootCmd.AddCommand(updateCmd)
}

type gitHubRelease struct {
	TagName string               `json:"tag_name"`
	Name    string               `json:"name"`
	Assets  []gitHubReleaseAsset `json:"assets"`
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
	explicitVersion, _ := cmd.Flags().GetString("version")

	var rel *gitHubRelease
	var err error
	if explicitVersion != "" {
		rel, err = fetchReleaseByTag(normalizeVersion(explicitVersion))
	} else {
		rel, err = fetchLatestRelease()
	}
	if err != nil {
		return err
	}

	current := normalizeVersion(version)
	latest := normalizeVersion(rel.TagName)

	// Skip the version-compare output when the user pinned a specific version.
	// Just install whatever they asked for.
	if explicitVersion == "" {
		cmp, cmpErr := compareVersions(current, latest)
		if cmpErr != nil {
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
	}

	if checkOnly {
		return nil
	}

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
}

func fetchLatestRelease() (*gitHubRelease, error) {
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

func compareVersions(a, b string) (int, error) {
	if !semver.IsValid(a) || !semver.IsValid(b) {
		return 0, fmt.Errorf("not semver: %q vs %q", a, b)
	}
	return semver.Compare(a, b), nil
}

func pickAsset(rel *gitHubRelease) (*gitHubReleaseAsset, error) {
	want := fmt.Sprintf("%s-%s-%s", assetPrefix, runtime.GOOS, runtime.GOARCH)
	for i, a := range rel.Assets {
		if a.Name == want {
			return &rel.Assets[i], nil
		}
	}
	return nil, fmt.Errorf("no release asset for %s/%s (looked for %q)", runtime.GOOS, runtime.GOARCH, want)
}

// downloadAssetFunc is the seam tests use to provide a deterministic asset
// payload without hitting the wider network.
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

// osExecutable is the seam tests use to inject a fake "current binary" path.
var osExecutable = os.Executable

// fetchReleaseByTag returns the release for a specific tag (used by --version).
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

// installRelease downloads the asset, verifies it runs, and atomically
// replaces the running binary.
func installRelease(cmd *cobra.Command, asset *gitHubReleaseAsset) error {
	stdout := cmd.OutOrStdout()

	exe, err := osExecutable()
	if err != nil {
		return fmt.Errorf("update: locate running binary: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}

	dir := filepath.Dir(exe)
	tmp, err := os.CreateTemp(dir, ".deploysentry-update-*")
	if err != nil {
		return fmt.Errorf("update: create temp file: %w", err)
	}
	tmpPath := tmp.Name()

	_, _ = fmt.Fprintf(stdout, "Downloading %s (%d bytes)…\n", asset.Name, asset.Size)
	if err := downloadAssetFunc(asset.DownloadURL, tmp); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
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

	// Smoke-test: run the new binary with --version. If it fails, the
	// download is corrupted or wrong and we abort before overwriting.
	out, err := exec.Command(tmpPath, "--version").CombinedOutput()
	if err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("update: downloaded binary failed --version: %w (output: %s)", err, string(out))
	}

	if err := os.Rename(tmpPath, exe); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("update: replace running binary: %w", err)
	}

	_, _ = fmt.Fprintf(stdout, "Updated to %s. Re-run `deploysentry --version` to confirm.\n", asset.Name)
	return nil
}
