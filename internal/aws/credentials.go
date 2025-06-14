package aws

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"awsm/internal/util"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/aws-sdk-go-v2/service/sts/types"
	ini "gopkg.in/ini.v1"
)

// ErrSsoSessionExpired is a special error used to signal the shell wrapper.
var ErrSsoSessionExpired = errors.New("sso session is expired or invalid")

// TempCredentials holds a set of temporary AWS credentials.
type TempCredentials struct {
	AccessKeyId     string
	SecretAccessKey string
	SessionToken    string
}

// profileConfig holds the relevant configuration details extracted from a profile.
type profileConfig struct {
	MfaSerial     string
	RoleArn       string
	SourceProfile string
}

// GetCredentialsForProfile is the main entry point for getting credentials.
// It inspects the profile and dispatches to the correct handler.
func GetCredentialsForProfile(profileName string) (creds *TempCredentials, isStatic bool, err error) {
	pConfig, profileType, err := inspectProfile(profileName)
	if err != nil {
		return nil, false, err
	}

	switch profileType {
	case "iam":
		tempCreds, err := handleIamProfile(profileName, pConfig)
		if err != nil {
			return nil, false, err
		}
		return &TempCredentials{
			AccessKeyId:     *tempCreds.AccessKeyId,
			SecretAccessKey: *tempCreds.SecretAccessKey,
			SessionToken:    *tempCreds.SessionToken,
		}, false, nil

	case "sso":
		awsCfg, err := config.LoadDefaultConfig(context.TODO(), config.WithSharedConfigProfile(profileName))
		if err != nil {
			return nil, false, fmt.Errorf("failed to load AWS config for SSO profile: %w", err)
		}
		sdkCreds, err := awsCfg.Credentials.Retrieve(context.TODO())
		if err != nil {
			if strings.Contains(err.Error(), "token has expired") || strings.Contains(err.Error(), "InvalidGrantException") {
				return nil, false, ErrSsoSessionExpired // Return our special error.
			}
			return nil, false, err // Return the original error for other issues.
		}
		return &TempCredentials{
			AccessKeyId:     sdkCreds.AccessKeyID,
			SecretAccessKey: sdkCreds.SecretAccessKey,
			SessionToken:    sdkCreds.SessionToken,
		}, false, nil

	case "static":
		return nil, true, nil

	default:
		return nil, false, fmt.Errorf("unknown profile type for '%s'", profileName)
	}
}

// inspectProfile reads the config file to determine the profile type.
func inspectProfile(profileName string) (*profileConfig, string, error) {
	configPath, err := GetAWSConfigPath()
	if err != nil {
		return nil, "", err
	}
	cfgFile, err := ini.Load(configPath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read AWS config file: %w", err)
	}

	sectionName := "profile " + profileName
	section, err := cfgFile.GetSection(sectionName)
	if err != nil {
		section, err = cfgFile.GetSection(profileName)
		if err != nil {
			return nil, "", fmt.Errorf("could not find profile section for '%s'", profileName)
		}
	}

	pConfig := &profileConfig{
		MfaSerial:     section.Key("mfa_serial").String(),
		RoleArn:       section.Key("role_arn").String(),
		SourceProfile: section.Key("source_profile").String(),
	}

	if pConfig.RoleArn != "" || pConfig.MfaSerial != "" {
		return pConfig, "iam", nil
	}
	if section.HasKey("sso_session") {
		return pConfig, "sso", nil
	}
	if section.HasKey("aws_access_key_id") {
		return pConfig, "static", nil
	}
	return nil, "unknown", fmt.Errorf("could not determine type of profile '%s'", profileName)
}

// handleIamProfile contains the logic for IAM-based profiles (MFA/role assumption).

func handleIamProfile(profileName string, pConfig *profileConfig) (*types.Credentials, error) {
	if pConfig.RoleArn != "" {
		return assumeRole(profileName, pConfig)
	}
	return getSessionToken(profileName, pConfig)
}

// assumeRole handles the specific logic for calling sts:AssumeRole.
func assumeRole(profileName string, pConfig *profileConfig) (*types.Credentials, error) {
	util.InfoColor.Fprintf(os.Stderr, "Assuming role %s...\n", util.BoldColor.Sprint(pConfig.RoleArn))

	stsClientProfile := profileName
	if pConfig.SourceProfile != "" {
		stsClientProfile = pConfig.SourceProfile
	}

	awsCfg, err := config.LoadDefaultConfig(context.TODO(), config.WithSharedConfigProfile(stsClientProfile))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config for source profile '%s': %w", stsClientProfile, err)
	}

	var tokenCode *string
	if pConfig.MfaSerial != "" {
		prompt := fmt.Sprintf("Enter MFA token for %s: ", util.BoldColor.Sprint(pConfig.MfaSerial))
		code, err := util.PromptForInput(prompt)
		if err != nil {
			return nil, fmt.Errorf("failed to read MFA token: %w", err)
		}
		tokenCode = aws.String(code)
	}

	input := &sts.AssumeRoleInput{
		RoleArn:         aws.String(pConfig.RoleArn),
		RoleSessionName: aws.String(fmt.Sprintf("awsm-session-%d", time.Now().Unix())),
		DurationSeconds: aws.Int32(3600),
		SerialNumber:    aws.String(pConfig.MfaSerial),
		TokenCode:       tokenCode,
	}

	stsClient := sts.NewFromConfig(awsCfg)
	result, err := stsClient.AssumeRole(context.TODO(), input)
	if err != nil {
		return nil, fmt.Errorf("failed to assume role: %w", err)
	}
	return result.Credentials, nil
}

// getSessionToken handles the specific logic for calling sts:GetSessionToken.
func getSessionToken(profileName string, pConfig *profileConfig) (*types.Credentials, error) {
	util.InfoColor.Fprintf(os.Stderr, "Getting session token for profile %s...\n", util.BoldColor.Sprint(profileName))

	awsCfg, err := config.LoadDefaultConfig(context.TODO(), config.WithSharedConfigProfile(profileName))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config for profile '%s': %w", profileName, err)
	}

	prompt := fmt.Sprintf("Enter MFA token for %s: ", util.BoldColor.Sprint(pConfig.MfaSerial))
	code, err := util.PromptForInput(prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to read MFA token: %w", err)
	}

	input := &sts.GetSessionTokenInput{
		DurationSeconds: aws.Int32(3600),
		SerialNumber:    aws.String(pConfig.MfaSerial),
		TokenCode:       aws.String(code),
	}

	stsClient := sts.NewFromConfig(awsCfg)
	result, err := stsClient.GetSessionToken(context.TODO(), input)
	if err != nil {
		return nil, fmt.Errorf("failed to get session token: %w", err)
	}
	return result.Credentials, nil
}
