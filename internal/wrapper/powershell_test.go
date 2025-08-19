package wrapper

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewPowerShellWrapper(t *testing.T) {
	wrapper := NewPowerShellWrapper()

	if wrapper == nil {
		t.Fatal("NewPowerShellWrapper() returned nil")
	}

	// Verify it's a PowerShellWrapper
	powershellWrapper, ok := wrapper.(*PowerShellWrapper)
	if !ok {
		t.Fatal("NewPowerShellWrapper() did not return a *PowerShellWrapper")
	}

	if powershellWrapper.shell != ShellPowerShell {
		t.Errorf("Expected shell to be %s, got %s", ShellPowerShell, powershellWrapper.shell)
	}
}

func TestPowerShellWrapper_GenerateFunction(t *testing.T) {
	wrapper := NewPowerShellWrapper()
	function := wrapper.GenerateFunction()

	// Check that function is not empty
	if function == "" {
		t.Fatal("GenerateFunction() returned empty string")
	}

	// Check for essential components
	expectedComponents := []string{
		"function aws {",
		"aws sts get-caller-identity",
		"awsm refresh",
		"Get-Command aws -CommandType Application",
		"@args",
		"}",
	}

	for _, component := range expectedComponents {
		if !strings.Contains(function, component) {
			t.Errorf("Generated function missing component: %s", component)
		}
	}

	// Check that it's a proper PowerShell function
	if !strings.Contains(function, "function aws {") {
		t.Error("Generated function should define function aws")
	}

	// Check for proper error handling
	if !strings.Contains(function, "$LASTEXITCODE") {
		t.Error("Generated function should include PowerShell exit code checking")
	}

	// Check for credential refresh logic
	if !strings.Contains(function, "AWS credentials expired or invalid") {
		t.Error("Generated function should include user-friendly error messages")
	}

	// Check for PowerShell-specific syntax
	if !strings.Contains(function, "Write-Host") {
		t.Error("Generated function should use PowerShell Write-Host for output")
	}
}

func TestPowerShellWrapper_GetInstallPath(t *testing.T) {
	wrapper := NewPowerShellWrapper()

	path, err := wrapper.GetInstallPath()
	if err != nil {
		t.Fatalf("GetInstallPath() returned error: %v", err)
	}

	// For PowerShell, install path should be the config file path
	configFile, err := wrapper.GetConfigFile()
	if err != nil {
		t.Fatalf("GetConfigFile() returned error: %v", err)
	}

	if path != configFile {
		t.Errorf("Install path should equal config file path for PowerShell, got install: %s, config: %s", path, configFile)
	}

	// Should be an absolute path
	if !filepath.IsAbs(path) {
		t.Errorf("Install path should be absolute, got: %s", path)
	}
}

func TestPowerShellWrapper_GetConfigFile(t *testing.T) {
	wrapper := NewPowerShellWrapper()

	configFile, err := wrapper.GetConfigFile()
	if err != nil {
		t.Fatalf("GetConfigFile() returned error: %v", err)
	}

	// Should end with PowerShell profile file
	if !strings.HasSuffix(configFile, "Microsoft.PowerShell_profile.ps1") {
		t.Errorf("Config file should end with Microsoft.PowerShell_profile.ps1, got: %s", configFile)
	}

	// Should be an absolute path
	if !filepath.IsAbs(configFile) {
		t.Errorf("Config file path should be absolute, got: %s", configFile)
	}
}

func TestPowerShellWrapper_Install(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Mock home directory
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	wrapper := NewPowerShellWrapper()
	functionContent := wrapper.GenerateFunction()

	// Test successful installation
	err := wrapper.Install(functionContent)
	if err != nil {
		t.Fatalf("Install() returned error: %v", err)
	}

	// Verify config file was created/updated
	configFile, _ := wrapper.GetConfigFile()
	if !FileExists(configFile) {
		t.Error("Config file was not created")
	}

	// Verify function content was appended
	content, err := os.ReadFile(configFile)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "Added by awsm wrapper command") {
		t.Error("Config file should contain awsm wrapper comment")
	}

	if !strings.Contains(contentStr, "function aws {") {
		t.Error("Config file should contain aws function definition")
	}

	if !strings.Contains(contentStr, functionContent) {
		t.Error("Config file should contain the generated function content")
	}

	// Test installing when already installed (should fail)
	err = wrapper.Install(functionContent)
	if err == nil {
		t.Error("Install() should fail when already installed")
	}

	if !strings.Contains(err.Error(), "already installed") {
		t.Errorf("Error should mention already installed, got: %v", err)
	}
}

