package aws

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"awsm/internal/awsini"

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

// loadOrCreateIni loads an AWS ini file or creates an empty one if it doesn't
// exist. Always goes through awsini so nested sub-sections and "#"/";" inside
// values survive the round trip; see the awsini package comment.
func loadOrCreateIni(path string) (*ini.File, error) {
	return awsini.LoadOrEmpty(path)
}

// getProfileSection finds a profile section trying "profile <name>" first, then "<name>".
func getProfileSection(cfg *ini.File, profileName string) (*ini.Section, error) {
	section, err := cfg.GetSection("profile " + profileName)
	if err != nil {
		section, err = cfg.GetSection(profileName)
		if err != nil {
			return nil, fmt.Errorf("could not find profile section for '%s'", profileName)
		}
	}
	return section, nil
}

// ListProfiles lists all profiles from both the AWS config and credentials files.
func ListProfiles() ([]string, error) {
	profilesMap := make(map[string]bool)

	// Load config file
	configPath, err := GetAWSConfigPath()
	if err == nil {
		if cfg, err := awsini.Load(configPath); err == nil {
			for _, section := range cfg.Sections() {
				name := section.Name()
				if name == "DEFAULT" || strings.HasPrefix(name, "sso-session ") {
					continue
				}
				profilesMap[strings.TrimPrefix(name, "profile ")] = true
			}
		}
	}

	// Load credentials file
	credentialsPath, err := GetAWSCredentialsPath()
	if err == nil {
		if cfg, err := awsini.Load(credentialsPath); err == nil {
			for _, section := range cfg.Sections() {
				name := section.Name()
				if name == "DEFAULT" {
					continue
				}
				profilesMap[name] = true
			}
		}
	}

	var profiles []string
	for name := range profilesMap {
		profiles = append(profiles, name)
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

// GetSsoSessionForProfile finds the sso_session value for a given profile, traversing source_profile if needed.
func GetSsoSessionForProfile(profileName string) (string, error) {
	return getSsoSessionRecursive(profileName, make(map[string]bool))
}

func getSsoSessionRecursive(profileName string, visited map[string]bool) (string, error) {
	if visited[profileName] {
		return "", fmt.Errorf("circular profile dependency detected at '%s'", profileName)
	}
	visited[profileName] = true

	configPath, err := GetAWSConfigPath()
	if err != nil {
		return "", err
	}
	cfgFile, err := awsini.Load(configPath)
	if err != nil {
		return "", fmt.Errorf("failed to read AWS config file: %w", err)
	}

	section, err := getProfileSection(cfgFile, profileName)
	if err != nil {
		return "", err
	}

	// 1. Check for sso_session directly
	if section.HasKey("sso_session") {
		return section.Key("sso_session").String(), nil
	}

	// 2. Check for source_profile and recurse
	if section.HasKey("source_profile") {
		sourceProfile := section.Key("source_profile").String()
		authSession, err := getSsoSessionRecursive(sourceProfile, visited)
		if err == nil {
			return authSession, nil
		}
		// If recursion failed (e.g. source profile doesn't have SSO session either),
		// we return the error to the caller.
		return "", fmt.Errorf("failed to find SSO session in source profile '%s': %w", sourceProfile, err)
	}

	return "", fmt.Errorf("profile '%s' is not an SSO profile (missing 'sso_session' configuration)", profileName)
}

// ProfileType represents the type of AWS profile
type ProfileType string

const (
	ProfileTypeSSO     ProfileType = "SSO"
	ProfileTypeIAM     ProfileType = "IAM"
	ProfileTypeKey     ProfileType = "Key"
	ProfileTypeProcess ProfileType = "Process"
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
	ExternalID    string
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
	if section.HasKey("credential_process") {
		return ProfileTypeProcess
	}
	return ProfileTypeKey
}

// ListProfilesDetailed returns detailed information about all AWS profiles from both config and credentials files
func ListProfilesDetailed() ([]ProfileInfo, error) {
	configPath, err := GetAWSConfigPath()
	if err != nil {
		return nil, err
	}

	cfg, err := awsini.Load(configPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to read AWS config file at %s: %w", configPath, err)
	}

	credentialsPath, err := GetAWSCredentialsPath()
	if err != nil {
		return nil, err
	}

	credCfg, err := awsini.Load(credentialsPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to read AWS credentials file at %s: %w", credentialsPath, err)
	}

	// Get current active profile from credentials file or environment
	activeProfile := GetCurrentProfileName()
	if activeProfile == "" {
		activeProfile = os.Getenv("AWS_PROFILE")
	}

	profilesMap := make(map[string]ProfileInfo)
	processedInConfig := make(map[string]bool)

	// 1. Process config file (main source of truth for structure)
	if cfg != nil {
		for _, section := range cfg.Sections() {
			name := section.Name()
			if name == "DEFAULT" || strings.HasPrefix(name, "sso-session ") {
				continue
			}

			profileName := strings.TrimPrefix(name, "profile ")
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
				ExternalID:    section.Key("external_id").String(),
				IsActive:      profileName == activeProfile,
			}

			// Static keys can be in the profile itself or in the credentials
			// file. The credentials file wins when both define the profile,
			// matching how the AWS CLI resolves them.
			if profile.Type == ProfileTypeKey {
				profile.AccessKey = section.Key("aws_access_key_id").String()
				profile.SecretKey = section.Key("aws_secret_access_key").String()
				if credCfg != nil {
					if credSection, err := credCfg.GetSection(profileName); err == nil {
						if key := credSection.Key("aws_access_key_id").String(); key != "" {
							profile.AccessKey = key
							profile.SecretKey = credSection.Key("aws_secret_access_key").String()
						}
					}
				}
			}

			profilesMap[profileName] = profile
			processedInConfig[profileName] = true
		}
	}

	// 2. Process credentials file for profiles that DON'T exist in config
	if credCfg != nil {
		for _, section := range credCfg.Sections() {
			name := section.Name()
			if name == "DEFAULT" || processedInConfig[name] {
				continue
			}

			profilesMap[name] = ProfileInfo{
				Name:      name,
				Type:      ProfileTypeKey,
				AccessKey: section.Key("aws_access_key_id").String(),
				SecretKey: section.Key("aws_secret_access_key").String(),
				IsActive:  name == activeProfile,
			}
		}
	}

	var profiles []ProfileInfo
	for _, p := range profilesMap {
		profiles = append(profiles, p)
	}

	return profiles, nil
}

// GetProfileRegion gets the region for a specific profile
func GetProfileRegion(profileName string) (string, error) {
	configPath, err := GetAWSConfigPath()
	if err != nil {
		return "", err
	}

	cfgFile, err := awsini.Load(configPath)
	if err != nil {
		return "", fmt.Errorf("failed to read AWS config file: %w", err)
	}

	section, err := getProfileSection(cfgFile, profileName)
	if err != nil {
		return "", err
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

	cfg, err := loadOrCreateIni(configPath)
	if err != nil {
		return err
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

	return awsini.Save(cfg, configPath)
}

// ChangeProfileRegion changes the region for a specific profile
func ChangeProfileRegion(profileName, region string) error {
	configPath, err := GetAWSConfigPath()
	if err != nil {
		return err
	}

	cfg, err := awsini.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config file: %w", err)
	}

	section, err := getProfileSection(cfg, profileName)
	if err != nil {
		return err
	}

	// Update the region
	section.Key("region").SetValue(region)

	return awsini.Save(cfg, configPath)
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

	cfg, err := awsini.Load(configPath)
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

// AddIAMUserProfile adds a new IAM user profile with static credentials.
//
// Everything goes into one [profile x] section in the config file, keys
// included. The AWS CLI reads aws_access_key_id from either file, and keeping
// the profile in one place avoids the split that made a profile's region and
// its keys live in two files that can drift apart. It also survives tools that
// rewrite ~/.aws/credentials wholesale, which is how Leapp behaves.
//
// The config then holds secrets, so callers should follow up with the
// permission check in awsini: a config is normally world-readable.
func AddIAMUserProfile(profileName, accessKey, secretKey, region string) error {
	configPath, err := GetAWSConfigPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return fmt.Errorf("failed to create AWS directory: %w", err)
	}

	configCfg, err := loadOrCreateIni(configPath)
	if err != nil {
		return err
	}

	// Reuse the section when it is already there: NewSection on an existing
	// name hands back the same section anyway, and going through
	// getProfileSection keeps any key the user put there by hand.
	sectionName := fmt.Sprintf("profile %s", profileName)
	configSection, err := configCfg.GetSection(sectionName)
	if err != nil {
		configSection, err = configCfg.NewSection(sectionName)
		if err != nil {
			return fmt.Errorf("failed to create config profile section: %w", err)
		}
	}

	configSection.Key("aws_access_key_id").SetValue(accessKey)
	configSection.Key("aws_secret_access_key").SetValue(secretKey)
	if region != "" {
		configSection.Key("region").SetValue(region)
	}

	if err := awsini.Save(configCfg, configPath); err != nil {
		return fmt.Errorf("failed to save config file: %w", err)
	}

	// Invalidate profile cache since profiles have changed
	InvalidateProfileCache()
	return nil
}

// IAMRoleProfile describes an assume-role profile as it is written to the
// config file. It is a struct rather than a parameter list because the AWS CLI
// keeps adding keys here and six positional strings in a row invite silent
// argument swaps.
type IAMRoleProfile struct {
	RoleARN       string
	SourceProfile string
	MFASerial     string
	ExternalID    string
	Region        string
}

// UpdateIAMUserProfile updates an existing IAM user profile in place.
//
// The keys are rewritten wherever they already are. New profiles go into the
// config, but a profile whose keys are in ~/.aws/credentials keeps them there:
// writing the new pair into the config while the old pair stayed in the
// credentials file would leave the old one in force, since that file takes
// precedence, and the profile would keep authenticating with the keys the user
// just replaced.
//
// Updating in place also preserves keys awsm knows nothing about, which
// delete-then-recreate silently dropped.
func UpdateIAMUserProfile(profileName, accessKey, secretKey, region string) error {
	return withCredentialsLock(func() error { return updateIAMUserProfile(profileName, accessKey, secretKey, region) })
}

func updateIAMUserProfile(profileName, accessKey, secretKey, region string) error {
	credentialsPath, err := GetAWSCredentialsPath()
	if err != nil {
		return err
	}

	credCfg, credErr := awsini.Load(credentialsPath)
	if credErr == nil {
		if credSection, err := credCfg.GetSection(profileName); err == nil &&
			credSection.Key("aws_access_key_id").String() != "" {
			credSection.Key("aws_access_key_id").SetValue(accessKey)
			credSection.Key("aws_secret_access_key").SetValue(secretKey)
			if err := saveCredentialsPreservingDefault(credCfg, credentialsPath); err != nil {
				return err
			}
			if region != "" {
				return UpdateProfileRegion(profileName, region)
			}
			InvalidateProfileCache()
			return nil
		}
	}

	// Not in the credentials file: the keys belong in the profile, and
	// AddIAMUserProfile updates the section rather than replacing it.
	return AddIAMUserProfile(profileName, accessKey, secretKey, region)
}

// AddIAMRoleProfile adds a new IAM role profile
func AddIAMRoleProfile(profileName string, p IAMRoleProfile) error {
	configPath, err := GetAWSConfigPath()
	if err != nil {
		return err
	}

	// Create .aws directory if it doesn't exist
	awsDir := filepath.Dir(configPath)
	if err := os.MkdirAll(awsDir, 0755); err != nil {
		return fmt.Errorf("failed to create AWS directory: %w", err)
	}

	cfg, err := loadOrCreateIni(configPath)
	if err != nil {
		return err
	}

	// Create profile section
	sectionName := fmt.Sprintf("profile %s", profileName)
	section, err := cfg.NewSection(sectionName)
	if err != nil {
		return fmt.Errorf("failed to create profile section: %w", err)
	}

	section.Key("role_arn").SetValue(p.RoleARN)
	for _, kv := range []struct{ key, value string }{
		{"source_profile", p.SourceProfile},
		{"mfa_serial", p.MFASerial},
		{"external_id", p.ExternalID},
		{"region", p.Region},
	} {
		if kv.value != "" {
			section.Key(kv.key).SetValue(kv.value)
		}
	}

	return awsini.Save(cfg, configPath)
}

// UpdateIAMRoleProfile updates an existing IAM role profile in place.
//
// In place matters: the section keeps any key awsm does not know about, such as
// a nested "s3 =" block. Deleting and recreating the profile would drop them.
func UpdateIAMRoleProfile(profileName string, p IAMRoleProfile) error {
	configPath, err := GetAWSConfigPath()
	if err != nil {
		return err
	}

	cfg, err := awsini.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config file: %w", err)
	}

	section, err := getProfileSection(cfg, profileName)
	if err != nil {
		return err
	}

	section.Key("role_arn").SetValue(p.RoleARN)
	for _, kv := range []struct{ key, value string }{
		{"source_profile", p.SourceProfile},
		{"mfa_serial", p.MFASerial},
		{"external_id", p.ExternalID},
		{"region", p.Region},
	} {
		if kv.value != "" {
			section.Key(kv.key).SetValue(kv.value)
		} else {
			section.DeleteKey(kv.key)
		}
	}

	return awsini.Save(cfg, configPath)
}

// DeleteProfile removes a profile from both config and credentials files
func DeleteProfile(profileName string) error {
	return withCredentialsLock(func() error { return deleteProfile(profileName) })
}

func deleteProfile(profileName string) error {
	// Delete from config file
	configPath, err := GetAWSConfigPath()
	if err != nil {
		return err
	}

	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		cfg, err := awsini.Load(configPath)
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

		if err := awsini.Save(cfg, configPath); err != nil {
			return fmt.Errorf("failed to save config file: %w", err)
		}
	}

	// Delete from credentials file
	credentialsPath, err := GetAWSCredentialsPath()
	if err != nil {
		return err
	}

	if _, err := os.Stat(credentialsPath); !os.IsNotExist(err) {
		cfg, err := awsini.Load(credentialsPath)
		if err != nil {
			return fmt.Errorf("failed to load credentials file: %w", err)
		}

		if cfg.HasSection(profileName) {
			cfg.DeleteSection(profileName)
			// Invalidate profile cache since profiles have changed
			InvalidateProfileCache()
			return saveCredentialsPreservingDefault(cfg, credentialsPath)
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

	cfg, err := awsini.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config file: %w", err)
	}

	sectionName := fmt.Sprintf("sso-session %s", sessionName)
	if cfg.HasSection(sectionName) {
		cfg.DeleteSection(sectionName)
	}

	return awsini.Save(cfg, configPath)
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

	cfg, err := awsini.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config file: %w", err)
	}

	sectionName := fmt.Sprintf("profile %s", profileName)
	section := cfg.Section(sectionName)
	if section == nil {
		section = cfg.Section(profileName)
	}

	section.Key("region").SetValue(region)
	return awsini.Save(cfg, configPath)
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

	cfg, err := loadOrCreateIni(configPath)
	if err != nil {
		return err
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

	return awsini.Save(cfg, configPath)
}

// ImportProfile imports a profile based on its type
func ImportProfile(profile ProfileInfo) error {
	switch profile.Type {
	case ProfileTypeKey:
		// Import IAM user profile with actual credentials from export
		return AddIAMUserProfile(profile.Name, profile.AccessKey, profile.SecretKey, profile.Region)
	case ProfileTypeIAM:
		return AddIAMRoleProfile(profile.Name, IAMRoleProfile{
			RoleARN:       profile.RoleARN,
			SourceProfile: profile.SourceProfile,
			MFASerial:     profile.MFASerial,
			ExternalID:    profile.ExternalID,
			Region:        profile.Region,
		})
	case ProfileTypeSSO:
		return AddSSOProfile(profile.Name, profile.SSOSession, profile.SSOAccountID, profile.SSORoleName, profile.Region)
	default:
		return fmt.Errorf("cannot import profile type: %s", profile.Type)
	}
}

// saveCredentialsPreservingDefault preserves the active profile metadata on edits.
func saveCredentialsPreservingDefault(cfg *ini.File, credentialsPath string) error {
	// Keep the original section, including nested keys and comments.
	// Never expose a credentials file with default removed.
	if cfg.HasSection("default") {
		section := cfg.Section("default")
		for _, key := range []string{"# source_profile", defaultExpiresKey} {
			if value := readDefaultComment(key); value != "" {
				section.Key(key).SetValue(value)
			}
		}
	}
	return awsini.Save(cfg, credentialsPath)
}

// RestoreConfigFiles restores the AWS config and credentials files from raw content
func RestoreConfigFiles(configContent, credentialsContent string) error {
	return withCredentialsLock(func() error { return restoreConfigFiles(configContent, credentialsContent) })
}

func restoreConfigFiles(configContent, credentialsContent string) error {
	// 1. Get paths
	configPath, err := GetAWSConfigPath()
	if err != nil {
		return err
	}
	credentialsPath, err := GetAWSCredentialsPath()
	if err != nil {
		return err
	}

	// 2. Create .aws directory if it doesn't exist
	awsDir := filepath.Dir(configPath)
	if err := os.MkdirAll(awsDir, 0755); err != nil {
		return fmt.Errorf("failed to create AWS directory: %w", err)
	}

	// 3. Backup existing files if they exist
	if _, err := os.Stat(configPath); err == nil {
		_ = os.Remove(configPath + ".bak") // Ignore error if bak doesn't exist
		if err := os.Rename(configPath, configPath+".bak"); err != nil {
			return fmt.Errorf("failed to backup config file: %w", err)
		}
	}
	if _, err := os.Stat(credentialsPath); err == nil {
		_ = os.Remove(credentialsPath + ".bak") // Ignore error if bak doesn't exist
		if err := os.Rename(credentialsPath, credentialsPath+".bak"); err != nil {
			return fmt.Errorf("failed to backup credentials file: %w", err)
		}
	}

	// 4. Write new content
	if configContent != "" {
		if err := awsini.WriteFile(configPath, []byte(configContent)); err != nil {
			return fmt.Errorf("failed to write config file: %w", err)
		}
	}
	if credentialsContent != "" {
		if err := awsini.WriteFile(credentialsPath, []byte(credentialsContent)); err != nil {
			return fmt.Errorf("failed to write credentials file: %w", err)
		}
	}

	InvalidateProfileCache()
	return nil
}

// ChangeProfileRegionAndActive updates the named profile and its active copy.
// Switching accounts cannot interleave between the active check and the write.
func ChangeProfileRegionAndActive(profileName, region string) error {
	return withCredentialsLock(func() error {
		if err := ChangeProfileRegion(profileName, region); err != nil {
			return err
		}
		if GetCurrentProfileName() == profileName {
			return setRegion(region)
		}
		return nil
	})
}
