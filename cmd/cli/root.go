package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	// Version information set at build time via ldflags.
	version   = "dev"
	commit    = "none"
	buildDate = "unknown"

	cfgFile string
)

var rootCmd = &cobra.Command{
	Use:   "deploysentry",
	Short: "DeploySentry - deploy release and feature flag management",
	Long: `DeploySentry CLI provides a unified interface for managing deployments,
releases, and feature flags across your infrastructure.

Use DeploySentry to safely roll out changes with canary deployments,
control feature flags in real time, and promote releases across
environments with full observability.

Configuration can be provided via:
  - Flags on each command
  - Environment variables prefixed with DEPLOYSENTRY_
  - A config file (.deploysentry.yml in the current directory or $HOME)

Examples:
  # Initialize a project configuration
  deploysentry config init

  # Create a new deployment with canary strategy
  deploysentry deploy create --release v1.2.0 --env production --strategy canary

  # Toggle a feature flag on
  deploysentry flags toggle my-feature --on

  # List recent releases
  deploysentry releases list`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version information",
	Long:  `Display the version, commit hash, and build date of the DeploySentry CLI.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Fprintf(cmd.OutOrStdout(), "DeploySentry CLI\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  Version:    %s\n", version)
		fmt.Fprintf(cmd.OutOrStdout(), "  Commit:     %s\n", commit)
		fmt.Fprintf(cmd.OutOrStdout(), "  Build Date: %s\n", buildDate)
	},
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global persistent flags available to all commands.
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ./.deploysentry.yml or $HOME/.deploysentry.yml)")
	rootCmd.PersistentFlags().String("org", "", "organization slug")
	rootCmd.PersistentFlags().String("project", "", "project slug")
	rootCmd.PersistentFlags().String("env", "", "target environment (e.g., dev, staging, production)")
	rootCmd.PersistentFlags().StringP("output", "o", "table", "output format: table or json")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "enable verbose output for debugging")

	// Bind persistent flags to viper for config-file and env-var support.
	_ = viper.BindPFlag("org", rootCmd.PersistentFlags().Lookup("org"))
	_ = viper.BindPFlag("project", rootCmd.PersistentFlags().Lookup("project"))
	_ = viper.BindPFlag("env", rootCmd.PersistentFlags().Lookup("env"))
	_ = viper.BindPFlag("output", rootCmd.PersistentFlags().Lookup("output"))
	_ = viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))

	rootCmd.AddCommand(versionCmd)
}

// initConfig reads the config file and environment variables.
func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		// Search for config in the current directory first, then home directory.
		cwd, err := os.Getwd()
		if err == nil {
			viper.AddConfigPath(cwd)
		}

		home, err := os.UserHomeDir()
		if err == nil {
			viper.AddConfigPath(home)
			viper.AddConfigPath(filepath.Join(home, ".config", "deploysentry"))
		}

		viper.SetConfigName(".deploysentry")
		viper.SetConfigType("yaml")
	}

	// Environment variable support: DEPLOYSENTRY_ORG, DEPLOYSENTRY_PROJECT, etc.
	viper.SetEnvPrefix("DEPLOYSENTRY")
	viper.AutomaticEnv()

	// Silently ignore missing config files; they are optional.
	if err := viper.ReadInConfig(); err == nil {
		if viper.GetBool("verbose") {
			fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
		}
	}
}

// getOrg returns the organization from flags, config, or env.
func getOrg() string {
	return viper.GetString("org")
}

// getProject returns the project from flags, config, or env.
func getProject() string {
	return viper.GetString("project")
}

// getEnv returns the target environment from flags, config, or env.
func getEnv() string {
	return viper.GetString("env")
}

// getOutputFormat returns the desired output format (table or json).
func getOutputFormat() string {
	return viper.GetString("output")
}

// isVerbose returns whether verbose mode is enabled.
func isVerbose() bool {
	return viper.GetBool("verbose")
}

// requireOrg validates that the org flag is set and returns it.
func requireOrg() (string, error) {
	org := getOrg()
	if org == "" {
		return "", fmt.Errorf("organization is required; set via --org flag, DEPLOYSENTRY_ORG env var, or config file")
	}
	return org, nil
}

// requireProject validates that the project flag is set and returns it.
func requireProject() (string, error) {
	project := getProject()
	if project == "" {
		return "", fmt.Errorf("project is required; set via --project flag, DEPLOYSENTRY_PROJECT env var, or config file")
	}
	return project, nil
}

// requireEnv validates that the env flag is set and returns it.
func requireEnv() (string, error) {
	env := getEnv()
	if env == "" {
		return "", fmt.Errorf("environment is required; set via --env flag, DEPLOYSENTRY_ENV env var, or config file")
	}
	return env, nil
}