func TestPowerShellWrapper_Uninstall(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Mock home directory
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	wrapper := NewPowerShellWrapper()

	// Test uninstalling when not installed (should fail)
	err := wrapper.Uninstall()
	if err == nil {
		t.Error("Uninstall() should fail when not installed")
	}

	if !strings.Contains(err.Error(), "not installed") {
		t.Errorf("Error should mention not installed, got: %v", err)
	}

	// Install first
	functionContent := wrapper.GenerateFunction()
	err = wrapper.Install(functionContent)
	if err != nil {
		t.Fatalf("Failed to install for uninstall test: %v", err)
	}

	// Verify it's installed
	installed, _ := wrapper.IsInstalled()
	if !installed {
		t.Fatal("Wrapper should be installed before uninstall test")
	}

	// Test successful uninstall
	err = wrapper.Uninstall()
	if err != nil {
		t.Fatalf("Uninstall() returned error: %v", err)
	}

	// Verify function was removed from config file
	installed, _ = wrapper.IsInstalled()
	if installed {
		t.Error("Wrapper should not be installed after uninstall")
	}

	// Verify config file still exists but without wrapper function
	configFile, _ := wrapper.GetConfigFile()
	if !FileExists(configFile) {
		t.Error("Config file should still exist after uninstall")
	}

	content, err := os.ReadFile(configFile)
	if err != nil {
		t.Fatalf("Failed to read config file after uninstall: %v", err)
	}

	contentStr := string(content)
	if strings.Contains(contentStr, "Added by awsm wrapper command") {
		t.Error("Config file should not contain awsm wrapper comment after uninstall")
	}

	if strings.Contains(contentStr, "function aws {") {
		t.Error("Config file should not contain aws function after uninstall")
	}
}

func TestPowerShellWrapper_IsInstalled(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Mock home directory
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	wrapper := NewPowerShellWrapper()

	// Test when not installed
	installed, err := wrapper.IsInstalled()
	if err != nil {
		t.Fatalf("IsInstalled() returned error: %v", err)
	}

	if installed {
		t.Error("IsInstalled() should return false when not installed")
	}

	// Install the wrapper
	functionContent := wrapper.GenerateFunction()
	err = wrapper.Install(functionContent)
	if err != nil {
		t.Fatalf("Failed to install for IsInstalled test: %v", err)
	}

	// Test when installed
	installed, err = wrapper.IsInstalled()
	if err != nil {
		t.Fatalf("IsInstalled() returned error after install: %v", err)
	}

	if !installed {
		t.Error("IsInstalled() should return true when installed")
	}
}

