package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

// flagsCmd is the parent command for feature flag operations.
var flagsCmd = &cobra.Command{
	Use:     "flags",
	Aliases: []string{"flag", "ff"},
	Short:   "Manage feature flags",
	Long: `Create, update, toggle, and evaluate feature flags.

Feature flags allow you to control the rollout of new features without
deploying new code. Use targeting rules to gradually roll out features
to specific user segments, and instantly toggle flags on or off.

Examples:
  # Create a boolean feature flag
  deploysentry flags create --key new-checkout --type boolean --default false

  # Toggle a flag on in production
  deploysentry flags toggle new-checkout --on --env production

  # List all active flags
  deploysentry flags list --status active

  # Test flag evaluation with a user context
  deploysentry flags evaluate new-checkout --context '{"user_id": "123", "plan": "pro"}'`,
}

var flagsCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new feature flag",
	Long: `Create a new feature flag with a specified key, type, and default value.

Supported flag types:
  - boolean: Simple on/off toggle (default: false)
  - string:  Returns a string value
  - number:  Returns a numeric value
  - json:    Returns a JSON object

Examples:
  # Create a boolean flag
  deploysentry flags create --key dark-mode --type boolean --default false

  # Create a string flag for A/B testing
  deploysentry flags create --key checkout-variant --type string --default "control" \
    --description "Checkout page A/B test variant"

  # Create a flag with tags
  deploysentry flags create --key api-rate-limit --type integer --default 100 \
    --tag backend --tag performance`,
	RunE: runFlagsCreate,
}

var flagsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List feature flags",
	Long: `List feature flags, optionally filtered by tag, status, or search term.

Examples:
  # List all flags
  deploysentry flags list

  # Search for flags by name
  deploysentry flags list --search checkout

  # List flags with a specific tag
  deploysentry flags list --tag frontend

  # List only archived flags
  deploysentry flags list --status archived

  # Output in JSON format
  deploysentry flags list -o json`,
	RunE: runFlagsList,
}

var flagsGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Get feature flag details",
	Long: `Display detailed information about a feature flag, including its
current value, targeting rules, and evaluation statistics.

Examples:
  # Get flag details
  deploysentry flags get dark-mode

  # Get flag details in JSON
  deploysentry flags get dark-mode -o json`,
	Args: cobra.ExactArgs(1),
	RunE: runFlagsGet,
}

var flagsToggleCmd = &cobra.Command{
	Use:   "toggle <key>",
	Short: "Toggle a feature flag on or off",
	Long: `Toggle a boolean feature flag on or off.

You must specify either --on or --off. If the flag is not a boolean
type, this command will return an error.

Examples:
  # Turn a flag on
  deploysentry flags toggle dark-mode --on

  # Turn a flag off
  deploysentry flags toggle dark-mode --off

  # Toggle in a specific environment
  deploysentry flags toggle dark-mode --on --env production`,
	Args: cobra.ExactArgs(1),
	RunE: runFlagsToggle,
}

var flagsUpdateCmd = &cobra.Command{
	Use:   "update <key>",
	Short: "Update a feature flag configuration",
	Long: `Update the configuration of an existing feature flag.

You can modify the default value, description, tags, and targeting
rules. To add a targeting rule, use --add-rule with a JSON object.

Targeting Rule JSON format:
  {
    "attribute": "user.plan",
    "operator": "in",
    "values": ["pro", "enterprise"],
    "percentage": 100,
    "value": true
  }

Examples:
  # Update the default value
  deploysentry flags update checkout-variant --default "variant-b"

  # Add a targeting rule
  deploysentry flags update dark-mode --add-rule '{"attribute":"user.plan","operator":"eq","values":["pro"],"value":true}'

  # Update the description
  deploysentry flags update dark-mode --description "Controls dark mode UI theme"`,
	Args: cobra.ExactArgs(1),
	RunE: runFlagsUpdate,
}

var flagsEvaluateCmd = &cobra.Command{
	Use:   "evaluate <key>",
	Short: "Test flag evaluation with a context",
	Long: `Evaluate a feature flag against a given context to see what value
would be returned for a specific user or scenario.

This is useful for debugging targeting rules and verifying flag
behavior before deploying changes.

The --context flag accepts a JSON object representing the evaluation
context (user attributes, environment, etc.).

Examples:
  # Evaluate with a user context
  deploysentry flags evaluate dark-mode --context '{"user_id":"u123","plan":"pro"}'

  # Evaluate with environment context
  deploysentry flags evaluate api-rate-limit --context '{"environment":"staging","region":"us-east"}'

  # Evaluate and get JSON output
  deploysentry flags evaluate dark-mode --context '{"user_id":"u123"}' -o json`,
	Args: cobra.ExactArgs(1),
	RunE: runFlagsEvaluate,
}

