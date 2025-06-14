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
