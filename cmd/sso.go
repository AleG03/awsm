package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"awsm/internal/aws"
	"awsm/internal/util"
	ini "gopkg.in/ini.v1"

	"github.com/spf13/cobra"
)

var ssoCmd = &cobra.Command{
	Use:   "sso",
	Short: "Manage AWS SSO (IAM Identity Center) sessions",
}

var ssoLoginCmd = &cobra.Command{
	Use:   "login <profile>",
	Short: "Log in to an AWS SSO session associated with a profile",
	Long: `Starts the AWS SSO login process for a given profile.
This will typically open a browser window for authentication.
This is the equivalent of 'aws sso login --sso-session ...'.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		profileName := args[0]

		// Find the sso_session name from the config file
		configPath, err := aws.GetAWSConfigPath()
		if err != nil {
			return err
		}
		cfgFile, err := ini.Load(configPath)
		if err != nil {
			return fmt.Errorf("failed to read AWS config file: %w", err)
		}

		section, err := cfgFile.GetSection("profile " + profileName)
		if err != nil {
			section, err = cfgFile.GetSection(profileName)
			if err != nil {
				return fmt.Errorf("could not find profile section for '%s'", profileName)
			}
		}

		ssoSession := section.Key("sso_session").String()
		if ssoSession == "" {
			return fmt.Errorf("profile '%s' is not an SSO profile (missing 'sso_session' configuration)", profileName)
		}

		util.InfoColor.Printf("Attempting SSO login for session: %s\n", util.BoldColor.Sprint(ssoSession))
		util.InfoColor.Println("Your browser should open. Please follow the instructions.")

		// We shell out to the aws cli because it handles the complex device flow
		awsCmd := exec.Command("aws", "sso", "login", "--sso-session", ssoSession)
		awsCmd.Stdin = os.Stdin
		awsCmd.Stdout = os.Stdout
		awsCmd.Stderr = os.Stderr

		if err := awsCmd.Run(); err != nil {
			return fmt.Errorf("aws sso login failed: %w", err)
		}

		util.SuccessColor.Println("\n✔ SSO login successful. You can now use this profile.")
		return nil
	},
}

func init() {
	ssoCmd.AddCommand(ssoLoginCmd)
	rootCmd.AddCommand(ssoCmd)
}
