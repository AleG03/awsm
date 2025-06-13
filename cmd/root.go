package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "awsm",
	Short: "A fancy CLI to manage your AWS profiles and sessions",
	Long: `AWSM (AWS Manager) is a tool to simplify switching between AWS profiles,
managing regions, and assuming roles with MFA.`,
	// To prevent Cobra from printing usage on every error
	SilenceUsage: true,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		// No need to print the error here, as Cobra does it by default.
		// We only exit with a non-zero status code.
		os.Exit(1)
	}
}

// NOTE: This init() function is now empty.
// Best practice is to have each command file register itself via its own init().
// Since our other command files (console.go, profile.go, etc.) already do this,
// this file no longer needs to do anything. If you add a new command,
// add `rootCmd.AddCommand(newCmd)` in the `init()` of that command's file.
func init() {
	// All commands are now added to rootCmd in their own respective files.
	// This prevents duplication.
}