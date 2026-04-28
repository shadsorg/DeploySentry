package main

import (
	"encoding/json"
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var flagsRulesCmd = &cobra.Command{
	Use:   "rules",
	Short: "Manage targeting rules on a flag",
}

var flagsRulesListCmd = &cobra.Command{
	Use:   "list <flag-key>",
	Short: "List targeting rules for a flag",
	Args:  cobra.ExactArgs(1),
	RunE:  runFlagsRulesList,
}

var flagsRulesAddCmd = &cobra.Command{
	Use:   "add <flag-key>",
	Short: "Add a targeting rule",
	Long: `Add a targeting rule to a flag. Use --rule-type and the appropriate type-specific flags.

Examples:
  # Percentage rollout to 25%
  deploysentry flags rules add dark-mode --rule-type percentage --percentage 25 --value true --priority 10

  # Attribute match
  deploysentry flags rules add dark-mode --rule-type attribute --attribute plan --operator eq --value pro

  # User target
  deploysentry flags rules add dark-mode --rule-type user_target --target-values u1,u2,u3 --value true`,
	Args: cobra.ExactArgs(1),
	RunE: runFlagsRulesAdd,
}

var flagsRulesUpdateCmd = &cobra.Command{
	Use:   "update <flag-key> <rule-id>",
	Short: "Update a targeting rule",
	Args:  cobra.ExactArgs(2),
	RunE:  runFlagsRulesUpdate,
}

var flagsRulesDeleteCmd = &cobra.Command{
	Use:   "delete <flag-key> <rule-id>",
	Short: "Delete a targeting rule",
	Args:  cobra.ExactArgs(2),
	RunE:  runFlagsRulesDelete,
}

var flagsRulesEnableEnvCmd = &cobra.Command{
	Use:   "set-env-state <flag-key> <rule-id>",
	Short: "Enable or disable a rule in a specific environment",
	Args:  cobra.ExactArgs(2),
	RunE:  runFlagsRulesSetEnvState,
}

func init() {
	flagsRulesAddCmd.Flags().String("rule-type", "", "rule type: percentage, user_target, attribute, segment, schedule (required)")
	flagsRulesAddCmd.Flags().Int("priority", 100, "rule priority (lower = higher precedence)")
	flagsRulesAddCmd.Flags().String("value", "", "value returned when rule matches")
	flagsRulesAddCmd.Flags().Int("percentage", 0, "percentage 0-100 (for percentage rules)")
	flagsRulesAddCmd.Flags().String("attribute", "", "attribute name (for attribute rules)")
	flagsRulesAddCmd.Flags().String("operator", "", "operator: eq, neq, in, contains, etc.")
	flagsRulesAddCmd.Flags().StringSlice("target-values", nil, "target values (for user_target / attribute in)")
	flagsRulesAddCmd.Flags().String("segment-id", "", "segment UUID (for segment rules)")
	flagsRulesAddCmd.Flags().Bool("disabled", false, "create the rule disabled")
	_ = flagsRulesAddCmd.MarkFlagRequired("rule-type")

	flagsRulesUpdateCmd.Flags().Int("priority", 0, "new priority")
	flagsRulesUpdateCmd.Flags().String("value", "", "new value")
	flagsRulesUpdateCmd.Flags().Int("percentage", 0, "new percentage")
	flagsRulesUpdateCmd.Flags().Bool("enabled", true, "rule enabled")

	flagsRulesEnableEnvCmd.Flags().Bool("on", false, "enable the rule in this env")
	flagsRulesEnableEnvCmd.Flags().Bool("off", false, "disable the rule in this env")

	flagsRulesCmd.AddCommand(flagsRulesListCmd)
	flagsRulesCmd.AddCommand(flagsRulesAddCmd)
	flagsRulesCmd.AddCommand(flagsRulesUpdateCmd)
	flagsRulesCmd.AddCommand(flagsRulesDeleteCmd)
	flagsRulesCmd.AddCommand(flagsRulesEnableEnvCmd)
	flagsCmd.AddCommand(flagsRulesCmd)
}

// resolveFlagFromArgs is a shared helper used by every flags-rules subcommand.
// Resolves org/project from viper config, then resolves project slug -> UUID
// and flag key -> UUID via the API.
func resolveFlagFromArgs(flagKey string) (*apiClient, string, string, error) {
	org, err := requireOrg()
	if err != nil {
		return nil, "", "", err
	}
	projectSlug, err := requireProject()
	if err != nil {
		return nil, "", "", err
	}
	client, err := clientFromConfig()
	if err != nil {
		return nil, "", "", err
	}
	projectID, err := resolveProjectID(client, org, projectSlug)
	if err != nil {
		return nil, "", "", err
	}
	flagID, err := resolveFlagID(client, projectID, flagKey)
	if err != nil {
		return nil, "", "", err
	}
	return client, flagID, projectID, nil
}

func runFlagsRulesList(cmd *cobra.Command, args []string) error {
	client, flagID, _, err := resolveFlagFromArgs(args[0])
	if err != nil {
		return err
	}
	resp, err := client.get(fmt.Sprintf("/api/v1/flags/%s/rules", flagID))
	if err != nil {
		return fmt.Errorf("failed to list rules: %w", err)
	}
	if getOutputFormat() == "json" {
		data, _ := json.MarshalIndent(resp, "", "  ")
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}
	rules, _ := resp["rules"].([]any)
	if len(rules) == 0 {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No targeting rules.")
		return nil
	}
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "ID\tTYPE\tPRIORITY\tVALUE\tENABLED")
	for _, r := range rules {
		rule, _ := r.(map[string]any)
		id, _ := rule["id"].(string)
		t, _ := rule["rule_type"].(string)
		p, _ := rule["priority"].(float64)
		v := rule["value"]
		en, _ := rule["enabled"].(bool)
		_, _ = fmt.Fprintf(w, "%s\t%s\t%d\t%v\t%t\n", id, t, int(p), v, en)
	}
	return w.Flush()
}

