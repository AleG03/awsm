package wrapper

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewZshWrapper(t *testing.T) {
	wrapper := NewZshWrapper()

	if wrapper == nil {
		t.Fatal("NewZshWrapper() returned nil")
	}

	// Verify it's a ZshWrapper
	zshWrapper, ok := wrapper.(*ZshWrapper)
	if !ok {
		t.Fatal("NewZshWrapper() did not return a *ZshWrapper")
	}

	if zshWrapper.shell != ShellZsh {
		t.Errorf("Expected shell to be %s, got %s", ShellZsh, zshWrapper.shell)
	}
}

func TestZshWrapper_GenerateFunction(t *testing.T) {
	wrapper := NewZshWrapper()
	function := wrapper.GenerateFunction()

	// Check that function is not empty
	if function == "" {
		t.Fatal("GenerateFunction() returned empty string")
	}

	// Check for essential components
	expectedComponents := []string{
		"#compdef aws",
		"aws() {",
		"aws sts get-caller-identity",
		"awsm refresh",
		"command aws",
		"$@",
	}

	for _, component := range expectedComponents {
		if !strings.Contains(function, component) {
			t.Errorf("Generated function missing component: %s", component)
		}
	}

	// Check that it's a proper Zsh function
	if !strings.Contains(function, "aws() {") {
		t.Error("Generated function should define aws() function")
	}

	// Check for autoload compatibility (compdef)
	if !strings.Contains(function, "#compdef aws") {
		t.Error("Generated function should include #compdef for autoload compatibility")
	}
}

func TestZshWrapper_GetInstallPath(t *testing.T) {
	wrapper := NewZshWrapper()

	path, err := wrapper.GetInstallPath()
	if err != nil {
		t.Fatalf("GetInstallPath() returned error: %v", err)
	}

	// Should end with .zsh/functions/aws
	if !strings.HasSuffix(path, filepath.Join(".zsh", "functions", "aws")) {
		t.Errorf("Install path should end with .zsh/functions/aws, got: %s", path)
	}

	// Should be an absolute path
	if !filepath.IsAbs(path) {
		t.Errorf("Install path should be absolute, got: %s", path)
	}
}

func TestZshWrapper_GetConfigFile(t *testing.T) {
	wrapper := NewZshWrapper()

	configFile, err := wrapper.GetConfigFile()
	if err != nil {
		t.Fatalf("GetConfigFile() returned error: %v", err)
	}

	// Should end with .zshrc
	if !strings.HasSuffix(configFile, ".zshrc") {
		t.Errorf("Config file should end with .zshrc, got: %s", configFile)
	}

	// Should be an absolute path
	if !filepath.IsAbs(configFile) {
		t.Errorf("Config file path should be absolute, got: %s", configFile)
	}
}

