package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

var rolloutsCmd = &cobra.Command{
	Use:     "rollouts",
	Aliases: []string{"rollout", "ro"},
	Short:   "Inspect and control live rollouts",
}

var rolloutsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List recent rollouts for an org",
	RunE: func(cmd *cobra.Command, args []string) error {
		org, err := requireOrg()
		if err != nil {
			return err
		}
		path := "/api/v1/orgs/" + org + "/rollouts"
		if s, _ := cmd.Flags().GetString("status"); s != "" {
			path += "?status=" + s
		}
		client, err := clientFromConfig()
		if err != nil {
			return err
		}
		body, err := client.getRaw(path)
		if err != nil {
			return err
		}
		fmt.Println(string(body))
		return nil
	},
}

var rolloutsGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Show a rollout's detail",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		org, err := requireOrg()
		if err != nil {
			return err
		}
		client, err := clientFromConfig()
		if err != nil {
			return err
		}
		body, err := client.getRaw("/api/v1/orgs/" + org + "/rollouts/" + args[0])
		if err != nil {
			return err
		}
		var pretty any
		_ = json.Unmarshal(body, &pretty)
		out, _ := json.MarshalIndent(pretty, "", "  ")
		fmt.Println(string(out))
		return nil
	},
}

func rolloutActionCmd(action string, needReason bool) *cobra.Command {
	return &cobra.Command{
		Use:   action + " <id>",
		Short: fmt.Sprintf("%s a rollout", action),
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			org, err := requireOrg()
			if err != nil {
				return err
			}
			reason, _ := cmd.Flags().GetString("reason")
			if needReason && reason == "" {
				return fmt.Errorf("--reason is required for %s", action)
			}
			payload, _ := json.Marshal(map[string]string{"reason": reason})
			client, err := clientFromConfig()
			if err != nil {
				return err
			}
			resp, err := client.postRaw("/api/v1/orgs/"+org+"/rollouts/"+args[0]+"/"+action, "application/json", payload)
			if err != nil {
				return err
			}
			fmt.Println(string(resp))
			return nil
		},
	}
}

func init() {
	actions := []struct {
		name   string
		reason bool
	}{
		{"pause", false},
		{"resume", false},
		{"promote", false},
		{"approve", false},
		{"rollback", true},
		{"force-promote", true},
	}

	var actionCmds []*cobra.Command
	for _, a := range actions {
		actionCmds = append(actionCmds, rolloutActionCmd(a.name, a.reason))
	}

	rolloutsListCmd.Flags().String("status", "", "Filter by status (active|paused|succeeded|rolled_back|...)")
	for _, c := range actionCmds {
		c.Flags().String("reason", "", "Audit reason (required for rollback and force-promote)")
	}

	rolloutsCmd.AddCommand(rolloutsListCmd, rolloutsGetCmd)
	for _, c := range actionCmds {
		rolloutsCmd.AddCommand(c)
	}
	rootCmd.AddCommand(rolloutsCmd)
}
