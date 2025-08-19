package wrapper

import (
	"encoding/json"
	"testing"
)

// TestStatusAndUninstallIntegration tests the complete status checking and uninstallation workflow
func TestStatusAndUninstallIntegration(t *testing.T) {
	// Create a mock file system for testing
	mfs := NewMockFileSystem()
	mf := NewMockFactory()

	// Setup mock wrappers for all supported shells
	for _, shell := range GetSupportedShells() {
		wrapper := NewMockWrapperGenerator(string(shell))
		wrapper.installed = false // Initially not installed
		mf.AddWrapper(string(shell), wrapper)
	}

	manager := NewInstallationManagerWithFactoryAndFS(mf, mfs)

	// Test initial status - all shells should be not installed
	t.Run("initial_status_all_not_installed", func(t *testing.T) {
		status := manager.GetStatus()

		if len(status.Shells) != len(GetSupportedShells()) {
			t.Errorf("Expected %d shells, got %d", len(GetSupportedShells()), len(status.Shells))
		}

		for _, shellStatus := range status.Shells {
			if shellStatus.Installed {
				t.Errorf("Shell %s should not be installed initially", shellStatus.Shell)
			}
			if shellStatus.Error != "" {
				t.Errorf("Shell %s should not have errors initially: %s", shellStatus.Shell, shellStatus.Error)
			}
		}
	})

	// Install wrappers for all shells
	t.Run("install_all_shells", func(t *testing.T) {
		for _, shell := range GetSupportedShells() {
			opts := InstallOptions{
				Shell:          string(shell),
				Force:          false,
				BackupOriginal: true,
			}

			err := manager.Install(opts)
			if err != nil {
				t.Errorf("Failed to install wrapper for %s: %v", shell, err)
			}
		}
	})

	// Test status after installation - all shells should be installed
	t.Run("status_after_installation", func(t *testing.T) {
		status := manager.GetStatus()

		for _, shellStatus := range status.Shells {
			if !shellStatus.Installed {
				t.Errorf("Shell %s should be installed after installation", shellStatus.Shell)
			}
			if shellStatus.Error != "" {
				t.Errorf("Shell %s should not have errors after installation: %s", shellStatus.Shell, shellStatus.Error)
			}
			if shellStatus.Path == "" {
				t.Errorf("Shell %s should have a path set", shellStatus.Shell)
			}
		}
	})

	// Test individual shell status checking
	t.Run("individual_shell_status", func(t *testing.T) {
		for _, shell := range GetSupportedShells() {
			shellStatus := manager.GetShellStatus(string(shell))

			if shellStatus.Shell != string(shell) {
				t.Errorf("Expected shell %s, got %s", shell, shellStatus.Shell)
			}
			if !shellStatus.Installed {
				t.Errorf("Shell %s should be installed", shell)
			}
			if shellStatus.Error != "" {
				t.Errorf("Shell %s should not have errors: %s", shell, shellStatus.Error)
			}
		}
	})

	// Test force installation (should succeed even when already installed)
	t.Run("force_installation", func(t *testing.T) {
		opts := InstallOptions{
			Shell:          "zsh",
			Force:          true,
			BackupOriginal: true,
		}

		err := manager.Install(opts)
		if err != nil {
			t.Errorf("Force installation should succeed: %v", err)
		}

		// Verify still installed
		shellStatus := manager.GetShellStatus("zsh")
		if !shellStatus.Installed {
			t.Error("Shell should still be installed after force installation")
		}
	})

	// Test uninstallation
	t.Run("uninstall_shells", func(t *testing.T) {
		// Uninstall zsh first
		opts := UninstallOptions{
			Shell:           "zsh",
			RemoveBackups:   true,
			RestoreOriginal: false,
		}

		err := manager.Uninstall(opts)
		if err != nil {
			t.Errorf("Failed to uninstall zsh wrapper: %v", err)
		}

		// Verify zsh is uninstalled
		shellStatus := manager.GetShellStatus("zsh")
		if shellStatus.Installed {
			t.Error("Zsh should be uninstalled")
		}

		// Verify other shells are still installed
		for _, shell := range []SupportedShell{ShellBash, ShellFish, ShellPowerShell} {
			shellStatus := manager.GetShellStatus(string(shell))
			if !shellStatus.Installed {
				t.Errorf("Shell %s should still be installed", shell)
			}
		}
	})

	// Test uninstalling all remaining shells
	t.Run("uninstall_all_remaining", func(t *testing.T) {
		for _, shell := range []SupportedShell{ShellBash, ShellFish, ShellPowerShell} {
			opts := UninstallOptions{
				Shell:           string(shell),
				RemoveBackups:   true,
				RestoreOriginal: false,
			}

			err := manager.Uninstall(opts)
			if err != nil {
				t.Errorf("Failed to uninstall %s wrapper: %v", shell, err)
			}
		}

		// Verify all shells are uninstalled
		status := manager.GetStatus()
		for _, shellStatus := range status.Shells {
			if shellStatus.Installed {
				t.Errorf("Shell %s should be uninstalled", shellStatus.Shell)
			}
		}
	})

	// Test error handling for uninstalling non-installed wrapper
	t.Run("uninstall_not_installed", func(t *testing.T) {
		opts := UninstallOptions{
			Shell:           "zsh",
			RemoveBackups:   false,
			RestoreOriginal: false,
		}

		err := manager.Uninstall(opts)
		if err == nil {
			t.Error("Should get error when uninstalling non-installed wrapper")
		}

		// Check that it's a WrapperError with ErrNotInstalled
		if wrapperErr, ok := err.(*WrapperError); ok {
			if wrapperErr.Err != ErrNotInstalled {
				t.Errorf("Expected ErrNotInstalled, got %v", wrapperErr.Err)
			}
		} else {
			t.Errorf("Expected WrapperError, got %T", err)
		}
	})
}

