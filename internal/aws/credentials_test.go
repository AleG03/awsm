package aws

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetAWSCredentialsPath(t *testing.T) {
	// Test with environment variable
	testPath := "/custom/aws/credentials"
	os.Setenv("AWS_SHARED_CREDENTIALS_FILE", testPath)
	defer os.Unsetenv("AWS_SHARED_CREDENTIALS_FILE")

	path, err := GetAWSCredentialsPath()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if path != testPath {
		t.Errorf("Expected %s, got %s", testPath, path)
	}

	// Test without environment variable
	os.Unsetenv("AWS_SHARED_CREDENTIALS_FILE")
	path, err = GetAWSCredentialsPath()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".aws", "credentials")
	if path != expected {
		t.Errorf("Expected %s, got %s", expected, path)
	}
}

func TestTempCredentials(t *testing.T) {
	creds := &TempCredentials{
		AccessKeyId:     "AKIA123",
		SecretAccessKey: "secret123",
		SessionToken:    "token123",
	}

	if creds.AccessKeyId != "AKIA123" {
		t.Errorf("Expected AKIA123, got %s", creds.AccessKeyId)
	}
	if creds.SecretAccessKey != "secret123" {
		t.Errorf("Expected secret123, got %s", creds.SecretAccessKey)
	}
	if creds.SessionToken != "token123" {
		t.Errorf("Expected token123, got %s", creds.SessionToken)
	}
}

func TestProfileConfig(t *testing.T) {
	config := &profileConfig{
		MfaSerial:     "arn:aws:iam::123456789012:mfa/user",
		RoleArn:       "arn:aws:iam::123456789012:role/test",
		SourceProfile: "default",
	}

	if config.MfaSerial != "arn:aws:iam::123456789012:mfa/user" {
		t.Errorf("Unexpected MfaSerial: %s", config.MfaSerial)
	}
	if config.RoleArn != "arn:aws:iam::123456789012:role/test" {
		t.Errorf("Unexpected RoleArn: %s", config.RoleArn)
	}
	if config.SourceProfile != "default" {
		t.Errorf("Unexpected SourceProfile: %s", config.SourceProfile)
	}
}
