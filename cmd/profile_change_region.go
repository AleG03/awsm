package cmd

import (
	"awsm/internal/aws"
	"awsm/internal/util"
	"fmt"

	"github.com/spf13/cobra"
)

var profileChangeRegionCmd = &cobra.Command{
	Use:               "change-default-region <profile> <region>",
	Short:             "Change the default region for a profile",
	Long:              `Updates the region setting for the specified profile in ~/.aws/config.`,
	Args:              cobra.ExactArgs(2),
	ValidArgsFunction: completeProfiles,
	RunE: func(cmd *cobra.Command, args []string) error {
		profileName := args[0]
		region := args[1]

		if err := aws.ChangeProfileRegion(profileName, region); err != nil {
			return fmt.Errorf("failed to change region: %w", err)
		}

		util.SuccessColor.Printf("✔ Region for profile '%s' changed to '%s'\n", profileName, region)
		return nil
	},
}

func init() {
	profileCmd.AddCommand(profileChangeRegionCmd)
}
