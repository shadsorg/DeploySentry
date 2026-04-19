package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var strategiesCmd = &cobra.Command{
	Use:     "strategies",
	Aliases: []string{"strategy", "strat"},
	Short:   "Manage rollout strategy templates",
}

var strategiesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List strategies visible at the current scope (org/project/app)",
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := strategyScopePath()
		if err != nil {
			return err
		}
		client, err := clientFromConfig()
		if err != nil {
			return err
		}
		resp, err := client.get(path + "/strategies")
		if err != nil {
			return err
		}
		b, err := json.Marshal(resp)
		if err != nil {
			return err
		}
		var result struct {
			Items []struct {
				Strategy struct {
					Name       string `json:"name"`
					TargetType string `json:"target_type"`
					Version    int    `json:"version"`
				} `json:"strategy"`
				OriginScope struct {
					Type string `json:"type"`
				} `json:"origin_scope"`
				IsInherited bool `json:"is_inherited"`
			} `json:"items"`
		}
		if err := json.Unmarshal(b, &result); err != nil {
			return err
		}
		fmt.Printf("%-30s %-10s %-8s %-12s\n", "NAME", "TARGET", "VERSION", "ORIGIN")
		for _, it := range result.Items {
			origin := it.OriginScope.Type
			if it.IsInherited {
				origin += " (inh)"
			}
			fmt.Printf("%-30s %-10s %-8d %-12s\n", it.Strategy.Name, it.Strategy.TargetType, it.Strategy.Version, origin)
		}
		return nil
	},
}

var strategiesGetCmd = &cobra.Command{
	Use:   "get <name>",
	Short: "Fetch a strategy by name",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := strategyScopePath()
		if err != nil {
			return err
		}
		client, err := clientFromConfig()
		if err != nil {
			return err
		}
		resp, err := client.get(path + "/strategies/" + args[0])
		if err != nil {
			return err
		}
		out, _ := json.MarshalIndent(resp, "", "  ")
		fmt.Println(string(out))
		return nil
	},
}

var strategiesApplyCmd = &cobra.Command{
	Use:   "apply -f <file.yaml>",
	Short: "Create or update a strategy from YAML",
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := strategyScopePath()
		if err != nil {
			return err
		}
		file, _ := cmd.Flags().GetString("file")
		if file == "" {
			return fmt.Errorf("--file is required")
		}
		b, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		client, err := clientFromConfig()
		if err != nil {
			return err
		}
		body, err := client.postRaw(path+"/strategies/import", "application/yaml", b)
		if err != nil {
			return err
		}
		fmt.Println(string(body))
		return nil
	},
}

var strategiesDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a strategy (blocked if referenced)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := strategyScopePath()
		if err != nil {
			return err
		}
		client, err := clientFromConfig()
		if err != nil {
			return err
		}
		_, err = client.delete(path + "/strategies/" + args[0])
		return err
	},
}

var strategiesExportCmd = &cobra.Command{
	Use:   "export <name>",
	Short: "Export a strategy as YAML",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := strategyScopePath()
		if err != nil {
			return err
		}
		client, err := clientFromConfig()
		if err != nil {
			return err
		}
		body, err := client.getRaw(path + "/strategies/" + args[0] + "/export")
		if err != nil {
			return err
		}
		out, _ := cmd.Flags().GetString("out")
		if out == "" {
			fmt.Print(string(body))
			return nil
		}
		return os.WriteFile(out, body, 0o644)
	},
}

// strategyScopePath returns the API URL prefix for the current org/project/app scope.
// --org is required. --project is optional. --app requires --project.
func strategyScopePath() (string, error) {
	org := viper.GetString("org")
	if org == "" {
		return "", fmt.Errorf("organization is required; set via --org flag, DEPLOYSENTRY_ORG env var, or config file")
	}
	proj := viper.GetString("project")
	app := viper.GetString("app")

	path := "/api/v1/orgs/" + org
	if proj != "" {
		path += "/projects/" + proj
	}
	if app != "" {
		if proj == "" {
			return "", fmt.Errorf("--project is required when --app is given")
		}
		path += "/apps/" + app
	}
	return path, nil
}

func init() {
	// --app is not a root persistent flag; declare it as a persistent flag on strategiesCmd.
	strategiesCmd.PersistentFlags().String("app", "", "application slug (optional, for app scope)")
	_ = viper.BindPFlag("app", strategiesCmd.PersistentFlags().Lookup("app"))

	strategiesApplyCmd.Flags().StringP("file", "f", "", "YAML file to apply")
	strategiesExportCmd.Flags().String("out", "", "write output to file (default: stdout)")

	strategiesCmd.AddCommand(strategiesListCmd, strategiesGetCmd, strategiesApplyCmd, strategiesDeleteCmd, strategiesExportCmd)

	rootCmd.AddCommand(strategiesCmd)
}
