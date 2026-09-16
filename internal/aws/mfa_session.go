package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
// The name is prefixed so it cannot collide with the per-profile credential
// cache, which lives in the same directory and holds a different thing.
func mfaSessionCachePath(profileName string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not get user home directory: %w", err)
	}
	return filepath.Join(home, ".awsm", "cache", "mfa-session-"+profileName+".json"), nil
}

// GetCachedMFASession returns a profile's cached MFA session, or nil when there
// is none worth using.
func GetCachedMFASession(profileName string) *TempCredentials {
	path, err := mfaSessionCachePath(profileName)
	if err != nil {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var creds TempCredentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil
	}
	if time.Until(creds.Expires) < mfaSessionRenewBefore {
		return nil
	}
	return &creds
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
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return
	}
	data, err := json.Marshal(creds)
	if err != nil {
		return
	}
	// Longer-lived than anything else awsm caches: keep it owner-only.
	_ = os.WriteFile(path, data, 0600)
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
	setCachedMFASession(stsProfile, session)
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
