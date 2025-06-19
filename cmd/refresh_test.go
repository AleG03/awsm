package cmd

import (
	"testing"
)

func TestRefreshCommand(t *testing.T) {
	if refreshCmd.Use != "refresh [profile]" {
		t.Errorf("Expected command use 'refresh [profile]', got %s", refreshCmd.Use)
	}

	if refreshCmd.Short == "" {
		t.Error("Short description should not be empty")
	}

	if refreshCmd.Long == "" {
		t.Error("Long description should not be empty")
	}

	if refreshCmd.RunE == nil {
		t.Error("Command should have a RunE function")
	}
}
