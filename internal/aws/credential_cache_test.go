package aws

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRoleCacheInvalidatedByProfileAndSourceChanges(t *testing.T) {
	for _, change := range []string{"role", "source", "source keys", "external id", "MFA serial"} {
		t.Run(change, func(t *testing.T) {
			configPath, _ := useTempAWSFiles(t)
			original := "[profile work]\nrole_arn = arn:aws:iam::111111111111:role/old\nsource_profile = base\nexternal_id = old\nmfa_serial = old-device\n[profile base]\naws_access_key_id = OLD\naws_secret_access_key = fake\n"
			if err := os.WriteFile(configPath, []byte(original), 0600); err != nil {
				t.Fatal(err)
			}
			setCachedCreds("work", &TempCredentials{AccessKeyId: "CACHED-OLD-ACCOUNT", Expires: time.Now().Add(time.Hour)})
			if !HasValidCachedCredentials("work") {
				t.Fatal("unchanged profile lost cache")
			}
			replacements := map[string][2]string{
				"role":        {"111111111111:role/old", "222222222222:role/new"},
				"source":      {"source_profile = base", "source_profile = other"},
				"source keys": {"aws_access_key_id = OLD", "aws_access_key_id = NEW"},
				"external id": {"external_id = old", "external_id = new"},
				"MFA serial":  {"mfa_serial = old-device", "mfa_serial = new-device"},
			}
			r := replacements[change]
			if err := os.WriteFile(configPath, []byte(strings.ReplaceAll(original, r[0], r[1])), 0600); err != nil {
				t.Fatal(err)
			}
			if HasValidCachedCredentials("work") {
				t.Fatal("changed identity still uses old credentials")
			}
			if _, ok := CachedCredentialsExpiry("work"); ok {
				t.Fatal("old cache expiry still shown")
			}
		})
	}
}

func TestEditRoleRejectsOldCachedAccount(t *testing.T) {
	useTempAWSFiles(t)
	if err := AddIAMRoleProfile("work", IAMRoleProfile{RoleARN: "arn:aws:iam::111111111111:role/old", SourceProfile: "missing"}); err != nil {
		t.Fatal(err)
	}
	setCachedCreds("work", &TempCredentials{AccessKeyId: "OLD-ACCOUNT", Expires: time.Now().Add(time.Hour)})
	if err := UpdateIAMRoleProfile("work", IAMRoleProfile{RoleARN: "arn:aws:iam::222222222222:role/new", SourceProfile: "missing"}); err != nil {
		t.Fatal(err)
	}
	got, _, err := GetCredentialsForProfile("work")
	// Missing source forces a local error rather than contacting STS.
	if err == nil || got != nil {
		t.Fatal("edited role returned stale credentials")
	}
}

func TestCacheNamespacesAndProfilePaths(t *testing.T) {
	useTempAWSFiles(t)
	role, _ := credsCachePath("mfa-session-base")
	mfa, _ := mfaSessionCachePath("base")
	if role == mfa {
		t.Fatal("role and MFA caches collide")
	}
	for _, name := range []string{"base", "../base", "../../mfa/base", `..\base`, "/absolute"} {
		path, _ := credsCachePath(name)
		if filepath.Dir(path) != filepath.Dir(role) {
			t.Fatalf("profile %q escaped cache directory", name)
		}
	}
	setCachedMFASession("base", &TempCredentials{AccessKeyId: "MFA", Expires: time.Now().Add(time.Hour)})
	setCachedCreds("mfa-session-base", &TempCredentials{AccessKeyId: "ROLE", Expires: time.Now().Add(time.Hour)})
	if got := GetCachedMFASession("base"); got == nil || got.AccessKeyId != "MFA" {
		t.Fatal("role write replaced MFA session")
	}
	if got := getCachedCreds("mfa-session-base"); got == nil || got.AccessKeyId != "ROLE" {
		t.Fatal("MFA write replaced role")
	}
}

func TestCacheSurvivesUnrelatedChangesButNotSourceKeyRotation(t *testing.T) {
	_, credentialsPath := useTempAWSFiles(t)
	if err := AddIAMUserProfile("base", "OLD", "fake", ""); err != nil {
		t.Fatal(err)
	}
	if err := AddIAMRoleProfile("work", IAMRoleProfile{RoleARN: "arn:aws:iam::1:role/test", SourceProfile: "base"}); err != nil {
		t.Fatal(err)
	}
	c := &TempCredentials{AccessKeyId: "CACHED", Expires: time.Now().Add(time.Hour)}
	setCachedCreds("work", c)
	setCachedMFASession("base", c)
	if err := UpdateCredentialsFile(c, "eu-west-1", "unrelated"); err != nil {
		t.Fatal(err)
	}
	if !HasValidCachedCredentials("work") || !HasValidMFASession("base") {
		t.Fatal("default write invalidated unrelated cache")
	}
	if err := os.WriteFile(credentialsPath, []byte("[base]\naws_access_key_id = ROTATED\naws_secret_access_key = fake\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if HasValidCachedCredentials("work") || HasValidMFASession("base") {
		t.Fatal("source key rotation retained role or MFA credentials")
	}
}

func TestCacheIsolatedByAWSFilePaths(t *testing.T) {
	configPath, _ := useTempAWSFiles(t)
	if err := AddIAMRoleProfile("work", IAMRoleProfile{RoleARN: "arn:aws:iam::1:role/test"}); err != nil {
		t.Fatal(err)
	}
	setCachedCreds("work", &TempCredentials{Expires: time.Now().Add(time.Hour)})
	raw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(other, raw, 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AWS_CONFIG_FILE", other)
	if HasValidCachedCredentials("work") {
		t.Fatal("different config reused cache by name")
	}
}

func TestLegacyCacheIsNotTrusted(t *testing.T) {
	configPath, _ := useTempAWSFiles(t)
	dir := filepath.Join(filepath.Dir(configPath), ".awsm", "cache")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "work.json"), []byte(`{"access_key_id":"OLD","expires":"2099-01-01T00:00:00Z"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if HasValidCachedCredentials("work") {
		t.Fatal("legacy cache has no verifiable identity")
	}
}

func TestCredentialsWithoutConfig(t *testing.T) {
	_, cp := useTempAWSFiles(t)
	if err := os.WriteFile(cp, []byte("[work]\naws_access_key_id = FAKE\naws_secret_access_key = fake\n"), 0600); err != nil {
		t.Fatal(err)
	}
	got, static, err := GetCredentialsForProfile("work")
	if err != nil {
		t.Fatal(err)
	}
	if !static || got.AccessKeyId != "FAKE" {
		t.Fatal("credentials-only profile resolved incorrectly")
	}
	if err := UpdateStaticProfile("work"); err != nil {
		t.Fatal(err)
	}
	if GetCurrentProfileName() != "work" {
		t.Fatal("could not activate credentials-only profile")
	}
}