func TestPowerShellWrapper_functionExistsInFile(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	wrapper := &PowerShellWrapper{shell: ShellPowerShell}

	// Test with non-existent file
	nonExistentFile := filepath.Join(tempDir, "nonexistent")
	exists := wrapper.functionExistsInFile(nonExistentFile)
	if exists {
		t.Error("functionExistsInFile should return false for non-existent file")
	}

	// Create a test PowerShell profile file without wrapper function
	profilePath := filepath.Join(tempDir, "Microsoft.PowerShell_profile.ps1")
	err := os.WriteFile(profilePath, []byte("# Some other content\n$env:PATH += ';C:\\Tools'\n"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test profile: %v", err)
	}

	exists = wrapper.functionExistsInFile(profilePath)
	if exists {
		t.Error("functionExistsInFile should return false when wrapper function doesn't exist")
	}

	// Add wrapper function
	wrapperContent := `
# Added by awsm wrapper command
function aws {
    $null = aws sts get-caller-identity 2>$null
    if ($LASTEXITCODE -ne 0) {
        Write-Host "AWS credentials expired or invalid. Refreshing..."
        awsm refresh
        if ($LASTEXITCODE -ne 0) {
            Write-Host "Failed to refresh AWS credentials."
            return
        }
        Write-Host "Credentials refreshed successfully."
    }
    & (Get-Command aws -CommandType Application) @args
}
`
	file, err := os.OpenFile(profilePath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("Failed to open profile for append: %v", err)
	}
	file.WriteString(wrapperContent)
	file.Close()

	exists = wrapper.functionExistsInFile(profilePath)
	if !exists {
		t.Error("functionExistsInFile should return true when wrapper function exists")
	}

	// Test with just the comment but no function
	profilePath2 := filepath.Join(tempDir, "Microsoft.PowerShell_profile2.ps1")
	commentOnlyContent := "# Added by awsm wrapper command\n# Some other content\n"
	err = os.WriteFile(profilePath2, []byte(commentOnlyContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test profile2: %v", err)
	}

	exists = wrapper.functionExistsInFile(profilePath2)
	if exists {
		t.Error("functionExistsInFile should return false when only comment exists without function")
	}
}

func TestPowerShellWrapper_appendToConfigFile(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	wrapper := &PowerShellWrapper{shell: ShellPowerShell}
	configFile := filepath.Join(tempDir, "Microsoft.PowerShell_profile.ps1")
	functionContent := "function aws {\n    Write-Host 'test'\n}"

	// Test appending to non-existent file
	err := wrapper.appendToConfigFile(configFile, functionContent)
	if err != nil {
		t.Fatalf("appendToConfigFile() failed: %v", err)
	}

	// Verify file was created and contains expected content
	if !FileExists(configFile) {
		t.Error("Config file should be created")
	}

	content, err := os.ReadFile(configFile)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "Added by awsm wrapper command") {
		t.Error("Config file should contain awsm wrapper comment")
	}

	if !strings.Contains(contentStr, functionContent) {
		t.Error("Config file should contain the function content")
	}

	// Test appending to existing file
	originalContent := string(content)
	additionalContent := "function Test-Function {\n    Write-Host 'additional'\n}"

	err = wrapper.appendToConfigFile(configFile, additionalContent)
	if err != nil {
		t.Fatalf("appendToConfigFile() failed on existing file: %v", err)
	}

	newContent, err := os.ReadFile(configFile)
	if err != nil {
		t.Fatalf("Failed to read config file after second append: %v", err)
	}

	newContentStr := string(newContent)
	if !strings.Contains(newContentStr, originalContent) {
		t.Error("Config file should still contain original content")
	}

	if !strings.Contains(newContentStr, additionalContent) {
		t.Error("Config file should contain the additional content")
	}
}

func TestPowerShellWrapper_removeFromConfigFile(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	wrapper := &PowerShellWrapper{shell: ShellPowerShell}
	configFile := filepath.Join(tempDir, "Microsoft.PowerShell_profile.ps1")

	// Create a config file with wrapper function and other content
	initialContent := `# Initial content
$env:PATH += ';C:\Tools'

# Added by awsm wrapper command
function aws {
    # Check if credentials are valid
    $null = aws sts get-caller-identity 2>$null
    if ($LASTEXITCODE -ne 0) {
        Write-Host "AWS credentials expired or invalid. Refreshing..."
        awsm refresh
        if ($LASTEXITCODE -ne 0) {
            Write-Host "Failed to refresh AWS credentials."
            return
        }
        Write-Host "Credentials refreshed successfully."
    }
    & (Get-Command aws -CommandType Application) @args
}

# More content after
Set-Alias ll Get-ChildItem
`

	err := os.WriteFile(configFile, []byte(initialContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	// Test removing wrapper function
	err = wrapper.removeFromConfigFile(configFile)
	if err != nil {
		t.Fatalf("removeFromConfigFile() failed: %v", err)
	}

	// Verify wrapper function was removed
	content, err := os.ReadFile(configFile)
	if err != nil {
		t.Fatalf("Failed to read config file after removal: %v", err)
	}

	contentStr := string(content)

	// Should not contain wrapper-related content
	if strings.Contains(contentStr, "Added by awsm wrapper command") {
		t.Error("Config file should not contain awsm wrapper comment after removal")
	}

	if strings.Contains(contentStr, "function aws {") {
		t.Error("Config file should not contain aws function after removal")
	}

	if strings.Contains(contentStr, "awsm refresh") {
		t.Error("Config file should not contain awsm refresh call after removal")
	}

	// Should still contain other content
	if !strings.Contains(contentStr, "Initial content") {
		t.Error("Config file should still contain initial content")
	}

	if !strings.Contains(contentStr, "$env:PATH") {
		t.Error("Config file should still contain PATH modification")
	}

	if !strings.Contains(contentStr, "Set-Alias ll") {
		t.Error("Config file should still contain other aliases")
	}
}

func TestPowerShellWrapper_InstallWithExistingConfig(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Mock home directory
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	wrapper := NewPowerShellWrapper()
	configFile, _ := wrapper.GetConfigFile()

	// Ensure the config directory exists
	configDir := filepath.Dir(configFile)
	if err := EnsureDir(configDir); err != nil {
		t.Fatalf("Failed to create config directory: %v", err)
	}

	// Create existing config file with some content
	existingContent := "# Existing PowerShell profile content\n$env:PATH += ';C:\\Tools'\n"
	err := os.WriteFile(configFile, []byte(existingContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create existing config file: %v", err)
	}

	// Install wrapper
	functionContent := wrapper.GenerateFunction()
	err = wrapper.Install(functionContent)
	if err != nil {
		t.Fatalf("Install() failed with existing config: %v", err)
	}

	// Verify existing content is preserved
	content, err := os.ReadFile(configFile)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, existingContent) {
		t.Error("Existing config content should be preserved")
	}

	if !strings.Contains(contentStr, "Added by awsm wrapper command") {
		t.Error("Wrapper function should be added")
	}

	// Verify backup was created with original content
	backupFile := configFile + ".awsm-backup"
	if !FileExists(backupFile) {
		t.Error("Backup file should be created")
	}

	backupContent, err := os.ReadFile(backupFile)
	if err != nil {
		t.Fatalf("Failed to read backup file: %v", err)
	}

	if string(backupContent) != existingContent {
		t.Error("Backup file should contain original content")
	}
}

func TestPowerShellWrapper_removeFromConfigFile_ComplexFunction(t *testing.T) {
	// Test removing a function with nested braces
	tempDir := t.TempDir()
	wrapper := &PowerShellWrapper{shell: ShellPowerShell}
	configFile := filepath.Join(tempDir, "Microsoft.PowerShell_profile.ps1")

	// Create a config file with wrapper function that has nested braces
	initialContent := `# Initial content
$env:PATH += ';C:\Tools'

# Added by awsm wrapper command
function aws {
    if ($args.Count -gt 0) {
        if ($args[0] -eq "help") {
            Write-Host "AWS CLI wrapper help"
        } else {
            # Check if credentials are valid
            $null = aws sts get-caller-identity 2>$null
            if ($LASTEXITCODE -ne 0) {
                Write-Host "AWS credentials expired or invalid. Refreshing..."
                awsm refresh
                if ($LASTEXITCODE -ne 0) {
                    Write-Host "Failed to refresh AWS credentials."
                    return
                }
                Write-Host "Credentials refreshed successfully."
            }
            & (Get-Command aws -CommandType Application) @args
        }
    }
}

# More content after
Set-Alias ll Get-ChildItem
`

	err := os.WriteFile(configFile, []byte(initialContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	// Test removing wrapper function with nested braces
	err = wrapper.removeFromConfigFile(configFile)
	if err != nil {
		t.Fatalf("removeFromConfigFile() failed: %v", err)
	}

	// Verify wrapper function was removed
	content, err := os.ReadFile(configFile)
	if err != nil {
		t.Fatalf("Failed to read config file after removal: %v", err)
	}

	contentStr := string(content)

	// Should not contain wrapper-related content
	if strings.Contains(contentStr, "Added by awsm wrapper command") {
		t.Error("Config file should not contain awsm wrapper comment after removal")
	}

	if strings.Contains(contentStr, "function aws {") {
		t.Error("Config file should not contain aws function after removal")
	}

	if strings.Contains(contentStr, "awsm refresh") {
		t.Error("Config file should not contain awsm refresh call after removal")
	}

	// Should still contain other content
	if !strings.Contains(contentStr, "Initial content") {
		t.Error("Config file should still contain initial content")
	}

	if !strings.Contains(contentStr, "$env:PATH") {
		t.Error("Config file should still contain PATH modification")
	}

	if !strings.Contains(contentStr, "Set-Alias ll") {
		t.Error("Config file should still contain other aliases")
	}
}
