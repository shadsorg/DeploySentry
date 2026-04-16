package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// configCmd is the parent command for CLI configuration operations.
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage CLI configuration",
	Long: `Initialize, view, and modify the DeploySentry CLI configuration.

Configuration is stored in .deploysentry.yml and can set default
values for organization, project, environment, and other options
so you don't have to specify them on every command.

Configuration precedence (highest to lowest):
  1. Command-line flags
  2. Environment variables (DEPLOYSENTRY_*)
  3. Config file (.deploysentry.yml)

Examples:
  # Initialize a new configuration file
  deploysentry config init

  # Set default organization
  deploysentry config set org my-org

  # Get a config value
  deploysentry config get org`,
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a .deploysentry.yml configuration file",
	Long: `Create a new .deploysentry.yml configuration file in the current directory.

This command creates a configuration file with sensible defaults and
comments explaining each option. If a file already exists, it will not
be overwritten unless --force is specified.

Examples:
  # Initialize config in the current directory
  deploysentry config init

  # Initialize with specific values
  deploysentry config init --org my-org --project my-api

  # Force overwrite existing config
  deploysentry config init --force`,
	RunE: runConfigInit,
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value",
	Long: `Set a configuration value in the .deploysentry.yml file.

Available configuration keys:
  org              Organization slug
  project          Project slug
  env              Default target environment
  api_url          API base URL
  output           Default output format (table/json)

Examples:
  # Set default organization
  deploysentry config set org my-org

  # Set default project
  deploysentry config set project my-api

  # Set default environment
  deploysentry config set env staging

  # Set API URL
  deploysentry config set api_url https://api.dr-sentry.com`,
	Args: cobra.ExactArgs(2),
	RunE: runConfigSet,
}

var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Get a configuration value",
	Long: `Display the current value of a configuration key.

The value is resolved from all sources (flags, env vars, config file)
with standard precedence rules.

Examples:
  # Get the configured organization
  deploysentry config get org

  # Get the API URL
  deploysentry config get api_url`,
	Args: cobra.ExactArgs(1),
	RunE: runConfigGet,
}

func init() {
	configInitCmd.Flags().Bool("force", false, "overwrite existing configuration file")

	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configGetCmd)

	rootCmd.AddCommand(configCmd)
}

const configTemplate = `# DeploySentry CLI Configuration
# https://docs.deploysentry.io/cli/configuration

# Organization slug
org: %s

# Project slug
project: %s

# Default target environment (dev, staging, production)
env: %s

# DeploySentry API base URL
api_url: https://api.dr-sentry.com

# Default output format: table or json
output: table

# Enable verbose logging
verbose: false
`

func runConfigInit(cmd *cobra.Command, args []string) error {
	force, _ := cmd.Flags().GetBool("force")

	configPath := ".deploysentry.yml"
	if _, err := os.Stat(configPath); err == nil && !force {
		return fmt.Errorf("configuration file %s already exists; use --force to overwrite", configPath)
	}

	org := getOrg()
	project := getProject()
	env := getEnv()

	// Use empty strings as placeholders if not set.
	if org == "" {
		org = ""
	}
	if project == "" {
		project = ""
	}
	if env == "" {
		env = "dev"
	}

	content := fmt.Sprintf(configTemplate, org, project, env)

	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write configuration file: %w", err)
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Configuration file created: %s\n", configPath)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\nEdit the file to set your organization and project, or use:\n")
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  deploysentry config set org <your-org>\n")
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  deploysentry config set project <your-project>\n")
	return nil
}

func runConfigSet(cmd *cobra.Command, args []string) error {
	key := args[0]
	value := args[1]

	// Validate the key.
	validKeys := map[string]bool{
		"org":     true,
		"project": true,
		"env":     true,
		"api_url": true,
		"output":  true,
		"verbose": true,
		"api_key": true,
	}

	if !validKeys[key] {
		validList := make([]string, 0, len(validKeys))
		for k := range validKeys {
			validList = append(validList, k)
		}
		return fmt.Errorf("unknown config key %q; valid keys: %s", key, strings.Join(validList, ", "))
	}

	// Validate specific values.
	if key == "output" && value != "table" && value != "json" {
		return fmt.Errorf("invalid output format %q; must be 'table' or 'json'", value)
	}
	if key == "verbose" && value != "true" && value != "false" {
		return fmt.Errorf("invalid verbose value %q; must be 'true' or 'false'", value)
	}

	viper.Set(key, value)

	// Write the config file.
	configFile := viper.ConfigFileUsed()
	if configFile == "" {
		configFile = ".deploysentry.yml"
	}

	if err := viper.WriteConfigAs(configFile); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Set %s = %s\n", key, value)
	return nil
}

func runConfigGet(cmd *cobra.Command, args []string) error {
	key := args[0]

	value := viper.Get(key)
	if value == nil {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s is not set\n", key)
		return nil
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s = %v\n", key, value)
	return nil
}
