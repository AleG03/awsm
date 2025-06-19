package cmd

import (
	"testing"
)

func TestSetVersionInfo(t *testing.T) {
	testVersion := "1.0.0"
	testCommit := "abc123"
	testDate := "2024-01-01"

	SetVersionInfo(testVersion, testCommit, testDate)

	if version != testVersion {
		t.Errorf("Expected version %s, got %s", testVersion, version)
	}
	if commit != testCommit {
		t.Errorf("Expected commit %s, got %s", testCommit, commit)
	}
	if date != testDate {
		t.Errorf("Expected date %s, got %s", testDate, date)
	}
}

func TestRootCommand(t *testing.T) {
	if rootCmd.Use != "awsm" {
		t.Errorf("Expected command name 'awsm', got %s", rootCmd.Use)
	}

	if rootCmd.Short == "" {
		t.Error("Short description should not be empty")
	}
}
