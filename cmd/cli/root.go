package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	// Version information set at build time via ldflags.
	version   = "dev"
	commit    = "none"
	buildDate = "unknown"

	cfgFile string
	noColor bool
)

// ---------------------------------------------------------------------------
// ANSI color helpers
// ---------------------------------------------------------------------------

const (
	ansiReset  = "\033[0m"
	ansiBold   = "\033[1m"
	ansiRed    = "\033[31m"
	ansiGreen  = "\033[32m"
	ansiYellow = "\033[33m"
	ansiCyan   = "\033[36m"
)

// colorEnabled reports whether colored output should be used. Color is
// disabled when the --no-color flag is set or the NO_COLOR environment
// variable is present (see https://no-color.org).
func colorEnabled() bool {
	if noColor {
		return false
	}
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		return false
	}
	return true
}

// colorGreen wraps s in green ANSI escape codes.
func colorGreen(s string) string {
	if !colorEnabled() {
		return s
	}
	return ansiGreen + s + ansiReset
}

// colorRed wraps s in red ANSI escape codes.
func colorRed(s string) string {
	if !colorEnabled() {
		return s
	}
	return ansiRed + s + ansiReset
}

// colorYellow wraps s in yellow ANSI escape codes.
func colorYellow(s string) string {
	if !colorEnabled() {
		return s
	}
	return ansiYellow + s + ansiReset
}

// colorCyan wraps s in cyan ANSI escape codes.
func colorCyan(s string) string {
	if !colorEnabled() {
		return s
	}
	return ansiCyan + s + ansiReset
}

// colorBold wraps s in bold ANSI escape codes.
func colorBold(s string) string {
	if !colorEnabled() {
		return s
	}
	return ansiBold + s + ansiReset
}

// ---------------------------------------------------------------------------
// Spinner / progress indicator
// ---------------------------------------------------------------------------

// spinner displays an animated progress indicator for long-running operations.
type spinner struct {
	message string
	done    chan struct{}
	once    sync.Once
}

// newSpinner creates and starts a spinner that writes to stderr.
func newSpinner(message string) *spinner {
	s := &spinner{
		message: message,
		done:    make(chan struct{}),
	}
	go s.run()
	return s
}

// run drives the spinner animation until Stop is called.
func (s *spinner) run() {
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	ticker := time.NewTicker(80 * time.Millisecond)
	defer ticker.Stop()

	i := 0
	for {
		select {
		case <-s.done:
			// Clear the spinner line.
			_, _ = fmt.Fprintf(os.Stderr, "\r\033[K")
			return
		case <-ticker.C:
			frame := frames[i%len(frames)]
			if colorEnabled() {
				_, _ = fmt.Fprintf(os.Stderr, "\r%s %s", colorCyan(frame), s.message)
			} else {
				_, _ = fmt.Fprintf(os.Stderr, "\r%s %s", frame, s.message)
			}
			i++
		}
	}
}

// Stop terminates the spinner. It is safe to call multiple times.
func (s *spinner) Stop() {
	s.once.Do(func() {
		close(s.done)
		// Small sleep to let the goroutine clean up the line.
		time.Sleep(100 * time.Millisecond)
	})
}

// StopWithMessage terminates the spinner and prints a final status message.
func (s *spinner) StopWithMessage(msg string) {
	s.Stop()
	_, _ = fmt.Fprintln(os.Stderr, msg)
}

var rootCmd = &cobra.Command{
	Use:   "deploysentry",
	Short: "DeploySentry - deploy release and feature flag management",
	Long: `DeploySentry CLI provides a unified interface for managing deployments,
releases, and feature flags across your infrastructure.

Use DeploySentry to safely roll out changes with canary deployments,
control feature flags in real time, and promote releases across
environments with full observability.

Configuration can be provided via:
  - Flags on each command
  - Environment variables prefixed with DEPLOYSENTRY_
  - A config file (.deploysentry.yml in the current directory or $HOME)

Examples:
  # Initialize a project configuration
  deploysentry config init

  # Create a new deployment with canary strategy
  deploysentry deploy create --release v1.2.0 --env production --strategy canary

  # Toggle a feature flag on
  deploysentry flags toggle my-feature --on

  # List recent releases
  deploysentry releases list`,
	Version:       version,
	SilenceUsage:  true,
	SilenceErrors: true,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version information",
	Long:  `Display the version, commit hash, and build date of the DeploySentry CLI.`,
	Run: func(cmd *cobra.Command, args []string) {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "DeploySentry CLI\n")
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Version:    %s\n", version)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Commit:     %s\n", commit)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Build Date: %s\n", buildDate)
	},
}

