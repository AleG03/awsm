package aws

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetAWSConfigPath(t *testing.T) {
	// Test with environment variable
	testPath := "/custom/aws/config"
	os.Setenv("AWS_CONFIG_FILE", testPath)
	defer os.Unsetenv("AWS_CONFIG_FILE")

	path, err := GetAWSConfigPath()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if path != testPath {
		t.Errorf("Expected %s, got %s", testPath, path)
	}

	// Test without environment variable
	os.Unsetenv("AWS_CONFIG_FILE")
	path, err = GetAWSConfigPath()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".aws", "config")
	if path != expected {
		t.Errorf("Expected %s, got %s", expected, path)
	}
}

func TestProfileTypes(t *testing.T) {
	tests := []struct {
		name     string
		pType    ProfileType
		expected string
	}{
		{"SSO Profile", ProfileTypeSSO, "SSO"},
		{"IAM Profile", ProfileTypeIAM, "IAM"},
		{"Key Profile", ProfileTypeKey, "Key"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.pType) != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, string(tt.pType))
			}
		})
	}
}

func TestProfileInfo(t *testing.T) {
	profile := ProfileInfo{
		Name:         "test-profile",
		Type:         ProfileTypeSSO,
		Region:       "us-east-1",
		SSOAccountID: "123456789012",
		IsActive:     true,
	}

	if profile.Name != "test-profile" {
		t.Errorf("Expected test-profile, got %s", profile.Name)
	}
	if profile.Type != ProfileTypeSSO {
		t.Errorf("Expected SSO, got %s", profile.Type)
	}
	if !profile.IsActive {
		t.Error("Expected profile to be active")
	}
}

func TestGetSsoSessionForProfile_Chained(t *testing.T) {
	// Create a temporary config file
	content := `
[profile sso-profile]
sso_session = my-session
sso_account_id = 123456789012
sso_role_name = Admin
region = us-east-1

[profile intermediate-profile]
source_profile = sso-profile
role_arn = arn:aws:iam::123456789012:role/RoleA

[profile leaf-profile]
source_profile = intermediate-profile
role_arn = arn:aws:iam::123456789012:role/RoleB

[sso-session my-session]
sso_start_url = https://example.awsapps.com/start
sso_region = us-east-1
sso_registration_scopes = sso:account:access
`
	tmpfile, err := os.CreateTemp("", "aws-config")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name()) // clean up

	if _, err := tmpfile.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	// Override config path
	os.Setenv("AWS_CONFIG_FILE", tmpfile.Name())
	defer os.Unsetenv("AWS_CONFIG_FILE")

	// Test case 1: Direct SSO profile
	session, err := GetSsoSessionForProfile("sso-profile")
	if err != nil {
		t.Errorf("Failed to get session for sso-profile: %v", err)
	}
	if session != "my-session" {
		t.Errorf("Expected session 'my-session' for sso-profile, got '%s'", session)
	}

	// Test case 2: Chained profile (1 level)
	session, err = GetSsoSessionForProfile("intermediate-profile")
	if err != nil {
		t.Errorf("Failed to get session for intermediate-profile: %v", err)
	}
	if session != "my-session" {
		t.Errorf("Expected session 'my-session' for intermediate-profile, got '%s'", session)
	}

	// Test case 3: Chained profile (2 levels)
	session, err = GetSsoSessionForProfile("leaf-profile")
	if err != nil {
		t.Errorf("Failed to get session for leaf-profile: %v", err)
	}
	if session != "my-session" {
		t.Errorf("Expected session 'my-session' for leaf-profile, got '%s'", session)
	}
}
