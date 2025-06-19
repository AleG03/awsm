package aws

import (
	"testing"
)

func TestCredentialStatus(t *testing.T) {
	tests := []struct {
		name     string
		status   CredentialStatus
		expected string
	}{
		{"Valid", CredentialValid, "valid"},
		{"Expired", CredentialExpired, "expired"},
		{"Expiring Soon", CredentialExpiringSoon, "expiring soon"},
		{"Missing", CredentialMissing, "missing"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that the constants are defined correctly
			if tt.status < 0 || tt.status > 3 {
				t.Errorf("Invalid credential status value: %d", tt.status)
			}
		})
	}
}

func TestGetProfileTypeByName(t *testing.T) {
	// This would require mocking the config file
	// For now, just test that the function exists
	_, err := getProfileTypeByName("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent profile")
	}
}
