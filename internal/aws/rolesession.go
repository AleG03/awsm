package aws

import (
	"fmt"
	"os/user"
	"regexp"
	"strings"
)

// roleSessionNameUnsafe matches every character AWS does not allow in a role
// session name. STS accepts [\w+=,.@-] only, between 2 and 64 characters.
var roleSessionNameUnsafe = regexp.MustCompile(`[^\w+=,.@-]`)

// maxRoleSessionName is the length limit STS enforces on RoleSessionName.
const maxRoleSessionName = 64

// BuildRoleSessionName returns the RoleSessionName to use when assuming a role.
//
// The name is not cosmetic: it is embedded in the assumed-role ARN and recorded
// on every CloudTrail event the session produces, and trust policies can gate
// on it with sts:RoleSessionName. So it should say who is assuming the role and
// through which profile, which "awsm-session" did not.
//
// configured wins, so a profile that sets role_session_name behaves the way the
// AWS CLI would. Otherwise the name is built from the local user and the
// profile. No timestamp: it would make the ARN differ on every call and defeat
// both CloudTrail filtering and any trust policy condition on the name.
func BuildRoleSessionName(configured, profileName string) string {
	if configured != "" {
		return truncateRoleSessionName(sanitizeRoleSessionName(configured))
	}

	name := "awsm"
	if u, err := user.Current(); err == nil && u.Username != "" {
		// Domain accounts come through as "DOMAIN\user" on Windows.
		username := u.Username
		if i := strings.LastIndexAny(username, `\/`); i >= 0 {
			username = username[i+1:]
		}
		name = fmt.Sprintf("awsm-%s", username)
	}
	if profileName != "" {
		name = fmt.Sprintf("%s-%s", name, profileName)
	}
	return truncateRoleSessionName(sanitizeRoleSessionName(name))
}

// sanitizeRoleSessionName replaces the characters STS rejects and collapses the
// runs they leave behind, so "my profile!!" does not become "my-profile--".
func sanitizeRoleSessionName(name string) string {
	cleaned := roleSessionNameUnsafe.ReplaceAllString(name, "-")
	for strings.Contains(cleaned, "--") {
		cleaned = strings.ReplaceAll(cleaned, "--", "-")
	}
	return strings.Trim(cleaned, "-")
}

// truncateRoleSessionName enforces the 2-64 character range. Truncation keeps
// the head, which carries the user name; the tail is the profile and is the
// part that can be lost without making the session anonymous.
func truncateRoleSessionName(name string) string {
	if len(name) > maxRoleSessionName {
		name = strings.TrimRight(name[:maxRoleSessionName], "-")
	}
	if len(name) < 2 {
		// Nothing usable survived sanitization; fall back to something STS
		// will accept rather than letting it reject the whole call.
		return "awsm-session"
	}
	return name
}
