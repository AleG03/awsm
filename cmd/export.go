package cmd

import (
	"errors"
	"fmt"
	"os"

	"awsm/internal/aws"
	"awsm/internal/util"

	"github.com/spf13/cobra"
)

// --- Command Definitions ---

var exportCmd = &cobra.Command{
	Use:               "export <profile>",
	Short:             "Export temporary credentials for a profile (IAM or SSO)",
	Long:              `This is the main plumbing command used by the 'awsmp' shell alias.`,
	Args:              cobra.ExactArgs(1),
	RunE:              runExport,
	ValidArgsFunction: completeProfiles,
}

// --- Main Logic ---

func runExport(cmd *cobra.Command, args []string) error {
	profileName := args[0]

	creds, isStatic, err := aws.GetCredentialsForProfile(profileName)
	if err != nil {
		if errors.Is(err, aws.ErrSsoSessionExpired) {
			fmt.Fprintln(os.Stderr, util.WarnColor.Sprintf("SSO session for profile '%s' has expired. Please log in again.", profileName))
			os.Exit(10) // Exit with special code 10 to signal the shell wrapper.
			return nil
		}
		fmt.Fprintln(os.Stderr, util.ErrorColor.Sprintf("Error: %v", err))
		return fmt.Errorf("credential acquisition failed")
	}

	if isStatic {
		fmt.Printf("export AWS_PROFILE='%s';\n", profileName)
		fmt.Println("unset AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY AWS_SESSION_TOKEN;")
		util.SuccessColor.Fprintf(os.Stderr, "✔ Switched to profile '%s'.\n", profileName)
		return nil
	}

	fmt.Printf("export AWS_ACCESS_KEY_ID='%s';\n", creds.AccessKeyId)
	fmt.Printf("export AWS_SECRET_ACCESS_KEY='%s';\n", creds.SecretAccessKey)
	fmt.Printf("export AWS_SESSION_TOKEN='%s';\n", creds.SessionToken)
	fmt.Printf("export AWS_PROFILE='%s';\n", profileName)

	util.SuccessColor.Fprintf(os.Stderr, "✔ Credentials for profile '%s' are set.\n", profileName)
	return nil
}

// --- Autocompletion Logic ---

func completeProfiles(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) != 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	profiles, err := aws.ListProfiles()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	return profiles, cobra.ShellCompDirectiveNoFileComp
}

// --- Initialization ---

func init() {
	rootCmd.AddCommand(exportCmd)
}
