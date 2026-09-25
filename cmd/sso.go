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
	Long:              `Signs in to the SSO session and renews the active profile if it uses that session.`,
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeSSOSessions,
	RunE: func(cmd *cobra.Command, args []string) error {
		return loginAndRefreshActive(cmd.Context(), args[0], aws.PerformSSOLoginContext, func(profile string) (*aws.TempCredentials, bool, error) {
			return aws.GetFreshCredentialsForProfile(profile)
		})
	},
}

func init() {
	ssoCmd.AddCommand(ssoLoginCmd)
	rootCmd.AddCommand(ssoCmd)
}
