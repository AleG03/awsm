package cmd

import (
	"awsm/internal/aws"
	"awsm/internal/util"
	"fmt"

	"github.com/spf13/cobra"
)

var ssoAddCmd = &cobra.Command{
	Use:   "add <session-name> <start-url> <region>",
	Short: "Add an SSO session to AWS config",
	Long: `Adds a new SSO session configuration to ~/.aws/config.

Example:
  awsm sso add company https://d-1234567a10.awsapps.com/start/ eu-west-1

This will add:
[sso-session company]
sso_start_url           = https://d-1234567a10.awsapps.com/start/
sso_region              = eu-west-1
sso_registration_scopes = sso:account:access`,
	Args: cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		sessionName := args[0]
		startURL := args[1]
		region := args[2]

		if err := aws.AddSSOSession(sessionName, startURL, region); err != nil {
			return fmt.Errorf("failed to add SSO session: %w", err)
		}

		util.SuccessColor.Printf("✔ SSO session '%s' added successfully to ~/.aws/config\n", sessionName)
		return nil
	},
}

func init() {
	ssoCmd.AddCommand(ssoAddCmd)
}
