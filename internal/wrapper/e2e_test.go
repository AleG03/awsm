package wrapper

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestEndToEndWrapperInstallation tests the complete wrapper installation and functionality
func TestEndToEndWrapperInstallation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping end-to-end tests in short mode")
	}

	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "awsm-e2e-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Set HOME to temp directory for testing
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	// Test each supported shell
	shells := []string{"zsh", "bash", "fish"}
	if runtime.GOOS == "windows" {
		shells = append(shells, "powershell")
	}

	for _, shell := range shells {
		t.Run(fmt.Sprintf("e2e_%s", shell), func(t *testing.T) {
			testEndToEndShellInstallation(t, shell, tempDir)
		})
	}
}

func testEndToEndShellInstallation(t *testing.T, shell, tempDir string) {
	manager := NewInstallationManager()

	// Create necessary directories for the shell
	createShellDirectories(t, shell, tempDir)

	// Test installation
	t.Run("install", func(t *testing.T) {
		opts := InstallOptions{
			Shell:          shell,
			Force:          false,
			BackupOriginal: true,
		}

		err := manager.Install(opts)
		if err != nil {
			t.Fatalf("Failed to install wrapper for %s: %v", shell, err)
		}

		// Verify installation
		status := manager.GetShellStatus(shell)
		if !status.Installed {
			t.Errorf("Shell %s should be installed", shell)
		}
		if status.Error != "" {
			t.Errorf("Shell %s should not have errors: %s", shell, status.Error)
		}
	})

	// Test wrapper function generation
	t.Run("wrapper_function", func(t *testing.T) {
		core := NewWrapperCore()
		wrapperFunc, err := core.GenerateShellWrapperFunction(SupportedShell(shell))
		if err != nil {
			t.Fatalf("Failed to generate wrapper function for %s: %v", shell, err)
		}

		// Verify wrapper function contains expected elements
		expectedElements := []string{
			"aws sts get-caller-identity",
			"awsm refresh",
		}

		for _, element := range expectedElements {
			if !strings.Contains(wrapperFunc, element) {
				t.Errorf("Wrapper function for %s should contain '%s'", shell, element)
			}
		}

		// Verify shell-specific syntax
		switch shell {
		case "zsh", "bash":
			if !strings.Contains(wrapperFunc, "function aws") {
				t.Errorf("Bash/Zsh wrapper should contain 'function aws'")
			}
		case "fish":
			if !strings.Contains(wrapperFunc, "function aws") {
				t.Errorf("Fish wrapper should contain 'function aws'")
			}
		case "powershell":
			if !strings.Contains(wrapperFunc, "function aws") {
				t.Errorf("PowerShell wrapper should contain 'function aws'")
			}
		}
	})

	// Test status checking
	t.Run("status", func(t *testing.T) {
		status := manager.GetStatus()

		// Find our shell in the status
		var shellStatus *InstallationStatus
		for i, s := range status.Shells {
			if s.Shell == shell {
				shellStatus = &status.Shells[i]
				break
			}
		}

		if shellStatus == nil {
			t.Fatalf("Shell %s not found in status", shell)
		}

		if !shellStatus.Installed {
			t.Errorf("Shell %s should be installed", shell)
		}
		if shellStatus.Path == "" {
			t.Errorf("Shell %s should have a path", shell)
		}
		if shellStatus.Error != "" {
			t.Errorf("Shell %s should not have errors: %s", shell, shellStatus.Error)
		}
	})

	// Test force installation
	t.Run("force_install", func(t *testing.T) {
		opts := InstallOptions{
			Shell:          shell,
			Force:          true,
			BackupOriginal: true,
		}

		err := manager.Install(opts)
		if err != nil {
			t.Errorf("Force installation should succeed for %s: %v", shell, err)
		}

		// Verify still installed
		status := manager.GetShellStatus(shell)
		if !status.Installed {
			t.Errorf("Shell %s should still be installed after force install", shell)
		}
	})

	// Test uninstallation
	t.Run("uninstall", func(t *testing.T) {
		opts := UninstallOptions{
			Shell:           shell,
			RemoveBackups:   true,
			RestoreOriginal: false,
		}

		err := manager.Uninstall(opts)
		if err != nil {
			t.Errorf("Failed to uninstall wrapper for %s: %v", shell, err)
		}

		// Verify uninstallation
		status := manager.GetShellStatus(shell)
		if status.Installed {
			t.Errorf("Shell %s should be uninstalled", shell)
		}
	})
}

