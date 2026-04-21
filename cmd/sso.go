package cmd

import (
	"awsm/internal/aws"

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
		return aws.PerformSSOLogin(args[0])
	},
}

func init() {
	ssoCmd.AddCommand(ssoLoginCmd)
	rootCmd.AddCommand(ssoCmd)
}