var flagsArchiveCmd = &cobra.Command{
	Use:   "archive <key>",
	Short: "Archive a feature flag",
	Long: `Archive a feature flag, making it inactive.

Archived flags are no longer evaluated and will return their default
value. Archiving is reversible; use 'flags update' to restore.

Examples:
  # Archive a flag
  deploysentry flags archive old-feature

  # Archive a flag in JSON output
  deploysentry flags archive old-feature -o json`,
	Args: cobra.ExactArgs(1),
	RunE: runFlagsArchive,
}

func init() {
	// flags create flags
	flagsCreateCmd.Flags().String("key", "", "unique flag key (required)")
	flagsCreateCmd.Flags().String("type", "boolean", "flag type: boolean, string, integer, json")
	flagsCreateCmd.Flags().String("default", "", "default value for the flag")
	flagsCreateCmd.Flags().String("description", "", "description of the flag")
	flagsCreateCmd.Flags().StringSlice("tag", nil, "tags for the flag (can be specified multiple times)")
	_ = flagsCreateCmd.MarkFlagRequired("key")

	// flags list flags
	flagsListCmd.Flags().String("tag", "", "filter by tag")
	flagsListCmd.Flags().String("status", "", "filter by status (active, archived)")
	flagsListCmd.Flags().String("search", "", "search flags by key or description")
	flagsListCmd.Flags().Int("limit", 50, "maximum number of results")

	// flags toggle flags
	flagsToggleCmd.Flags().Bool("on", false, "turn the flag on")
	flagsToggleCmd.Flags().Bool("off", false, "turn the flag off")

	// flags update flags
	flagsUpdateCmd.Flags().String("default", "", "new default value")
	flagsUpdateCmd.Flags().String("description", "", "updated description")
	flagsUpdateCmd.Flags().String("add-rule", "", "add a targeting rule (JSON)")
	flagsUpdateCmd.Flags().StringSlice("tag", nil, "set tags (replaces existing)")

	// flags evaluate flags
	flagsEvaluateCmd.Flags().String("context", "{}", "evaluation context as JSON")

	flagsCmd.AddCommand(flagsCreateCmd)
	flagsCmd.AddCommand(flagsListCmd)
	flagsCmd.AddCommand(flagsGetCmd)
	flagsCmd.AddCommand(flagsToggleCmd)
	flagsCmd.AddCommand(flagsUpdateCmd)
	flagsCmd.AddCommand(flagsEvaluateCmd)
	flagsCmd.AddCommand(flagsArchiveCmd)

	rootCmd.AddCommand(flagsCmd)
}

func runFlagsCreate(cmd *cobra.Command, args []string) error {
	org, err := requireOrg()
	if err != nil {
		return err
	}
	project, err := requireProject()
	if err != nil {
		return err
	}

	key, _ := cmd.Flags().GetString("key")
	flagType, _ := cmd.Flags().GetString("type")
	defaultVal, _ := cmd.Flags().GetString("default")
	description, _ := cmd.Flags().GetString("description")
	tags, _ := cmd.Flags().GetStringSlice("tag")

	// Validate flag type.
	validTypes := map[string]bool{"boolean": true, "string": true, "integer": true, "json": true}
	if !validTypes[flagType] {
		return fmt.Errorf("invalid flag type %q; must be one of: boolean, string, integer, json", flagType)
	}

	client, err := clientFromConfig()
	if err != nil {
		return err
	}

	body := map[string]interface{}{
		"key":  key,
		"type": flagType,
	}
	if defaultVal != "" {
		body["default_value"] = defaultVal
	}
	if description != "" {
		body["description"] = description
	}
	if len(tags) > 0 {
		body["tags"] = tags
	}

	env := getEnv()
	if env != "" {
		body["environment"] = env
	}

	path := fmt.Sprintf("/api/v1/orgs/%s/projects/%s/flags", org, project)
	resp, err := client.post(path, body)
	if err != nil {
		return fmt.Errorf("failed to create flag: %w", err)
	}

	if getOutputFormat() == "json" {
		data, _ := json.MarshalIndent(resp, "", "  ")
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Feature flag created successfully.\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  Key:     %s\n", key)
	fmt.Fprintf(cmd.OutOrStdout(), "  Type:    %s\n", flagType)
	if defaultVal != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "  Default: %s\n", defaultVal)
	}
	return nil
}

