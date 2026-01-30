package aws

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInspectProfileCredentialProcess(t *testing.T) {
	// Create temporary config file
	tmpDir, err := os.MkdirTemp("", "awsm-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config")
	configContent := `[profile test-process]
credential_process = echo '{"Version": 1, "AccessKeyId": "AKIA123", "SecretAccessKey": "secret123", "SessionToken": "token123", "Expiration": "2026-01-30T15:00:00Z"}'
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Set environment variable to point to our temp config
	originalConfig := os.Getenv("AWS_CONFIG_FILE")
	os.Setenv("AWS_CONFIG_FILE", configPath)
	defer os.Setenv("AWS_CONFIG_FILE", originalConfig)

	// For credentials file, just use an empty one to avoid interference
	credPath := filepath.Join(tmpDir, "credentials")
	if err := os.WriteFile(credPath, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	originalCreds := os.Getenv("AWS_SHARED_CREDENTIALS_FILE")
	os.Setenv("AWS_SHARED_CREDENTIALS_FILE", credPath)
	defer os.Setenv("AWS_SHARED_CREDENTIALS_FILE", originalCreds)

	_, pType, err := inspectProfile("test-process")
	if err != nil {
		t.Fatalf("inspectProfile failed: %v", err)
	}

	if pType != "credential-process" {
		t.Errorf("Expected profile type 'credential-process', got '%s'", pType)
	}
}

func TestGetCredentialsForCredentialProcess(t *testing.T) {
	// Create temporary config file
	tmpDir, err := os.MkdirTemp("", "awsm-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config")
	// Use a mock process that returns JSON credentials
	configContent := `[profile test-process]
credential_process = echo '{"Version": 1, "AccessKeyId": "AKIA-MOCK", "SecretAccessKey": "secret-mock", "SessionToken": "token-mock", "Expiration": "2026-01-30T21:00:00Z"}'
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Set environment variable
	originalConfig := os.Getenv("AWS_CONFIG_FILE")
	os.Setenv("AWS_CONFIG_FILE", configPath)
	defer os.Setenv("AWS_CONFIG_FILE", originalConfig)

	// For credentials file, just use an empty one
	credPath := filepath.Join(tmpDir, "credentials")
	if err := os.WriteFile(credPath, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	originalCreds := os.Getenv("AWS_SHARED_CREDENTIALS_FILE")
	os.Setenv("AWS_SHARED_CREDENTIALS_FILE", credPath)
	defer os.Setenv("AWS_SHARED_CREDENTIALS_FILE", originalCreds)

	creds, isStatic, err := GetCredentialsForProfile("test-process")
	if err != nil {
		t.Fatalf("GetCredentialsForProfile failed: %v", err)
	}

	if isStatic {
		t.Error("Expected isStatic to be false")
	}

	if creds.AccessKeyId != "AKIA-MOCK" {
		t.Errorf("Expected AccessKeyId 'AKIA-MOCK', got '%s'", creds.AccessKeyId)
	}
}
