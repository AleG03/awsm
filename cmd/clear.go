package cmd

import (
	"awsm/internal/aws"
	"awsm/internal/tui"
	"fmt"

	"github.com/spf13/cobra"
)

var clearIfProfile string

var clearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear the currently set profile and region from default credentials",
	Long: `Removes all credentials and region information from the default profile in ~/.aws/credentials.
This effectively clears any active AWS session.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if cmd.Flags().Changed("if-profile") {
			if clearIfProfile == "" {
				return fmt.Errorf("--if-profile requires a profile name")
			}
			if err := aws.ClearDefaultProfileIfCurrent(clearIfProfile); err != nil {
				return err
			}
			tui.PrintSuccess("Default profile cleared successfully.")
			return nil
		}
		currentProfile := aws.GetCurrentProfileName()
		if currentProfile == "" {
			tui.PrintWarning("No active profile found to clear.")
			return nil
		}

		tui.PrintInfo(fmt.Sprintf("Clearing profile '%s' from default credentials...", tui.FormatBold(currentProfile)))

		if err := aws.ClearDefaultProfile(); err != nil {
			return err
		}

		tui.PrintSuccess("Default profile cleared successfully.")
		return nil
	},
}

func init() {
	clearCmd.Flags().StringVar(&clearIfProfile, "if-profile", "", "Clear only if this profile is still active (checked atomically)")
	rootCmd.AddCommand(clearCmd)
}
