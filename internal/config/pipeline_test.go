package config

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestSSOUpdatePipeline exercises the exact sequence runSSOUpdate applies to
// ~/.aws/config once the AWS API calls are done: read, drop the session's old
// profiles, append the freshly discovered ones, write, organize.
//
// Every step used to lose something: the removal took neighbouring sections
// with it and the rewrite flattened nested sub-sections and deleted comments.
func TestSSOUpdatePipeline(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	original := `# Hand written, please keep
[default]
region = eu-west-1
s3 =
  max_concurrent_requests = 20

# ─── SSO: alpha ───

[sso-session alpha]
sso_start_url = https://alpha.awsapps.com/start/
sso_region = eu-west-1
sso_registration_scopes = sso:account:access

# Account: 111111111111
[profile alpha-old-role]
sso_session = alpha
sso_account_id = 111111111111
sso_role_name = OldRole
region = eu-west-1

# ─── SSO: beta ───

[sso-session beta]
sso_start_url = https://beta.awsapps.com/start/
sso_region = us-east-1
sso_registration_scopes = sso:account:access

[profile beta-admin]
sso_session = beta
sso_account_id = 222222222222
sso_role_name = Admin
region = us-east-1

[services my-svc]
sso-oidc =
  endpoint_url = https://example.com

[profile static-user]
region = eu-central-1
`

	path := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(path, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}

	existing, err := ReadConfigFile(path)
	if err != nil {
		t.Fatal(err)
	}

	existing, removed := RemoveAllProfilesForSession(existing, "alpha")
	if len(removed) != 1 || removed[0] != "alpha-old-role" {
		t.Fatalf("removed = %v, want [alpha-old-role]", removed)
	}

	generated, renames := ResolveProfileNameCollisions([]GeneratedProfile{
		{Name: "alpha-prod-admin", SSOSession: "alpha", AccountID: "111111111111", RoleName: "Admin", Region: "eu-west-1"},
		{Name: "alpha-prod-admin", SSOSession: "alpha", AccountID: "333333333333", RoleName: "Admin", Region: "eu-west-1"},
	})
	if len(renames) != 2 {
		t.Fatalf("renames = %v, want 2", renames)
	}

	var b strings.Builder
	for _, g := range generated {
		b.WriteString("[profile " + g.Name + "]\nsso_session = " + g.SSOSession +
			"\nsso_account_id = " + g.AccountID + "\nsso_role_name = " + g.RoleName +
			"\nregion = " + g.Region + "\n\n")
	}
	if err := WriteConfigFile(path, existing+"\n"+b.String()); err != nil {
		t.Fatal(err)
	}
	if err := OrganizeConfigFile(path); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	final := string(data)

	// Nothing that belonged to another session or to the user may be gone.
	for _, want := range []string{
		"# Hand written, please keep",
		"[default]",
		"  max_concurrent_requests = 20",
		"[sso-session alpha]",
		"[sso-session beta]",
		"[profile beta-admin]",
		"[profile static-user]",
		"[services my-svc]",
		"  endpoint_url = https://example.com",
		"[profile alpha-prod-admin-111111111111]",
		"[profile alpha-prod-admin-333333333333]",
	} {
		if !strings.Contains(final, want) {
			t.Errorf("pipeline lost %q:\n%s", want, final)
		}
	}

	// The stale profile is the only thing that should have disappeared.
	if strings.Contains(final, "alpha-old-role") {
		t.Errorf("stale profile survived:\n%s", final)
	}

	// A duplicate header makes the AWS CLI refuse to parse the whole file.
	assertNoDuplicateSections(t, final)

	// Nested children must never be promoted to profile level.
	for _, line := range strings.Split(final, "\n") {
		if strings.HasPrefix(line, "max_concurrent_requests") || strings.HasPrefix(line, "endpoint_url") {
			t.Errorf("nested key flattened to top level:\n%s", final)
		}
	}
}

func assertNoDuplicateSections(t *testing.T, config string) {
	t.Helper()
	headerRegex := regexp.MustCompile(`(?m)^\[([^\]]*)\]`)
	seen := map[string]bool{}
	for _, m := range headerRegex.FindAllStringSubmatch(config, -1) {
		if seen[m[1]] {
			t.Errorf("duplicate section [%s]:\n%s", m[1], config)
		}
		seen[m[1]] = true
	}
}
