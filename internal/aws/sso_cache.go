package aws

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"awsm/internal/awsini"
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
// The lookup is best-effort: it scans ~/.aws/sso/cache for a JSON entry whose
// startUrl matches the profile's sso_start_url (either set directly on the
// profile or via its sso-session). If nothing matches, ok=false.
func SSOTokenExpiry(profileName string) (time.Time, bool) {
	startURL := lookupSSOStartURL(profileName)
	if startURL == "" {
		return time.Time{}, false
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return time.Time{}, false
	}
	cacheDir := filepath.Join(home, ".aws", "sso", "cache")
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return time.Time{}, false
	}

	wantedURL := strings.TrimRight(startURL, "/")
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(cacheDir, e.Name()))
		if err != nil {
			continue
		}
		var entry ssoTokenCacheEntry
		if err := json.Unmarshal(data, &entry); err != nil {
			continue
		}
		if strings.TrimRight(entry.StartURL, "/") != wantedURL {
			continue
		}
		if entry.ExpiresAt == "" {
			continue
		}
		t, err := time.Parse(time.RFC3339, entry.ExpiresAt)
		if err != nil {
			continue
		}
		return t, true
	}
	return time.Time{}, false
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
	sessionName := section.Key("sso_session").String()
	if sessionName == "" {
		return ""
	}
	sessionSection, err := cfg.GetSection("sso-session " + sessionName)
	if err != nil {
		return ""
	}
	return sessionSection.Key("sso_start_url").String()
}
