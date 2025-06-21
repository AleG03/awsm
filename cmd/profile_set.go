package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"

	"awsm/internal/aws"
	"awsm/internal/tui"

	"github.com/spf13/cobra"
)

// --- Command Definitions ---
var profileSetCmd = &cobra.Command{
	Use:               "set <profile>",
	Short:             "Set credentials for a profile in the default AWS credentials file",
	Long:              `Updates the default profile in ~/.aws/credentials with the specified profile's credentials.`,
	Args:              cobra.ExactArgs(1),
	RunE:              runProfileSet,
	ValidArgsFunction: completeProfiles,
}

// --- Main Logic ---
func runProfileSet(cmd *cobra.Command, args []string) error {
	profileName := args[0]

	// Get profile region first
	region, err := aws.GetProfileRegion(profileName)
	if err != nil {
		// Region is optional, continue without it
		region = ""
	}

	// Use spinner for credential acquisition
	var creds *aws.TempCredentials
	var isStatic bool
	err = tui.ShowSpinner(context.Background(), fmt.Sprintf("Getting credentials for profile '%s'", profileName), func() error {
		var spinnerErr error
		creds, isStatic, spinnerErr = aws.GetCredentialsForProfile(profileName)
		return spinnerErr
	})

	if err != nil {
		if errors.Is(err, aws.ErrSsoSessionExpired) {
			fmt.Fprintln(os.Stderr, tui.WarningStyle.Render("⚠ SSO session for profile '"+profileName+"' has expired. Please log in again."))
			os.Exit(10) // Exit with special code 10 to signal the shell wrapper.
			return nil
		}
		fmt.Fprintln(os.Stderr, tui.ErrorStyle.Render("✗ Error: "+err.Error()))
		return fmt.Errorf("credential acquisition failed")
	}

	if isStatic {
		err = aws.UpdateStaticProfile(profileName)
		if err != nil {
			fmt.Fprintln(os.Stderr, tui.ErrorStyle.Render("✗ Error updating credentials file: "+err.Error()))
			return fmt.Errorf("failed to update credentials file")
		}
		fmt.Fprintln(os.Stderr, tui.SuccessStyle.Render("✓ Switched to profile '"+profileName+"' in default credentials."))
		return nil
	}

	if creds == nil {
		fmt.Fprintln(os.Stderr, tui.ErrorStyle.Render("✗ Error: No credentials available for profile '"+profileName+"'"))
		return fmt.Errorf("no credentials available")
	}

	err = aws.UpdateCredentialsFile(creds, region, profileName)
	if err != nil {
		fmt.Fprintln(os.Stderr, tui.ErrorStyle.Render("✗ Error updating credentials file: "+err.Error()))
		return fmt.Errorf("failed to update credentials file")
	}

	fmt.Fprintln(os.Stderr, tui.SuccessStyle.Render("✓ Credentials for profile '"+profileName+"' are set."))
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
	// Command will be added to profile subcommand in profile.go
}
