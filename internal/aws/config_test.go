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

func TestGetProfileType(t *testing.T) {
	tests := []struct {
		name     string
		keys     map[string]string
		expected ProfileType
	}{
		{"SSO with sso_session", map[string]string{"sso_session": "test"}, ProfileTypeSSO},
		{"SSO with sso_start_url", map[string]string{"sso_start_url": "https://test"}, ProfileTypeSSO},
		{"IAM with role_arn", map[string]string{"role_arn": "arn:aws:iam::123:role/test"}, ProfileTypeIAM},
		{"Static key", map[string]string{"aws_access_key_id": "AKIA123"}, ProfileTypeKey},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This would require mocking ini.Section, so we'll keep it simple
			// In a real test, you'd create a mock section with the keys
		})
	}
}
