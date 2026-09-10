package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const annotatedConfig = `# My AWS config - keep these notes
[default]
region = eu-west-1
s3 =
  max_concurrent_requests = 20
  addressing_style = path

# prod is the one that matters
[profile prod]
sso_session = company
sso_account_id = 111111111111
sso_role_name = Admin
region = eu-west-1

[sso-session company]
sso_start_url = https://company.awsapps.com/start/
sso_region = eu-west-1
sso_registration_scopes = sso:account:access

[services my-svc]
sso-oidc =
  endpoint_url = https://example.com
`

// organize runs OrganizeConfigFile on content and returns the result. HOME is
// redirected so backups land in a scratch directory.
func organize(t *testing.T, content string) string {
	t.Helper()
	t.Setenv("HOME", t.TempDir())

	path := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	if err := OrganizeConfigFile(path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// Rebuilding the file used to drop the indented children of a nested key,
// leaving "s3 =" with an empty value - the exact shape that makes botocore
// abort with "'str' object has no attribute 'get'".
func TestOrganizeConfigFileKeepsNestedSubSections(t *testing.T) {
	out := organize(t, annotatedConfig)

	for _, want := range []string{
		"  max_concurrent_requests = 20",
		"  addressing_style = path",
		"  endpoint_url = https://example.com",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("nested value %q lost or unindented:\n%s", want, out)
		}
	}
	for _, line := range strings.Split(out, "\n") {
		for _, orphan := range []string{"max_concurrent_requests", "addressing_style", "endpoint_url"} {
			if strings.HasPrefix(line, orphan) {
				t.Errorf("nested key %q flattened to top level:\n%s", orphan, out)
			}
		}
	}
}

func TestOrganizeConfigFileKeepsUserComments(t *testing.T) {
	out := organize(t, annotatedConfig)

	for _, want := range []string{
		"# My AWS config - keep these notes",
		"# prod is the one that matters",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("user comment %q was deleted:\n%s", want, out)
		}
	}
}

// The banners the organizer writes come back attached to the following section
// on the next read, so a second run must not stack another copy of them.
func TestOrganizeConfigFileIsIdempotent(t *testing.T) {
	first := organize(t, annotatedConfig)
	second := organize(t, first)

	if first != second {
		t.Errorf("second run changed the file:\n--- first ---\n%s\n--- second ---\n%s", first, second)
	}
	if n := strings.Count(second, "# ─── SSO: company"); n != 1 {
		t.Errorf("SSO banner appears %d times, want 1:\n%s", n, second)
	}
	if n := strings.Count(second, "# Account: 111111111111"); n != 1 {
		t.Errorf("account heading appears %d times, want 1:\n%s", n, second)
	}
	if n := strings.Count(second, "# My AWS config - keep these notes"); n != 1 {
		t.Errorf("user comment appears %d times, want 1:\n%s", n, second)
	}
}

// The organized file has to stay parseable, and every section has to survive.
func TestOrganizeConfigFileKeepsEverySection(t *testing.T) {
	out := organize(t, annotatedConfig)

	for _, want := range []string{
		"[default]",
		"[profile prod]",
		"[sso-session company]",
		"[services my-svc]",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("section %q was dropped:\n%s", want, out)
		}
	}
}