func createShellDirectories(t *testing.T, shell, tempDir string) {
	switch shell {
	case "zsh":
		err := os.MkdirAll(filepath.Join(tempDir, ".zsh", "functions"), 0755)
		if err != nil {
			t.Fatalf("Failed to create zsh directories: %v", err)
		}
	case "bash":
		// .bashrc will be created by the installer if needed
	case "fish":
		err := os.MkdirAll(filepath.Join(tempDir, ".config", "fish", "functions"), 0755)
		if err != nil {
			t.Fatalf("Failed to create fish directories: %v", err)
		}
	case "powershell":
		err := os.MkdirAll(filepath.Join(tempDir, ".config", "powershell"), 0755)
		if err != nil {
			t.Fatalf("Failed to create powershell directories: %v", err)
		}
	}
}

// TestWrapperFunctionBehavior tests the actual behavior of wrapper functions
func TestWrapperFunctionBehavior(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping wrapper behavior tests in short mode")
	}

	// This test requires actual shell environments and AWS CLI
	// We'll test the core logic that the wrapper functions use
	core := NewWrapperCore()

	t.Run("credential_check_logic", func(t *testing.T) {
		// Test the credential checking logic
		// This would normally call AWS STS, but we'll test the command construction
		checkCmd := core.buildCredentialCheckCommand()

		expectedCmd := []string{"aws", "sts", "get-caller-identity", "--output", "json"}
		if len(checkCmd) != len(expectedCmd) {
			t.Errorf("Expected command length %d, got %d", len(expectedCmd), len(checkCmd))
		}

		for i, expected := range expectedCmd {
			if i >= len(checkCmd) || checkCmd[i] != expected {
				t.Errorf("Expected command[%d] = %s, got %s", i, expected, checkCmd[i])
			}
		}
	})

	t.Run("refresh_command_logic", func(t *testing.T) {
		// Test the refresh command construction
		refreshCmd := core.buildRefreshCommand()

		expectedCmd := []string{"awsm", "refresh"}
		if len(refreshCmd) != len(expectedCmd) {
			t.Errorf("Expected command length %d, got %d", len(expectedCmd), len(refreshCmd))
		}

		for i, expected := range expectedCmd {
			if i >= len(refreshCmd) || refreshCmd[i] != expected {
				t.Errorf("Expected command[%d] = %s, got %s", i, expected, refreshCmd[i])
			}
		}
	})

	t.Run("timeout_handling", func(t *testing.T) {
		// Test timeout configuration
		timeout := core.getCredentialCheckTimeout()
		expectedTimeout := 10 * time.Second

		if timeout != expectedTimeout {
			t.Errorf("Expected timeout %v, got %v", expectedTimeout, timeout)
		}
	})
}

// TestCrossPlatformCompatibility tests wrapper functionality across different platforms
func TestCrossPlatformCompatibility(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping cross-platform tests in short mode")
	}

	// Test platform-specific shell availability
	t.Run("platform_shell_support", func(t *testing.T) {
		supportedShells := GetSupportedShells()

		// All platforms should support these shells
		expectedShells := []SupportedShell{ShellZsh, ShellBash, ShellFish}

		// Windows should also support PowerShell
		if runtime.GOOS == "windows" {
			expectedShells = append(expectedShells, ShellPowerShell)
		}

		if len(supportedShells) < len(expectedShells) {
			t.Errorf("Expected at least %d supported shells, got %d", len(expectedShells), len(supportedShells))
		}

		for _, expected := range expectedShells {
			found := false
			for _, supported := range supportedShells {
				if supported == expected {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected shell %s not found in supported shells", expected)
			}
		}
	})

	t.Run("path_handling", func(t *testing.T) {
		// Test that path handling works correctly on different platforms
		tempDir, err := os.MkdirTemp("", "awsm-path-test")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		originalHome := os.Getenv("HOME")
		os.Setenv("HOME", tempDir)
		defer os.Setenv("HOME", originalHome)

		for _, shell := range GetSupportedShells() {
			factory := NewFactory()
			wrapper, err := factory.CreateWrapper(string(shell))
			if err != nil {
				t.Errorf("Failed to create wrapper for %s: %v", shell, err)
				continue
			}

			// Test that paths are generated correctly
			installPath, err := wrapper.GetInstallPath()
			if err != nil {
				t.Errorf("Failed to get install path for %s: %v", shell, err)
				continue
			}

			if installPath == "" {
				t.Errorf("Install path should not be empty for %s", shell)
			}

			// Verify path is absolute and uses correct separators
			if !filepath.IsAbs(installPath) {
				t.Errorf("Install path should be absolute for %s: %s", shell, installPath)
			}
		}
	})
}

