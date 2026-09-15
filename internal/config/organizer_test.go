package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOrganizeConfigFile(t *testing.T) {
	// Writing the config takes a backup under $HOME; keep it out of the
	// developer's real ~/.awsm.
	t.Setenv("HOME", t.TempDir())

	// Create a messy config file that simulates real-world disorder
	messyConfig := `[profile dev-admin]
sso_session = company
sso_account_id = 111111111111
sso_role_name = Admin
region = us-east-1

[sso-session company]
sso_start_url = https://company.awsapps.com/start
sso_region = us-east-1
sso_registration_scopes = sso:account:access

[profile prod-readonly]
sso_session = company
sso_account_id = 222222222222
sso_role_name = ReadOnly
region = eu-west-1

[profile my-iam-role]
role_arn = arn:aws:iam::333333333333:role/MyRole
source_profile = dev-admin
region = us-east-1

[profile dev-readonly]
sso_session = company
sso_account_id = 111111111111
sso_role_name = ReadOnly
region = us-east-1

[sso-session other-org]
sso_start_url = https://other.awsapps.com/start
sso_region = eu-west-1

[profile other-dev-admin]
sso_session = other-org
sso_account_id = 444444444444
sso_role_name = Admin
region = eu-west-1

[profile static-keys]
region = us-west-2
output = json
`

	// Write messy config to temp file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config")
	if err := os.WriteFile(configPath, []byte(messyConfig), 0600); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	// Organize it
	if err := OrganizeConfigFile(configPath); err != nil {
		t.Fatalf("OrganizeConfigFile failed: %v", err)
	}

	// Read the result
	result, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read organized config: %v", err)
	}
	organized := string(result)

	// Verify structure: each SSO session block contains its session + profiles
	companyIdx := strings.Index(organized, "# ─── SSO: company")
	otherOrgIdx := strings.Index(organized, "# ─── SSO: other-org")
	iamRolesIdx := strings.Index(organized, "# ─── IAM Role Profiles")
	staticKeysIdx := strings.Index(organized, "# ─── Static Key Profiles")

	if companyIdx == -1 {
		t.Fatal("missing SSO: company header")
	}
	if otherOrgIdx == -1 {
		t.Fatal("missing SSO: other-org header")
	}
	if iamRolesIdx == -1 {
		t.Fatal("missing IAM Role Profiles header")
	}
	if staticKeysIdx == -1 {
		t.Fatal("missing Static Key Profiles header")
	}

	// Verify ordering: SSO blocks first, then IAM, then static
	if companyIdx >= iamRolesIdx {
		t.Error("SSO: company should come before IAM Role Profiles")
	}
	if otherOrgIdx >= iamRolesIdx {
		t.Error("SSO: other-org should come before IAM Role Profiles")
	}
	if iamRolesIdx >= staticKeysIdx {
		t.Error("IAM Role Profiles should come before Static Key Profiles")
	}

	// Verify sso-session company appears BEFORE its profiles
	ssoSessionCompanyIdx := strings.Index(organized, "[sso-session company]")
	devAdminIdx := strings.Index(organized, "[profile dev-admin]")
	devReadonlyIdx := strings.Index(organized, "[profile dev-readonly]")
	prodReadonlyIdx := strings.Index(organized, "[profile prod-readonly]")

	if ssoSessionCompanyIdx == -1 {
		t.Fatal("missing [sso-session company]")
	}
	if ssoSessionCompanyIdx > devAdminIdx {
		t.Error("[sso-session company] should appear before its profiles")
	}

	// Verify company profiles are between "company" and "other-org" headers
	if devAdminIdx < companyIdx || devAdminIdx > otherOrgIdx {
		t.Error("dev-admin should be within company block")
	}
	if devReadonlyIdx < companyIdx || devReadonlyIdx > otherOrgIdx {
		t.Error("dev-readonly should be within company block")
	}
	if prodReadonlyIdx < companyIdx || prodReadonlyIdx > otherOrgIdx {
		t.Error("prod-readonly should be within company block")
	}

	// Verify other-org profiles are in other-org block
	otherDevAdminIdx := strings.Index(organized, "[profile other-dev-admin]")
	if otherDevAdminIdx < otherOrgIdx || otherDevAdminIdx > iamRolesIdx {
		t.Error("other-dev-admin should be within other-org block")
	}

	// Verify grouping: profiles from same account should be together
	if devAdminIdx == -1 || devReadonlyIdx == -1 || prodReadonlyIdx == -1 {
		t.Fatal("missing expected profile sections")
	}

	// dev-admin and dev-readonly share account 111111111111, should be adjacent
	if devAdminIdx > prodReadonlyIdx || devReadonlyIdx > prodReadonlyIdx {
		t.Error("profiles from the same account should be grouped together")
	}

	// Verify account comments are present
	if !strings.Contains(organized, "# Account: 111111111111") {
		t.Error("missing account grouping comment for 111111111111")
	}
	if !strings.Contains(organized, "# Account: 222222222222") {
		t.Error("missing account grouping comment for 222222222222")
	}

	// Verify all profiles preserved
	expectedProfiles := []string{
		"[profile dev-admin]",
		"[profile dev-readonly]",
		"[profile prod-readonly]",
		"[profile my-iam-role]",
		"[profile other-dev-admin]",
		"[profile static-keys]",
	}
	for _, p := range expectedProfiles {
		if !strings.Contains(organized, p) {
			t.Errorf("missing expected profile: %s", p)
		}
	}

	// Verify key alignment (%-24s format: "sso_session" is 11 chars, padded to 24)
	if !strings.Contains(organized, "sso_session             = company") {
		t.Errorf("keys should be aligned with padding, got:\n%s", organized)
	}
}

