package cmd

import (
	"awsm/internal/aws"
	"awsm/internal/tui"
	"fmt"

	"github.com/spf13/cobra"
)

var syncActiveRegion bool

var profileChangeRegionCmd = &cobra.Command{
	Use:               "change-default-region <profile> <region>",
	Short:             "Change the default region for a profile",
	Long:              `Updates the region setting for the specified profile in ~/.aws/config.`,
	Args:              cobra.ExactArgs(2),
	ValidArgsFunction: completeProfiles,
	RunE: func(cmd *cobra.Command, args []string) error {
		profileName := args[0]
		region := args[1]

		if !aws.IsValidRegion(region) {
			return fmt.Errorf("invalid region: %s", region)
		}

		change := aws.ChangeProfileRegion
		if syncActiveRegion {
			change = aws.ChangeProfileRegionAndActive
		}
		if err := change(profileName, region); err != nil {
			return fmt.Errorf("failed to change region: %w", err)
		}

		tui.PrintSuccess(fmt.Sprintf("Region for profile '%s' changed to '%s'", profileName, region))
		return nil
	},
}

func init() {
	profileChangeRegionCmd.Flags().BoolVar(&syncActiveRegion, "sync-active", false, "Also update default credentials if this profile is active")
	profileCmd.AddCommand(profileChangeRegionCmd)
}