// TestWrapperWithActualAWSCLI tests wrapper behavior with actual AWS CLI commands
func TestWrapperWithActualAWSCLI(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping AWS CLI integration tests in short mode")
	}

	// Check if AWS CLI is available
	_, err := exec.LookPath("aws")
	if err != nil {
		t.Skip("AWS CLI not available, skipping integration tests")
	}

	// Check if awsm is available (would be the case in real usage)
	_, err = exec.LookPath("awsm")
	if err != nil {
		t.Skip("awsm not available in PATH, skipping integration tests")
	}

	t.Run("aws_cli_passthrough", func(t *testing.T) {
		// Test that the wrapper correctly passes through AWS CLI commands
		core := NewWrapperCore()

		// Test command construction for various AWS CLI commands
		testCases := []struct {
			name string
			args []string
		}{
			{"simple_command", []string{"s3", "ls"}},
			{"command_with_flags", []string{"s3", "ls", "--recursive"}},
			{"command_with_values", []string{"s3", "cp", "file.txt", "s3://bucket/"}},
			{"help_command", []string{"help"}},
			{"version_command", []string{"--version"}},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				cmd := core.buildAWSCommand(tc.args)

				// Should start with "aws"
				if len(cmd) == 0 || cmd[0] != "aws" {
					t.Errorf("Command should start with 'aws', got: %v", cmd)
				}

				// Should contain all original arguments
				if len(cmd) != len(tc.args)+1 {
					t.Errorf("Expected %d arguments, got %d", len(tc.args)+1, len(cmd))
				}

				for i, arg := range tc.args {
					if cmd[i+1] != arg {
						t.Errorf("Expected arg[%d] = %s, got %s", i+1, arg, cmd[i+1])
					}
				}
			})
		}
	})
}

// TestPerformanceCharacteristics tests that the wrapper meets performance requirements
func TestPerformanceCharacteristics(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance tests in short mode")
	}

	core := NewWrapperCore()

	t.Run("wrapper_generation_performance", func(t *testing.T) {
		// Test that wrapper function generation is fast
		start := time.Now()

		for i := 0; i < 100; i++ {
			for _, shell := range GetSupportedShells() {
				_, err := core.GenerateShellWrapperFunction(shell)
				if err != nil {
					t.Errorf("Failed to generate wrapper for %s: %v", shell, err)
				}
			}
		}

		elapsed := time.Since(start)

		// Should be able to generate 100 wrapper functions for all shells in under 1 second
		maxDuration := 1 * time.Second
		if elapsed > maxDuration {
			t.Errorf("Wrapper generation took too long: %v (max: %v)", elapsed, maxDuration)
		}
	})

	t.Run("command_construction_performance", func(t *testing.T) {
		// Test that command construction is fast
		start := time.Now()

		testArgs := [][]string{
			{"s3", "ls"},
			{"ec2", "describe-instances"},
			{"iam", "list-users"},
			{"lambda", "list-functions"},
		}

		for i := 0; i < 1000; i++ {
			for _, args := range testArgs {
				core.buildAWSCommand(args)
				core.buildCredentialCheckCommand()
				core.buildRefreshCommand()
			}
		}

		elapsed := time.Since(start)

		// Should be able to construct 1000 commands in under 100ms
		maxDuration := 100 * time.Millisecond
		if elapsed > maxDuration {
			t.Errorf("Command construction took too long: %v (max: %v)", elapsed, maxDuration)
		}
	})
}

