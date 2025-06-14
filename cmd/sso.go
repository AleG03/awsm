package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"awsm/internal/aws"
	"awsm/internal/util"

	"github.com/spf13/cobra"
)

var ssoCmd = &cobra.Command{
	Use:   "sso",
	Short: "Manage AWS SSO (IAM Identity Center) sessions",
}

var ssoLoginCmd = &cobra.Command{
	Use:   "login <profile>",
	Short: "Log in to an SSO session",
	Long:  `Initiates the AWS SSO login flow. This command only handles the login process and does not activate the profile or export credentials.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		profileName := args[0]
		ssoSession, err := aws.GetSsoSessionForProfile(profileName)
		if err != nil {
			return err
		}

		util.InfoColor.Fprintf(os.Stderr, "Attempting SSO login for session: %s\n", util.BoldColor.Sprint(ssoSession))
		util.InfoColor.Fprintln(os.Stderr, "Your browser should open. Please follow the instructions.")

		awsCmd := exec.Command("aws", "sso", "login", "--sso-session", ssoSession)
		awsCmd.Stdin = os.Stdin
		awsCmd.Stdout = os.Stderr
		awsCmd.Stderr = os.Stderr

		if err := awsCmd.Run(); err != nil {
			return fmt.Errorf("aws sso login failed: %w", err)
		}
		util.SuccessColor.Fprintln(os.Stderr, "\n✔ SSO login successful.")
		return nil
	},
}


func init() {
	ssoCmd.AddCommand(ssoLoginCmd)
	rootCmd.AddCommand(ssoCmd)
}
