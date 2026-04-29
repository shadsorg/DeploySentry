package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func resetUpdateFlags(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		for _, name := range []string{"check", "yes", "version"} {
			if f := updateCmd.Flags().Lookup(name); f != nil {
				f.Changed = false
				_ = f.Value.Set(f.DefValue)
			}
		}
	})
}

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
	resetUpdateFlags(t)
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
	resetUpdateFlags(t)
	srv := fakeReleaseServer(t, gitHubRelease{TagName: "v0.2.0"})
	swapUpdateAPI(t, srv.URL)
	swapVersion(t, "v0.2.0")

	stdout, _, err := runCmd(t, rootCmd, "update", "--check")
	require.NoError(t, err)
	require.Contains(t, stdout, "Already up to date")
}

func TestUpdateCheck_LocalIsNewer(t *testing.T) {
	resetUpdateFlags(t)
	srv := fakeReleaseServer(t, gitHubRelease{TagName: "v0.1.0"})
	swapUpdateAPI(t, srv.URL)
	swapVersion(t, "v0.5.0")

	stdout, _, err := runCmd(t, rootCmd, "update", "--check")
	require.NoError(t, err)
	require.Contains(t, stdout, "newer than the latest release")
}

func TestUpdateCheck_DevBuildAcceptsCheck(t *testing.T) {
	resetUpdateFlags(t)
	srv := fakeReleaseServer(t, gitHubRelease{TagName: "v0.2.0"})
	swapUpdateAPI(t, srv.URL)
	swapVersion(t, "dev")

	stdout, _, err := runCmd(t, rootCmd, "update", "--check")
	require.NoError(t, err)
	require.Contains(t, stdout, "non-semver build")
}

func TestUpdateCheck_NetworkError(t *testing.T) {
	resetUpdateFlags(t)
	swapUpdateAPI(t, "http://127.0.0.1:1") // closed port
	swapVersion(t, "v0.1.0")

	_, _, err := runCmd(t, rootCmd, "update", "--check")
	require.Error(t, err)
}

func TestPickAsset_PicksCurrentPlatform(t *testing.T) {
	resetUpdateFlags(t)
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
	resetUpdateFlags(t)
	rel := gitHubRelease{Assets: []gitHubReleaseAsset{
		{Name: "deploysentry-windows-amd64"},
	}}
	_, err := pickAsset(&rel)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no release asset for")
}

func TestUpdateInstall_FullFlow(t *testing.T) {
	resetUpdateFlags(t)
	if runtime.GOOS == "windows" {
		t.Skip("self-update install not supported on Windows in v1")
	}

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

	prevDL := downloadAssetFunc
	downloadAssetFunc = func(url string, w io.Writer) error {
		_, err := io.WriteString(w, "#!/bin/sh\necho v0.2.0\n")
		return err
	}
	t.Cleanup(func() { downloadAssetFunc = prevDL })

	tmpDir := t.TempDir()
	fakeBinary := filepath.Join(tmpDir, "deploysentry-fake")
	require.NoError(t, os.WriteFile(fakeBinary, []byte("#!/bin/sh\necho v0.1.0\n"), 0o755))

	prevExe := osExecutable
	osExecutable = func() (string, error) { return fakeBinary, nil }
	t.Cleanup(func() { osExecutable = prevExe })

	stdout, _, err := runCmd(t, rootCmd, "update", "--yes")
	require.NoError(t, err)
	require.Contains(t, stdout, "Updated to")

	data, err := os.ReadFile(fakeBinary)
	require.NoError(t, err)
	require.Contains(t, string(data), "v0.2.0")
}

func TestUpdate_PinnedVersion(t *testing.T) {
	resetUpdateFlags(t)
	if runtime.GOOS == "windows" {
		t.Skip("self-update install not supported on Windows in v1")
	}

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
	resetUpdateFlags(t)
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