func runFlagsList(cmd *cobra.Command, args []string) error {
	org, err := requireOrg()
	if err != nil {
		return err
	}
	project, err := requireProject()
	if err != nil {
		return err
	}

	client, err := clientFromConfig()
	if err != nil {
		return err
	}

	params := []string{}
	if tag, _ := cmd.Flags().GetString("tag"); tag != "" {
		params = append(params, "tag="+tag)
	}
	if status, _ := cmd.Flags().GetString("status"); status != "" {
		params = append(params, "status="+status)
	}
	if search, _ := cmd.Flags().GetString("search"); search != "" {
		params = append(params, "search="+search)
	}
	if limit, _ := cmd.Flags().GetInt("limit"); limit > 0 {
		params = append(params, fmt.Sprintf("limit=%d", limit))
	}
	if env := getEnv(); env != "" {
		params = append(params, "environment="+env)
	}

	path := fmt.Sprintf("/api/v1/orgs/%s/projects/%s/flags", org, project)
	if len(params) > 0 {
		path += "?" + strings.Join(params, "&")
	}

	resp, err := client.get(path)
	if err != nil {
		return fmt.Errorf("failed to list flags: %w", err)
	}

	if getOutputFormat() == "json" {
		data, _ := json.MarshalIndent(resp, "", "  ")
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	flags, _ := resp["flags"].([]interface{})
	if len(flags) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No feature flags found.")
		return nil
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "KEY\tTYPE\tSTATUS\tDEFAULT\tTAGS\tUPDATED")
	for _, f := range flags {
		flag, ok := f.(map[string]interface{})
		if !ok {
			continue
		}
		key, _ := flag["key"].(string)
		flagType, _ := flag["type"].(string)
		status, _ := flag["status"].(string)
		defaultVal, _ := flag["default_value"].(string)
		updatedAt, _ := flag["updated_at"].(string)

		tagList := ""
		if t, ok := flag["tags"].([]interface{}); ok {
			tagStrs := make([]string, 0, len(t))
			for _, tag := range t {
				if s, ok := tag.(string); ok {
					tagStrs = append(tagStrs, s)
				}
			}
			tagList = strings.Join(tagStrs, ", ")
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			key, flagType, status, defaultVal, tagList, updatedAt)
	}
	return w.Flush()
}

func runFlagsGet(cmd *cobra.Command, args []string) error {
	org, err := requireOrg()
	if err != nil {
		return err
	}
	project, err := requireProject()
	if err != nil {
		return err
	}

	client, err := clientFromConfig()
	if err != nil {
		return err
	}

	key := args[0]
	path := fmt.Sprintf("/api/v1/orgs/%s/projects/%s/flags/%s", org, project, key)
	if env := getEnv(); env != "" {
		path += "?environment=" + env
	}

	resp, err := client.get(path)
	if err != nil {
		return fmt.Errorf("failed to get flag %q: %w", key, err)
	}

	if getOutputFormat() == "json" {
		data, _ := json.MarshalIndent(resp, "", "  ")
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Feature Flag: %s\n", key)
	if t, ok := resp["type"].(string); ok {
		fmt.Fprintf(cmd.OutOrStdout(), "  Type:        %s\n", t)
	}
	if s, ok := resp["status"].(string); ok {
		fmt.Fprintf(cmd.OutOrStdout(), "  Status:      %s\n", s)
	}
	if d, ok := resp["default_value"]; ok {
		fmt.Fprintf(cmd.OutOrStdout(), "  Default:     %v\n", d)
	}
	if desc, ok := resp["description"].(string); ok && desc != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "  Description: %s\n", desc)
	}
	if created, ok := resp["created_at"].(string); ok {
		fmt.Fprintf(cmd.OutOrStdout(), "  Created:     %s\n", created)
	}
	if updated, ok := resp["updated_at"].(string); ok {
		fmt.Fprintf(cmd.OutOrStdout(), "  Updated:     %s\n", updated)
	}

	// Print targeting rules if present.
	if rules, ok := resp["rules"].([]interface{}); ok && len(rules) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "\n  Targeting Rules:\n")
		for i, r := range rules {
			rule, ok := r.(map[string]interface{})
			if !ok {
				continue
			}
			attr, _ := rule["attribute"].(string)
			op, _ := rule["operator"].(string)
			val, _ := rule["value"]
			pct, _ := rule["percentage"].(float64)
			fmt.Fprintf(cmd.OutOrStdout(), "    %d. %s %s -> %v (%.0f%%)\n", i+1, attr, op, val, pct)
		}
	}

	return nil
}

