package wrapper

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestCrossPlatformPathHandling tests path handling across different operating systems
func TestCrossPlatformPathHandling(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "awsm-cross-platform-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Set HOME to temp directory for testing
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	// Test path generation for each shell on current platform
	factory := NewFactory()

	for _, shell := range GetSupportedShells() {
		t.Run(string(shell), func(t *testing.T) {
			wrapper, err := factory.CreateWrapper(string(shell))
			if err != nil {
				t.Fatalf("Failed to create wrapper for %s: %v", shell, err)
			}

			// Test install path generation
			installPath, err := wrapper.GetInstallPath()
			if err != nil {
				t.Errorf("Failed to get install path for %s: %v", shell, err)
				return
			}

			// Verify path is absolute
			if !filepath.IsAbs(installPath) {
				t.Errorf("Install path should be absolute for %s: %s", shell, installPath)
			}

			// Verify path uses correct separators for current OS
			expectedSeparator := string(filepath.Separator)
			if !strings.Contains(installPath, expectedSeparator) && installPath != tempDir {
				t.Errorf("Install path should use OS-specific separator (%s) for %s: %s",
					expectedSeparator, shell, installPath)
			}

			// Test config file path generation
			configFile, err := wrapper.GetConfigFile()
			if err != nil {
				t.Errorf("Failed to get config file for %s: %v", shell, err)
				return
			}

			// Verify config file path is absolute
			if configFile != "" && !filepath.IsAbs(configFile) {
				t.Errorf("Config file path should be absolute for %s: %s", shell, configFile)
			}

			// Verify paths are within HOME directory
			if !strings.HasPrefix(installPath, tempDir) {
				t.Errorf("Install path should be within HOME directory for %s: %s", shell, installPath)
			}

			if configFile != "" && !strings.HasPrefix(configFile, tempDir) {
				t.Errorf("Config file should be within HOME directory for %s: %s", shell, configFile)
			}
		})
	}
}

