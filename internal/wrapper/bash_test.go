package wrapper

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewBashWrapper(t *testing.T) {
	wrapper := NewBashWrapper()

	if wrapper == nil {
		t.Fatal("NewBashWrapper() returned nil")
	}

	// Verify it's a BashWrapper
	bashWrapper, ok := wrapper.(*BashWrapper)
	if !ok {
		t.Fatal("NewBashWrapper() did not return a *BashWrapper")
	}

	if bashWrapper.shell != ShellBash {
		t.Errorf("Expected shell to be %s, got %s", ShellBash, bashWrapper.shell)
	}
}

func TestBashWrapper_GenerateFunction(t *testing.T) {
	wrapper := NewBashWrapper()
	function := wrapper.GenerateFunction()

	// Check that function is not empty
	if function == "" {
		t.Fatal("GenerateFunction() returned empty string")
	}

	// Check for essential components
	expectedComponents := []string{
		"aws() {",
		"aws sts get-caller-identity",
		"awsm refresh",
		"command aws",
		"$@",
		"}",
	}

	for _, component := range expectedComponents {
		if !strings.Contains(function, component) {
			t.Errorf("Generated function missing component: %s", component)
		}
	}

	// Check that it's a proper Bash function
	if !strings.Contains(function, "aws() {") {
		t.Error("Generated function should define aws() function")
	}

	// Check for proper error handling
	if !strings.Contains(function, "return 1") {
		t.Error("Generated function should include error handling with return 1")
	}

	// Check for credential refresh logic
	if !strings.Contains(function, "AWS credentials expired or invalid") {
		t.Error("Generated function should include user-friendly error messages")
	}
}

func TestBashWrapper_GetInstallPath(t *testing.T) {
	wrapper := NewBashWrapper()

	path, err := wrapper.GetInstallPath()
	if err != nil {
		t.Fatalf("GetInstallPath() returned error: %v", err)
	}

	// For Bash, install path should be the config file path
	configFile, err := wrapper.GetConfigFile()
	if err != nil {
		t.Fatalf("GetConfigFile() returned error: %v", err)
	}

	if path != configFile {
		t.Errorf("Install path should equal config file path for Bash, got install: %s, config: %s", path, configFile)
	}

	// Should be an absolute path
	if !filepath.IsAbs(path) {
		t.Errorf("Install path should be absolute, got: %s", path)
	}
}

func TestBashWrapper_GetConfigFile(t *testing.T) {
	wrapper := NewBashWrapper()

	configFile, err := wrapper.GetConfigFile()
	if err != nil {
		t.Fatalf("GetConfigFile() returned error: %v", err)
	}

	// Should end with .bashrc (default) or other valid bash config files
	validSuffixes := []string{".bashrc", ".bash_profile", ".profile"}
	hasValidSuffix := false
	for _, suffix := range validSuffixes {
		if strings.HasSuffix(configFile, suffix) {
			hasValidSuffix = true
			break
		}
	}

	if !hasValidSuffix {
		t.Errorf("Config file should end with a valid bash config suffix, got: %s", configFile)
	}

	// Should be an absolute path
	if !filepath.IsAbs(configFile) {
		t.Errorf("Config file path should be absolute, got: %s", configFile)
	}
}

func TestBashWrapper_Install(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Mock home directory
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	wrapper := NewBashWrapper()
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

	if !strings.Contains(contentStr, "aws() {") {
		t.Error("Config file should contain aws function definition")
	}

	if !strings.Contains(contentStr, functionContent) {
		t.Error("Config file should contain the generated function content")
	}

	// Note: Since we started with a new temp directory, no backup should be created for new file
	// This is correct behavior - we only backup existing files

	// Test installing when already installed (should fail)
	err = wrapper.Install(functionContent)
	if err == nil {
		t.Error("Install() should fail when already installed")
	}

	if !strings.Contains(err.Error(), "already installed") {
		t.Errorf("Error should mention already installed, got: %v", err)
	}
}

func TestBashWrapper_Uninstall(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Mock home directory
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	wrapper := NewBashWrapper()

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

	if strings.Contains(contentStr, "aws() {") {
		t.Error("Config file should not contain aws function after uninstall")
	}
}

