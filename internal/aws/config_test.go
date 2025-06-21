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
