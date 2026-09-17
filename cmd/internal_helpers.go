package cmd

import (
	"context"
	"errors"
	"fmt"

	"awsm/internal/aws"
	"awsm/internal/tui"
)

// ensureCredentialsWithLogin resolves credentials for a profile, prompting for
// MFA when needed and automatically performing an SSO login if the SSO
// session has expired. It is the central helper used by commands that need to
// act on resolved credentials (set, run, whoami, ...).
//
// Returns the credentials, whether they are static, and any error.
func ensureCredentialsWithLogin(ctx context.Context, profileName string) (*aws.TempCredentials, bool, error) {
	// Handle MFA prompt before any spinner so stdin remains usable.
	var mfaToken string
	if needsMFA, mfaSerial, mfaErr := aws.ProfileNeedsMFA(profileName); mfaErr == nil && needsMFA && !aws.HasValidCachedCredentials(profileName) {
		token, err := tui.PromptInput(fmt.Sprintf("MFA token for %s", tui.FormatBold(mfaSerial)))
		if err != nil {
			return nil, false, fmt.Errorf("failed to read MFA token: %w", err)
		}
		mfaToken = token
	}

	var (
		creds    *aws.TempCredentials
		isStatic bool
	)
	err := tui.ShowSpinner(ctx, fmt.Sprintf("Getting credentials for profile '%s'", profileName), func() error {
		var spinnerErr error
		creds, isStatic, spinnerErr = aws.GetCredentialsForProfile(profileName, mfaToken)
		return spinnerErr
	})

	// If we got a definitive answer, return.
	if err == nil && (creds != nil || isStatic) {
		return creds, isStatic, nil
	}

	// Detect whether we should attempt an SSO login.
	shouldLogin := false
	if err != nil && errors.Is(err, aws.ErrSsoSessionExpired) {
		shouldLogin = true
	} else if err == nil && creds == nil && !isStatic {
		if _, ssoErr := aws.GetSsoSessionForProfile(profileName); ssoErr == nil {
			shouldLogin = true
		}
	}

	if !shouldLogin {
		if err != nil {
			return nil, false, err
		}
		return nil, false, fmt.Errorf("no credentials available for profile '%s'", profileName)
	}

	ssoSession, ssoErr := aws.GetSsoSessionForProfile(profileName)
	if ssoErr != nil {
		return nil, false, fmt.Errorf("failed to get SSO session: %w", ssoErr)
	}
	if loginErr := aws.PerformSSOLogin(ssoSession); loginErr != nil {
		return nil, false, loginErr
	}

	err = tui.ShowSpinner(ctx, fmt.Sprintf("Getting credentials for profile '%s' (retry)", profileName), func() error {
		var spinnerErr error
		creds, isStatic, spinnerErr = aws.GetCredentialsForProfile(profileName)
		return spinnerErr
	})
	if err != nil {
		return nil, false, fmt.Errorf("credential acquisition failed after login: %w", err)
	}
	if creds == nil && !isStatic {
		return nil, false, fmt.Errorf("no credentials available for profile '%s' after login", profileName)
	}
	return creds, isStatic, nil
}

// resolveProfileName returns the explicit profile if provided, otherwise the
// currently active profile, or ErrNoActiveProfile.
func resolveProfileName(explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	cur := aws.GetCurrentProfileName()
	if cur == "" {
		return "", aws.ErrNoActiveProfile
	}
	return cur, nil
}
