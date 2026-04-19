package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var rolloutPolicyCmd = &cobra.Command{
	Use:     "rollout-policy",
	Aliases: []string{"rp"},
	Short:   "Manage rollout onboarding + mandate policy per scope",
}

var rolloutPolicyGetCmd = &cobra.Command{
	Use:   "get",
	Short: "List policy rows defined at the scope",
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := strategyScopePath()
		if err != nil {
			return err
		}
		client, err := clientFromConfig()
		if err != nil {
			return err
		}
		resp, err := client.get(path + "/rollout-policy")
		if err != nil {
			return err
		}
		out, _ := json.MarshalIndent(resp, "", "  ")
		fmt.Println(string(out))
		return nil
	},
}

var rolloutPolicySetCmd = &cobra.Command{
	Use:   "set",
	Short: "Upsert a policy row (off|prompt|mandate) for a scope",
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := strategyScopePath()
		if err != nil {
			return err
		}
		policy, _ := cmd.Flags().GetString("policy")
		enabled, _ := cmd.Flags().GetBool("enabled")
		env, _ := cmd.Flags().GetString("env")
		target, _ := cmd.Flags().GetString("target")
		payload := map[string]any{"enabled": enabled, "policy": policy}
		if env != "" {
			payload["environment"] = env
		}
		if target != "" {
			payload["target_type"] = target
		}
		client, err := clientFromConfig()
		if err != nil {
			return err
		}
		resp, err := client.put(path+"/rollout-policy", payload)
		if err != nil {
			return err
		}
		out, _ := json.MarshalIndent(resp, "", "  ")
		fmt.Println(string(out))
		return nil
	},
}

var strategyDefaultsCmd = &cobra.Command{
	Use:     "strategy-defaults",
	Aliases: []string{"sd"},
	Short:   "Manage default strategy assignments per (scope, env, target)",
}

var strategyDefaultsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List default strategy rows defined at the scope",
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := strategyScopePath()
		if err != nil {
			return err
		}
		client, err := clientFromConfig()
		if err != nil {
			return err
		}
		resp, err := client.get(path + "/strategy-defaults")
		if err != nil {
			return err
		}
		out, _ := json.MarshalIndent(resp, "", "  ")
		fmt.Println(string(out))
		return nil
	},
}

var strategyDefaultsSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Upsert a default strategy assignment",
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := strategyScopePath()
		if err != nil {
			return err
		}
		strategy, _ := cmd.Flags().GetString("strategy")
		env, _ := cmd.Flags().GetString("env")
		target, _ := cmd.Flags().GetString("target")
		payload := map[string]any{"strategy_name": strategy}
		if env != "" {
			payload["environment"] = env
		}
		if target != "" {
			payload["target_type"] = target
		}
		client, err := clientFromConfig()
		if err != nil {
			return err
		}
		resp, err := client.put(path+"/strategy-defaults", payload)
		if err != nil {
			return err
		}
		out, _ := json.MarshalIndent(resp, "", "  ")
		fmt.Println(string(out))
		return nil
	},
}

func init() {
	// --app is not a root persistent flag; declare it as a persistent flag on each
	// parent command, mirroring the pattern in strategies.go.
	rolloutPolicyCmd.PersistentFlags().String("app", "", "application slug (optional, for app scope)")
	_ = viper.BindPFlag("app", rolloutPolicyCmd.PersistentFlags().Lookup("app"))
	strategyDefaultsCmd.PersistentFlags().String("app", "", "application slug (optional, for app scope)")
	_ = viper.BindPFlag("app", strategyDefaultsCmd.PersistentFlags().Lookup("app"))

	rolloutPolicySetCmd.Flags().String("policy", "off", "Policy: off|prompt|mandate")
	rolloutPolicySetCmd.Flags().Bool("enabled", true, "Enable rollout control on this scope")
	rolloutPolicySetCmd.Flags().String("env", "", "Environment name (optional narrowing)")
	rolloutPolicySetCmd.Flags().String("target", "", "deploy|config (optional narrowing)")

	strategyDefaultsSetCmd.Flags().String("strategy", "", "Strategy name (required)")
	strategyDefaultsSetCmd.Flags().String("env", "", "Environment name (optional)")
	strategyDefaultsSetCmd.Flags().String("target", "", "deploy|config (optional)")
	_ = strategyDefaultsSetCmd.MarkFlagRequired("strategy")

	rolloutPolicyCmd.AddCommand(rolloutPolicyGetCmd, rolloutPolicySetCmd)
	strategyDefaultsCmd.AddCommand(strategyDefaultsListCmd, strategyDefaultsSetCmd)

	rootCmd.AddCommand(rolloutPolicyCmd, strategyDefaultsCmd)
}
