package cmd

import (
	"context"
	"errors"
	"fmt"

	"awsm/internal/aws"
)

// Complete an explicit login by renewing the active profile using that session.
// The browser can stay open for minutes: never restore a profile that the user
// cleared, changed or renewed elsewhere while signing in.
func loginAndRefreshActive(ctx context.Context, session string,
	login func(context.Context, string) error,
	resolve func(string) (*aws.TempCredentials, bool, error),
) error {
	revision, revisionErr := aws.ActiveCredentialsRevision()
	profile := aws.GetCurrentProfileName()
	activeSession, _ := aws.GetSsoSessionForProfile(profile)
	region, _ := aws.GetProfileRegion(profile)
	if err := login(ctx, session); err != nil {
		return err
	}
	if profile == "" || activeSession != session {
		return nil
	}
	if revisionErr != nil {
		return fmt.Errorf("signed in, but could not read active credentials: %w", revisionErr)
	}
	current, err := aws.ActiveCredentialsRevision()
	if err != nil {
		return err
	}
	if current != revision {
		return nil
	}
	creds, isStatic, err := resolve(profile)
	if err != nil {
		return fmt.Errorf("signed in, but could not renew profile %q: %w", profile, err)
	}
	if isStatic || creds == nil {
		return fmt.Errorf("signed in, but no temporary credentials returned for %q", profile)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	err = aws.UpdateCredentialsFileIfCurrent(creds, region, profile, revision)
	if errors.Is(err, aws.ErrActiveProfileChanged) {
		return nil
	}
	return err
}
