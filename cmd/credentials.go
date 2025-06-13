package cmd

import (
	"context"
	"fmt"
	"os"

	"awsm/internal/aws"
	"awsm/internal/util"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	ini "gopkg.in/ini.v1"
)

// getCredentialsForProfile inspects a profile and returns temporary credentials.
// It handles IAM (MFA/role_arn) and SSO profiles.
// It returns a boolean `isStatic` which is true if the profile uses static keys.
func getCredentialsForProfile(profileName string) (creds *aws.Credentials, isStatic bool, err error) {
	configPath, err := aws.GetAWSConfigPath()
	if err != nil {
		return nil, false, err
	}
	cfgFile, err := ini.Load(configPath)
	if err != nil {
		return nil, false, fmt.Errorf("failed to read AWS config file: %w", err)
	}

	section, err := cfgFile.GetSection("profile " + profileName)
	if err != nil {
		section, err = cfgFile.GetSection(profileName)
		if err != nil {
			return nil, false, fmt.Errorf("could not find profile section for '%s'", profileName)
		}
	}

	// Check for IAM Role/MFA profile
	if section.HasKey("role_arn") || section.HasKey("mfa_serial") {

		util.InfoColor.Fprintln(os.Stderr, "IAM profile detected. Using STS to get credentials...")
		tempCreds, err := aws.GetTemporaryCredentials(profileName)
		if err != nil {
			return nil, false, fmt.Errorf("error getting IAM credentials: %w", err)
		}
		return &aws.Credentials{
			AccessKeyId:     *tempCreds.AccessKeyId,
			SecretAccessKey: *tempCreds.SecretAccessKey,
			SessionToken:    *tempCreds.SessionToken,
		}, false, nil
	}

	// Check for SSO profile
	if section.HasKey("sso_session") {
		util.InfoColor.Fprintln(os.Stderr, "SSO profile detected. Using cached session to get credentials...")
		awsCfg, err := awsconfig.LoadDefaultConfig(context.TODO(), awsconfig.WithSharedConfigProfile(profileName))
		if err != nil {
			return nil, false, fmt.Errorf("failed to load AWS config for SSO profile: %w", err)
		}
		sdkCreds, err := awsCfg.Credentials.Retrieve(context.TODO())
		if err != nil {
			return nil, false, fmt.Errorf("failed to retrieve SSO credentials: %w\nHint: Your session may have expired. Try running 'awsm sso login %s'", err, profileName)
		}
		return &aws.Credentials{
			AccessKeyId:     sdkCreds.AccessKeyID,
			SecretAccessKey: sdkCreds.SecretAccessKey,
			SessionToken:    sdkCreds.SessionToken,
		}, false, nil
	}


	util.WarnColor.Fprintf(os.Stderr, "Profile '%s' appears to use static credentials.\n", profileName)
	return nil, true, nil
}
