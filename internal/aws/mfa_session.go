package aws

import (
	"context"
	"fmt"
	"os"
	"time"

	"awsm/internal/util"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// MFASessionDuration is how long an MFA session token is requested for.
//
// 36 hours is the maximum sts:GetSessionToken grants when called with an IAM
// user's long-term keys. Asking for the maximum is the point of the exercise:
// the MFA code is typed once and role credentials can then be re-issued from
// the session without prompting again.
const MFASessionDuration = 36 * time.Hour

// mfaSessionRenewBefore is how much remaining life makes a cached session
// unusable. A session about to expire would produce role credentials that die
// with it, so it is treated as absent and re-acquired.
const mfaSessionRenewBefore = 15 * time.Minute

// mfaSessionCachePath returns the file holding a profile's MFA session.
//
// Session tokens live in a separate namespace from assumed-role credentials.
func mfaSessionCachePath(profileName string) (string, error) {
	return credentialCachePath("mfa", profileName)
}

// GetCachedMFASession returns a profile's cached MFA session, or nil when there
// is none worth using.
func GetCachedMFASession(profileName string) *TempCredentials {
	path, err := mfaSessionCachePath(profileName)
	if err != nil {
		return nil
	}
	creds := readCredentialCache(path, profileName)
	if creds == nil || time.Until(creds.Expires) < mfaSessionRenewBefore {
		return nil
	}
	return creds
}

// HasValidMFASession reports whether a role can currently be assumed without
// asking for an MFA code.
//
// This is what lets an unattended caller decide, without touching the network
// and without risking a prompt, whether it can refresh an MFA profile at all.
func HasValidMFASession(profileName string) bool {
	return GetCachedMFASession(profileName) != nil
}

// MFASessionExpiry returns when a cached MFA session runs out, for display.
func MFASessionExpiry(profileName string) (time.Time, bool) {
	creds := GetCachedMFASession(profileName)
	if creds == nil {
		return time.Time{}, false
	}
	return creds.Expires, true
}

func setCachedMFASession(profileName string, creds *TempCredentials) {
	path, err := mfaSessionCachePath(profileName)
	if err == nil {
		writeCredentialCache(path, profileName, creds)
	}
}

// ClearMFASession removes a profile's cached MFA session.
func ClearMFASession(profileName string) error {
	path, err := mfaSessionCachePath(profileName)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// mfaSessionCredentials returns session credentials for stsProfile that already
// carry the MFA claim, reusing the cached session when there is one.
//
// Obtaining them requires an MFA code, so it prompts when mfaToken is empty and
// nothing is cached. Callers that must not prompt have to check
// HasValidMFASession first.
func mfaSessionCredentials(stsProfile, mfaSerial, mfaToken string) (*TempCredentials, error) {
	if cached := GetCachedMFASession(stsProfile); cached != nil {
		return cached, nil
	}

	fingerprint, err := profileFingerprint(stsProfile)
	if err != nil {
		return nil, err
	}
	awsCfg, err := config.LoadDefaultConfig(context.TODO(), config.WithSharedConfigProfile(stsProfile))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config for profile '%s': %w", stsProfile, err)
	}

	code := mfaToken
	if code == "" {
		prompt := fmt.Sprintf("Enter MFA token for %s: ", util.BoldColor.Sprint(mfaSerial))
		code, err = util.PromptForInput(prompt)
		if err != nil {
			return nil, fmt.Errorf("failed to read MFA token: %w", err)
		}
	}

	out, err := sts.NewFromConfig(awsCfg).GetSessionToken(context.TODO(), &sts.GetSessionTokenInput{
		DurationSeconds: aws.Int32(int32(MFASessionDuration.Seconds())),
		SerialNumber:    aws.String(mfaSerial),
		TokenCode:       aws.String(code),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get MFA session token: %w", err)
	}

	session := &TempCredentials{
		AccessKeyId:     *out.Credentials.AccessKeyId,
		SecretAccessKey: *out.Credentials.SecretAccessKey,
		SessionToken:    *out.Credentials.SessionToken,
		Expires:         *out.Credentials.Expiration,
	}
	current, err := profileFingerprint(stsProfile)
	if err != nil {
		return nil, err
	}
	if current != fingerprint {
		return nil, fmt.Errorf("profile %q changed while acquiring an MFA session; retry the command", stsProfile)
	}
	if path, err := mfaSessionCachePath(stsProfile); err == nil {
		writeCredentialCacheWithFingerprint(path, fingerprint, session)
	}
	return session, nil
}

// canUseMFASession reports whether stsProfile holds long-term IAM user keys.
//
// sts:GetSessionToken only accepts those: called with credentials that are
// already temporary, such as an SSO profile's, it fails. Profiles like that
// keep the original behaviour of passing the MFA code straight to AssumeRole.
func canUseMFASession(stsProfile string) bool {
	_, profileType, err := inspectProfile(stsProfile)
	if err != nil {
		return false
	}
	return profileType == "iam-user" || profileType == "static"
}

// staticCredentialsConfig builds an AWS config that authenticates with the
// given credentials instead of resolving a profile.
func staticCredentialsConfig(creds *TempCredentials, region string) (aws.Config, error) {
	opts := []func(*config.LoadOptions) error{
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			creds.AccessKeyId, creds.SecretAccessKey, creds.SessionToken)),
	}
	if region != "" {
		opts = append(opts, config.WithRegion(region))
	}
	return config.LoadDefaultConfig(context.TODO(), opts...)
}
