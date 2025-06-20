package cmd

import (
	"awsm/internal/aws"
	"awsm/internal/util"

	"github.com/spf13/cobra"
)

var clearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear the currently set profile and region from default credentials",
	Long: `Removes all credentials and region information from the default profile in ~/.aws/credentials.
This effectively clears any active AWS session.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		currentProfile := aws.GetCurrentProfileName()
		if currentProfile == "" {
			util.WarnColor.Println("No active profile found to clear.")
			return nil
		}

		util.InfoColor.Printf("Clearing profile '%s' from default credentials...\n", util.BoldColor.Sprint(currentProfile))

		if err := aws.ClearDefaultProfile(); err != nil {
			return err
		}

		util.SuccessColor.Println("✔ Default profile cleared successfully.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(clearCmd)
}
