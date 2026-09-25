package aws

import (
	"awsm/internal/awsini"
	"time"
)

// ssoTokenCacheEntry mirrors the relevant fields of the JSON files written by
// the AWS CLI v2 / SDK to ~/.aws/sso/cache/.
type ssoTokenCacheEntry struct {
	StartURL    string `json:"startUrl"`
	Region      string `json:"region"`
	AccessToken string `json:"accessToken"`
	ExpiresAt   string `json:"expiresAt"`
}

// SSOTokenExpiry returns the expiration time of the cached SSO access token
// associated with the given profile, if any.
//
// Use the same cache entry as login and token refresh. A legacy entry for the
// same start URL must not hide the token that was just issued for this session.
func SSOTokenExpiry(profileName string) (time.Time, bool) {
	path, ok := ssoTokenCacheFile(profileName)
	if !ok {
		return time.Time{}, false
	}
	entry, ok := readSSOTokenCacheEntry(path)
	if !ok || entry.AccessToken == "" {
		return time.Time{}, false
	}
	expiry, err := time.Parse(time.RFC3339, entry.ExpiresAt)
	return expiry, err == nil
}

// lookupSSOStartURL resolves the SSO start URL for the given profile, either
// from the profile section directly (legacy format) or via the linked
// sso-session block (modern format).
func lookupSSOStartURL(profileName string) string {
	cfgPath, err := GetAWSConfigPath()
	if err != nil {
		return ""
	}
	cfg, err := awsini.Load(cfgPath)
	if err != nil {
		return ""
	}
	section, err := getProfileSection(cfg, profileName)
	if err != nil {
		return ""
	}
	if url := section.Key("sso_start_url").String(); url != "" {
		return url
	}
	sessionName, _ := GetSsoSessionForProfile(profileName)
	if sessionName == "" {
		return ""
	}
	sessionSection, err := cfg.GetSection("sso-session " + sessionName)
	if err != nil {
		return ""
	}
	return sessionSection.Key("sso_start_url").String()
}