func runFlagsRulesAdd(cmd *cobra.Command, args []string) error {
	client, flagID, _, err := resolveFlagFromArgs(args[0])
	if err != nil {
		return err
	}
	ruleType, _ := cmd.Flags().GetString("rule-type")
	priority, _ := cmd.Flags().GetInt("priority")
	value, _ := cmd.Flags().GetString("value")
	pct, _ := cmd.Flags().GetInt("percentage")
	attr, _ := cmd.Flags().GetString("attribute")
	op, _ := cmd.Flags().GetString("operator")
	tv, _ := cmd.Flags().GetStringSlice("target-values")
	seg, _ := cmd.Flags().GetString("segment-id")
	disabled, _ := cmd.Flags().GetBool("disabled")

	body := map[string]any{
		"rule_type": ruleType,
		"priority":  priority,
		"value":     value,
		"enabled":   !disabled,
	}
	if cmd.Flags().Changed("percentage") {
		body["percentage"] = pct
	}
	if attr != "" {
		body["attribute"] = attr
	}
	if op != "" {
		body["operator"] = op
	}
	if len(tv) > 0 {
		body["target_values"] = tv
	}
	if seg != "" {
		body["segment_id"] = seg
	}

	resp, err := client.post(fmt.Sprintf("/api/v1/flags/%s/rules", flagID), body)
	if err != nil {
		return fmt.Errorf("failed to add rule: %w", err)
	}
	if getOutputFormat() == "json" {
		data, _ := json.MarshalIndent(resp, "", "  ")
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}
	id, _ := resp["id"].(string)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Rule added: %s\n", id)
	return nil
}

func runFlagsRulesUpdate(cmd *cobra.Command, args []string) error {
	client, flagID, _, err := resolveFlagFromArgs(args[0])
	if err != nil {
		return err
	}
	ruleID := args[1]

	body := map[string]any{}
	if cmd.Flags().Changed("priority") {
		v, _ := cmd.Flags().GetInt("priority")
		body["priority"] = v
	}
	if cmd.Flags().Changed("value") {
		v, _ := cmd.Flags().GetString("value")
		body["value"] = v
	}
	if cmd.Flags().Changed("percentage") {
		v, _ := cmd.Flags().GetInt("percentage")
		body["percentage"] = v
	}
	if cmd.Flags().Changed("enabled") {
		v, _ := cmd.Flags().GetBool("enabled")
		body["enabled"] = v
	}
	if len(body) == 0 {
		return fmt.Errorf("no updates specified")
	}

	resp, err := client.put(fmt.Sprintf("/api/v1/flags/%s/rules/%s", flagID, ruleID), body)
	if err != nil {
		return fmt.Errorf("failed to update rule: %w", err)
	}
	if getOutputFormat() == "json" {
		data, _ := json.MarshalIndent(resp, "", "  ")
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Rule %s updated.\n", ruleID)
	return nil
}

func runFlagsRulesDelete(cmd *cobra.Command, args []string) error {
	client, flagID, _, err := resolveFlagFromArgs(args[0])
	if err != nil {
		return err
	}
	ruleID := args[1]
	if _, err := client.delete(fmt.Sprintf("/api/v1/flags/%s/rules/%s", flagID, ruleID)); err != nil {
		return fmt.Errorf("failed to delete rule: %w", err)
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Rule %s deleted.\n", ruleID)
	return nil
}

func runFlagsRulesSetEnvState(cmd *cobra.Command, args []string) error {
	client, flagID, _, err := resolveFlagFromArgs(args[0])
	if err != nil {
		return err
	}
	ruleID := args[1]

	on, _ := cmd.Flags().GetBool("on")
	off, _ := cmd.Flags().GetBool("off")
	if !on && !off {
		return fmt.Errorf("you must specify --on or --off")
	}
	if on && off {
		return fmt.Errorf("cannot specify both --on and --off")
	}

	envSlug := getEnv()
	if envSlug == "" {
		return fmt.Errorf("--env (or DS env config) is required for set-env-state")
	}
	org, _ := requireOrg()
	envID, err := resolveEnvID(client, org, envSlug)
	if err != nil {
		return err
	}

	body := map[string]any{"enabled": on}
	resp, err := client.put(fmt.Sprintf("/api/v1/flags/%s/rules/%s/environments/%s", flagID, ruleID, envID), body)
	if err != nil {
		return fmt.Errorf("failed to set rule env state: %w", err)
	}
	if getOutputFormat() == "json" {
		data, _ := json.MarshalIndent(resp, "", "  ")
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}
	state := "OFF"
	if on {
		state = "ON"
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Rule %s in %s: %s\n", ruleID, envSlug, state)
	return nil
}
