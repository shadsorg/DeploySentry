package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

var flagsSetValueCmd = &cobra.Command{
	Use:   "set-value <flag-key>",
	Short: "Set the per-environment default value for a flag",
	Long: `Set or change the per-environment default value of a flag.

Requires --env (or DS env config). Optionally toggle enabled in the same call
with --enabled / --disabled.

Examples:
  # Change the prod default value of a string flag
  deploysentry flags set-value checkout-variant --env production --value "variant-b"

  # Set a numeric default and enable it in staging in one call
  deploysentry flags set-value rate-limit --env staging --value "100" --enabled`,
	Args: cobra.ExactArgs(1),
	RunE: runFlagsSetValue,
}

func init() {
	flagsSetValueCmd.Flags().String("value", "", "new default value for this environment")
	flagsSetValueCmd.Flags().Bool("enabled", false, "set enabled=true in this env")
	flagsSetValueCmd.Flags().Bool("disabled", false, "set enabled=false in this env")
	flagsCmd.AddCommand(flagsSetValueCmd)
}

func runFlagsSetValue(cmd *cobra.Command, args []string) error {
	client, flagID, _, err := resolveFlagFromArgs(args[0])
	if err != nil {
		return err
	}

	envSlug := getEnv()
	if envSlug == "" {
		return fmt.Errorf("--env (or DS env config) is required for set-value")
	}
	org, _ := requireOrg()
	envID, err := resolveEnvID(client, org, envSlug)
	if err != nil {
		return err
	}

	body := map[string]any{}
	if cmd.Flags().Changed("value") {
		v, _ := cmd.Flags().GetString("value")
		body["value"] = v
	}
	en, _ := cmd.Flags().GetBool("enabled")
	dis, _ := cmd.Flags().GetBool("disabled")
	if en && dis {
		return fmt.Errorf("cannot specify both --enabled and --disabled")
	}
	if en {
		body["enabled"] = true
	}
	if dis {
		body["enabled"] = false
	}
	if len(body) == 0 {
		return fmt.Errorf("nothing to update; use --value, --enabled, or --disabled")
	}

	resp, err := client.put(fmt.Sprintf("/api/v1/flags/%s/environments/%s", flagID, envID), body)
	if err != nil {
		return fmt.Errorf("failed to set env state: %w", err)
	}
	if getOutputFormat() == "json" {
		data, _ := json.MarshalIndent(resp, "", "  ")
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Flag %q updated in env %q.\n", args[0], envSlug)
	return nil
}