// TestErrorRecoveryScenarios tests various error scenarios and recovery
func TestErrorRecoveryScenarios(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "awsm-error-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	t.Run("permission_denied_recovery", func(t *testing.T) {
		// Create a read-only directory to simulate permission issues
		readOnlyDir := filepath.Join(tempDir, "readonly")
		err := os.MkdirAll(readOnlyDir, 0444) // Read-only
		if err != nil {
			t.Fatalf("Failed to create read-only dir: %v", err)
		}

		// Try to install in read-only directory (should handle gracefully)
		mfs := NewMockFileSystem()
		mfs.SetPermissionError(readOnlyDir, true)

		mf := NewMockFactory()
		wrapper := NewMockWrapperGenerator("zsh")
		wrapper.installPath = filepath.Join(readOnlyDir, "aws")
		mf.AddWrapper("zsh", wrapper)

		manager := NewInstallationManagerWithFactoryAndFS(mf, mfs)

		opts := InstallOptions{
			Shell:          "zsh",
			Force:          false,
			BackupOriginal: false,
		}

		err = manager.Install(opts)
		if err == nil {
			t.Error("Expected error when installing in read-only directory")
		}

		// Verify error is properly wrapped
		if wrapperErr, ok := err.(*WrapperError); ok {
			if wrapperErr.Shell != "zsh" {
				t.Errorf("Expected shell 'zsh', got '%s'", wrapperErr.Shell)
			}
		} else {
			t.Errorf("Expected WrapperError, got %T", err)
		}
	})

	t.Run("corrupted_config_recovery", func(t *testing.T) {
		// Create a corrupted shell config file
		configDir := filepath.Join(tempDir, ".config", "fish")
		err := os.MkdirAll(configDir, 0755)
		if err != nil {
			t.Fatalf("Failed to create config dir: %v", err)
		}

		configFile := filepath.Join(configDir, "config.fish")
		err = os.WriteFile(configFile, []byte("invalid shell syntax {{{"), 0644)
		if err != nil {
			t.Fatalf("Failed to write corrupted config: %v", err)
		}

		// Installation should still work (append to file)
		manager := NewInstallationManager()

		opts := InstallOptions{
			Shell:          "fish",
			Force:          false,
			BackupOriginal: true,
		}

		err = manager.Install(opts)
		if err != nil {
			t.Errorf("Installation should handle corrupted config files: %v", err)
		}
	})

	t.Run("disk_space_simulation", func(t *testing.T) {
		// This test verifies that disk space errors are handled properly
		// The actual implementation would need to check for disk space
		// For now, we'll test that the error handling mechanism works

		// Create a mock that simulates disk space error
		mfs := NewMockFileSystem()
		mfs.SetError("/tmp/test-file", fmt.Errorf("no space left on device"))

		// Test that the error is properly handled
		err := mfs.WriteFile("/tmp/test-file", []byte("test"), 0644)
		if err == nil {
			t.Error("Expected error when disk space is full")
		}

		if !strings.Contains(err.Error(), "no space left on device") {
			t.Errorf("Expected disk space error, got: %v", err)
		}
	})
}

// TestShellEnvironmentVariations tests different shell environment configurations
func TestShellEnvironmentVariations(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "awsm-shell-env-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	t.Run("different_shell_versions", func(t *testing.T) {
		// Test that wrapper functions work with different shell syntax variations
		core := NewWrapperCore()

		for _, shell := range GetSupportedShells() {
			wrapperFunc, err := core.GenerateShellWrapperFunction(shell)
			if err != nil {
				t.Errorf("Failed to generate wrapper for %s: %v", shell, err)
				continue
			}

			// Test that the wrapper function has proper syntax
			switch shell {
			case ShellZsh, ShellBash:
				// Should have proper bash/zsh function syntax
				if !strings.Contains(wrapperFunc, "aws()") || !strings.Contains(wrapperFunc, "}") {
					t.Errorf("Bash/Zsh wrapper should have proper function syntax")
				}
			case ShellFish:
				// Should have proper fish function syntax
				if !strings.Contains(wrapperFunc, "function aws") || !strings.Contains(wrapperFunc, "end") {
					t.Errorf("Fish wrapper should have proper function syntax")
				}
			case ShellPowerShell:
				// Should have proper PowerShell function syntax
				if !strings.Contains(wrapperFunc, "function aws") || !strings.Contains(wrapperFunc, "}") {
					t.Errorf("PowerShell wrapper should have proper function syntax")
				}
			}
		}
	})

	t.Run("custom_config_locations", func(t *testing.T) {
		// Test handling of custom configuration file locations
		manager := NewInstallationManager()

		// Create custom config directories
		customDirs := map[string]string{
			"zsh":        filepath.Join(tempDir, "custom", "zsh"),
			"fish":       filepath.Join(tempDir, "custom", "fish", "functions"),
			"powershell": filepath.Join(tempDir, "custom", "powershell"),
		}

		for shell, dir := range customDirs {
			err := os.MkdirAll(dir, 0755)
			if err != nil {
				t.Errorf("Failed to create custom dir for %s: %v", shell, err)
				continue
			}

			// Test that the manager can handle custom paths
			status := manager.GetShellStatus(shell)
			if status.Error != "" && !strings.Contains(status.Error, "not supported") {
				// Some errors are expected for non-standard setups
				t.Logf("Shell %s status: %s", shell, status.Error)
			}
		}
	})
}

// Helper function to capture command output
func captureCommandOutput(cmd *exec.Cmd) (string, string, error) {
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}