func runFlagsToggle(cmd *cobra.Command, args []string) error {
	org, err := requireOrg()
	if err != nil {
		return err
	}
	project, err := requireProject()
	if err != nil {
		return err
	}

	on, _ := cmd.Flags().GetBool("on")
	off, _ := cmd.Flags().GetBool("off")

	if !on && !off {
		return fmt.Errorf("you must specify either --on or --off")
	}
	if on && off {
		return fmt.Errorf("cannot specify both --on and --off")
	}

	client, err := clientFromConfig()
	if err != nil {
		return err
	}

	key := args[0]
	enabled := on

	body := map[string]interface{}{
		"enabled": enabled,
	}
	if env := getEnv(); env != "" {
		body["environment"] = env
	}

	path := fmt.Sprintf("/api/v1/orgs/%s/projects/%s/flags/%s/toggle", org, project, key)
	resp, err := client.post(path, body)
	if err != nil {
		return fmt.Errorf("failed to toggle flag %q: %w", key, err)
	}

	if getOutputFormat() == "json" {
		data, _ := json.MarshalIndent(resp, "", "  ")
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	state := "OFF"
	if enabled {
		state = "ON"
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Flag %q toggled %s.\n", key, state)
	return nil
}

func runFlagsUpdate(cmd *cobra.Command, args []string) error {
	org, err := requireOrg()
	if err != nil {
		return err
	}
	project, err := requireProject()
	if err != nil {
		return err
	}

	client, err := clientFromConfig()
	if err != nil {
		return err
	}

	key := args[0]
	body := map[string]interface{}{}

	if cmd.Flags().Changed("default") {
		defaultVal, _ := cmd.Flags().GetString("default")
		body["default_value"] = defaultVal
	}
	if cmd.Flags().Changed("description") {
		description, _ := cmd.Flags().GetString("description")
		body["description"] = description
	}
	if cmd.Flags().Changed("tag") {
		tags, _ := cmd.Flags().GetStringSlice("tag")
		body["tags"] = tags
	}
	if cmd.Flags().Changed("add-rule") {
		ruleJSON, _ := cmd.Flags().GetString("add-rule")
		var rule map[string]interface{}
		if err := json.Unmarshal([]byte(ruleJSON), &rule); err != nil {
			return fmt.Errorf("invalid rule JSON: %w", err)
		}
		body["add_rule"] = rule
	}

	if len(body) == 0 {
		return fmt.Errorf("no updates specified; use --default, --description, --tag, or --add-rule")
	}

	if env := getEnv(); env != "" {
		body["environment"] = env
	}

	path := fmt.Sprintf("/api/v1/orgs/%s/projects/%s/flags/%s", org, project, key)
	resp, err := client.patch(path, body)
	if err != nil {
		return fmt.Errorf("failed to update flag %q: %w", key, err)
	}

	if getOutputFormat() == "json" {
		data, _ := json.MarshalIndent(resp, "", "  ")
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Flag %q updated successfully.\n", key)
	return nil
}

func runFlagsEvaluate(cmd *cobra.Command, args []string) error {
	org, err := requireOrg()
	if err != nil {
		return err
	}
	project, err := requireProject()
	if err != nil {
		return err
	}

	client, err := clientFromConfig()
	if err != nil {
		return err
	}

	key := args[0]
	contextJSON, _ := cmd.Flags().GetString("context")

	var evalContext map[string]interface{}
	if err := json.Unmarshal([]byte(contextJSON), &evalContext); err != nil {
		return fmt.Errorf("invalid context JSON: %w", err)
	}

	body := map[string]interface{}{
		"context": evalContext,
	}
	if env := getEnv(); env != "" {
		body["environment"] = env
	}

	path := fmt.Sprintf("/api/v1/orgs/%s/projects/%s/flags/%s/evaluate", org, project, key)
	resp, err := client.post(path, body)
	if err != nil {
		return fmt.Errorf("failed to evaluate flag %q: %w", key, err)
	}

	if getOutputFormat() == "json" {
		data, _ := json.MarshalIndent(resp, "", "  ")
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	value, _ := resp["value"]
	reason, _ := resp["reason"].(string)
	ruleIndex, hasRule := resp["rule_index"].(float64)

	fmt.Fprintf(cmd.OutOrStdout(), "Flag:   %s\n", key)
	fmt.Fprintf(cmd.OutOrStdout(), "Value:  %v\n", value)
	fmt.Fprintf(cmd.OutOrStdout(), "Reason: %s\n", reason)
	if hasRule {
		fmt.Fprintf(cmd.OutOrStdout(), "Rule:   #%.0f\n", ruleIndex)
	}
	return nil
}

func runFlagsArchive(cmd *cobra.Command, args []string) error {
	org, err := requireOrg()
	if err != nil {
		return err
	}
	project, err := requireProject()
	if err != nil {
		return err
	}

	client, err := clientFromConfig()
	if err != nil {
		return err
	}

	key := args[0]
	path := fmt.Sprintf("/api/v1/orgs/%s/projects/%s/flags/%s/archive", org, project, key)
	resp, err := client.post(path, nil)
	if err != nil {
		return fmt.Errorf("failed to archive flag %q: %w", key, err)
	}

	if getOutputFormat() == "json" {
		data, _ := json.MarshalIndent(resp, "", "  ")
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Flag %q archived successfully.\n", key)
	return nil
}