func TestBashWrapper_IsInstalled(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Mock home directory
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	wrapper := NewBashWrapper()

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

func TestBashWrapper_functionExistsInFile(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	wrapper := &BashWrapper{shell: ShellBash}

	// Test with non-existent file
	nonExistentFile := filepath.Join(tempDir, "nonexistent")
	exists := wrapper.functionExistsInFile(nonExistentFile)
	if exists {
		t.Error("functionExistsInFile should return false for non-existent file")
	}

	// Create a test .bashrc file without wrapper function
	bashrcPath := filepath.Join(tempDir, ".bashrc")
	err := os.WriteFile(bashrcPath, []byte("# Some other content\nexport PATH=$PATH:/usr/local/bin\n"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test .bashrc: %v", err)
	}

	exists = wrapper.functionExistsInFile(bashrcPath)
	if exists {
		t.Error("functionExistsInFile should return false when wrapper function doesn't exist")
	}

	// Add wrapper function
	wrapperContent := `
# Added by awsm wrapper command
aws() {
    # Check if credentials are valid
    if ! command aws sts get-caller-identity >/dev/null 2>&1; then
        echo "AWS credentials expired or invalid. Refreshing..."
        if ! awsm refresh; then
            echo "Failed to refresh AWS credentials."
            return 1
        fi
        echo "Credentials refreshed successfully."
    fi
    command aws "$@"
}
`
	file, err := os.OpenFile(bashrcPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("Failed to open .bashrc for append: %v", err)
	}
	file.WriteString(wrapperContent)
	file.Close()

	exists = wrapper.functionExistsInFile(bashrcPath)
	if !exists {
		t.Error("functionExistsInFile should return true when wrapper function exists")
	}

	// Test with just the comment but no function
	bashrcPath2 := filepath.Join(tempDir, ".bashrc2")
	commentOnlyContent := "# Added by awsm wrapper command\n# Some other content\n"
	err = os.WriteFile(bashrcPath2, []byte(commentOnlyContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test .bashrc2: %v", err)
	}

	exists = wrapper.functionExistsInFile(bashrcPath2)
	if exists {
		t.Error("functionExistsInFile should return false when only comment exists without function")
	}
}

func TestBashWrapper_appendToConfigFile(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	wrapper := &BashWrapper{shell: ShellBash}
	configFile := filepath.Join(tempDir, ".bashrc")
	functionContent := "aws() {\n    echo 'test'\n}"

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
	additionalContent := "test_function() {\n    echo 'additional'\n}"

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

func TestBashWrapper_removeFromConfigFile(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	wrapper := &BashWrapper{shell: ShellBash}
	configFile := filepath.Join(tempDir, ".bashrc")

	// Create a config file with wrapper function and other content
	initialContent := `# Initial content
export PATH=$PATH:/usr/local/bin

# Added by awsm wrapper command
aws() {
    # Check if credentials are valid
    if ! command aws sts get-caller-identity >/dev/null 2>&1; then
        echo "AWS credentials expired or invalid. Refreshing..."
        if ! awsm refresh; then
            echo "Failed to refresh AWS credentials."
            return 1
        fi
        echo "Credentials refreshed successfully."
    fi
    command aws "$@"
}

# More content after
alias ll='ls -la'
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

	if strings.Contains(contentStr, "aws() {") {
		t.Error("Config file should not contain aws function after removal")
	}

	if strings.Contains(contentStr, "awsm refresh") {
		t.Error("Config file should not contain awsm refresh call after removal")
	}

	// Should still contain other content
	if !strings.Contains(contentStr, "Initial content") {
		t.Error("Config file should still contain initial content")
	}

	if !strings.Contains(contentStr, "export PATH") {
		t.Error("Config file should still contain PATH export")
	}

	if !strings.Contains(contentStr, "alias ll") {
		t.Error("Config file should still contain other aliases")
	}
}

func TestBashWrapper_InstallWithExistingConfig(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Mock home directory
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	wrapper := NewBashWrapper()
	configFile, _ := wrapper.GetConfigFile()

	// Create existing config file with some content
	existingContent := "# Existing bashrc content\nexport PATH=$PATH:/usr/local/bin\n"
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
