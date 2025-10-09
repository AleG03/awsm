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
	AccessKey     string `json:"access_key,omitempty"`
	SecretKey     string `json:"secret_key,omitempty"`
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

		// For IAM user profiles, get credentials from credentials file
		if profile.Type == ProfileTypeKey {
			credentialsPath, err := GetAWSCredentialsPath()
			if err == nil {
				if credCfg, err := ini.Load(credentialsPath); err == nil {
					if credSection, err := credCfg.GetSection(profileName); err == nil {
						profile.AccessKey = credSection.Key("aws_access_key_id").String()
						profile.SecretKey = credSection.Key("aws_secret_access_key").String()
					}
				}
			}
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

// AddIAMUserProfile adds a new IAM user profile with static credentials
func AddIAMUserProfile(profileName, accessKey, secretKey, region string) error {
	// Add credentials to credentials file
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
	var credCfg *ini.File
	if _, err := os.Stat(credentialsPath); os.IsNotExist(err) {
		credCfg = ini.Empty()
	} else {
		credCfg, err = ini.Load(credentialsPath)
		if err != nil {
			return fmt.Errorf("failed to load credentials file: %w", err)
		}
	}

	// Create profile section in credentials
	credSection, err := credCfg.NewSection(profileName)
	if err != nil {
		return fmt.Errorf("failed to create profile section: %w", err)
	}

	credSection.Key("aws_access_key_id").SetValue(accessKey)
	credSection.Key("aws_secret_access_key").SetValue(secretKey)

	if err := saveCredentialsWithDefaultLast(credCfg, credentialsPath); err != nil {
		return err
	}

	// Add profile to config file
	configPath, err := GetAWSConfigPath()
	if err != nil {
		return err
	}

	// Load or create config file
	var configCfg *ini.File
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		configCfg = ini.Empty()
	} else {
		configCfg, err = ini.Load(configPath)
		if err != nil {
			return fmt.Errorf("failed to load config file: %w", err)
		}
	}

	// Create profile section in config
	configSectionName := fmt.Sprintf("profile %s", profileName)
	configSection, err := configCfg.NewSection(configSectionName)
	if err != nil {
		return fmt.Errorf("failed to create config profile section: %w", err)
	}

	configSection.Key("region").SetValue(region)

	if err := configCfg.SaveTo(configPath); err != nil {
		return fmt.Errorf("failed to save config file: %w", err)
	}

	// Invalidate profile cache since profiles have changed
	InvalidateProfileCache()
	return nil
}

// AddIAMRoleProfile adds a new IAM role profile
func AddIAMRoleProfile(profileName, roleArn, sourceProfile, mfaSerial, region string) error {
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

	// Create profile section
	sectionName := fmt.Sprintf("profile %s", profileName)
	section, err := cfg.NewSection(sectionName)
	if err != nil {
		return fmt.Errorf("failed to create profile section: %w", err)
	}

	section.Key("role_arn").SetValue(roleArn)
	if sourceProfile != "" {
		section.Key("source_profile").SetValue(sourceProfile)
	}
	if mfaSerial != "" {
		section.Key("mfa_serial").SetValue(mfaSerial)
	}
	if region != "" {
		section.Key("region").SetValue(region)
	}

	return cfg.SaveTo(configPath)
}

// DeleteProfile removes a profile from both config and credentials files
func DeleteProfile(profileName string) error {
	// Delete from config file
	configPath, err := GetAWSConfigPath()
	if err != nil {
		return err
	}

	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		cfg, err := ini.Load(configPath)
		if err != nil {
			return fmt.Errorf("failed to load config file: %w", err)
		}

		// Try both profile formats
		sectionNames := []string{fmt.Sprintf("profile %s", profileName), profileName}
		for _, sectionName := range sectionNames {
			if cfg.HasSection(sectionName) {
				cfg.DeleteSection(sectionName)
				break
			}
		}

		if err := cfg.SaveTo(configPath); err != nil {
			return fmt.Errorf("failed to save config file: %w", err)
		}
	}

	// Delete from credentials file
	credentialsPath, err := GetAWSCredentialsPath()
	if err != nil {
		return err
	}

	if _, err := os.Stat(credentialsPath); !os.IsNotExist(err) {
		cfg, err := ini.Load(credentialsPath)
		if err != nil {
			return fmt.Errorf("failed to load credentials file: %w", err)
		}

		if cfg.HasSection(profileName) {
			cfg.DeleteSection(profileName)
			// Invalidate profile cache since profiles have changed
			InvalidateProfileCache()
			return saveCredentialsWithDefaultLast(cfg, credentialsPath)
		}
	}

	// Invalidate profile cache since profiles have changed
	InvalidateProfileCache()
	return nil
}

// DeleteSSOSession removes an SSO session from config file
func DeleteSSOSession(sessionName string) error {
	configPath, err := GetAWSConfigPath()
	if err != nil {
		return err
	}

	cfg, err := ini.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config file: %w", err)
	}

	sectionName := fmt.Sprintf("sso-session %s", sessionName)
	if cfg.HasSection(sectionName) {
		cfg.DeleteSection(sectionName)
	}

	return cfg.SaveTo(configPath)
}

