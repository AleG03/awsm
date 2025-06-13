package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"awsm/internal/util"

	"github.com/spf13/cobra"
)

var execCmd = &cobra.Command{
	Use:   "exec <profile> -- <command> [args...]",
	Short: "Execute a command with temporary credentials",
	Long: `Assumes the role for the given profile and executes a command with
the temporary credentials set as environment variables. Handles IAM and SSO profiles.

Example:
  awsm exec my-profile -- aws s3 ls`,
	// This tells cobra to stop parsing flags after --
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) < 2 || args[1] != "--" {
			return fmt.Errorf("usage: awsm exec <profile> -- <command> [args...]")
		}
		profileName := args[0]
		commandAndArgs := args[2:]

		if len(commandAndArgs) == 0 {
			return fmt.Errorf("no command provided to execute")
		}

		creds, isStatic, err := getCredentialsForProfile(profileName)
		if err != nil {
			return fmt.Errorf("error getting credentials: %w", err)
		}

		command := exec.Command(commandAndArgs[0], commandAndArgs[1:]...)
		command.Stdout = os.Stdout
		command.Stderr = os.Stderr
		command.Stdin = os.Stdin

		// Start with the current environment
		env := os.Environ()
		// Always set the AWS_PROFILE
		env = append(env, fmt.Sprintf("AWS_PROFILE=%s", profileName))

		// If the profile returned dynamic credentials, set them
		if !isStatic && creds != nil {
			env = append(env, fmt.Sprintf("AWS_ACCESS_KEY_ID=%s", creds.AccessKeyId))
			env = append(env, fmt.Sprintf("AWS_SECRET_ACCESS_KEY=%s", creds.SecretAccessKey))
			env = append(env, fmt.Sprintf("AWS_SESSION_TOKEN=%s", creds.SessionToken))
		}
		command.Env = env

		util.SuccessColor.Printf("Executing '%s' with profile '%s'...\n\n", commandAndArgs[0], profileName)
		return command.Run()
	},
}

func init() {
	rootCmd.AddCommand(execCmd)
}
