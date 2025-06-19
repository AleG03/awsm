package cmd

import (
	"testing"
)

func TestSSOCommand(t *testing.T) {
	if ssoCmd.Use != "sso" {
		t.Errorf("Expected command use 'sso', got %s", ssoCmd.Use)
	}

	if ssoCmd.Short == "" {
		t.Error("Short description should not be empty")
	}

	// Test that ssoLoginCmd is properly configured
	if ssoLoginCmd.Use != "login <sso-session>" {
		t.Errorf("Expected login command use 'login <sso-session>', got %s", ssoLoginCmd.Use)
	}

	if ssoLoginCmd.Short == "" {
		t.Error("Login command short description should not be empty")
	}

	if ssoLoginCmd.Long == "" {
		t.Error("Login command long description should not be empty")
	}

	if ssoLoginCmd.RunE == nil {
		t.Error("Login command should have a RunE function")
	}
}
