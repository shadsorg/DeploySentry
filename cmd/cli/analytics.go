package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

// analyticsCmd is the parent command for analytics operations.
var analyticsCmd = &cobra.Command{
	Use:     "analytics",
	Aliases: []string{"stats"},
	Short:   "View analytics and performance metrics",
	Long: `Access analytics data for flags, deployments, and system performance.

Analytics help you understand how your feature flags and deployments are
performing, identify trends, and optimize your deployment strategies.

Examples:
  # View high-level analytics summary
  deploysentry analytics summary --env production --time-range 7d

  # Get detailed flag usage statistics
  deploysentry analytics flags stats --env production --time-range 24h

  # Monitor specific flag performance
  deploysentry analytics flags usage my-feature --env production --time-range 7d

  # Check deployment performance metrics
  deploysentry analytics deployments stats --env production --time-range 30d

  # Monitor system health in real-time
  deploysentry analytics health --watch`,
}

var analyticsaSummaryCmd = &cobra.Command{
	Use:   "summary",
	Short: "View high-level analytics summary",
	Long: `Display a comprehensive overview of system performance including
flag evaluations, deployment success rates, and API performance metrics.

Examples:
  # View summary for production environment over the last 7 days
  deploysentry analytics summary --env production --time-range 7d

  # View summary with JSON output
  deploysentry analytics summary --env production --time-range 24h -o json`,
	RunE: runAnalyticsSummary,
}

var analyticsFlagsCmd = &cobra.Command{
	Use:   "flags",
	Short: "View flag analytics",
	Long: `Access detailed analytics for feature flags including evaluation
counts, result distribution, and performance metrics.

Subcommands:
  stats     View aggregate flag statistics
  usage     View specific flag usage details`,
}

var analyticsFlagsStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "View flag statistics",
	Long: `Display aggregate statistics for all flags including evaluation
counts, cache hit rates, and performance metrics.

Examples:
  # View flag stats for the last 24 hours
  deploysentry analytics flags stats --env production --time-range 24h

  # View top 10 most active flags
  deploysentry analytics flags stats --env production --time-range 7d --limit 10

  # Export flag stats as JSON
  deploysentry analytics flags stats --env production --time-range 7d -o json`,
	RunE: runAnalyticsFlagsStats,
}

var analyticsFlagsUsageCmd = &cobra.Command{
	Use:   "usage <flag-key>",
	Short: "View specific flag usage details",
	Long: `Display detailed usage analytics for a specific flag including
evaluation trends, result distribution, and performance over time.

Examples:
  # View usage for a specific flag
  deploysentry analytics flags usage my-feature --env production --time-range 7d

  # View usage with hourly breakdown
  deploysentry analytics flags usage my-feature --env production --time-range 24h --breakdown hourly`,
	Args: cobra.ExactArgs(1),
	RunE: runAnalyticsFlagsUsage,
}

var analyticsDeploymentsCmd = &cobra.Command{
	Use:     "deployments",
	Aliases: []string{"deploy", "deploys"},
	Short:   "View deployment analytics",
	Long: `Access analytics for deployment performance including success rates,
duration metrics, and strategy effectiveness.`,
}

var analyticsDeploymentsStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "View deployment statistics",
	Long: `Display aggregate deployment statistics including success rates,
average duration, and strategy performance.

Examples:
  # View deployment stats for all environments
  deploysentry analytics deployments stats --time-range 30d

  # View stats for specific environment
  deploysentry analytics deployments stats --env production --time-range 7d

  # Compare deployment strategies
  deploysentry analytics deployments stats --time-range 30d --breakdown strategy`,
	RunE: runAnalyticsDeploymentsStats,
}

var analyticsHealthCmd = &cobra.Command{
	Use:   "health",
	Short: "View system health metrics",
	Long: `Display real-time system health metrics including API performance,
resource usage, database metrics, and cache performance.

Examples:
  # View current system health
  deploysentry analytics health

  # Monitor health with live updates
  deploysentry analytics health --watch

  # View detailed health metrics
  deploysentry analytics health --detailed`,
	RunE: runAnalyticsHealth,
}

var analyticsExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export analytics data",
	Long: `Export analytics data in various formats for further analysis
or integration with external tools.

Examples:
  # Export all analytics data for a date range
  deploysentry analytics export --start-date 2026-03-01 --end-date 2026-03-07 --format csv

  # Export only flag analytics
  deploysentry analytics export --type flags --time-range 30d --format json`,
	RunE: runAnalyticsExport,
}

func init() {
	// analytics summary flags
	analyticsaSummaryCmd.Flags().String("time-range", "24h", "time range: 1h, 6h, 24h, 7d, 30d")

	// analytics flags stats flags
	analyticsFlagsStatsCmd.Flags().String("time-range", "24h", "time range: 1h, 6h, 24h, 7d, 30d")
	analyticsFlagsStatsCmd.Flags().Int("limit", 50, "limit number of results")

	// analytics flags usage flags
	analyticsFlagsUsageCmd.Flags().String("time-range", "24h", "time range: 1h, 6h, 24h, 7d, 30d")
	analyticsFlagsUsageCmd.Flags().String("breakdown", "daily", "breakdown: hourly, daily, weekly")

	// analytics deployments stats flags
	analyticsDeploymentsStatsCmd.Flags().String("time-range", "7d", "time range: 1h, 6h, 24h, 7d, 30d")
	analyticsDeploymentsStatsCmd.Flags().String("breakdown", "", "breakdown: strategy, environment")

	// analytics health flags
	analyticsHealthCmd.Flags().Bool("watch", false, "continuously update health metrics")
	analyticsHealthCmd.Flags().Bool("detailed", false, "show detailed health metrics")
	analyticsHealthCmd.Flags().Int("interval", 5, "update interval in seconds (when using --watch)")

	// analytics export flags
	analyticsExportCmd.Flags().String("start-date", "", "start date (YYYY-MM-DD)")
	analyticsExportCmd.Flags().String("end-date", "", "end date (YYYY-MM-DD)")
	analyticsExportCmd.Flags().String("time-range", "", "time range: 1h, 6h, 24h, 7d, 30d (alternative to start/end dates)")
	analyticsExportCmd.Flags().String("format", "json", "export format: json, csv")
	analyticsExportCmd.Flags().String("type", "", "data type to export: flags, deployments, all")

	// Build command tree
	analyticsFlagsCmd.AddCommand(analyticsFlagsStatsCmd)
	analyticsFlagsCmd.AddCommand(analyticsFlagsUsageCmd)

	analyticsDeploymentsCmd.AddCommand(analyticsDeploymentsStatsCmd)

	analyticsCmd.AddCommand(analyticsaSummaryCmd)
	analyticsCmd.AddCommand(analyticsFlagsCmd)
	analyticsCmd.AddCommand(analyticsDeploymentsCmd)
	analyticsCmd.AddCommand(analyticsHealthCmd)
	analyticsCmd.AddCommand(analyticsExportCmd)

	rootCmd.AddCommand(analyticsCmd)
}

