package daemon

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"
)

// ssoHome lays out a home with one SSO profile active, a live SSO token, and
// credentials in the default profile expiring at the time given.
func ssoHome(t *testing.T, credentialsExpiry string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)

	const session = "corp"
	const startURL = "https://example.awsapps.com/start"

	if err := writeFile(filepath.Join(home, ".aws", "config"), `[sso-session `+session+`]
sso_start_url = `+startURL+`
sso_region = eu-west-1

[profile work]
sso_session = `+session+`
sso_account_id = 123456789012
sso_role_name = Admin
region = eu-west-1
`); err != nil {
		t.Fatal(err)
	}

	credentials := `[default]
aws_access_key_id = AKIAEXAMPLE
aws_secret_access_key = secret
aws_session_token = token
region = eu-west-1
# source_profile = work
`
	if credentialsExpiry != "" {
		credentials += "# expires = " + credentialsExpiry + "\n"
	}
	if err := writeFile(filepath.Join(home, ".aws", "credentials"), credentials); err != nil {
		t.Fatal(err)
	}

	digest := sha1.Sum([]byte(session))
	token, err := json.Marshal(map[string]any{
		"startUrl":  startURL,
		"region":    "eu-west-1",
		"expiresAt": time.Now().UTC().Add(50 * time.Minute).Format("2006-01-02T15:04:05Z"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := writeFile(filepath.Join(home, ".aws", "sso", "cache",
		hex.EncodeToString(digest[:])+".json"), string(token)); err != nil {
		t.Fatal(err)
	}
}

// TestAnSsoProfileIsSeenAsHavingCredentials.
//
// It was not. awsm's credential cache only ever held the profiles resolved
// through STS, so HaveCredentials was false for every SSO profile, Decide
// answered "no cached credentials", and the renewal below that line was never
// reached: the daemon rotated the SSO token and let the credentials in the
// default profile lapse. Sixteen renewals in one real daemon's whole history,
// every one of them a role profile.
func TestAnSsoProfileIsSeenAsHavingCredentials(t *testing.T) {
	expiry := time.Now().Add(40 * time.Minute).UTC().Truncate(time.Second)
	ssoHome(t, expiry.Format(time.RFC3339))

	s := takeSnapshot(time.Now())

	if s.Kind != KindSSO {
		t.Fatalf("Kind = %v, want the SSO profile to be recognised", s.Kind)
	}
	if !s.HaveCredentials {
		t.Fatal("HaveCredentials is false for an SSO profile whose credentials expire in 40 minutes; " +
			"Decide would answer \"no cached credentials\" and never renew them")
	}
	if !s.CredentialsExpiry.Equal(expiry) {
		t.Errorf("CredentialsExpiry = %s, want %s", s.CredentialsExpiry, expiry)
	}
}

// TestAnSsoProfileNearExpiryIsRenewed: the decision, not just the snapshot.
func TestAnSsoProfileNearExpiryIsRenewed(t *testing.T) {
	// Inside the threshold: the point at which the daemon should act.
	expiry := time.Now().Add(CredentialsThreshold / 2).UTC().Truncate(time.Second)
	ssoHome(t, expiry.Format(time.RFC3339))

	decision := Decide(takeSnapshot(time.Now()))

	if decision.Action != ActionRefresh {
		t.Errorf("Decide() = %v (%s), want %v: credentials this close to expiry must be renewed",
			decision.Action, decision.Reason, ActionRefresh)
	}
}

// TestWithoutARecordedExpiryThereIsStillNothingToRenew keeps the old behaviour
// honest: no record and nothing cached means the daemon says so, rather than
// inventing a deadline.
func TestWithoutARecordedExpiryThereIsStillNothingToRenew(t *testing.T) {
	ssoHome(t, "")

	s := takeSnapshot(time.Now())

	if s.HaveCredentials {
		t.Error("claimed credentials whose expiry nothing records")
	}
}
