package aws

import (
	"errors"
	"os"
	"testing"
	"time"

	"awsm/internal/util"
)

// TestFreshCredentialsIgnoreTheCache guards the failure that would make a
// scheduled refresh a silent no-op: the cache is considered good until a minute
// before expiry, so anything renewing ten minutes ahead would be handed the
// very credentials it is trying to replace.
func TestFreshCredentialsIgnoreTheCache(t *testing.T) {
	configPath, _ := useTempAWSFiles(t)
	// source_profile names a profile that does not exist, so a real renewal
	// fails locally without any network call. That failure is the evidence:
	// the cached credentials were not returned.
	if err := os.WriteFile(configPath, []byte(
		"[profile r]\nrole_arn = arn:aws:iam::123456789012:role/t\nsource_profile = nowhere\n"), 0600); err != nil {
		t.Fatal(err)
	}
	setCachedCreds("r", &TempCredentials{
		AccessKeyId:     "CACHED",
		SecretAccessKey: "s",
		SessionToken:    "t",
		Expires:         time.Now().Add(30 * time.Minute),
	})

	cached, _, err := GetCredentialsForProfile("r")
	if err != nil {
		t.Fatalf("the ordinary path should serve the cache: %v", err)
	}
	if cached.AccessKeyId != "CACHED" {
		t.Fatalf("expected the cached credentials, got %q", cached.AccessKeyId)
	}

	fresh, _, err := GetFreshCredentialsForProfile("r")
	if err == nil {
		t.Fatalf("expected a real renewal attempt, got credentials %q", fresh.AccessKeyId)
	}
	if fresh != nil && fresh.AccessKeyId == "CACHED" {
		t.Error("the cache was served despite the cache being bypassed")
	}
}

// TestUnattendedRefreshCannotBlockOnAPrompt guards the other hazard: under a
// service manager there is no terminal, and a path that asks for an MFA code
// would hang or fail obscurely. Declaring the process unattended turns that
// into an immediate, named error.
func TestUnattendedRefreshCannotBlockOnAPrompt(t *testing.T) {
	configPath, _ := useTempAWSFiles(t)
	if err := os.WriteFile(configPath, []byte(`[profile src]
aws_access_key_id = AKIAIOSFODNN7EXAMPLE
aws_secret_access_key = wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
region = eu-west-1

[profile mfa-role]
role_arn = arn:aws:iam::123456789012:role/t
source_profile = src
mfa_serial = arn:aws:iam::123456789012:mfa/u
`), 0600); err != nil {
		t.Fatal(err)
	}

	util.SetNonInteractive(true)
	t.Cleanup(func() { util.SetNonInteractive(false) })

	done := make(chan error, 1)
	go func() {
		_, _, err := GetFreshCredentialsForProfile("mfa-role")
		done <- err
	}()

	select {
	case err := <-done:
		if !errors.Is(err, util.ErrNonInteractive) {
			t.Fatalf("expected ErrNonInteractive, got %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the refresh blocked: an unattended run must never wait on input")
	}
}

// TestMFASessionIsUsedInsteadOfPrompting checks the other half: with a session
// cached, the same profile needs no code at all.
func TestMFASessionIsUsedInsteadOfPrompting(t *testing.T) {
	configPath, _ := useTempAWSFiles(t)
	if err := os.WriteFile(configPath, []byte(`[profile src]
aws_access_key_id = AKIAIOSFODNN7EXAMPLE
aws_secret_access_key = wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY

[profile mfa-role]
role_arn = arn:aws:iam::123456789012:role/t
source_profile = src
mfa_serial = arn:aws:iam::123456789012:mfa/u
`), 0600); err != nil {
		t.Fatal(err)
	}

	if HasValidMFASession("src") {
		t.Fatal("no session should exist yet")
	}
	setCachedMFASession("src", &TempCredentials{
		AccessKeyId:     "ASIASESSION",
		SecretAccessKey: "s",
		SessionToken:    "t",
		Expires:         time.Now().Add(20 * time.Hour),
	})
	if !HasValidMFASession("src") {
		t.Fatal("the cached session should be reported as usable")
	}

	// The session belongs to the source profile, not the role profile: that is
	// whose long-term keys GetSessionToken was called with.
	serial, sessionProfile := MFASerialForProfile("mfa-role")
	if serial == "" {
		t.Error("mfa_serial should have been found")
	}
	if sessionProfile != "src" {
		t.Errorf("session profile = %q, want src", sessionProfile)
	}
}

func TestMFASessionExpiringSoonIsTreatedAsAbsent(t *testing.T) {
	// A session about to expire would mint role credentials that die with it.
	useTempAWSFiles(t)
	setCachedMFASession("src", &TempCredentials{
		AccessKeyId: "ASIA", SecretAccessKey: "s", SessionToken: "t",
		Expires: time.Now().Add(time.Minute),
	})
	if HasValidMFASession("src") {
		t.Error("a session with a minute left should not be considered usable")
	}
}

func TestClassifyProfile(t *testing.T) {
	configPath, _ := useTempAWSFiles(t)
	if err := os.WriteFile(configPath, []byte(`[profile static]
aws_access_key_id = AKIA
aws_secret_access_key = s

[profile role]
role_arn = arn:aws:iam::1:role/t
source_profile = static

[profile role-mfa]
role_arn = arn:aws:iam::1:role/t
source_profile = static
mfa_serial = arn:aws:iam::1:mfa/u

[profile sso]
sso_session = corp
sso_account_id = 123456789012
sso_role_name = Admin

[profile proc]
credential_process = /bin/echo
`), 0600); err != nil {
		t.Fatal(err)
	}

	for profile, want := range map[string]ProfileKind{
		"static":   KindStatic,
		"role":     KindRole,
		"role-mfa": KindRoleMFA,
		"sso":      KindSSO,
		"proc":     KindProcess,
	} {
		got, err := ClassifyProfile(profile)
		if err != nil {
			t.Errorf("ClassifyProfile(%q): %v", profile, err)
			continue
		}
		if got != want {
			t.Errorf("ClassifyProfile(%q) = %q, want %q", profile, got, want)
		}
	}

	// Only the kinds that need nothing from a person are refreshable on their
	// own; role-mfa depends on a cached session and is decided elsewhere.
	if KindRoleMFA.Refreshable() || KindStatic.Refreshable() {
		t.Error("role-mfa and static must not claim to be unattended-refreshable")
	}
	if !KindRole.Refreshable() || !KindSSO.Refreshable() {
		t.Error("role and sso are refreshable without a person")
	}
}
