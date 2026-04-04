package main

import (
	"encoding/json"
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

// settingsCmd is the parent command for settings management operations.
var settingsCmd = &cobra.Command{
	Use:     "settings",
	Aliases: []string{"setting"},
	Short:   "Manage settings",
	Long: `View and manage configuration settings scoped to organizations, projects,
environments, or other resources.

Settings are key-value pairs that control the behaviour of DeploySentry
at various levels of the hierarchy. Use --scope and --target to identify
the resource whose settings you want to inspect or modify.

Examples:
  # List settings for a project
  deploysentry settings list --scope project --target my-project

  # Set a value
  deploysentry settings set --scope project --target my-project \
    --key rollout.max_batch_size --value 50

  # Delete a setting by ID
  deploysentry settings delete abc123`,
}

var settingsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List settings for a scope and target",
	Long: `List all settings for the given scope and target resource.

Examples:
  # List settings for an organization
  deploysentry settings list --scope org --target my-org

  # List settings for a project
  deploysentry settings list --scope project --target my-project

  # Output as JSON
  deploysentry settings list --scope project --target my-project -o json`,
	RunE: runSettingsList,
}

var settingsSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Set a configuration value",
	Long: `Create or update a configuration setting for the given scope and target.

Examples:
  # Set a project-level setting
  deploysentry settings set --scope project --target my-project \
    --key rollout.strategy --value canary

  # Set an environment-level setting
  deploysentry settings set --scope environment --target production \
    --key notifications.slack_channel --value "#deploys"`,
	RunE: runSettingsSet,
}

var settingsDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a setting by ID",
	Long: `Delete a configuration setting by its ID.

Examples:
  # Delete a setting
  deploysentry settings delete abc123`,
	Args: cobra.ExactArgs(1),
	RunE: runSettingsDelete,
}

func init() {
	// settings list flags
	settingsListCmd.Flags().String("scope", "", "scope of the setting (e.g. org, project, environment)")
	settingsListCmd.Flags().String("target", "", "target resource ID or slug")
	_ = settingsListCmd.MarkFlagRequired("scope")
	_ = settingsListCmd.MarkFlagRequired("target")

	// settings set flags
	settingsSetCmd.Flags().String("scope", "", "scope of the setting (e.g. org, project, environment)")
	settingsSetCmd.Flags().String("target", "", "target resource ID or slug")
	settingsSetCmd.Flags().String("key", "", "setting key")
	settingsSetCmd.Flags().String("value", "", "setting value")
	_ = settingsSetCmd.MarkFlagRequired("scope")
	_ = settingsSetCmd.MarkFlagRequired("target")
	_ = settingsSetCmd.MarkFlagRequired("key")
	_ = settingsSetCmd.MarkFlagRequired("value")

	settingsCmd.AddCommand(settingsListCmd)
	settingsCmd.AddCommand(settingsSetCmd)
	settingsCmd.AddCommand(settingsDeleteCmd)

	rootCmd.AddCommand(settingsCmd)
}

func runSettingsList(cmd *cobra.Command, args []string) error {
	scope, _ := cmd.Flags().GetString("scope")
	target, _ := cmd.Flags().GetString("target")

	client, err := clientFromConfig()
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/api/v1/settings?scope=%s&target=%s", scope, target)
	resp, err := client.get(path)
	if err != nil {
		return fmt.Errorf("failed to list settings: %w", err)
	}

	if getOutputFormat() == "json" {
		data, _ := json.MarshalIndent(resp, "", "  ")
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	settings, _ := resp["settings"].([]interface{})
	if len(settings) == 0 {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No settings found.")
		return nil
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "KEY\tVALUE\tUPDATED_BY\tUPDATED_AT")
	for _, s := range settings {
		setting, ok := s.(map[string]interface{})
		if !ok {
			continue
		}

		key, _ := setting["key"].(string)
		value, _ := setting["value"].(string)
		updatedBy, _ := setting["updated_by"].(string)
		updatedAt, _ := setting["updated_at"].(string)

		// Truncate long values for table display.
		displayValue := value
		if len(displayValue) > 40 {
			displayValue = displayValue[:37] + "..."
		}

		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", key, displayValue, updatedBy, updatedAt)
	}
	return w.Flush()
}

func runSettingsSet(cmd *cobra.Command, args []string) error {
	scope, _ := cmd.Flags().GetString("scope")
	target, _ := cmd.Flags().GetString("target")
	key, _ := cmd.Flags().GetString("key")
	value, _ := cmd.Flags().GetString("value")

	client, err := clientFromConfig()
	if err != nil {
		return err
	}

	body := map[string]interface{}{
		"scope":     scope,
		"target_id": target,
		"key":       key,
		"value":     value,
	}

	resp, err := client.put("/api/v1/settings", body)
	if err != nil {
		return fmt.Errorf("failed to set setting: %w", err)
	}

	if getOutputFormat() == "json" {
		data, _ := json.MarshalIndent(resp, "", "  ")
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Setting updated successfully.\n")
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Scope:  %s\n", scope)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Target: %s\n", target)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Key:    %s\n", key)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Value:  %s\n", value)
	return nil
}

func runSettingsDelete(cmd *cobra.Command, args []string) error {
	settingID := args[0]

	client, err := clientFromConfig()
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/api/v1/settings/%s", settingID)
	_, err = client.delete(path)
	if err != nil {
		return fmt.Errorf("failed to delete setting: %w", err)
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Setting %s deleted successfully.\n", settingID)
	return nil
}