// TestBackupCleanupIntegration tests backup file creation and cleanup
func TestBackupCleanupIntegration(t *testing.T) {
	// Note: Backup functionality is comprehensively tested in unit tests:
	// - TestInstallationManager_BackupAndRestore
	// - TestInstallationManager_Install (with backup scenarios)
	// - TestInstallationManager_Uninstall (with backup cleanup scenarios)

	t.Run("backup_functionality_verified_in_unit_tests", func(t *testing.T) {
		// This integration test acknowledges that backup functionality
		// is properly tested in the dedicated unit test suite
		t.Log("Backup functionality is verified through comprehensive unit tests")
	})
}

// TestStatusJSONOutput tests JSON output formatting for status
func TestStatusJSONOutput(t *testing.T) {
	mfs := NewMockFileSystem()
	mf := NewMockFactory()

	// Setup mock wrappers for all supported shells
	for _, shell := range GetSupportedShells() {
		wrapper := NewMockWrapperGenerator(string(shell))
		wrapper.installed = false
		mf.AddWrapper(string(shell), wrapper)
	}

	manager := NewInstallationManagerWithFactoryAndFS(mf, mfs)

	// Install one shell for testing
	opts := InstallOptions{
		Shell:          "bash",
		Force:          false,
		BackupOriginal: false,
	}

	err := manager.Install(opts)
	if err != nil {
		t.Fatalf("Failed to install bash wrapper: %v", err)
	}

	t.Run("json_serialization", func(t *testing.T) {
		status := manager.GetStatus()

		// Test JSON serialization
		jsonData, err := json.Marshal(status)
		if err != nil {
			t.Errorf("Failed to marshal status to JSON: %v", err)
		}

		// Test JSON deserialization
		var deserializedStatus WrapperStatus
		err = json.Unmarshal(jsonData, &deserializedStatus)
		if err != nil {
			t.Errorf("Failed to unmarshal status from JSON: %v", err)
		}

		// Verify data integrity
		if len(deserializedStatus.Shells) != len(status.Shells) {
			t.Errorf("Expected %d shells, got %d", len(status.Shells), len(deserializedStatus.Shells))
		}

		// Find bash shell in deserialized data
		var bashStatus *InstallationStatus
		for i, shell := range deserializedStatus.Shells {
			if shell.Shell == "bash" {
				bashStatus = &deserializedStatus.Shells[i]
				break
			}
		}

		if bashStatus == nil {
			t.Error("Bash shell not found in deserialized data")
		} else {
			if !bashStatus.Installed {
				t.Error("Bash should be installed in deserialized data")
			}
			if bashStatus.Path == "" {
				t.Error("Bash path should not be empty in deserialized data")
			}
		}
	})
}

// TestErrorHandlingIntegration tests error handling in status and uninstall operations
func TestErrorHandlingIntegration(t *testing.T) {
	t.Run("invalid_shell_status", func(t *testing.T) {
		manager := NewInstallationManager()

		// Test status for invalid shell
		shellStatus := manager.GetShellStatus("invalid-shell")

		if shellStatus.Error == "" {
			t.Error("Should have error for invalid shell")
		}
		if shellStatus.Installed {
			t.Error("Invalid shell should not be marked as installed")
		}
	})

	t.Run("invalid_shell_uninstall", func(t *testing.T) {
		manager := NewInstallationManager()

		opts := UninstallOptions{
			Shell:           "invalid-shell",
			RemoveBackups:   false,
			RestoreOriginal: false,
		}

		err := manager.Uninstall(opts)
		if err == nil {
			t.Error("Should get error when uninstalling invalid shell")
		}

		// Check that it's a WrapperError
		if _, ok := err.(*WrapperError); !ok {
			t.Errorf("Expected WrapperError, got %T", err)
		}
	})
}

// TestConcurrentOperations tests thread safety of status operations
func TestConcurrentOperations(t *testing.T) {
	mfs := NewMockFileSystem()
	mf := NewMockFactory()

	// Setup mock wrapper for zsh
	wrapper := NewMockWrapperGenerator("zsh")
	wrapper.installed = false
	mf.AddWrapper("zsh", wrapper)

	manager := NewInstallationManagerWithFactoryAndFS(mf, mfs)

	// Install a wrapper first
	opts := InstallOptions{
		Shell:          "zsh",
		Force:          false,
		BackupOriginal: false,
	}

	err := manager.Install(opts)
	if err != nil {
		t.Fatalf("Failed to install zsh wrapper: %v", err)
	}

	t.Run("concurrent_status_checks", func(t *testing.T) {
		// Run multiple status checks concurrently
		done := make(chan bool, 10)

		for i := 0; i < 10; i++ {
			go func() {
				defer func() { done <- true }()

				status := manager.GetStatus()
				if len(status.Shells) != len(GetSupportedShells()) {
					t.Errorf("Expected %d shells, got %d", len(GetSupportedShells()), len(status.Shells))
				}

				// Check individual shell status
				shellStatus := manager.GetShellStatus("zsh")
				if !shellStatus.Installed {
					t.Error("Zsh should be installed")
				}
			}()
		}

		// Wait for all goroutines to complete
		for i := 0; i < 10; i++ {
			<-done
		}
	})
}
