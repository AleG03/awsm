package wrapper

import (
	"testing"
)

func TestGetSupportedShells(t *testing.T) {
	shells := GetSupportedShells()

	expectedShells := []SupportedShell{ShellZsh, ShellBash, ShellFish, ShellPowerShell}

	if len(shells) != len(expectedShells) {
		t.Errorf("Expected %d supported shells, got %d", len(expectedShells), len(shells))
	}

	for i, expected := range expectedShells {
		if shells[i] != expected {
			t.Errorf("Expected shell %s at index %d, got %s", expected, i, shells[i])
		}
	}
}

func TestIsValidShell(t *testing.T) {
	tests := []struct {
		shell    string
		expected bool
	}{
		{"zsh", true},
		{"bash", true},
		{"fish", true},
		{"powershell", true},
		{"invalid", false},
		{"", false},
		{"ZSH", false}, // case sensitive
	}

	for _, test := range tests {
		result := IsValidShell(test.shell)
		if result != test.expected {
			t.Errorf("IsValidShell(%s) = %v, expected %v", test.shell, result, test.expected)
		}
	}
}

func TestWrapperError(t *testing.T) {
	err := NewWrapperError("zsh", "install", ErrAlreadyInstalled, "custom message")

	if err.Shell != "zsh" {
		t.Errorf("Expected shell 'zsh', got '%s'", err.Shell)
	}

	if err.Op != "install" {
		t.Errorf("Expected operation 'install', got '%s'", err.Op)
	}

	if err.Err != ErrAlreadyInstalled {
		t.Errorf("Expected error ErrAlreadyInstalled, got %v", err.Err)
	}

	if err.Message != "custom message" {
		t.Errorf("Expected message 'custom message', got '%s'", err.Message)
	}

	expectedErrorString := "wrapper install for zsh: custom message (wrapper already installed)"
	if err.Error() != expectedErrorString {
		t.Errorf("Expected error string '%s', got '%s'", expectedErrorString, err.Error())
	}
}

func TestInstallationStatus(t *testing.T) {
	status := InstallationStatus{
		Shell:     "zsh",
		Installed: true,
		Path:      "/home/user/.zsh/functions/aws",
		Error:     "",
	}

	if status.Shell != "zsh" {
		t.Errorf("Expected shell 'zsh', got '%s'", status.Shell)
	}

	if !status.Installed {
		t.Errorf("Expected installed to be true")
	}

	if status.Path != "/home/user/.zsh/functions/aws" {
		t.Errorf("Expected path '/home/user/.zsh/functions/aws', got '%s'", status.Path)
	}
}

func TestWrapperConfig(t *testing.T) {
	config := WrapperConfig{
		Shell:           "bash",
		FunctionContent: "function aws() { echo 'wrapper'; }",
		InstallPath:     "/home/user/.bashrc",
		ConfigFile:      "/home/user/.bashrc",
		BackupOriginal:  true,
	}

	if config.Shell != "bash" {
		t.Errorf("Expected shell 'bash', got '%s'", config.Shell)
	}

	if !config.BackupOriginal {
		t.Errorf("Expected BackupOriginal to be true")
	}
}
