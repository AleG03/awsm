package cmd

import (
	"strings"
	"testing"
)

func TestConsoleCommand(t *testing.T) {
	if consoleCmd.Use != "console" {
		t.Errorf("Expected command use 'console', got %s", consoleCmd.Use)
	}

	if consoleCmd.Short == "" {
		t.Error("Short description should not be empty")
	}

	if consoleCmd.Long == "" {
		t.Error("Long description should not be empty")
	}

	if len(consoleCmd.Aliases) == 0 {
		t.Error("Command should have aliases")
	}

	if consoleCmd.RunE == nil {
		t.Error("Command should have a RunE function")
	}

	// Test flag initialization
	foundNoOpen := false
	foundFirefox := false
	foundChrome := false

	flagUsages := consoleCmd.Flags().FlagUsages()
	if flagUsages != "" {
		if strings.Contains(flagUsages, "no-open") {
			foundNoOpen = true
		}
		if strings.Contains(flagUsages, "firefox-container") {
			foundFirefox = true
		}
		if strings.Contains(flagUsages, "chrome-profile") {
			foundChrome = true
		}
	}

	if !foundNoOpen {
		t.Error("Command should have a no-open flag")
	}
	if !foundFirefox {
		t.Error("Command should have a firefox-container flag")
	}
	if !foundChrome {
		t.Error("Command should have a chrome-profile flag")
	}
}
