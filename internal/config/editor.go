package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"awsm/internal/awsini"
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

// RemoveAllProfilesForSession removes all profiles that reference a specific sso_session
// from the config content. Returns the cleaned config and a list of removed profile names.
func RemoveAllProfilesForSession(configContent, sessionName string) (string, []string) {
	_, profileContentMap := ParseExistingProfiles(configContent)

	// go-ini's pretty-printer (run by OrganizeConfigFile) pads keys with
	// variable amounts of whitespace before "=", so match on that instead
	// of a fixed single-space/no-space string.
	sessionRegex := regexp.MustCompile(fmt.Sprintf(`(?m)^\s*sso_session\s*=\s*%s\s*$`, regexp.QuoteMeta(sessionName)))

	var removed []string
	for profileName, content := range profileContentMap {
		// Check if this profile references the session
		if sessionRegex.MatchString(content) {
			configContent = RemoveProfileFromConfig(configContent, profileName)
			removed = append(removed, profileName)
		}
	}

	return configContent, removed
}

// sectionHeaderRegex matches any ini section header.
//
// A profile ends at the next header of *any* kind. Scanning only for the next
// "[profile ...]" header used to make removal swallow every section in
// between - "[sso-session ...]", "[default]", "[services ...]" - or the whole
// rest of the file when the removed profile was the last one.
var sectionHeaderRegex = regexp.MustCompile(`(?m)^\[[^\]]*\]`)

// RemoveProfileFromConfig removes a specific profile from the config content.
func RemoveProfileFromConfig(config, profileName string) string {
	profileHeaderRegex := regexp.MustCompile(fmt.Sprintf(`(?m)^\[profile %s\]`, regexp.QuoteMeta(profileName)))
	match := profileHeaderRegex.FindStringIndex(config)
	if match == nil {
		return config
	}

	// A comment block sitting directly above the header documents this
	// profile, so it goes away with it.
	profileStart := introStart(config, match[0])

	// The profile ends at the next section header; the comment block
	// introducing that header belongs to it and has to survive.
	profileEnd := len(config)
	if next := sectionHeaderRegex.FindStringIndex(config[match[1]:]); next != nil {
		profileEnd = introStart(config, match[1]+next[0])
	}

	return config[:profileStart] + config[profileEnd:]
}

// introStart returns the offset at which the comment block introducing the
// section header at headerPos begins, or headerPos itself when the header is
// not preceded by one. Blank lines mixed into the block are included so the
// separating newline stays with the block.
func introStart(config string, headerPos int) int {
	start := headerPos
	sawComment := false
	for start > 0 {
		lineStart := 0
		if i := strings.LastIndexByte(config[:start-1], '\n'); i >= 0 {
			lineStart = i + 1
		}
		line := strings.TrimSpace(config[lineStart : start-1])
		if line != "" && !strings.HasPrefix(line, "#") && !strings.HasPrefix(line, ";") {
			break
		}
		if line != "" {
			sawComment = true
		}
		start = lineStart
	}
	if !sawComment {
		return headerPos
	}
	return start
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
				// Find the end of this profile: the next section header of
				// any kind, or the end of the file. Stopping only at the next
				// "[profile ...]" header made a profile's content absorb the
				// sections that followed it.
				bodyStart := profileStart + len(match[0])
				profileEnd := len(configContent)
				if next := sectionHeaderRegex.FindStringIndex(configContent[bodyStart:]); next != nil {
					profileEnd = bodyStart + next[0]
				}
				existingProfileContent[profileName] = configContent[profileStart:profileEnd]
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
	return awsini.WriteFile(path, []byte(content))
}
