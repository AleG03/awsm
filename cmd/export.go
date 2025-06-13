package cmd

import (
	"fmt"
	"os"

	"awsm/internal/util"

	"github.com/spf13/cobra"
)

var useCmd = &cobra.Command{
	Use:   "use <profile>",
	Short: "Alias for 'export'. Use with eval: eval $(awsm use <profile>)",
	Long: `DEPRECATED. This command is an alias for 'export'.
It generates shell commands to set temporary AWS credentials for any profile type.
Use it with your shell's eval command:

  eval $(awsm use my-profile-name)

This is the recommended way to set credentials for your current shell session.`,
	Args: cobra.ExactArgs(1),
	RunE: runExport,
}

var exportCmd = &cobra.Command{
	Use:   "export <profile>",
	Short: "Export temporary credentials for a profile (IAM or SSO)",
	Long: `Generates shell commands to set temporary AWS credentials.
This command is smart:
- For IAM profiles with MFA/role_arn, it will prompt for an MFA token.
- For SSO profiles, it will use the cached SSO session to get credentials.
  (If the session is expired, run 'awsm sso login <profile>' first).
- For static profiles, it will set AWS_PROFILE and unset temporary keys.

Use it with your shell's eval command:

  eval $(awsm export my-profile-name)`,
	Args: cobra.ExactArgs(1),
	RunE: runExport,
}

func runExport(cmd *cobra.Command, args []string) error {
	profileName := args[0]
	creds, isStatic, err := getCredentialsForProfile(profileName)
	if err != nil {
		// Print user-facing error to stderr
		fmt.Fprintln(os.Stderr, util.ErrorColor.Sprintf("Error: %v", err))
		// Return a generic error so `eval` doesn't try to execute the error message
		return fmt.Errorf("credential acquisition failed")
	}

	if isStatic {
		fmt.Printf("export AWS_PROFILE='%s';\n", profileName)
		fmt.Println("unset AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY AWS_SESSION_TOKEN;")
		util.SuccessColor.Fprintf(os.Stderr, "✔ Switched to static profile '%s'.\n", profileName)
		return nil
	}

	// Print export commands to stdout for `eval` to capture
	fmt.Printf("export AWS_ACCESS_KEY_ID='%s';\n", creds.AccessKeyId)
	fmt.Printf("export AWS_SECRET_ACCESS_KEY='%s';\n", creds.SecretAccessKey)
	fmt.Printf("export AWS_SESSION_TOKEN='%s';\n", creds.SessionToken)
	fmt.Printf("export AWS_PROFILE='%s';\n", profileName)

	// Print success message to stderr for the user to see
	util.SuccessColor.Fprintf(os.Stderr, "✔ Credentials for profile '%s' are set.\n", profileName)
	return nil
}

func init() {
	rootCmd.AddCommand(useCmd)
	rootCmd.AddCommand(exportCmd)
}
