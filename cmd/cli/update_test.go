package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

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
