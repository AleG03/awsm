package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
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

	// Get profile region first
	region, err := aws.GetProfileRegion(profileName)
	if err != nil {
		// Region is optional, continue without it
		region = ""
	}
	if region != "" && !aws.IsValidRegion(region) {
		return fmt.Errorf("invalid region for profile '%s': %s", profileName, region)
	}

	// Check if profile needs MFA and prompt before starting the spinner
	// (the spinner captures stdin, preventing interactive input)
	// Skip if we have valid cached credentials
	var mfaToken string
	if needsMFA, mfaSerial, mfaErr := aws.ProfileNeedsMFA(profileName); mfaErr == nil && needsMFA && !aws.HasValidCachedCredentials(profileName) {
		prompt := fmt.Sprintf("Enter MFA token for %s: ", util.BoldColor.Sprint(mfaSerial))
		mfaToken, err = util.PromptForInput(prompt)
		if err != nil {
			return fmt.Errorf("failed to read MFA token: %w", err)
		}
	}

	// Use spinner for credential acquisition
	var creds *aws.TempCredentials
	var isStatic bool
	err = tui.ShowSpinner(context.Background(), fmt.Sprintf("Getting credentials for profile '%s'", profileName), func() error {
		var spinnerErr error
		creds, isStatic, spinnerErr = aws.GetCredentialsForProfile(profileName, mfaToken)
		return spinnerErr
	})

	if err != nil || (creds == nil && !isStatic) {
		// Check if we should try to login (SSO expired or missing credentials for SSO profile)
		shouldLogin := false
		if err != nil && errors.Is(err, aws.ErrSsoSessionExpired) {
			shouldLogin = true
		} else if creds == nil && !isStatic {
			// If no credentials and not static, check if it's an SSO profile
			if _, ssoErr := aws.GetSsoSessionForProfile(profileName); ssoErr == nil {
				shouldLogin = true
			}
		}

		if shouldLogin {
			// Get SSO session
			ssoSession, ssoErr := aws.GetSsoSessionForProfile(profileName)
			if ssoErr != nil {
				fmt.Fprintln(os.Stderr, tui.ErrorStyle.Render("✗ Error getting SSO session: "+ssoErr.Error()))
				return fmt.Errorf("failed to get SSO session")
			}

			// Perform login
			if loginErr := aws.PerformSSOLogin(ssoSession); loginErr != nil {
				return loginErr
			}

			// Retry getting credentials
			err = tui.ShowSpinner(context.Background(), fmt.Sprintf("Getting credentials for profile '%s' (retry)", profileName), func() error {
				var spinnerErr error
				creds, isStatic, spinnerErr = aws.GetCredentialsForProfile(profileName)
				return spinnerErr
			})

			if err != nil {
				fmt.Fprintln(os.Stderr, tui.ErrorStyle.Render("✗ Error after login: "+err.Error()))
				return fmt.Errorf("credential acquisition failed after login")
			}
		} else if err != nil {
			fmt.Fprintln(os.Stderr, tui.ErrorStyle.Render("✗ Error: "+err.Error()))
			return fmt.Errorf("credential acquisition failed")
		} else if creds == nil {
			fmt.Fprintln(os.Stderr, tui.ErrorStyle.Render("✗ Error: No credentials available for profile '"+profileName+"'"))
			return fmt.Errorf("no credentials available")
		}
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
		return fmt.Errorf("unexpected error: credentials are nil")
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

// --- Initialization ---
func init() {
	// Command will be added to profile subcommand in profile.go
}
