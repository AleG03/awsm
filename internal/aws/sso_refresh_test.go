package aws

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeSSOCache(t *testing.T, home string, entry map[string]any) string {
	t.Helper()
	dir := filepath.Join(home, ".aws", "sso", "cache")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "token.json")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func ssoProfileConfig(t *testing.T, configPath string) {
	t.Helper()
	if err := os.WriteFile(configPath, []byte(`[sso-session corp]
sso_start_url = https://example.awsapps.com/start/
sso_region = eu-west-1

[profile work]
sso_session = corp
sso_account_id = 123456789012
sso_role_name = Admin
`), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestSSOTokenCacheFileIsFoundByStartURL(t *testing.T) {
	configPath, _ := useTempAWSFiles(t)
	home, _ := os.UserHomeDir()
	ssoProfileConfig(t, configPath)
	want := writeSSOCache(t, home, map[string]any{
		"startUrl":  "https://example.awsapps.com/start/",
		"region":    "eu-west-1",
		"expiresAt": "2026-09-16T12:00:00Z",
	})

	got, ok := ssoTokenCacheFile("work")
	if !ok {
		t.Fatal("the cache entry should have been found via the sso-session start URL")
	}
	if got != want {
		t.Errorf("found %q, want %q", got, want)
	}
}

// writeSSOCacheAt writes a cache entry under a chosen file name, which is how a
// test can put something at the name sha1(session) produces.
func writeSSOCacheAt(t *testing.T, home, name string, entry map[string]any) string {
	t.Helper()
	dir := filepath.Join(home, ".aws", "sso", "cache")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestSSOTokenCacheFileIsFoundByTheSessionName: the deterministic name is where
// a login writes and where the AWS CLI looks, so it must win over a scan.
func TestSSOTokenCacheFileIsFoundByTheSessionName(t *testing.T) {
	configPath, _ := useTempAWSFiles(t)
	home, _ := os.UserHomeDir()
	ssoProfileConfig(t, configPath)

	deterministic, err := ssoCacheFileFor("corp")
	if err != nil {
		t.Fatal(err)
	}
	want := writeSSOCacheAt(t, home, filepath.Base(deterministic), map[string]any{
		"startUrl": "https://example.awsapps.com/start/",
		"region":   "eu-west-1",
	})

	// A second entry describing the same start URL, under a name the directory
	// scan reaches first -- os.ReadDir sorts, and "0" precedes any hex digest.
	//
	// Without it this test proves nothing: one matching file is found either
	// way, so a scan that has quietly replaced the deterministic lookup passes
	// unnoticed. Two files are what make the preference observable, and two
	// files are also the real situation -- a cache written before sso-session
	// blocks existed, sitting beside the one a login writes today.
	decoy := writeSSOCacheAt(t, home, "0-stale.json", map[string]any{
		"startUrl": "https://example.awsapps.com/start/",
		"region":   "eu-west-1",
	})

	got, ok := ssoTokenCacheFile("work")
	if !ok {
		t.Fatal("the cache entry was not found")
	}
	if got == decoy {
		t.Fatal("scanned the directory instead of going straight to sha1(session name); the stale entry won")
	}
	if got != want {
		t.Errorf("found %q, want the deterministic name %q", got, want)
	}
}

// TestSSOTokenCacheFileIgnoresAnEntryForAnotherStartURL.
//
// sha1 is taken over the session's *name*, so a name reused for a different
// Identity Center lands on the file the previous one left. Returning it would
// have RefreshSSOToken renew that stale entry, and write to it, while the live
// token sits in a file nobody opened -- a session that never renews and never
// says why.
func TestSSOTokenCacheFileIgnoresAnEntryForAnotherStartURL(t *testing.T) {
	configPath, _ := useTempAWSFiles(t)
	home, _ := os.UserHomeDir()
	ssoProfileConfig(t, configPath)

	deterministic, err := ssoCacheFileFor("corp")
	if err != nil {
		t.Fatal(err)
	}
	// A leftover from a different Identity Center, at exactly the name the
	// session's own entry would have.
	writeSSOCacheAt(t, home, filepath.Base(deterministic), map[string]any{
		"startUrl": "https://someone-else.awsapps.com/start/",
		"region":   "us-east-1",
	})
	// The live entry, under a name only a scan will reach.
	want := writeSSOCacheAt(t, home, "legacy.json", map[string]any{
		"startUrl": "https://example.awsapps.com/start/",
		"region":   "eu-west-1",
	})

	got, ok := ssoTokenCacheFile("work")
	if !ok {
		t.Fatal("the cache entry was not found")
	}
	if got == deterministic {
		t.Fatal("returned an entry belonging to a different start URL")
	}
	if got != want {
		t.Errorf("found %q, want %q", got, want)
	}
}

func TestRefreshSSOTokenRefusesATokenThatCannotBeRefreshed(t *testing.T) {
	// A token predating refresh tokens can only be replaced by a real login,
	// and saying so beats a failed API call the user has to decode.
	configPath, _ := useTempAWSFiles(t)
	home, _ := os.UserHomeDir()
	ssoProfileConfig(t, configPath)
	writeSSOCache(t, home, map[string]any{
		"startUrl":    "https://example.awsapps.com/start/",
		"region":      "eu-west-1",
		"accessToken": "old",
		"expiresAt":   "2026-09-16T12:00:00Z",
	})

	err := RefreshSSOToken("work")
	if err == nil {
		t.Fatal("expected an error for a token with no refreshToken")
	}
	if !strings.Contains(err.Error(), "new login") {
		t.Errorf("the error should say a login is needed, got: %v", err)
	}
}

func TestRefreshSSOTokenReportsAMissingCacheEntry(t *testing.T) {
	configPath, _ := useTempAWSFiles(t)
	ssoProfileConfig(t, configPath)

	if err := RefreshSSOToken("work"); err == nil {
		t.Fatal("expected an error when there is no cached token at all")
	}
}

func TestWriteSSOTokenCachePreservesEverythingElse(t *testing.T) {
	// The AWS CLI writes fields awsm has no reason to understand, and dropping
	// them on rotation would corrupt a file the CLI still has to read.
	home := t.TempDir()
	t.Setenv("HOME", home)
	path := writeSSOCache(t, home, map[string]any{
		"startUrl":              "https://example.awsapps.com/start/",
		"region":                "eu-west-1",
		"accessToken":           "old",
		"refreshToken":          "old-refresh",
		"clientId":              "cid",
		"clientSecret":          "secret",
		"expiresAt":             "2026-09-16T12:00:00Z",
		"registrationExpiresAt": "2026-11-16T10:15:38Z",
		"somethingNewFromAWS":   "keep me",
	})

	var fields map[string]any
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	fields["accessToken"] = "new"
	fields["refreshToken"] = "new-refresh"

	if err := writeSSOTokenCache(path, fields); err != nil {
		t.Fatalf("writeSSOTokenCache: %v", err)
	}

	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("the rewritten file is not valid JSON: %v", err)
	}
	for key, want := range map[string]string{
		"accessToken":           "new",
		"refreshToken":          "new-refresh",
		"registrationExpiresAt": "2026-11-16T10:15:38Z",
		"somethingNewFromAWS":   "keep me",
		"clientSecret":          "secret",
	} {
		if got[key] != want {
			t.Errorf("%s = %v, want %q", key, got[key], want)
		}
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("mode = %o, want 600: the file holds a bearer token", info.Mode().Perm())
	}
}

func TestRefreshSSOTokenDistinguishesADeadSessionFromAFailure(t *testing.T) {
	// The daemon retries transient failures every cycle but must announce a
	// dead refresh token once, so the two have to be distinguishable.
	err := fmt.Errorf("operation error SSO OIDC: CreateToken, InvalidGrantException: ")
	wrapped := fmt.Errorf("%w: the refresh token for 'x' is no longer valid", ErrSsoSessionExpired)

	if !errors.Is(wrapped, ErrSsoSessionExpired) {
		t.Error("a dead refresh token must be recognisable as an expired SSO session")
	}
	if errors.Is(err, ErrSsoSessionExpired) {
		t.Error("a raw API error must not be mistaken for a classified one")
	}
}