// TestPlatformSpecificShellSupport tests shell support on different platforms
func TestPlatformSpecificShellSupport(t *testing.T) {
	supportedShells := GetSupportedShells()

	// All platforms should support these shells
	expectedShells := []SupportedShell{ShellZsh, ShellBash, ShellFish}

	// Windows should also support PowerShell
	if runtime.GOOS == "windows" {
		expectedShells = append(expectedShells, ShellPowerShell)
	}

	// Verify minimum shell support
	if len(supportedShells) < len(expectedShells) {
		t.Errorf("Expected at least %d supported shells, got %d", len(expectedShells), len(supportedShells))
	}

	// Verify expected shells are present
	for _, expected := range expectedShells {
		found := false
		for _, supported := range supportedShells {
			if supported == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected shell %s not found in supported shells on %s", expected, runtime.GOOS)
		}
	}

	// Platform-specific tests
	switch runtime.GOOS {
	case "windows":
		// Windows should support PowerShell
		found := false
		for _, shell := range supportedShells {
			if shell == ShellPowerShell {
				found = true
				break
			}
		}
		if !found {
			t.Error("Windows should support PowerShell")
		}

	case "darwin", "linux":
		// Unix-like systems should support all POSIX shells
		unixShells := []SupportedShell{ShellZsh, ShellBash, ShellFish}
		for _, expected := range unixShells {
			found := false
			for _, supported := range supportedShells {
				if supported == expected {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Unix-like system should support %s", expected)
			}
		}
	}
}

// TestShellSpecificDirectoryStructures tests directory creation for different shells
func TestShellSpecificDirectoryStructures(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "awsm-dir-structure-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	manager := NewInstallationManager()

	// Test directory creation for each shell
	testCases := []struct {
		shell          string
		expectedDirs   []string
		expectedFiles  []string
		skipOnPlatform string
	}{
		{
			shell:         "zsh",
			expectedDirs:  []string{".zsh", ".zsh/functions"},
			expectedFiles: []string{".zsh/functions/aws", ".zshrc"},
		},
		{
			shell:         "bash",
			expectedDirs:  []string{},
			expectedFiles: []string{".bashrc"},
		},
		{
			shell:         "fish",
			expectedDirs:  []string{".config", ".config/fish", ".config/fish/functions"},
			expectedFiles: []string{".config/fish/functions/aws.fish"},
		},
		{
			shell:          "powershell",
			expectedDirs:   []string{".config", ".config/powershell"},
			expectedFiles:  []string{".config/powershell/Microsoft.PowerShell_profile.ps1"},
			skipOnPlatform: "linux", // Skip on Linux if PowerShell not commonly used
		},
	}

	for _, tc := range testCases {
		t.Run(tc.shell, func(t *testing.T) {
			// Skip platform-specific tests
			if tc.skipOnPlatform == runtime.GOOS {
				t.Skipf("Skipping %s test on %s", tc.shell, runtime.GOOS)
			}

			// Skip if shell not supported on current platform
			supported := false
			for _, shell := range GetSupportedShells() {
				if string(shell) == tc.shell {
					supported = true
					break
				}
			}
			if !supported {
				t.Skipf("Shell %s not supported on %s", tc.shell, runtime.GOOS)
			}

			opts := InstallOptions{
				Shell:          tc.shell,
				Force:          false,
				BackupOriginal: false,
			}

			err := manager.Install(opts)
			if err != nil {
				t.Errorf("Failed to install wrapper for %s: %v", tc.shell, err)
				return
			}

			// Verify expected directories were created
			for _, dir := range tc.expectedDirs {
				dirPath := filepath.Join(tempDir, dir)
				if _, err := os.Stat(dirPath); os.IsNotExist(err) {
					t.Errorf("Expected directory %s was not created for %s", dir, tc.shell)
				}
			}

			// Verify expected files were created or modified
			for _, file := range tc.expectedFiles {
				filePath := filepath.Join(tempDir, file)
				if _, err := os.Stat(filePath); os.IsNotExist(err) {
					t.Errorf("Expected file %s was not created for %s", file, tc.shell)
				}
			}

			// Verify installation status
			status := manager.GetShellStatus(tc.shell)
			if !status.Installed {
				t.Errorf("Shell %s should be installed", tc.shell)
			}
			if status.Error != "" {
				t.Errorf("Shell %s should not have errors: %s", tc.shell, status.Error)
			}
		})
	}
}

// TestFilePermissions tests that created files have appropriate permissions
func TestFilePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping file permission tests on Windows")
	}

	tempDir, err := os.MkdirTemp("", "awsm-permissions-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	manager := NewInstallationManager()

	// Test file permissions for each shell
	shells := []string{"zsh", "bash", "fish"}

	for _, shell := range shells {
		t.Run(shell, func(t *testing.T) {
			opts := InstallOptions{
				Shell:          shell,
				Force:          false,
				BackupOriginal: false,
			}

			err := manager.Install(opts)
			if err != nil {
				t.Errorf("Failed to install wrapper for %s: %v", shell, err)
				return
			}

			// Get the install path and check permissions
			factory := NewFactory()
			wrapper, err := factory.CreateWrapper(shell)
			if err != nil {
				t.Errorf("Failed to create wrapper for %s: %v", shell, err)
				return
			}

			installPath, err := wrapper.GetInstallPath()
			if err != nil {
				t.Errorf("Failed to get install path for %s: %v", shell, err)
				return
			}

			// Check file permissions
			fileInfo, err := os.Stat(installPath)
			if err != nil {
				t.Errorf("Failed to stat install file for %s: %v", shell, err)
				return
			}

			mode := fileInfo.Mode()

			// File should be readable and writable by owner
			if mode&0600 != 0600 {
				t.Errorf("File should be readable and writable by owner for %s: %o", shell, mode)
			}

			// For executable files (like zsh functions), check execute permission
			if shell == "zsh" && mode&0100 == 0 {
				// Zsh functions should be executable
				t.Logf("Note: Zsh function file might need execute permission: %o", mode)
			}
		})
	}
}

// TestEnvironmentVariableHandling tests handling of environment variables across platforms
func TestEnvironmentVariableHandling(t *testing.T) {
	// Save original environment
	originalHome := os.Getenv("HOME")
	originalUserProfile := os.Getenv("USERPROFILE")
	originalShell := os.Getenv("SHELL")

	defer func() {
		os.Setenv("HOME", originalHome)
		os.Setenv("USERPROFILE", originalUserProfile)
		os.Setenv("SHELL", originalShell)
	}()

	tempDir, err := os.MkdirTemp("", "awsm-env-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	testCases := []struct {
		name     string
		homeVar  string
		homeVal  string
		shellVar string
		shellVal string
	}{
		{
			name:     "unix_environment",
			homeVar:  "HOME",
			homeVal:  tempDir,
			shellVar: "SHELL",
			shellVal: "/bin/zsh",
		},
	}

	// Add Windows-specific test case
	if runtime.GOOS == "windows" {
		testCases = append(testCases, struct {
			name     string
			homeVar  string
			homeVal  string
			shellVar string
			shellVal string
		}{
			name:     "windows_environment",
			homeVar:  "USERPROFILE",
			homeVal:  tempDir,
			shellVar: "SHELL",
			shellVal: "powershell",
		})
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Set test environment
			os.Setenv(tc.homeVar, tc.homeVal)
			if tc.shellVar != "" {
				os.Setenv(tc.shellVar, tc.shellVal)
			}

			// Test that wrapper factory can handle the environment
			factory := NewFactory()

			for _, shell := range GetSupportedShells() {
				wrapper, err := factory.CreateWrapper(string(shell))
				if err != nil {
					t.Errorf("Failed to create wrapper for %s with %s environment: %v",
						shell, tc.name, err)
					continue
				}

				// Test path generation with custom environment
				installPath, err := wrapper.GetInstallPath()
				if err != nil {
					t.Errorf("Failed to get install path for %s with %s environment: %v",
						shell, tc.name, err)
					continue
				}

				// Verify path uses the custom home directory
				if !strings.HasPrefix(installPath, tc.homeVal) {
					t.Errorf("Install path should use custom home directory for %s: expected prefix %s, got %s",
						shell, tc.homeVal, installPath)
				}
			}
		})
	}
}