func TestZshWrapper_Install(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Mock home directory
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	wrapper := NewZshWrapper()
	functionContent := wrapper.GenerateFunction()

	// Test successful installation
	err := wrapper.Install(functionContent)
	if err != nil {
		t.Fatalf("Install() returned error: %v", err)
	}

	// Verify function file was created
	installPath, _ := wrapper.GetInstallPath()
	if !FileExists(installPath) {
		t.Error("Function file was not created")
	}

	// Verify function content
	content, err := os.ReadFile(installPath)
	if err != nil {
		t.Fatalf("Failed to read function file: %v", err)
	}

	if string(content) != functionContent {
		t.Error("Function file content does not match expected content")
	}

	// Verify .zshrc was updated
	configFile, _ := wrapper.GetConfigFile()
	if !FileExists(configFile) {
		t.Error(".zshrc file was not created")
	}

	zshrcContent, err := os.ReadFile(configFile)
	if err != nil {
		t.Fatalf("Failed to read .zshrc: %v", err)
	}

	zshrcStr := string(zshrcContent)
	if !strings.Contains(zshrcStr, "fpath=") {
		t.Error(".zshrc should contain fpath entry")
	}

	if !strings.Contains(zshrcStr, "autoload -Uz aws") {
		t.Error(".zshrc should contain autoload entry")
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

func TestZshWrapper_Uninstall(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Mock home directory
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	wrapper := NewZshWrapper()

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
	installPath, _ := wrapper.GetInstallPath()
	if !FileExists(installPath) {
		t.Fatal("Function file should exist before uninstall")
	}

	// Test successful uninstall
	err = wrapper.Uninstall()
	if err != nil {
		t.Fatalf("Uninstall() returned error: %v", err)
	}

	// Verify function file was removed
	if FileExists(installPath) {
		t.Error("Function file should be removed after uninstall")
	}

	// Note: We don't remove fpath entries from .zshrc, so we don't test for that
}

func TestZshWrapper_IsInstalled(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Mock home directory
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	wrapper := NewZshWrapper()

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

func TestZshWrapper_fpathEntryExists(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	wrapper := &ZshWrapper{shell: ShellZsh}

	// Test with non-existent file
	nonExistentFile := filepath.Join(tempDir, "nonexistent")
	functionsDir := filepath.Join(tempDir, ".zsh", "functions")

	exists := wrapper.fpathEntryExists(nonExistentFile, functionsDir)
	if exists {
		t.Error("fpathEntryExists should return false for non-existent file")
	}

	// Create a test .zshrc file without fpath entry
	zshrcPath := filepath.Join(tempDir, ".zshrc")
	err := os.WriteFile(zshrcPath, []byte("# Some other content\nexport PATH=$PATH:/usr/local/bin\n"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test .zshrc: %v", err)
	}

	exists = wrapper.fpathEntryExists(zshrcPath, functionsDir)
	if exists {
		t.Error("fpathEntryExists should return false when fpath entry doesn't exist")
	}

	// Add fpath entry
	fpathContent := fmt.Sprintf("fpath=(%s $fpath)\n", functionsDir)
	file, err := os.OpenFile(zshrcPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("Failed to open .zshrc for append: %v", err)
	}
	file.WriteString(fpathContent)
	file.Close()

	exists = wrapper.fpathEntryExists(zshrcPath, functionsDir)
	if !exists {
		t.Error("fpathEntryExists should return true when fpath entry exists")
	}

	// Test with autoload entry instead
	zshrcPath2 := filepath.Join(tempDir, ".zshrc2")
	autoloadContent := "autoload -Uz aws\n"
	err = os.WriteFile(zshrcPath2, []byte(autoloadContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test .zshrc2: %v", err)
	}

	exists = wrapper.fpathEntryExists(zshrcPath2, functionsDir)
	if !exists {
		t.Error("fpathEntryExists should return true when autoload entry exists")
	}
}

func TestZshWrapper_updateZshrc(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Mock home directory
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	wrapper := &ZshWrapper{shell: ShellZsh}

	// Test updating non-existent .zshrc
	err := wrapper.updateZshrc()
	if err != nil {
		t.Fatalf("updateZshrc() failed: %v", err)
	}

	// Verify .zshrc was created and contains expected content
	configFile, _ := wrapper.GetConfigFile()
	if !FileExists(configFile) {
		t.Error(".zshrc should be created")
	}

	content, err := os.ReadFile(configFile)
	if err != nil {
		t.Fatalf("Failed to read .zshrc: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "fpath=") {
		t.Error(".zshrc should contain fpath entry")
	}

	if !strings.Contains(contentStr, "autoload -Uz aws") {
		t.Error(".zshrc should contain autoload entry")
	}

	if !strings.Contains(contentStr, "Added by awsm wrapper command") {
		t.Error(".zshrc should contain comment about awsm")
	}

	// Test updating existing .zshrc that already has the entry (should not duplicate)
	originalContent := string(content)
	err = wrapper.updateZshrc()
	if err != nil {
		t.Fatalf("updateZshrc() failed on second call: %v", err)
	}

	newContent, err := os.ReadFile(configFile)
	if err != nil {
		t.Fatalf("Failed to read .zshrc after second update: %v", err)
	}

	if string(newContent) != originalContent {
		t.Error("updateZshrc() should not modify .zshrc when entry already exists")
	}
}
