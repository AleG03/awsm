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

// Autocomplete for SSO session names
func completeSSOSessions(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	sessions, err := aws.ListSSOSessions()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	var matches []string
	for _, s := range sessions {
		if toComplete == "" || len(toComplete) == 0 || (len(s.Name) >= len(toComplete) && s.Name[:len(toComplete)] == toComplete) {
			matches = append(matches, s.Name)
		}
	}
	return matches, cobra.ShellCompDirectiveNoFileComp
}

var ssoLoginCmd = &cobra.Command{
	Use:               "login <sso-session>",
	Short:             "Log in to an SSO session",
	Long:              `Initiates the AWS SSO login flow for the specified SSO session.`,
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeSSOSessions,
	RunE: func(cmd *cobra.Command, args []string) error {
		ssoSession := args[0]

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
