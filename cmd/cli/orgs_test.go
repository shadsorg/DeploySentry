package main

import (
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

// resetOrgsCreateFlags clears cobra flag state on orgsCreateCmd between tests.
func resetOrgsCreateFlags(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		for _, name := range []string{"name", "slug"} {
			if f := orgsCreateCmd.Flags().Lookup(name); f != nil {
				f.Changed = false
				_ = f.Value.Set(f.DefValue)
			}
		}
	})
}

// TestOrgsCreate_Success verifies POST /api/v1/orgs with name+slug body.
func TestOrgsCreate_Success(t *testing.T) {
	resetOrgsCreateFlags(t)
	srv := newMockServer(t)
	srv.onPathFunc("POST", "/api/v1/orgs", func(req recordedRequest) (int, any) {
		require.Equal(t, "/api/v1/orgs", req.Path)
		require.Equal(t, "Acme Corp", req.Body["name"])
		require.Equal(t, "acme", req.Body["slug"])
		return 201, map[string]any{"id": "org-1", "name": "Acme Corp", "slug": "acme"}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "", "", "")

	stdout, _, err := runCmd(t, rootCmd, "orgs", "create",
		"--name", "Acme Corp",
		"--slug", "acme",
	)
	require.NoError(t, err)
	require.Contains(t, stdout, "org-1")
	require.Contains(t, stdout, "acme")
}

// TestOrgsList_Success verifies GET /api/v1/orgs.
func TestOrgsList_Success(t *testing.T) {
	srv := newMockServer(t)
	srv.onPathFunc("GET", "/api/v1/orgs", func(req recordedRequest) (int, any) {
		require.Equal(t, "/api/v1/orgs", req.Path)
		return 200, map[string]any{
			"organizations": []map[string]any{
				{"id": "org-1", "slug": "acme", "name": "Acme Corp", "plan": "team"},
			},
		}
	})
	setTestConfig(t, srv.URL(), "ds_testkey", "", "", "")

	stdout, _, err := runCmd(t, rootCmd, "orgs", "list")
	require.NoError(t, err)
	require.Contains(t, stdout, "acme")
	require.Contains(t, stdout, "Acme Corp")
}

// TestOrgsSet_WritesConfig verifies that `orgs set <slug>` writes the slug to
// the local viper config file. Uses a temp dir for the config so we don't
// pollute the real working directory.
func TestOrgsSet_WritesConfig(t *testing.T) {
	dir := t.TempDir()
	viper.Reset()
	t.Cleanup(viper.Reset)

	// Configure viper to write to a known location in the temp dir.
	viper.AddConfigPath(dir)
	viper.SetConfigName(".deploysentry")
	viper.SetConfigType("yaml")
	// Pre-create the config file so WriteConfig finds something to update.
	cfgPath := filepath.Join(dir, ".deploysentry.yaml")
	viper.SetConfigFile(cfgPath)
	require.NoError(t, viper.WriteConfigAs(cfgPath))

	_, _, err := runCmd(t, rootCmd, "orgs", "set", "acme")
	require.NoError(t, err)
	require.Equal(t, "acme", viper.GetString("org"))
}
