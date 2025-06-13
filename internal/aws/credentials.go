package aws

import (
	"context"
	"fmt"
	"time"

	"awsm/internal/util"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/aws-sdk-go-v2/service/sts/types"
	ini "gopkg.in/ini.v1"
)

// Credentials holds a set of temporary AWS credentials.
// This is used to pass credentials between packages.
type Credentials struct {
	AccessKeyId     string
	SecretAccessKey string
	SessionToken    string
}

// GetTemporaryCredentials handles MFA and role assumption for IAM profiles.
func GetTemporaryCredentials(profileName string) (*types.Credentials, error) {
	exists, err := ProfileExists(profileName)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("profile '%s' not found", profileName)
	}

	configPath, err := GetAWSConfigPath()
	if err != nil {
		return nil, err
	}
	cfgFile, err := ini.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read AWS config file: %w", err)
	}

	// section can be `[profile my-profile]` or `[my-profile]`
	section, err := cfgFile.GetSection("profile " + profileName)
	if err != nil {
		section, err = cfgFile.GetSection(profileName)
		if err != nil {
			return nil, fmt.Errorf("could not find profile section for '%s'", profileName)
		}
	}

	mfaSerial := section.Key("mfa_serial").String()
	roleArn := section.Key("role_arn").String()
	sourceProfile := section.Key("source_profile").String()

	// The profile to use for the initial STS client
	stsClientProfile := profileName
	if sourceProfile != "" {
		stsClientProfile = sourceProfile
	}

	// Create initial AWS config
	awsCfg, err := config.LoadDefaultConfig(context.TODO(), config.WithSharedConfigProfile(stsClientProfile))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config for profile '%s': %w", stsClientProfile, err)
	}
	stsClient := sts.NewFromConfig(awsCfg)

	var tokenCode *string
	if mfaSerial != "" {
		prompt := fmt.Sprintf("Enter MFA token for %s: ", util.BoldColor.Sprint(mfaSerial))
		code, err := util.PromptForInput(prompt)
		if err != nil {
			return nil, fmt.Errorf("failed to read MFA token: %w", err)
		}
		tokenCode = aws.String(code)
	}

	if roleArn != "" {
		util.InfoColor.Printf("Assuming role %s...\n", util.BoldColor.Sprint(roleArn))
		input := &sts.AssumeRoleInput{
			RoleArn:         aws.String(roleArn),
			RoleSessionName: aws.String(fmt.Sprintf("awsm-session-%d", time.Now().Unix())),
			DurationSeconds: aws.Int32(3600),
		}
		if mfaSerial != "" {
			input.SerialNumber = aws.String(mfaSerial)
			input.TokenCode = tokenCode
		}
		result, err := stsClient.AssumeRole(context.TODO(), input)
		if err != nil {
			return nil, fmt.Errorf("failed to assume role: %w", err)
		}
		return result.Credentials, nil
	}

	if mfaSerial != "" {
		util.InfoColor.Printf("Getting session token for profile %s...\n", util.BoldColor.Sprint(profileName))
		input := &sts.GetSessionTokenInput{
			DurationSeconds: aws.Int32(3600),
			SerialNumber:    aws.String(mfaSerial),
			TokenCode:       tokenCode,
		}
		result, err := stsClient.GetSessionToken(context.TODO(), input)
		if err != nil {
			return nil, fmt.Errorf("failed to get session token: %w", err)
		}
		return result.Credentials, nil
	}

	return nil, fmt.Errorf("profile '%s' is not configured for role assumption or MFA. Use 'aws configure' for static credentials", profileName)
}