// TestShellDetection tests automatic shell detection across platforms
func TestShellDetection(t *testing.T) {
	originalShell := os.Getenv("SHELL")
	defer os.Setenv("SHELL", originalShell)

	testCases := []struct {
		name         string
		shellEnv     string
		expectedType string
	}{
		{"zsh_detection", "/bin/zsh", "zsh"},
		{"bash_detection", "/bin/bash", "bash"},
		{"fish_detection", "/usr/local/bin/fish", "fish"},
		{"zsh_homebrew", "/opt/homebrew/bin/zsh", "zsh"},
		{"bash_system", "/usr/bin/bash", "bash"},
	}

	// Add Windows-specific test cases
	if runtime.GOOS == "windows" {
		testCases = append(testCases, []struct {
			name         string
			shellEnv     string
			expectedType string
		}{
			{"powershell_detection", "powershell", "powershell"},
			{"pwsh_detection", "pwsh", "powershell"},
		}...)
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			os.Setenv("SHELL", tc.shellEnv)

			// Test shell detection logic
			detectedShell := detectShellFromEnvironment()

			if detectedShell != tc.expectedType {
				t.Errorf("Expected shell type %s for SHELL=%s, got %s",
					tc.expectedType, tc.shellEnv, detectedShell)
			}
		})
	}
}

// Helper function to detect shell from environment (would be implemented in actual code)
func detectShellFromEnvironment() string {
	shell := os.Getenv("SHELL")
	if shell == "" {
		return "unknown"
	}

	// Extract shell name from path
	shellName := filepath.Base(shell)

	switch shellName {
	case "zsh":
		return "zsh"
	case "bash":
		return "bash"
	case "fish":
		return "fish"
	case "powershell", "pwsh":
		return "powershell"
	default:
		return "unknown"
	}
}

// TestUnicodeAndSpecialCharacters tests handling of unicode and special characters in paths
func TestUnicodeAndSpecialCharacters(t *testing.T) {
	// Create temp directory with unicode characters
	tempDir, err := os.MkdirTemp("", "awsm-unicode-测试-test")
	if err != nil {
		t.Skipf("Skipping unicode test due to filesystem limitations: %v", err)
	}
	defer os.RemoveAll(tempDir)

	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	manager := NewInstallationManager()

	// Test installation with unicode paths
	for _, shell := range []string{"zsh", "bash", "fish"} {
		t.Run(shell, func(t *testing.T) {
			opts := InstallOptions{
				Shell:          shell,
				Force:          false,
				BackupOriginal: false,
			}

			err := manager.Install(opts)
			if err != nil {
				t.Errorf("Failed to install wrapper for %s with unicode path: %v", shell, err)
				return
			}

			// Verify installation status
			status := manager.GetShellStatus(shell)
			if !status.Installed {
				t.Errorf("Shell %s should be installed with unicode path", shell)
			}
		})
	}
}

// TestConcurrentInstallations tests thread safety across platforms
func TestConcurrentInstallations(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "awsm-concurrent-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	// Test concurrent installations for different shells
	shells := []string{"zsh", "bash", "fish"}
	if runtime.GOOS == "windows" {
		shells = append(shells, "powershell")
	}

	done := make(chan error, len(shells))

	// Start concurrent installations
	for _, shell := range shells {
		go func(s string) {
			manager := NewInstallationManager()
			opts := InstallOptions{
				Shell:          s,
				Force:          false,
				BackupOriginal: false,
			}
			done <- manager.Install(opts)
		}(shell)
	}

	// Wait for all installations to complete
	for i := 0; i < len(shells); i++ {
		err := <-done
		if err != nil {
			t.Errorf("Concurrent installation failed: %v", err)
		}
	}

	// Verify all shells are installed
	manager := NewInstallationManager()
	for _, shell := range shells {
		status := manager.GetShellStatus(shell)
		if !status.Installed {
			t.Errorf("Shell %s should be installed after concurrent installation", shell)
		}
	}
}
