package config

import (
	"strings"
	"testing"
)

// twoSessionConfig is shaped the way OrganizeConfigFile lays out a real file:
// each sso-session block is followed by its own profiles, and unrelated
// sections come last.
const twoSessionConfig = `[default]
region = eu-west-1

# ─── SSO: alpha ───

[sso-session alpha]
sso_start_url = https://alpha.awsapps.com/start/
sso_region = eu-west-1

# Account: 111111111111
[profile alpha-admin]
sso_session = alpha
sso_account_id = 111111111111

# ─── SSO: beta ───

[sso-session beta]
sso_start_url = https://beta.awsapps.com/start/
sso_region = us-east-1

[profile beta-admin]
sso_session = beta
sso_account_id = 222222222222

[services my-svc]
sso-oidc =
  endpoint_url = https://example.com
`

// Removing a session's profiles used to delete every section between the last
// removed profile and the next "[profile ...]" header, so syncing one session
// silently destroyed the other session's sso-session block.
func TestRemoveAllProfilesForSessionKeepsOtherSections(t *testing.T) {
	out, removed := RemoveAllProfilesForSession(twoSessionConfig, "alpha")

	if len(removed) != 1 || removed[0] != "alpha-admin" {
		t.Fatalf("removed = %v, want [alpha-admin]", removed)
	}
	if strings.Contains(out, "[profile alpha-admin]") {
		t.Error("the requested profile was not removed")
	}
	for _, want := range []string{
		"[default]",
		"[sso-session alpha]",
		"[sso-session beta]",
		"[profile beta-admin]",
		"[services my-svc]",
		"endpoint_url = https://example.com",
		"# ─── SSO: beta ───",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("removal swallowed %q:\n%s", want, out)
		}
	}
	// The comment introducing the removed profile belongs to it.
	if strings.Contains(out, "# Account: 111111111111") {
		t.Errorf("comment of the removed profile was left orphaned:\n%s", out)
	}
}

// When the removed profile was the last one in the file, the old end-of-section
// scan fell through to len(config) and truncated everything after it.
func TestRemoveAllProfilesForSessionDoesNotTruncateToEOF(t *testing.T) {
	out, removed := RemoveAllProfilesForSession(twoSessionConfig, "beta")

	if len(removed) != 1 || removed[0] != "beta-admin" {
		t.Fatalf("removed = %v, want [beta-admin]", removed)
	}
	if strings.Contains(out, "[profile beta-admin]") {
		t.Error("the requested profile was not removed")
	}
	if !strings.Contains(out, "[services my-svc]") {
		t.Errorf("trailing section was truncated:\n%s", out)
	}
	if !strings.Contains(out, "endpoint_url = https://example.com") {
		t.Errorf("trailing section lost its nested values:\n%s", out)
	}
}

func TestRemoveProfileFromConfigWithNoFollowingSection(t *testing.T) {
	in := "[default]\nregion = eu-west-1\n\n[profile only]\nregion = us-east-1\n"
	out := RemoveProfileFromConfig(in, "only")

	if strings.Contains(out, "[profile only]") {
		t.Error("profile not removed")
	}
	if !strings.Contains(out, "[default]") {
		t.Errorf("preceding section was removed too:\n%s", out)
	}
}

func TestRemoveProfileFromConfigLeavesUnknownProfileAlone(t *testing.T) {
	if out := RemoveProfileFromConfig(twoSessionConfig, "nope"); out != twoSessionConfig {
		t.Error("config changed while removing a profile that does not exist")
	}
}

// A profile's content must stop at the next section header of any kind.
// Absorbing the following sections made the sso_session lookup match on keys
// that belong to a different section.
func TestParseExistingProfilesStopsAtAnySectionHeader(t *testing.T) {
	_, contents := ParseExistingProfiles(twoSessionConfig)

	alpha, ok := contents["alpha-admin"]
	if !ok {
		t.Fatal("alpha-admin not parsed")
	}
	if strings.Contains(alpha, "sso-session beta") {
		t.Errorf("profile content absorbed the next section:\n%s", alpha)
	}
	if !strings.Contains(alpha, "sso_account_id = 111111111111") {
		t.Errorf("profile content is incomplete:\n%s", alpha)
	}

	beta, ok := contents["beta-admin"]
	if !ok {
		t.Fatal("beta-admin not parsed")
	}
	if strings.Contains(beta, "[services my-svc]") {
		t.Errorf("profile content absorbed a trailing section:\n%s", beta)
	}
}

// A profile referencing another session must not be removed just because an
// unrelated section between it and the next profile mentions that session.
func TestRemoveAllProfilesForSessionDoesNotMatchOtherSections(t *testing.T) {
	in := `[profile keep-me]
sso_session = alpha
sso_account_id = 111111111111

[default]
sso_session = beta
`
	out, removed := RemoveAllProfilesForSession(in, "beta")

	if len(removed) != 0 {
		t.Errorf("removed = %v, want none", removed)
	}
	if !strings.Contains(out, "[profile keep-me]") {
		t.Errorf("wrong profile removed:\n%s", out)
	}
}
