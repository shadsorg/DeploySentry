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

	rel, err := fetchLatestRelease()
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
	return errors.New("install not yet implemented; use --check for now")
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
