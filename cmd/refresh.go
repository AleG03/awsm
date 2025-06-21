package cmd

import (
	"awsm/internal/aws"
	"awsm/internal/util"
	"fmt"

	"github.com/spf13/cobra"
)

var refreshCmd = &cobra.Command{
	Use:   "refresh [profile]",
	Short: "Refresh AWS credentials for a profile",
	Long: `Refresh AWS credentials for the specified profile or current active profile.
This command will automatically detect the profile type and refresh accordingly:
- SSO profiles: Runs 'aws sso login'
- IAM profiles: Prompts for new MFA token
- IAM user profiles: Cannot be refreshed`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var profileName string
		if len(args) > 0 {
			profileName = args[0]
		} else {
			// Use current active profile
			profileName = aws.GetCurrentProfileName()
			if profileName == "" {
				return fmt.Errorf("no profile specified and no active profile found")
			}
		}

		util.InfoColor.Printf("Refreshing credentials for profile: %s\n", util.BoldColor.Sprint(profileName))

		if err := aws.AutoRefreshCredentials(profileName); err != nil {
			return fmt.Errorf("failed to refresh credentials: %w", err)
		}

		util.SuccessColor.Printf("✔ Credentials refreshed for profile: %s\n", profileName)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(refreshCmd)
}
