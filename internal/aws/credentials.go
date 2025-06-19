package aws

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
	// Auto-refresh credentials if needed
	if err := AutoRefreshCredentials(profileName); err != nil {
		// If auto-refresh fails, continue with existing logic (might still work)
		util.WarnColor.Fprintf(os.Stderr, "Auto-refresh failed: %v\n", err)
	}
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

// GetAWSCredentialsPath returns the path to the AWS credentials file.
func GetAWSCredentialsPath() (string, error) {
	credentialsPath := os.Getenv("AWS_SHARED_CREDENTIALS_FILE")
	if credentialsPath != "" {
		return credentialsPath, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not get user home directory: %w", err)
	}
	return filepath.Join(home, ".aws", "credentials"), nil
}

// UpdateCredentialsFile updates the default profile in the AWS credentials file
func UpdateCredentialsFile(creds *TempCredentials, region, profileName string) error {
	credentialsPath, err := GetAWSCredentialsPath()
	if err != nil {
		return err
	}

	// Create .aws directory if it doesn't exist
	awsDir := filepath.Dir(credentialsPath)
	if err := os.MkdirAll(awsDir, 0755); err != nil {
		return fmt.Errorf("failed to create AWS directory: %w", err)
	}

	// Load or create credentials file
	var cfg *ini.File
	if _, err := os.Stat(credentialsPath); os.IsNotExist(err) {
		cfg = ini.Empty()
	} else {
		cfg, err = ini.Load(credentialsPath)
		if err != nil {
			return fmt.Errorf("failed to load credentials file: %w", err)
		}
	}

	// Get or create default section
	section, err := cfg.GetSection("default")
	if err != nil {
		section, err = cfg.NewSection("default")
		if err != nil {
			return fmt.Errorf("failed to create default section: %w", err)
		}
	}

	// Update credentials
	section.Key("aws_access_key_id").SetValue(creds.AccessKeyId)
	section.Key("aws_secret_access_key").SetValue(creds.SecretAccessKey)
	section.Key("aws_session_token").SetValue(creds.SessionToken)

	// Update region if provided
	if region != "" {
		section.Key("region").SetValue(region)
	}

	// Track the source profile name
	section.Key("# source_profile").SetValue(profileName)

	// Save the file
	return cfg.SaveTo(credentialsPath)
}

// GetCurrentProfileName returns the name of the profile currently set in default
func GetCurrentProfileName() string {
	credentialsPath, err := GetAWSCredentialsPath()
	if err != nil {
		return ""
	}

	// Read file directly to parse comment
	file, err := os.Open(credentialsPath)
	if err != nil {
		return ""
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	inDefaultSection := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "[default]" {
			inDefaultSection = true
			continue
		}

		if strings.HasPrefix(line, "[") && line != "[default]" {
			inDefaultSection = false
			continue
		}

		if inDefaultSection && strings.HasPrefix(line, "# source_profile") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}

	return ""
}

// UpdateStaticProfile updates the default profile to use a static profile's credentials
func UpdateStaticProfile(profileName string) error {
	configPath, err := GetAWSConfigPath()
	if err != nil {
		return err
	}

	credentialsPath, err := GetAWSCredentialsPath()
	if err != nil {
		return err
	}

	// Load config file to get region
	cfgFile, err := ini.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to read AWS config file: %w", err)
	}

	sectionName := "profile " + profileName
	configSection, err := cfgFile.GetSection(sectionName)
	if err != nil {
		configSection, err = cfgFile.GetSection(profileName)
		if err != nil {
			return fmt.Errorf("could not find profile section for '%s'", profileName)
		}
	}

	region := configSection.Key("region").String()

	// Load credentials file to get static credentials
	credFile, err := ini.Load(credentialsPath)
	if err != nil {
		return fmt.Errorf("failed to read AWS credentials file: %w", err)
	}

	sourceSection, err := credFile.GetSection(profileName)
	if err != nil {
		return fmt.Errorf("could not find credentials for profile '%s'", profileName)
	}

	accessKey := sourceSection.Key("aws_access_key_id").String()
	secretKey := sourceSection.Key("aws_secret_access_key").String()

	if accessKey == "" || secretKey == "" {
		return fmt.Errorf("profile '%s' does not have static credentials", profileName)
	}

	// Update default section
	defaultSection, err := credFile.GetSection("default")
	if err != nil {
		defaultSection, err = credFile.NewSection("default")
		if err != nil {
			return fmt.Errorf("failed to create default section: %w", err)
		}
	}

	defaultSection.Key("aws_access_key_id").SetValue(accessKey)
	defaultSection.Key("aws_secret_access_key").SetValue(secretKey)
	defaultSection.DeleteKey("aws_session_token") // Remove session token for static credentials

	if region != "" {
		defaultSection.Key("region").SetValue(region)
	}

	// Track the source profile name
	defaultSection.Key("# source_profile").SetValue(profileName)

	return credFile.SaveTo(credentialsPath)
}

// SetRegion updates the region in the default profile
func SetRegion(region string) error {
	credentialsPath, err := GetAWSCredentialsPath()
	if err != nil {
		return err
	}

	// Create .aws directory if it doesn't exist
	awsDir := filepath.Dir(credentialsPath)
	if err := os.MkdirAll(awsDir, 0755); err != nil {
		return fmt.Errorf("failed to create AWS directory: %w", err)
	}

	// Load or create credentials file
	var cfg *ini.File
	if _, err := os.Stat(credentialsPath); os.IsNotExist(err) {
		cfg = ini.Empty()
	} else {
		cfg, err = ini.Load(credentialsPath)
		if err != nil {
			return fmt.Errorf("failed to load credentials file: %w", err)
		}
	}

	// Get or create default section
	section, err := cfg.GetSection("default")
	if err != nil {
		section, err = cfg.NewSection("default")
		if err != nil {
			return fmt.Errorf("failed to create default section: %w", err)
		}
	}

	// Update region
	section.Key("region").SetValue(region)

	// Save the file
	return cfg.SaveTo(credentialsPath)
}
