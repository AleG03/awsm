package aws

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"awsm/internal/util"

	"github.com/aws/aws-sdk-go-v2/config"
)

// CredentialStatus represents the state of credentials
type CredentialStatus int

const (
	CredentialValid CredentialStatus = iota
	CredentialExpired
	CredentialExpiringSoon
	CredentialMissing
)

// CheckCredentialStatus checks if current credentials are valid/expired
func CheckCredentialStatus(profileName string) (CredentialStatus, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithSharedConfigProfile(profileName))
	if err != nil {
		return CredentialMissing, err
	}

	creds, err := cfg.Credentials.Retrieve(context.TODO())
	if err != nil {
		if strings.Contains(err.Error(), "token has expired") ||
			strings.Contains(err.Error(), "InvalidGrantException") ||
			strings.Contains(err.Error(), "expired") {
			return CredentialExpired, nil
		}
		return CredentialMissing, err
	}

	// Check if credentials will expire soon (within 5 minutes)
	if !creds.Expires.IsZero() && time.Until(creds.Expires) < 5*time.Minute {
		return CredentialExpiringSoon, nil
	}

	return CredentialValid, nil
}

// AutoRefreshCredentials automatically refreshes credentials if needed
func AutoRefreshCredentials(profileName string) error {
	status, err := CheckCredentialStatus(profileName)
	if err != nil {
		return err
	}

	switch status {
	case CredentialValid:
		return nil
	case CredentialExpiringSoon:
		util.WarnColor.Fprintf(os.Stderr, "⚠ Credentials expiring soon, refreshing...\n")
		return refreshCredentials(profileName)
	case CredentialExpired:
		util.InfoColor.Fprintf(os.Stderr, "🔄 Credentials expired, refreshing...\n")
		return refreshCredentials(profileName)
	case CredentialMissing:
		util.InfoColor.Fprintf(os.Stderr, "🔄 No valid credentials found, refreshing...\n")
		return refreshCredentials(profileName)
	}

	return nil
}

// refreshCredentials handles the actual refresh logic
func refreshCredentials(profileName string) error {
	profileType, err := getProfileTypeByName(profileName)
	if err != nil {
		return err
	}

	switch profileType {
	case "sso":
		return refreshSSOCredentials(profileName)
	case "iam":
		// IAM profiles require manual MFA input, so just inform user
		return fmt.Errorf("IAM profile credentials expired. Please run: awsm %s", profileName)
	default:
		return fmt.Errorf("cannot auto-refresh static credentials")
	}
}

// refreshSSOCredentials refreshes SSO credentials
func refreshSSOCredentials(profileName string) error {
	ssoSession, err := GetSsoSessionForProfile(profileName)
	if err != nil {
		return fmt.Errorf("failed to get SSO session: %w", err)
	}

	util.InfoColor.Fprintf(os.Stderr, "Refreshing SSO session: %s\n", ssoSession)

	cmd := exec.Command("aws", "sso", "login", "--sso-session", ssoSession)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("SSO refresh failed: %w", err)
	}

	util.SuccessColor.Fprintf(os.Stderr, "✔ SSO credentials refreshed\n")
	return nil
}

// getProfileTypeByName determines profile type by name
func getProfileTypeByName(profileName string) (string, error) {
	_, profileType, err := inspectProfile(profileName)
	return profileType, err
}
