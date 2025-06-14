package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	version       string
	commit        string
	date          string
	chromeProfile string // This will hold the value from the flag
)

var rootCmd = &cobra.Command{
	Use:          "awsm",
	Short:        "A fancy CLI to manage your AWS profiles and sessions",
	Long:         `AWSM (AWS Manager) is a tool to simplify switching between AWS profiles, managing regions, and assuming roles with MFA.`,
	Version:      version,
	SilenceUsage: true,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// SetVersionInfo is called by main.go to pass in build-time variables.
func SetVersionInfo(v, c, d string) {
	version = v
	commit = c
	date = d
	rootCmd.Version = fmt.Sprintf("%s (commit: %s, date: %s)", version, commit, date)
}

func init() {
	// Add a persistent flag to the root command.
	// "Persistent" means it will be available to all subcommands.
	// It will store the provided value in the `chromeProfile` variable.
	rootCmd.PersistentFlags().StringVar(&chromeProfile, "chrome-profile", "", "Specify a Chrome profile alias or directory name (e.g., 'work')")
}
