package aws

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/ini.v1"
)

// GetAWSConfigPath returns the path to the AWS config file.
func GetAWSConfigPath() (string, error) {
	configPath := os.Getenv("AWS_CONFIG_FILE")
	if configPath != "" {
		return configPath, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not get user home directory: %w", err)
	}
	return filepath.Join(home, ".aws", "config"), nil
}

// ListProfiles lists all profiles from the AWS config file.
func ListProfiles() ([]string, error) {
	configPath, err := GetAWSConfigPath()
	if err != nil {
		return nil, err
	}

	cfg, err := ini.Load(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to read AWS config file at %s: %w", configPath, err)
	}

	var profiles []string
	for _, section := range cfg.Sections() {
		name := section.Name()
		if name == "DEFAULT" {
			continue
		}
		profileName := strings.TrimPrefix(name, "profile ")
		profiles = append(profiles, profileName)
	}

	return profiles, nil
}

// ProfileExists checks if a profile exists in the AWS config.
func ProfileExists(profileName string) (bool, error) {
	profiles, err := ListProfiles()
	if err != nil {
		return false, err
	}
	for _, p := range profiles {
		if p == profileName {
			return true, nil
		}
	}
	return false, nil
}

// GetSsoSessionForProfile finds the sso_session value for a given profile.
func GetSsoSessionForProfile(profileName string) (string, error) {
	configPath, err := GetAWSConfigPath()
	if err != nil {
		return "", err
	}
	cfgFile, err := ini.Load(configPath)
	if err != nil {
		return "", fmt.Errorf("failed to read AWS config file: %w", err)
	}

	sectionName := "profile " + profileName
	section, err := cfgFile.GetSection(sectionName)
	if err != nil {
		// Fallback for profiles without the "profile " prefix
		section, err = cfgFile.GetSection(profileName)
		if err != nil {
			return "", fmt.Errorf("could not find profile section for '%s'", profileName)
		}
	}

	// Get the value of the key directly. If the key doesn't exist, this returns an empty string.
	ssoSessionValue := section.Key("sso_session").String()

	// Check if the returned string is empty.
	if ssoSessionValue == "" {
		return "", fmt.Errorf("profile '%s' is not an SSO profile (missing 'sso_session' configuration)", profileName)
	}

	// If we get here, the value is valid. Return it.
	return ssoSessionValue, nil
}

// ProfileType represents the type of AWS profile
type ProfileType string

const (
	ProfileTypeSSO ProfileType = "SSO"
	ProfileTypeIAM ProfileType = "IAM"
	ProfileTypeKey ProfileType = "Key"
)

// ProfileInfo contains detailed information about an AWS profile
type ProfileInfo struct {
	Name          string
	Type          ProfileType
	Region        string
	RoleARN       string
	SourceProfile string
	SSOStartURL   string
	SSORegion     string
	SSOAccountID  string
	SSORoleName   string
	SSOSession    string
	MFASerial     string
	IsActive      bool
}

// GetProfileType determines the type of AWS profile based on its configuration
func getProfileType(section *ini.Section) ProfileType {
	if section.HasKey("sso_session") || section.HasKey("sso_start_url") {
		return ProfileTypeSSO
	}
	if section.HasKey("role_arn") {
		return ProfileTypeIAM
	}
	return ProfileTypeKey
}

// ListProfilesDetailed returns detailed information about all AWS profiles
func ListProfilesDetailed() ([]ProfileInfo, error) {
	configPath, err := GetAWSConfigPath()
	if err != nil {
		return nil, err
	}

	cfg, err := ini.Load(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []ProfileInfo{}, nil
		}
		return nil, fmt.Errorf("failed to read AWS config file at %s: %w", configPath, err)
	}

	// Get current active profile from credentials file or environment
	activeProfile := GetCurrentProfileName()
	if activeProfile == "" {
		activeProfile = os.Getenv("AWS_PROFILE")
	}

	var profiles []ProfileInfo
	for _, section := range cfg.Sections() {
		name := section.Name()
		if name == "DEFAULT" {
			continue
		}

		profileName := strings.TrimPrefix(name, "profile ")
		if profileName == name {
			// Skip SSO session sections
			if strings.HasPrefix(name, "sso-session") {
				continue
			}
		}

		profile := ProfileInfo{
			Name:          profileName,
			Type:          getProfileType(section),
			Region:        section.Key("region").String(),
			RoleARN:       section.Key("role_arn").String(),
			SourceProfile: section.Key("source_profile").String(),
			SSOStartURL:   section.Key("sso_start_url").String(),
			SSORegion:     section.Key("sso_region").String(),
			SSOAccountID:  section.Key("sso_account_id").String(),
			SSORoleName:   section.Key("sso_role_name").String(),
			SSOSession:    section.Key("sso_session").String(),
			MFASerial:     section.Key("mfa_serial").String(),
			IsActive:      profileName == activeProfile,
		}

		profiles = append(profiles, profile)
	}

	return profiles, nil
}

