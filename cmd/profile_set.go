package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"awsm/internal/aws"
	"awsm/internal/tui"
	"awsm/internal/util"

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

	// Check if it's an SSO profile and handle login if needed
	if ssoSession, err := aws.GetSsoSessionForProfile(profileName); err == nil {
		// It's an SSO profile, check if login is needed
		if needsLogin, checkErr := checkSSOLoginNeeded(profileName); checkErr == nil && needsLogin {
			if loginErr := performSSOLogin(ssoSession); loginErr != nil {
				return loginErr
			}
		}
	}

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
			os.Exit(1)
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
// completeProfiles provides completion for profile arguments, excluding sso-session profiles
var completeProfiles = aws.CompleteProfilesFiltered(func(profile string) bool {
	return !strings.HasPrefix(profile, "sso-session")
})

// --- Helper Functions ---
func checkSSOLoginNeeded(profileName string) (bool, error) {
	// Try to get credentials to see if SSO session is valid
	_, _, err := aws.GetCredentialsForProfile(profileName)
	if err != nil && errors.Is(err, aws.ErrSsoSessionExpired) {
		return true, nil
	}
	return false, err
}

func performSSOLogin(ssoSession string) error {
	util.InfoColor.Fprintf(os.Stderr, "SSO session expired. Attempting login for session: %s\n", util.BoldColor.Sprint(ssoSession))
	util.InfoColor.Fprintln(os.Stderr, "Your browser should open. Please follow the instructions.")

	awsCmd := exec.Command("aws", "sso", "login", "--sso-session", ssoSession)
	awsCmd.Stdin = os.Stdin
	awsCmd.Stdout = os.Stderr
	awsCmd.Stderr = os.Stderr

	if err := awsCmd.Run(); err != nil {
		return fmt.Errorf("aws sso login failed: %w", err)
	}

	util.SuccessColor.Fprintln(os.Stderr, "✔ SSO login successful.")
	return nil
}

// --- Initialization ---
func init() {
	// Command will be added to profile subcommand in profile.go
}