// completionCmd generates shell completion scripts for bash, zsh, and fish.
var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish]",
	Short: "Generate shell completion scripts",
	Long: `Generate shell completion scripts for DeploySentry CLI.

To load completions:

Bash:
  $ source <(deploysentry completion bash)

  # To load completions for each session, execute once:
  # Linux:
  $ deploysentry completion bash > /etc/bash_completion.d/deploysentry
  # macOS:
  $ deploysentry completion bash > $(brew --prefix)/etc/bash_completion.d/deploysentry

Zsh:
  # If shell completion is not already enabled in your environment,
  # you will need to enable it. You can execute the following once:
  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  # To load completions for each session, execute once:
  $ deploysentry completion zsh > "${fpath[1]}/_deploysentry"

  # You will need to start a new shell for this setup to take effect.

Fish:
  $ deploysentry completion fish | source

  # To load completions for each session, execute once:
  $ deploysentry completion fish > ~/.config/fish/completions/deploysentry.fish
`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish"},
	Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return rootCmd.GenBashCompletionV2(cmd.OutOrStdout(), true)
		case "zsh":
			return rootCmd.GenZshCompletion(cmd.OutOrStdout())
		case "fish":
			return rootCmd.GenFishCompletion(cmd.OutOrStdout(), true)
		default:
			return fmt.Errorf("unsupported shell %q; must be bash, zsh, or fish", args[0])
		}
	},
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global persistent flags available to all commands.
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ./.deploysentry.yml or $HOME/.deploysentry.yml)")
	rootCmd.PersistentFlags().String("org", "", "organization slug")
	rootCmd.PersistentFlags().String("project", "", "project slug")
	rootCmd.PersistentFlags().String("env", "", "target environment (e.g., dev, staging, production)")
	rootCmd.PersistentFlags().StringP("output", "o", "table", "output format: table or json")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "enable verbose output for debugging")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "disable colored output")

	// Bind persistent flags to viper for config-file and env-var support.
	_ = viper.BindPFlag("org", rootCmd.PersistentFlags().Lookup("org"))
	_ = viper.BindPFlag("project", rootCmd.PersistentFlags().Lookup("project"))
	_ = viper.BindPFlag("env", rootCmd.PersistentFlags().Lookup("env"))
	_ = viper.BindPFlag("output", rootCmd.PersistentFlags().Lookup("output"))
	_ = viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(completionCmd)
}

// initConfig reads the config file and environment variables.
func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		// Search for config in the current directory first, then home directory.
		cwd, err := os.Getwd()
		if err == nil {
			viper.AddConfigPath(cwd)
		}

		home, err := os.UserHomeDir()
		if err == nil {
			viper.AddConfigPath(home)
			viper.AddConfigPath(filepath.Join(home, ".config", "deploysentry"))
		}

		viper.SetConfigName(".deploysentry")
		viper.SetConfigType("yaml")
	}

	// Environment variable support: DEPLOYSENTRY_ORG, DEPLOYSENTRY_PROJECT, etc.
	viper.SetEnvPrefix("DEPLOYSENTRY")
	viper.AutomaticEnv()

	// Silently ignore missing config files; they are optional.
	if err := viper.ReadInConfig(); err == nil {
		if viper.GetBool("verbose") {
			_, _ = fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
		}
	}
}

// getOrg returns the organization from flags, config, or env.
func getOrg() string {
	return viper.GetString("org")
}

// getProject returns the project from flags, config, or env.
func getProject() string {
	return viper.GetString("project")
}

// getEnv returns the target environment from flags, config, or env.
func getEnv() string {
	return viper.GetString("env")
}

// getOutputFormat returns the desired output format (table or json).
func getOutputFormat() string {
	return viper.GetString("output")
}

// isVerbose returns whether verbose mode is enabled.
func isVerbose() bool {
	return viper.GetBool("verbose")
}

// requireOrg validates that the org flag is set and returns it.
func requireOrg() (string, error) {
	org := getOrg()
	if org == "" {
		return "", fmt.Errorf("organization is required; set via --org flag, DEPLOYSENTRY_ORG env var, or config file")
	}
	return org, nil
}

// requireProject validates that the project flag is set and returns it.
func requireProject() (string, error) {
	project := getProject()
	if project == "" {
		return "", fmt.Errorf("project is required; set via --project flag, DEPLOYSENTRY_PROJECT env var, or config file")
	}
	return project, nil
}

// requireEnv validates that the env flag is set and returns it.
func requireEnv() (string, error) {
	env := getEnv()
	if env == "" {
		return "", fmt.Errorf("environment is required; set via --env flag, DEPLOYSENTRY_ENV env var, or config file")
	}
	return env, nil
}
