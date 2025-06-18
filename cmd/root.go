package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	version string
	commit  string
	date    string
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
}