func TestOrganizeConfigFile_NonExistent(t *testing.T) {
	// Should not error on non-existent file
	err := OrganizeConfigFile("/tmp/definitely-does-not-exist-awsm-test")
	if err != nil {
		t.Errorf("expected nil error for non-existent file, got: %v", err)
	}
}

func TestOrganizeConfigFile_WithDefault(t *testing.T) {
	// Writing the config takes a backup under $HOME; keep it out of the
	// developer's real ~/.awsm.
	t.Setenv("HOME", t.TempDir())

	configContent := `[default]
region = us-east-1
output = json

[sso-session myorg]
sso_start_url = https://myorg.awsapps.com/start
sso_region = us-east-1

[profile dev]
sso_session = myorg
sso_account_id = 123456789012
sso_role_name = Dev
region = us-east-1
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config")
	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	if err := OrganizeConfigFile(configPath); err != nil {
		t.Fatalf("OrganizeConfigFile failed: %v", err)
	}

	result, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read: %v", err)
	}
	organized := string(result)

	// Default section should be at the very top
	defaultIdx := strings.Index(organized, "[default]")
	ssoBlockIdx := strings.Index(organized, "# ─── SSO: myorg")

	if defaultIdx == -1 {
		t.Fatal("missing [default] section")
	}
	if ssoBlockIdx == -1 {
		t.Fatal("missing SSO: myorg header")
	}
	if defaultIdx >= ssoBlockIdx {
		t.Error("[default] should come before SSO block")
	}
}

func TestOrganizerEmitsExternalIDWithTheRoleKeys(t *testing.T) {
	// external_id is meaningless on its own: it belongs next to role_arn and
	// source_profile, not pushed to the end with the unrecognised keys.
	t.Setenv("HOME", t.TempDir())

	path := filepath.Join(t.TempDir(), "config")
	input := `[profile cross]
region = eu-west-1
external_id = SHARED
source_profile = src
role_arn = arn:aws:iam::123456789012:role/target
`
	if err := os.WriteFile(path, []byte(input), 0600); err != nil {
		t.Fatal(err)
	}

	if err := OrganizeConfigFile(path); err != nil {
		t.Fatalf("OrganizeConfigFile: %v", err)
	}

	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)

	roleArn := strings.Index(got, "role_arn")
	externalID := strings.Index(got, "external_id")
	region := strings.Index(got, "region")
	if externalID == -1 {
		t.Fatalf("external_id missing from output:\n%s", got)
	}
	if !(roleArn < externalID && externalID < region) {
		t.Errorf("expected role_arn < external_id < region, got %d, %d, %d in:\n%s",
			roleArn, externalID, region, got)
	}
}
