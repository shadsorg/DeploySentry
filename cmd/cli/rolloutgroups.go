package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

var rolloutGroupsCmd = &cobra.Command{
	Use:     "rollout-groups",
	Aliases: []string{"rollout-group", "rg"},
	Short:   "Manage rollout groups (bundles of related rollouts)",
}

var rolloutGroupsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List rollout groups for an org",
	RunE: func(cmd *cobra.Command, args []string) error {
		org, err := requireOrg()
		if err != nil {
			return err
		}
		client, err := clientFromConfig()
		if err != nil {
			return err
		}
		body, err := client.getRaw("/api/v1/orgs/" + org + "/rollout-groups")
		if err != nil {
			return err
		}
		fmt.Println(string(body))
		return nil
	},
}

var rolloutGroupsGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Show a rollout group's detail",
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
		body, err := client.getRaw("/api/v1/orgs/" + org + "/rollout-groups/" + args[0])
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

var rolloutGroupsCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new rollout group",
	RunE: func(cmd *cobra.Command, args []string) error {
		org, err := requireOrg()
		if err != nil {
			return err
		}

		name, _ := cmd.Flags().GetString("name")
		if name == "" {
			return fmt.Errorf("--name is required")
		}

		description, _ := cmd.Flags().GetString("description")
		policy, _ := cmd.Flags().GetString("policy")

		payload := map[string]string{
			"name": name,
		}
		if description != "" {
			payload["description"] = description
		}
		if policy != "" {
			payload["coordination_policy"] = policy
		}

		body, _ := json.Marshal(payload)
		client, err := clientFromConfig()
		if err != nil {
			return err
		}
		resp, err := client.postRaw("/api/v1/orgs/"+org+"/rollout-groups", "application/json", body)
		if err != nil {
			return err
		}
		fmt.Println(string(resp))
		return nil
	},
}

var rolloutGroupsAttachCmd = &cobra.Command{
	Use:   "attach <group-id>",
	Short: "Attach a rollout to a group",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		org, err := requireOrg()
		if err != nil {
			return err
		}

		rolloutID, _ := cmd.Flags().GetString("rollout")
		if rolloutID == "" {
			return fmt.Errorf("--rollout is required")
		}

		payload, _ := json.Marshal(map[string]string{"rollout_id": rolloutID})
		client, err := clientFromConfig()
		if err != nil {
			return err
		}
		resp, err := client.postRaw("/api/v1/orgs/"+org+"/rollout-groups/"+args[0]+"/attach", "application/json", payload)
		if err != nil {
			return err
		}
		fmt.Println(string(resp))
		return nil
	},
}

func init() {
	rolloutGroupsCreateCmd.Flags().String("name", "", "Name of the rollout group (required)")
	rolloutGroupsCreateCmd.Flags().String("description", "", "Description of the rollout group")
	rolloutGroupsCreateCmd.Flags().String("policy", "", "Coordination policy: independent|pause_on_sibling_abort|cascade_abort")

	rolloutGroupsAttachCmd.Flags().String("rollout", "", "Rollout ID to attach to this group (required)")

	rolloutGroupsCmd.AddCommand(rolloutGroupsListCmd, rolloutGroupsGetCmd, rolloutGroupsCreateCmd, rolloutGroupsAttachCmd)
	rootCmd.AddCommand(rolloutGroupsCmd)
}