func runAnalyticsSummary(cmd *cobra.Command, args []string) error {
	org, err := requireOrg()
	if err != nil {
		return err
	}
	project, err := requireProject()
	if err != nil {
		return err
	}
	env, err := requireEnv()
	if err != nil {
		return err
	}

	client, err := clientFromConfig()
	if err != nil {
		return err
	}

	timeRange, _ := cmd.Flags().GetString("time-range")
	path := fmt.Sprintf("/api/v1/orgs/%s/projects/%s/analytics/summary?environment=%s&time_range=%s",
		org, project, env, timeRange)

	resp, err := client.get(path)
	if err != nil {
		return fmt.Errorf("failed to get analytics summary: %w", err)
	}

	if getOutputFormat() == "json" {
		data, _ := json.MarshalIndent(resp, "", "  ")
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	// Format table output
	fmt.Fprintf(cmd.OutOrStdout(), "Analytics Summary (%s)\n", timeRange)
	fmt.Fprintln(cmd.OutOrStdout(), strings.Repeat("=", 50))

	if summary, ok := resp["summary"].(map[string]interface{}); ok {
		if flags, ok := summary["flags"].(map[string]interface{}); ok {
			fmt.Fprintf(cmd.OutOrStdout(), "\n%s Flag Performance:\n", colorBold("📊"))
			if evals, ok := flags["total_evaluations"].(float64); ok {
				fmt.Fprintf(cmd.OutOrStdout(), "  Total Evaluations: %s\n", colorGreen(formatNumber(int(evals))))
			}
			if activeFlags, ok := flags["active_flags"].(float64); ok {
				fmt.Fprintf(cmd.OutOrStdout(), "  Active Flags: %s\n", colorCyan(formatNumber(int(activeFlags))))
			}
			if cacheHitRate, ok := flags["cache_hit_rate"].(float64); ok {
				color := colorGreen
				if cacheHitRate < 90 {
					color = colorYellow
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  Cache Hit Rate: %s\n", color(fmt.Sprintf("%.1f%%", cacheHitRate)))
			}
		}

		if deploys, ok := summary["deployments"].(map[string]interface{}); ok {
			fmt.Fprintf(cmd.OutOrStdout(), "\n%s Deployment Performance:\n", colorBold("🚀"))
			if totalDeploys, ok := deploys["total_deployments"].(float64); ok {
				fmt.Fprintf(cmd.OutOrStdout(), "  Total Deployments: %s\n", colorGreen(formatNumber(int(totalDeploys))))
			}
			if successRate, ok := deploys["success_rate"].(float64); ok {
				color := colorGreen
				if successRate < 95 {
					color = colorYellow
				}
				if successRate < 90 {
					color = colorRed
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  Success Rate: %s\n", color(fmt.Sprintf("%.1f%%", successRate)))
			}
			if avgDuration, ok := deploys["average_duration_minutes"].(float64); ok {
				fmt.Fprintf(cmd.OutOrStdout(), "  Average Duration: %s\n", colorCyan(fmt.Sprintf("%.1f min", avgDuration)))
			}
		}

		if api, ok := summary["api"].(map[string]interface{}); ok {
			fmt.Fprintf(cmd.OutOrStdout(), "\n%s API Performance:\n", colorBold("⚡"))
			if requests, ok := api["total_requests"].(float64); ok {
				fmt.Fprintf(cmd.OutOrStdout(), "  Total Requests: %s\n", colorGreen(formatNumber(int(requests))))
			}
			if avgLatency, ok := api["average_latency_ms"].(float64); ok {
				color := colorGreen
				if avgLatency > 200 {
					color = colorYellow
				}
				if avgLatency > 500 {
					color = colorRed
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  Average Latency: %s\n", color(fmt.Sprintf("%.0f ms", avgLatency)))
			}
			if errorRate, ok := api["error_rate"].(float64); ok {
				color := colorGreen
				if errorRate > 0.5 {
					color = colorYellow
				}
				if errorRate > 1.0 {
					color = colorRed
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  Error Rate: %s\n", color(fmt.Sprintf("%.2f%%", errorRate)))
			}
		}
	}

	return nil
}

func runAnalyticsFlagsStats(cmd *cobra.Command, args []string) error {
	org, err := requireOrg()
	if err != nil {
		return err
	}
	project, err := requireProject()
	if err != nil {
		return err
	}
	env, err := requireEnv()
	if err != nil {
		return err
	}

	client, err := clientFromConfig()
	if err != nil {
		return err
	}

	timeRange, _ := cmd.Flags().GetString("time-range")
	limit, _ := cmd.Flags().GetInt("limit")

	path := fmt.Sprintf("/api/v1/orgs/%s/projects/%s/analytics/flags/stats?environment=%s&time_range=%s&limit=%d",
		org, project, env, timeRange, limit)

	resp, err := client.get(path)
	if err != nil {
		return fmt.Errorf("failed to get flag stats: %w", err)
	}

	if getOutputFormat() == "json" {
		data, _ := json.MarshalIndent(resp, "", "  ")
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	flags, ok := resp["flags"].([]interface{})
	if !ok || len(flags) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No flag statistics found.")
		return nil
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "FLAG KEY\tEVALUATIONS\tCACHE HIT %\tAVG LATENCY\tERROR RATE")
	fmt.Fprintln(w, strings.Repeat("-", 70))

	for _, f := range flags {
		flag, ok := f.(map[string]interface{})
		if !ok {
			continue
		}

		key, _ := flag["flag_key"].(string)
		evals, _ := flag["total_evaluations"].(float64)
		cacheHit, _ := flag["cache_hit_rate"].(float64)
		latency, _ := flag["average_latency_ms"].(float64)
		errorRate, _ := flag["error_rate"].(float64)

		fmt.Fprintf(w, "%s\t%s\t%.1f%%\t%.0f ms\t%.2f%%\n",
			key, formatNumber(int(evals)), cacheHit, latency, errorRate)
	}

	return w.Flush()
}

func runAnalyticsFlagsUsage(cmd *cobra.Command, args []string) error {
	org, err := requireOrg()
	if err != nil {
		return err
	}
	project, err := requireProject()
	if err != nil {
		return err
	}
	env, err := requireEnv()
	if err != nil {
		return err
	}

	client, err := clientFromConfig()
	if err != nil {
		return err
	}

	flagKey := args[0]
	timeRange, _ := cmd.Flags().GetString("time-range")

	path := fmt.Sprintf("/api/v1/orgs/%s/projects/%s/analytics/flags/%s/usage?environment=%s&time_range=%s",
		org, project, flagKey, env, timeRange)

	resp, err := client.get(path)
	if err != nil {
		return fmt.Errorf("failed to get flag usage: %w", err)
	}

	if getOutputFormat() == "json" {
		data, _ := json.MarshalIndent(resp, "", "  ")
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Flag Usage: %s (%s)\n", flagKey, timeRange)
	fmt.Fprintln(cmd.OutOrStdout(), strings.Repeat("=", 50))

	if usage, ok := resp["usage"].(map[string]interface{}); ok {
		if totalEvals, ok := usage["total_evaluations"].(float64); ok {
			fmt.Fprintf(cmd.OutOrStdout(), "\nTotal Evaluations: %s\n", colorGreen(formatNumber(int(totalEvals))))
		}

		if results, ok := usage["result_distribution"].(map[string]interface{}); ok {
			fmt.Fprintf(cmd.OutOrStdout(), "\nResult Distribution:\n")
			for result, count := range results {
				if c, ok := count.(float64); ok {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s: %s\n", result, colorCyan(formatNumber(int(c))))
				}
			}
		}

		if performance, ok := usage["performance"].(map[string]interface{}); ok {
			fmt.Fprintf(cmd.OutOrStdout(), "\nPerformance:\n")
			if avgLatency, ok := performance["average_latency_ms"].(float64); ok {
				color := colorGreen
				if avgLatency > 10 {
					color = colorYellow
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  Average Latency: %s\n", color(fmt.Sprintf("%.1f ms", avgLatency)))
			}
			if cacheHitRate, ok := performance["cache_hit_rate"].(float64); ok {
				color := colorGreen
				if cacheHitRate < 90 {
					color = colorYellow
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  Cache Hit Rate: %s\n", color(fmt.Sprintf("%.1f%%", cacheHitRate)))
			}
		}
	}

	return nil
}

func runAnalyticsDeploymentsStats(cmd *cobra.Command, args []string) error {
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

	timeRange, _ := cmd.Flags().GetString("time-range")
	env := getEnv()

	params := []string{fmt.Sprintf("project_id=%s", project), fmt.Sprintf("time_range=%s", timeRange)}
	if env != "" {
		params = append(params, fmt.Sprintf("environment=%s", env))
	}

	path := fmt.Sprintf("/api/v1/orgs/%s/analytics/deployments/stats?%s", org, strings.Join(params, "&"))

	resp, err := client.get(path)
	if err != nil {
		return fmt.Errorf("failed to get deployment stats: %w", err)
	}

	if getOutputFormat() == "json" {
		data, _ := json.MarshalIndent(resp, "", "  ")
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Deployment Statistics (%s)\n", timeRange)
	fmt.Fprintln(cmd.OutOrStdout(), strings.Repeat("=", 50))

	if summary, ok := resp["summary"].(map[string]interface{}); ok {
		if totalDeploys, ok := summary["total_deployments"].(float64); ok {
			fmt.Fprintf(cmd.OutOrStdout(), "\nTotal Deployments: %s\n", colorGreen(formatNumber(int(totalDeploys))))
		}
		if successRate, ok := summary["success_rate"].(float64); ok {
			color := colorGreen
			if successRate < 95 {
				color = colorYellow
			}
			if successRate < 90 {
				color = colorRed
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Success Rate: %s\n", color(fmt.Sprintf("%.1f%%", successRate)))
		}
		if avgDuration, ok := summary["average_duration_minutes"].(float64); ok {
			fmt.Fprintf(cmd.OutOrStdout(), "Average Duration: %s\n", colorCyan(fmt.Sprintf("%.1f minutes", avgDuration)))
		}
	}

	if strategies, ok := resp["by_strategy"].([]interface{}); ok && len(strategies) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "\nBy Strategy:\n")
		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "STRATEGY\tDEPLOYMENTS\tSUCCESS RATE\tAVG DURATION")

		for _, s := range strategies {
			strategy, ok := s.(map[string]interface{})
			if !ok {
				continue
			}

			name, _ := strategy["strategy"].(string)
			count, _ := strategy["count"].(float64)
			successRate, _ := strategy["success_rate"].(float64)
			duration, _ := strategy["average_duration_minutes"].(float64)

			fmt.Fprintf(w, "%s\t%s\t%.1f%%\t%.1f min\n",
				name, formatNumber(int(count)), successRate, duration)
		}
		w.Flush()
	}

	return nil
}

func runAnalyticsHealth(cmd *cobra.Command, args []string) error {
	client, err := clientFromConfig()
	if err != nil {
		return err
	}

	watch, _ := cmd.Flags().GetBool("watch")
	detailed, _ := cmd.Flags().GetBool("detailed")
	interval, _ := cmd.Flags().GetInt("interval")

	if watch {
		return watchHealth(client, detailed, interval, cmd)
	}

	return showHealthOnce(client, detailed, cmd)
}

func showHealthOnce(client *apiClient, detailed bool, cmd *cobra.Command) error {
	resp, err := client.get("/api/v1/analytics/health")
	if err != nil {
		return fmt.Errorf("failed to get system health: %w", err)
	}

	if getOutputFormat() == "json" {
		data, _ := json.MarshalIndent(resp, "", "  ")
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "System Health (%s)\n", time.Now().Format("15:04:05"))
	fmt.Fprintln(cmd.OutOrStdout(), strings.Repeat("=", 50))

	if health, ok := resp["health"].(map[string]interface{}); ok {
		printHealthMetrics(cmd, health, detailed)
	}

	return nil
}

func watchHealth(client *apiClient, detailed bool, interval int, cmd *cobra.Command) error {
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	for {
		// Clear screen
		fmt.Fprint(cmd.OutOrStdout(), "\033[2J\033[H")

		if err := showHealthOnce(client, detailed, cmd); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "\nRefreshing every %d seconds... (Press Ctrl+C to stop)\n", interval)

		select {
		case <-ticker.C:
			// Continue to next iteration
		}
	}
}

func printHealthMetrics(cmd *cobra.Command, health map[string]interface{}, detailed bool) {
	if api, ok := health["api"].(map[string]interface{}); ok {
		fmt.Fprintf(cmd.OutOrStdout(), "\n%s API Performance:\n", colorBold("⚡"))
		if rps, ok := api["requests_per_second"].(float64); ok {
			fmt.Fprintf(cmd.OutOrStdout(), "  Requests/sec: %s\n", colorGreen(fmt.Sprintf("%.1f", rps)))
		}
		if latency, ok := api["avg_latency_ms"].(float64); ok {
			color := colorGreen
			if latency > 200 {
				color = colorYellow
			}
			if latency > 500 {
				color = colorRed
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  Avg Latency: %s\n", color(fmt.Sprintf("%.1f ms", latency)))
		}
		if errorRate, ok := api["error_rate"].(float64); ok {
			color := colorGreen
			if errorRate > 0.5 {
				color = colorYellow
			}
			if errorRate > 1.0 {
				color = colorRed
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  Error Rate: %s\n", color(fmt.Sprintf("%.2f%%", errorRate)))
		}
	}

	if detailed {
		if db, ok := health["database"].(map[string]interface{}); ok {
			fmt.Fprintf(cmd.OutOrStdout(), "\n%s Database:\n", colorBold("💾"))
			if conns, ok := db["connections"].(float64); ok {
				fmt.Fprintf(cmd.OutOrStdout(), "  Connections: %s\n", colorCyan(fmt.Sprintf("%.0f", conns)))
			}
			if queryLatency, ok := db["query_latency_ms"].(float64); ok {
				color := colorGreen
				if queryLatency > 20 {
					color = colorYellow
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  Query Latency: %s\n", color(fmt.Sprintf("%.1f ms", queryLatency)))
			}
		}

		if resources, ok := health["resources"].(map[string]interface{}); ok {
			fmt.Fprintf(cmd.OutOrStdout(), "\n%s Resources:\n", colorBold("🖥️"))
			if cpu, ok := resources["cpu_usage_percent"].(float64); ok {
				color := colorGreen
				if cpu > 60 {
					color = colorYellow
				}
				if cpu > 80 {
					color = colorRed
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  CPU Usage: %s\n", color(fmt.Sprintf("%.1f%%", cpu)))
			}
			if memory, ok := resources["memory_usage_percent"].(float64); ok {
				color := colorGreen
				if memory > 70 {
					color = colorYellow
				}
				if memory > 85 {
					color = colorRed
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  Memory Usage: %s\n", color(fmt.Sprintf("%.1f%%", memory)))
			}
		}
	}
}

func runAnalyticsExport(cmd *cobra.Command, args []string) error {
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

	startDate, _ := cmd.Flags().GetString("start-date")
	endDate, _ := cmd.Flags().GetString("end-date")
	timeRange, _ := cmd.Flags().GetString("time-range")
	format, _ := cmd.Flags().GetString("format")

	if startDate == "" && endDate == "" && timeRange == "" {
		return fmt.Errorf("you must specify either --start-date and --end-date, or --time-range")
	}

	params := []string{fmt.Sprintf("project_id=%s", project)}
	if startDate != "" && endDate != "" {
		params = append(params, fmt.Sprintf("start_date=%s", startDate))
		params = append(params, fmt.Sprintf("end_date=%s", endDate))
	} else if timeRange != "" {
		params = append(params, fmt.Sprintf("time_range=%s", timeRange))
	}
	if format != "" {
		params = append(params, fmt.Sprintf("format=%s", format))
	}

	path := fmt.Sprintf("/api/v1/orgs/%s/analytics/admin/export?%s", org, strings.Join(params, "&"))

	spinner := newSpinner("Exporting analytics data...")
	resp, err := client.get(path)
	spinner.Stop()

	if err != nil {
		return fmt.Errorf("failed to export analytics: %w", err)
	}

	if format == "csv" {
		// Handle CSV response
		if csvData, ok := resp["data"].(string); ok {
			fmt.Fprint(cmd.OutOrStdout(), csvData)
		}
	} else {
		// Handle JSON response
		data, _ := json.MarshalIndent(resp, "", "  ")
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
	}

	return nil
}

// Helper function to format numbers with commas
func formatNumber(n int) string {
	str := strconv.Itoa(n)
	if len(str) <= 3 {
		return str
	}

	result := make([]byte, 0, len(str)+((len(str)-1)/3))
	for i, digit := range []byte(str) {
		if i > 0 && (len(str)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, digit)
	}

	return string(result)
}