package wrapper

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewFishWrapper(t *testing.T) {
	wrapper := NewFishWrapper()

	if wrapper == nil {
		t.Fatal("NewFishWrapper() returned nil")
	}

	// Verify it's a FishWrapper
	fishWrapper, ok := wrapper.(*FishWrapper)
	if !ok {
		t.Fatal("NewFishWrapper() did not return a *FishWrapper")
	}

	if fishWrapper.shell != ShellFish {
		t.Errorf("Expected shell to be %s, got %s", ShellFish, fishWrapper.shell)
	}
}

func TestFishWrapper_GenerateFunction(t *testing.T) {
	wrapper := NewFishWrapper()
	function := wrapper.GenerateFunction()

	// Check that function is not empty
	if function == "" {
		t.Fatal("GenerateFunction() returned empty string")
	}

	// Check for essential components
	expectedComponents := []string{
		"function aws",
		"--description",
		"aws sts get-caller-identity",
		"awsm refresh",
		"command aws",
		"$argv",
		"end",
	}

	for _, component := range expectedComponents {
		if !strings.Contains(function, component) {
			t.Errorf("Generated function missing component: %s", component)
		}
	}

	// Check that it's a proper Fish function
	if !strings.Contains(function, "function aws") {
		t.Error("Generated function should define function aws")
	}

	// Check for Fish-specific syntax
	if !strings.Contains(function, "$argv") {
		t.Error("Generated function should use $argv for arguments")
	}

	if !strings.Contains(function, "end") {
		t.Error("Generated function should end with 'end'")
	}

	// Check for Fish conditional syntax
	if !strings.Contains(function, "if not") {
		t.Error("Generated function should use Fish 'if not' syntax")
	}
}

func TestFishWrapper_GetInstallPath(t *testing.T) {
	wrapper := NewFishWrapper()

	path, err := wrapper.GetInstallPath()
	if err != nil {
		t.Fatalf("GetInstallPath() returned error: %v", err)
	}

	// Should end with .config/fish/functions/aws.fish
	if !strings.HasSuffix(path, filepath.Join(".config", "fish", "functions", "aws.fish")) {
		t.Errorf("Install path should end with .config/fish/functions/aws.fish, got: %s", path)
	}

	// Should be an absolute path
	if !filepath.IsAbs(path) {
		t.Errorf("Install path should be absolute, got: %s", path)
	}
}

func TestFishWrapper_GetConfigFile(t *testing.T) {
	wrapper := NewFishWrapper()

	configFile, err := wrapper.GetConfigFile()
	if err != nil {
		t.Fatalf("GetConfigFile() returned error: %v", err)
	}

	// Should end with config.fish
	if !strings.HasSuffix(configFile, "config.fish") {
		t.Errorf("Config file should end with config.fish, got: %s", configFile)
	}

	// Should be an absolute path
	if !filepath.IsAbs(configFile) {
		t.Errorf("Config file path should be absolute, got: %s", configFile)
	}
}

func TestFishWrapper_Install(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Mock home directory
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	wrapper := NewFishWrapper()
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

	// Verify file permissions
	info, err := os.Stat(installPath)
	if err != nil {
		t.Fatalf("Failed to stat function file: %v", err)
	}

	expectedMode := os.FileMode(0644)
	if info.Mode().Perm() != expectedMode {
		t.Errorf("Function file should have mode %v, got %v", expectedMode, info.Mode().Perm())
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

func TestFishWrapper_Uninstall(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Mock home directory
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	wrapper := NewFishWrapper()

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
}

func TestFishWrapper_IsInstalled(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Mock home directory
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	wrapper := NewFishWrapper()

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

func TestFishWrapper_InstallCreatesDirectories(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Mock home directory
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	wrapper := NewFishWrapper()
	functionContent := wrapper.GenerateFunction()

	// Ensure the functions directory doesn't exist initially
	installPath, _ := wrapper.GetInstallPath()
	functionsDir := filepath.Dir(installPath)
	if FileExists(functionsDir) {
		t.Fatal("Functions directory should not exist initially")
	}

	// Install should create the directory
	err := wrapper.Install(functionContent)
	if err != nil {
		t.Fatalf("Install() returned error: %v", err)
	}

	// Verify directory was created
	if !FileExists(functionsDir) {
		t.Error("Functions directory should be created during install")
	}

	// Verify directory permissions
	info, err := os.Stat(functionsDir)
	if err != nil {
		t.Fatalf("Failed to stat functions directory: %v", err)
	}

	if !info.IsDir() {
		t.Error("Functions path should be a directory")
	}

	expectedMode := os.FileMode(0755)
	if info.Mode().Perm() != expectedMode {
		t.Errorf("Functions directory should have mode %v, got %v", expectedMode, info.Mode().Perm())
	}
}

func TestFishWrapper_InstallPathStructure(t *testing.T) {
	wrapper := NewFishWrapper()

	installPath, err := wrapper.GetInstallPath()
	if err != nil {
		t.Fatalf("GetInstallPath() returned error: %v", err)
	}

	// Verify the path structure follows Fish conventions
	pathParts := strings.Split(installPath, string(filepath.Separator))

	// Should contain .config, fish, functions, and aws.fish
	hasConfig := false
	hasFish := false
	hasFunctions := false
	hasAwsFish := false

	for _, part := range pathParts {
		switch part {
		case ".config":
			hasConfig = true
		case "fish":
			hasFish = true
		case "functions":
			hasFunctions = true
		case "aws.fish":
			hasAwsFish = true
		}
	}

	if !hasConfig {
		t.Error("Install path should contain .config directory")
	}
	if !hasFish {
		t.Error("Install path should contain fish directory")
	}
	if !hasFunctions {
		t.Error("Install path should contain functions directory")
	}
	if !hasAwsFish {
		t.Error("Install path should end with aws.fish")
	}
}
