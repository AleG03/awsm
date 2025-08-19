package wrapper

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestNormalizeShellName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"zsh", "zsh"},
		{"zsh5", "zsh"},
		{"bash", "bash"},
		{"bash4", "bash"},
		{"bash5", "bash"},
		{"fish", "fish"},
		{"powershell", "powershell"},
		{"pwsh", "powershell"},
		{"powershell.exe", "powershell"},
		{"pwsh.exe", "powershell"},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizeShellName(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeShellName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestValidateShell(t *testing.T) {
	tests := []struct {
		shell     string
		wantError bool
	}{
		{"", true},
		{"zsh", false},
		{"bash", false},
		{"fish", false},
		{"powershell", false},
		{"invalid", true},
		{"sh", true},
		{"csh", true},
	}

	for _, tt := range tests {
		t.Run(tt.shell, func(t *testing.T) {
			err := ValidateShell(tt.shell)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidateShell(%q) error = %v, wantError %v", tt.shell, err, tt.wantError)
			}
		})
	}
}

func TestDetectShellFromEnvVars(t *testing.T) {
	tests := []struct {
		name     string
		envVars  map[string]string
		expected string
	}{
		{
			name:     "zsh version",
			envVars:  map[string]string{"ZSH_VERSION": "5.8"},
			expected: "zsh",
		},
		{
			name:     "zsh name",
			envVars:  map[string]string{"ZSH_NAME": "zsh"},
			expected: "zsh",
		},
		{
			name:     "bash version",
			envVars:  map[string]string{"BASH_VERSION": "5.1.8"},
			expected: "bash",
		},
		{
			name:     "bash path",
			envVars:  map[string]string{"BASH": "/bin/bash"},
			expected: "bash",
		},
		{
			name:     "fish version",
			envVars:  map[string]string{"FISH_VERSION": "3.3.1"},
			expected: "fish",
		},
		{
			name:     "fish greeting",
			envVars:  map[string]string{"fish_greeting": "Welcome to fish"},
			expected: "fish",
		},
		{
			name:     "powershell version",
			envVars:  map[string]string{"PSVersionTable": "something"},
			expected: "powershell",
		},
		{
			name:     "no shell vars",
			envVars:  map[string]string{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original environment
			originalEnv := make(map[string]string)
			for key := range tt.envVars {
				originalEnv[key] = os.Getenv(key)
			}

			// Set test environment
			for key, value := range tt.envVars {
				os.Setenv(key, value)
			}

			// Test detection
			result := detectShellFromEnvVars()
			if result != tt.expected {
				t.Errorf("detectShellFromEnvVars() = %q, want %q", result, tt.expected)
			}

			// Restore original environment
			for key, originalValue := range originalEnv {
				if originalValue == "" {
					os.Unsetenv(key)
				} else {
					os.Setenv(key, originalValue)
				}
			}
		})
	}
}

func TestDetectPowerShell(t *testing.T) {
	tests := []struct {
		name     string
		envVars  map[string]string
		expected bool
	}{
		{
			name:     "PSModulePath set",
			envVars:  map[string]string{"PSModulePath": "/some/path"},
			expected: true,
		},
		{
			name:     "PowerShell in PATH",
			envVars:  map[string]string{"PATH": "/usr/bin:/usr/local/bin:/opt/PowerShell"},
			expected: true,
		},
		{
			name:     "PSVersionTable set",
			envVars:  map[string]string{"PSVersionTable": "something"},
			expected: true,
		},
		{
			name:     "PSHOME set",
			envVars:  map[string]string{"PSHOME": "/opt/microsoft/powershell"},
			expected: true,
		},
		{
			name:     "no PowerShell indicators",
			envVars:  map[string]string{"PATH": "/usr/bin:/usr/local/bin"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original environment
			originalEnv := make(map[string]string)
			for key := range tt.envVars {
				originalEnv[key] = os.Getenv(key)
			}

			// Set test environment
			for key, value := range tt.envVars {
				os.Setenv(key, value)
			}

			// Test detection
			result := detectPowerShell()
			if result != tt.expected {
				t.Errorf("detectPowerShell() = %v, want %v", result, tt.expected)
			}

			// Restore original environment
			for key, originalValue := range originalEnv {
				if originalValue == "" {
					os.Unsetenv(key)
				} else {
					os.Setenv(key, originalValue)
				}
			}
		})
	}
}

func TestDetectCurrentShell(t *testing.T) {
	tests := []struct {
		name     string
		envVars  map[string]string
		expected string
		wantErr  bool
	}{
		{
			name:     "detect from SHELL env var",
			envVars:  map[string]string{"SHELL": "/bin/zsh"},
			expected: "zsh",
			wantErr:  false,
		},
		{
			name:     "detect from ZSH_VERSION",
			envVars:  map[string]string{"SHELL": "", "ZSH_VERSION": "5.8"},
			expected: "zsh",
			wantErr:  false,
		},
		{
			name:     "detect from BASH_VERSION",
			envVars:  map[string]string{"SHELL": "", "BASH_VERSION": "5.1"},
			expected: "bash",
			wantErr:  false,
		},
		{
			name:     "no detection possible",
			envVars:  map[string]string{"SHELL": ""},
			expected: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original environment
			originalEnv := make(map[string]string)
			shellVars := []string{"SHELL", "ZSH_VERSION", "BASH_VERSION", "FISH_VERSION", "PSModulePath"}
			for _, key := range shellVars {
				originalEnv[key] = os.Getenv(key)
			}

			// Clear all shell-related env vars first
			for _, key := range shellVars {
				os.Unsetenv(key)
			}

			// Set test environment
			for key, value := range tt.envVars {
				if value != "" {
					os.Setenv(key, value)
				}
			}

			// Test detection
			result, err := DetectCurrentShell()
			if (err != nil) != tt.wantErr {
				t.Errorf("DetectCurrentShell() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if result != tt.expected {
				t.Errorf("DetectCurrentShell() = %q, want %q", result, tt.expected)
			}

			// Restore original environment
			for key, originalValue := range originalEnv {
				if originalValue == "" {
					os.Unsetenv(key)
				} else {
					os.Setenv(key, originalValue)
				}
			}
		})
	}
}

func TestGetShellInstallPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("Failed to get home directory: %v", err)
	}

	tests := []struct {
		shell    SupportedShell
		expected string
	}{
		{ShellZsh, filepath.Join(home, ".zsh", "functions", "aws")},
		{ShellFish, filepath.Join(home, ".config", "fish", "functions", "aws.fish")},
	}

	for _, tt := range tests {
		t.Run(string(tt.shell), func(t *testing.T) {
			result, err := GetShellInstallPath(tt.shell)
			if err != nil {
				t.Errorf("GetShellInstallPath(%v) error = %v", tt.shell, err)
				return
			}
			if result != tt.expected {
				t.Errorf("GetShellInstallPath(%v) = %q, want %q", tt.shell, result, tt.expected)
			}
		})
	}

	// Test bash and powershell (should return config file path)
	bashPath, err := GetShellInstallPath(ShellBash)
	if err != nil {
		t.Errorf("GetShellInstallPath(bash) error = %v", err)
	}
	if bashPath == "" {
		t.Error("GetShellInstallPath(bash) returned empty path")
	}

	psPath, err := GetShellInstallPath(ShellPowerShell)
	if err != nil {
		t.Errorf("GetShellInstallPath(powershell) error = %v", err)
	}
	if psPath == "" {
		t.Error("GetShellInstallPath(powershell) returned empty path")
	}
}

func TestGetShellFunctionDirectory(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("Failed to get home directory: %v", err)
	}

	tests := []struct {
		shell    SupportedShell
		expected string
		wantErr  bool
	}{
		{ShellZsh, filepath.Join(home, ".zsh", "functions"), false},
		{ShellFish, filepath.Join(home, ".config", "fish", "functions"), false},
		{ShellBash, "", true},
		{ShellPowerShell, "", true},
	}

	for _, tt := range tests {
		t.Run(string(tt.shell), func(t *testing.T) {
			result, err := GetShellFunctionDirectory(tt.shell)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetShellFunctionDirectory(%v) error = %v, wantErr %v", tt.shell, err, tt.wantErr)
				return
			}
			if result != tt.expected {
				t.Errorf("GetShellFunctionDirectory(%v) = %q, want %q", tt.shell, result, tt.expected)
			}
		})
	}
}

func TestGetShellConfigPaths(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("Failed to get home directory: %v", err)
	}

	tests := []struct {
		shell         SupportedShell
		expectedPaths []string
	}{
		{
			ShellZsh,
			[]string{
				filepath.Join(home, ".zshrc"),
				filepath.Join(home, ".zsh", ".zshrc"),
			},
		},
		{
			ShellBash,
			[]string{
				filepath.Join(home, ".bashrc"),
				filepath.Join(home, ".bash_profile"),
				filepath.Join(home, ".profile"),
			},
		},
		{
			ShellFish,
			[]string{
				filepath.Join(home, ".config", "fish", "config.fish"),
			},
		},
	}

	// Add PowerShell test based on OS
	if runtime.GOOS == "windows" {
		tests = append(tests, struct {
			shell         SupportedShell
			expectedPaths []string
		}{
			ShellPowerShell,
			[]string{
				filepath.Join(home, "Documents", "PowerShell", "Microsoft.PowerShell_profile.ps1"),
				filepath.Join(home, "Documents", "WindowsPowerShell", "Microsoft.PowerShell_profile.ps1"),
			},
		})
	} else {
		tests = append(tests, struct {
			shell         SupportedShell
			expectedPaths []string
		}{
			ShellPowerShell,
			[]string{
				filepath.Join(home, ".config", "powershell", "Microsoft.PowerShell_profile.ps1"),
			},
		})
	}

	for _, tt := range tests {
		t.Run(string(tt.shell), func(t *testing.T) {
			result, err := GetShellConfigPaths(tt.shell)
			if err != nil {
				t.Errorf("GetShellConfigPaths(%v) error = %v", tt.shell, err)
				return
			}
			if len(result) != len(tt.expectedPaths) {
				t.Errorf("GetShellConfigPaths(%v) returned %d paths, want %d", tt.shell, len(result), len(tt.expectedPaths))
				return
			}
			for i, path := range result {
				if path != tt.expectedPaths[i] {
					t.Errorf("GetShellConfigPaths(%v)[%d] = %q, want %q", tt.shell, i, path, tt.expectedPaths[i])
				}
			}
		})
	}
}

func TestCheckShellConfigAccess(t *testing.T) {
	// Test case: check access for different shells
	t.Run("check access for supported shells", func(t *testing.T) {
		shells := []SupportedShell{ShellZsh, ShellBash, ShellFish, ShellPowerShell}

		for _, shell := range shells {
			err := CheckShellConfigAccess(shell)
			// We can't predict the exact error, but it should either succeed or fail gracefully
			if err != nil {
				t.Logf("CheckShellConfigAccess(%s) returned expected error: %v", shell, err)
			} else {
				t.Logf("CheckShellConfigAccess(%s) succeeded", shell)
			}
		}
	})
}
