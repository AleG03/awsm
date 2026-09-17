package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ssooidc"
)

// RefreshSSOToken exchanges the cached refresh token for a new access token.
//
// This is deliberately not left to the SDK. Its SSOTokenProvider only refreshes
// once the access token has *already expired*:
//
//	if cachedToken.ExpiresAt != nil && sdk.NowTime().After(*cachedToken.ExpiresAt) {
//	        cachedToken, err = p.refreshToken(ctx, cachedToken)
//	}
//
// There is no margin, so simply resolving credentials ahead of expiry renews
// nothing, and the access token lapses even though the means to renew it was
// sitting in the cache file.
//
// Observed against IAM Identity Center: the access token lives one hour and
// CreateToken returns the *same* refresh token rather than a new one, so the
// refresh token's own lifetime is what bounds how long a session can be kept
// alive without a browser. The code replaces it anyway when a different one
// comes back, since that behaviour is the provider's to change.
//
// CreateToken is an unauthenticated operation, so no AWS credentials are
// involved in making this call.
func RefreshSSOToken(profileName string) error {
	path, ok := ssoTokenCacheFile(profileName)
	if !ok {
		return fmt.Errorf("no cached SSO token found for profile '%s'", profileName)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("could not read the cached SSO token: %w", err)
	}

	// Decoded into a map rather than a struct so that every field written by
	// the AWS CLI survives the round trip, including the ones awsm has no
	// reason to know about.
	var fields map[string]any
	if err := json.Unmarshal(raw, &fields); err != nil {
		return fmt.Errorf("the cached SSO token is not valid JSON: %w", err)
	}

	clientID, _ := fields["clientId"].(string)
	clientSecret, _ := fields["clientSecret"].(string)
	refreshToken, _ := fields["refreshToken"].(string)
	region, _ := fields["region"].(string)

	if clientID == "" || clientSecret == "" || refreshToken == "" {
		return fmt.Errorf("the cached SSO token for '%s' cannot be refreshed: it predates refresh tokens, so a new login is needed", profileName)
	}
	if region == "" {
		return fmt.Errorf("the cached SSO token for '%s' has no region", profileName)
	}

	client := ssooidc.New(ssooidc.Options{Region: region})
	out, err := client.CreateToken(context.TODO(), &ssooidc.CreateTokenInput{
		ClientId:     &clientID,
		ClientSecret: &clientSecret,
		GrantType:    strPtr("refresh_token"),
		RefreshToken: &refreshToken,
	})
	if err != nil {
		// InvalidGrantException means the refresh token itself is no longer
		// accepted, which is what the IdP's session policy running out looks
		// like. That needs a browser and must not be retried every cycle,
		// unlike a network failure.
		if strings.Contains(err.Error(), "InvalidGrantException") {
			return fmt.Errorf("%w: the refresh token for '%s' is no longer valid", ErrSsoSessionExpired, profileName)
		}
		return fmt.Errorf("could not refresh the SSO token: %w", err)
	}
	if out.AccessToken == nil {
		return fmt.Errorf("the SSO refresh returned no access token")
	}

	fields["accessToken"] = *out.AccessToken
	fields["expiresAt"] = time.Now().UTC().
		Add(time.Duration(out.ExpiresIn) * time.Second).
		Format(ssoCacheTimeLayout)
	// The refresh token rotates: keeping the old one would break the next
	// refresh and quietly end the run of browser-free renewals.
	if out.RefreshToken != nil && *out.RefreshToken != "" {
		fields["refreshToken"] = *out.RefreshToken
	}

	return writeSSOTokenCache(path, fields)
}

// writeSSOTokenCache replaces the token file atomically, so an interrupted
// write cannot leave a truncated token behind and force a login.
func writeSSOTokenCache(path string, fields map[string]any) error {
	data, err := json.Marshal(fields)
	if err != nil {
		return fmt.Errorf("could not serialize the SSO token: %w", err)
	}

	mode := os.FileMode(0600)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}

	tmp := path + ".awsm-tmp"
	if err := os.WriteFile(tmp, data, mode); err != nil {
		return fmt.Errorf("could not write the SSO token: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("could not replace the SSO token: %w", err)
	}
	return nil
}

// ssoTokenCacheFile locates the cache entry belonging to a profile's SSO
// session.
//
// The deterministic name comes first, because that is where PerformSSOLogin
// writes and where the AWS CLI looks: sha1 of the sso-session name. Scanning by
// start URL stays as a fallback for caches written before sso-session blocks
// existed, but it must not win -- once both a legacy entry and a current one
// describe the same start URL, the scan can return the stale one and the
// refresh would renew a token nothing reads.
func ssoTokenCacheFile(profileName string) (string, bool) {
	startURL := lookupSSOStartURL(profileName)
	if startURL == "" {
		return "", false
	}
	wanted := strings.TrimRight(startURL, "/")

	// The deterministic name, but only once the file inside it agrees about
	// which start URL it belongs to.
	//
	// Existence alone is not enough. sha1 is taken over the session's *name*,
	// so a name reused for a different Identity Center -- renamed, repointed,
	// or simply recreated -- lands on a file left by the previous one. Handing
	// that back would have RefreshSSOToken renew a stale entry and write to it,
	// while the live token sits in another file nobody looked at.
	if session, err := GetSsoSessionForProfile(profileName); err == nil && session != "" {
		if path, err := ssoCacheFileFor(session); err == nil {
			if entry, ok := readSSOTokenCacheEntry(path); ok &&
				strings.TrimRight(entry.StartURL, "/") == wanted {
				return path, true
			}
		}
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", false
	}
	cacheDir := filepath.Join(home, ".aws", "sso", "cache")
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return "", false
	}

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		path := filepath.Join(cacheDir, e.Name())
		entry, ok := readSSOTokenCacheEntry(path)
		if !ok {
			continue
		}
		if strings.TrimRight(entry.StartURL, "/") == wanted {
			return path, true
		}
	}
	return "", false
}

// readSSOTokenCacheEntry decodes one cache file, reporting whether it could be
// read at all. An unreadable or malformed entry is skipped rather than
// returned: the directory is shared with other tools.
func readSSOTokenCacheEntry(path string) (ssoTokenCacheEntry, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ssoTokenCacheEntry{}, false
	}
	var entry ssoTokenCacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return ssoTokenCacheEntry{}, false
	}
	return entry, true
}

func strPtr(s string) *string { return &s }