// GetProfileRegion gets the region for a specific profile
func GetProfileRegion(profileName string) (string, error) {
	configPath, err := GetAWSConfigPath()
	if err != nil {
		return "", err
	}

	cfgFile, err := ini.Load(configPath)
	if err != nil {
		return "", fmt.Errorf("failed to read AWS config file: %w", err)
	}

	sectionName := "profile " + profileName
	section, err := cfgFile.GetSection(sectionName)
	if err != nil {
		section, err = cfgFile.GetSection(profileName)
		if err != nil {
			return "", fmt.Errorf("could not find profile section for '%s'", profileName)
		}
	}

	region := section.Key("region").String()
	if region == "" {
		return "", fmt.Errorf("no region configured for profile '%s'", profileName)
	}

	return region, nil
}

// AddSSOSession adds a new SSO session to the AWS config file
func AddSSOSession(sessionName, startURL, region string) error {
	configPath, err := GetAWSConfigPath()
	if err != nil {
		return err
	}

	// Create .aws directory if it doesn't exist
	awsDir := filepath.Dir(configPath)
	if err := os.MkdirAll(awsDir, 0755); err != nil {
		return fmt.Errorf("failed to create AWS directory: %w", err)
	}

	// Load or create config file
	var cfg *ini.File
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		cfg = ini.Empty()
	} else {
		cfg, err = ini.Load(configPath)
		if err != nil {
			return fmt.Errorf("failed to load config file: %w", err)
		}
	}

	// Create SSO session section
	sectionName := fmt.Sprintf("sso-session %s", sessionName)
	section, err := cfg.NewSection(sectionName)
	if err != nil {
		return fmt.Errorf("failed to create SSO session section: %w", err)
	}

	// Set SSO session properties
	section.Key("sso_start_url").SetValue(startURL)
	section.Key("sso_region").SetValue(region)
	section.Key("sso_registration_scopes").SetValue("sso:account:access")

	return cfg.SaveTo(configPath)
}

// ChangeProfileRegion changes the region for a specific profile
func ChangeProfileRegion(profileName, region string) error {
	configPath, err := GetAWSConfigPath()
	if err != nil {
		return err
	}

	cfg, err := ini.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config file: %w", err)
	}

	// Try to find the profile section
	sectionName := "profile " + profileName
	section, err := cfg.GetSection(sectionName)
	if err != nil {
		// Fallback for profiles without the "profile " prefix
		section, err = cfg.GetSection(profileName)
		if err != nil {
			return fmt.Errorf("could not find profile section for '%s'", profileName)
		}
	}

	// Update the region
	section.Key("region").SetValue(region)

	return cfg.SaveTo(configPath)
}

// SSOSessionInfo contains information about an SSO session
type SSOSessionInfo struct {
	Name     string
	StartURL string
	Region   string
	Scopes   string
}

// ListSSOSessions returns all SSO sessions from the AWS config
func ListSSOSessions() ([]SSOSessionInfo, error) {
	configPath, err := GetAWSConfigPath()
	if err != nil {
		return nil, err
	}

	cfg, err := ini.Load(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []SSOSessionInfo{}, nil
		}
		return nil, fmt.Errorf("failed to read AWS config file: %w", err)
	}

	var sessions []SSOSessionInfo
	for _, section := range cfg.Sections() {
		name := section.Name()
		if strings.HasPrefix(name, "sso-session ") {
			sessionName := strings.TrimPrefix(name, "sso-session ")
			session := SSOSessionInfo{
				Name:     sessionName,
				StartURL: section.Key("sso_start_url").String(),
				Region:   section.Key("sso_region").String(),
				Scopes:   section.Key("sso_registration_scopes").String(),
			}
			sessions = append(sessions, session)
		}
	}

	return sessions, nil
}
