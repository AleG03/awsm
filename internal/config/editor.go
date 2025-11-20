package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// ExtractProfileConfig extracts just the configuration lines from a profile section
func ExtractProfileConfig(profileContent string) string {
	lines := strings.Split(profileContent, "\n")
	var configLines []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Skip the profile header and empty lines
		if line != "" && !strings.HasPrefix(line, "[profile ") {
			configLines = append(configLines, line)
		}
	}

	return strings.Join(configLines, "\n")
}

// RemoveProfileFromConfig removes a specific profile from the config content
func RemoveProfileFromConfig(config, profileName string) string {
	profileHeaderRegex := regexp.MustCompile(fmt.Sprintf(`(?m)^\[profile %s\]`, regexp.QuoteMeta(profileName)))
	match := profileHeaderRegex.FindStringIndex(config)
	if match == nil {
		return config
	}

	// Find the start of the profile
	profileStart := match[0]

	// Find the end of the profile (next profile or end of file)
	nextProfileRegex := regexp.MustCompile(`(?m)^\[profile [^\]]+\]`)
	nextMatches := nextProfileRegex.FindAllStringIndex(config[match[1]:], -1)

	var profileEnd int
	if len(nextMatches) > 0 {
		profileEnd = match[1] + nextMatches[0][0]
	} else {
		profileEnd = len(config)
	}

	// Remove the profile section
	return config[:profileStart] + config[profileEnd:]
}

// ExtractProfileNamesFromContent extracts profile names from generated profile content
func ExtractProfileNamesFromContent(content string) []string {
	var profileNames []string
	profileHeaderRegex := regexp.MustCompile(`(?m)^\[profile ([^\]]+)\]`)

	matches := profileHeaderRegex.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		if len(match) > 1 {
			profileNames = append(profileNames, match[1])
		}
	}

	return profileNames
}

// ParseExistingProfiles parses the existing config and returns a map of profile names to their content
func ParseExistingProfiles(configContent string) (map[string]bool, map[string]string) {
	existingProfiles := make(map[string]bool)
	existingProfileContent := make(map[string]string)
	profileHeaderRegex := regexp.MustCompile(`(?m)^\[profile ([^\]]+)\]`)

	for _, match := range profileHeaderRegex.FindAllStringSubmatch(configContent, -1) {
		if len(match) > 1 {
			existingProfiles[match[1]] = true
			// Extract the profile content for comparison
			profileName := match[1]
			profileStart := strings.Index(configContent, match[0])
			if profileStart != -1 {
				// Find the end of this profile (next profile or end of file)
				nextProfileStart := len(configContent)
				for _, nextMatch := range profileHeaderRegex.FindAllStringSubmatch(configContent[profileStart+len(match[0]):], -1) {
					if len(nextMatch) > 1 {
						nextProfileStart = profileStart + len(match[0]) + strings.Index(configContent[profileStart+len(match[0]):], nextMatch[0])
						break
					}
				}
				existingProfileContent[profileName] = configContent[profileStart:nextProfileStart]
			}
		}
	}
	return existingProfiles, existingProfileContent
}

// ReadConfigFile reads the content of the file at the given path
func ReadConfigFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(data), nil
}

// WriteConfigFile writes the content to the file at the given path
func WriteConfigFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0600)
}