// GetProfilesBySSO returns all profiles that use a specific SSO session
func GetProfilesBySSO(ssoSession string) ([]string, error) {
	profiles, err := ListProfilesDetailed()
	if err != nil {
		return nil, err
	}

	var ssoProfiles []string
	for _, profile := range profiles {
		if profile.Type == ProfileTypeSSO && profile.SSOSession == ssoSession {
			ssoProfiles = append(ssoProfiles, profile.Name)
		}
	}

	return ssoProfiles, nil
}

// UpdateProfileRegion updates the region for a profile
func UpdateProfileRegion(profileName, region string) error {
	configPath, err := GetAWSConfigPath()
	if err != nil {
		return err
	}

	cfg, err := ini.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config file: %w", err)
	}

	sectionName := fmt.Sprintf("profile %s", profileName)
	section := cfg.Section(sectionName)
	if section == nil {
		section = cfg.Section(profileName)
	}

	section.Key("region").SetValue(region)
	return cfg.SaveTo(configPath)
}

// OrphanedProfile represents a profile that should be cleaned up
type OrphanedProfile struct {
	Name   string
	Reason string
}

// FindOrphanedProfiles finds profiles that reference non-existent resources
func FindOrphanedProfiles() ([]OrphanedProfile, error) {
	profiles, err := ListProfilesDetailed()
	if err != nil {
		return nil, err
	}

	ssoSessions, err := ListSSOSessions()
	if err != nil {
		return nil, err
	}

	// Create map of existing SSO sessions
	ssoSessionMap := make(map[string]bool)
	for _, session := range ssoSessions {
		ssoSessionMap[session.Name] = true
	}

	var orphaned []OrphanedProfile
	for _, profile := range profiles {
		if profile.Type == ProfileTypeSSO {
			if !ssoSessionMap[profile.SSOSession] {
				orphaned = append(orphaned, OrphanedProfile{
					Name:   profile.Name,
					Reason: fmt.Sprintf("SSO session '%s' not found", profile.SSOSession),
				})
			}
		} else if profile.Type == ProfileTypeIAM && profile.SourceProfile != "" {
			// Check if source profile exists
			sourceExists := false
			for _, p := range profiles {
				if p.Name == profile.SourceProfile {
					sourceExists = true
					break
				}
			}
			if !sourceExists {
				orphaned = append(orphaned, OrphanedProfile{
					Name:   profile.Name,
					Reason: fmt.Sprintf("source profile '%s' not found", profile.SourceProfile),
				})
			}
		}
	}

	return orphaned, nil
}

// ImportSSOSession imports an SSO session
func ImportSSOSession(session SSOSessionInfo) error {
	return AddSSOSession(session.Name, session.StartURL, session.Region)
}

// AddSSOProfile adds a new SSO profile
func AddSSOProfile(profileName, ssoSession, ssoAccountID, ssoRoleName, region string) error {
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

	// Create profile section
	sectionName := fmt.Sprintf("profile %s", profileName)
	section, err := cfg.NewSection(sectionName)
	if err != nil {
		return fmt.Errorf("failed to create profile section: %w", err)
	}

	section.Key("sso_session").SetValue(ssoSession)
	section.Key("sso_account_id").SetValue(ssoAccountID)
	section.Key("sso_role_name").SetValue(ssoRoleName)
	if region != "" {
		section.Key("region").SetValue(region)
	}

	return cfg.SaveTo(configPath)
}

// ImportProfile imports a profile based on its type
func ImportProfile(profile ProfileInfo) error {
	switch profile.Type {
	case ProfileTypeKey:
		// Import IAM user profile with actual credentials from export
		return AddIAMUserProfile(profile.Name, profile.AccessKey, profile.SecretKey, profile.Region)
	case ProfileTypeIAM:
		return AddIAMRoleProfile(profile.Name, profile.RoleARN, profile.SourceProfile, profile.MFASerial, profile.Region)
	case ProfileTypeSSO:
		return AddSSOProfile(profile.Name, profile.SSOSession, profile.SSOAccountID, profile.SSORoleName, profile.Region)
	default:
		return fmt.Errorf("cannot import profile type: %s", profile.Type)
	}
}

// saveCredentialsWithDefaultLast ensures default profile is always last
func saveCredentialsWithDefaultLast(cfg *ini.File, credentialsPath string) error {
	// Get current source profile to preserve it
	currentSourceProfile := GetCurrentProfileName()

	// Get default section if it exists
	var defaultSection *ini.Section
	if cfg.HasSection("default") {
		defaultSection = cfg.Section("default")
		// Remove it temporarily
		cfg.DeleteSection("default")
	}

	// Save file without default
	if err := cfg.SaveTo(credentialsPath); err != nil {
		return err
	}

	// Add default section back if it existed
	if defaultSection != nil {
		newDefault, err := cfg.NewSection("default")
		if err != nil {
			return err
		}
		// Copy all keys
		for _, key := range defaultSection.Keys() {
			newDefault.Key(key.Name()).SetValue(key.Value())
		}
		// Preserve the source profile comment if it existed
		if currentSourceProfile != "" && !newDefault.HasKey("# source_profile") {
			newDefault.Key("# source_profile").SetValue(currentSourceProfile)
		}
		return cfg.SaveTo(credentialsPath)
	}

	return nil
}
