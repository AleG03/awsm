package config

import (
	"testing"
)

func TestGetChromeProfileDirectory(t *testing.T) {
	tests := []struct {
		name     string
		alias    string
		expected string
	}{
		{"Empty alias", "", ""},
		{"Non-existent alias", "nonexistent", "nonexistent"},
		{"Direct directory name", "Profile 1", "Profile 1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetChromeProfileDirectory(tt.alias)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}
