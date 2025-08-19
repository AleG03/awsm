package wrapper

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// GetHomeDir returns the user's home directory
func GetHomeDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return home, nil
}

// EnsureDir creates a directory if it doesn't exist
func EnsureDir(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.MkdirAll(path, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", path, err)
		}
	}
	return nil
}

// FileExists checks if a file exists
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// BackupFile creates a backup of the specified file
func BackupFile(path string) error {
	if !FileExists(path) {
		return nil // Nothing to backup
	}

	backupPath := path + ".awsm-backup"

	// Read original file
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file for backup: %w", err)
	}

	// Write backup
	if err := os.WriteFile(backupPath, content, 0644); err != nil {
		return fmt.Errorf("failed to create backup file: %w", err)
	}

	return nil
}

// RestoreBackup restores a file from its backup
func RestoreBackup(path string) error {
	backupPath := path + ".awsm-backup"

	if !FileExists(backupPath) {
		return fmt.Errorf("backup file not found: %s", backupPath)
	}

	// Read backup file
	content, err := os.ReadFile(backupPath)
	if err != nil {
		return fmt.Errorf("failed to read backup file: %w", err)
	}

	// Restore original file
	if err := os.WriteFile(path, content, 0644); err != nil {
		return fmt.Errorf("failed to restore file: %w", err)
	}

	// Remove backup file
	if err := os.Remove(backupPath); err != nil {
		return fmt.Errorf("failed to remove backup file: %w", err)
	}

	return nil
}

// GetShellConfigPaths returns common configuration file paths for different shells
func GetShellConfigPaths(shell SupportedShell) ([]string, error) {
	home, err := GetHomeDir()
	if err != nil {
		return nil, err
	}

	switch shell {
	case ShellZsh:
		return []string{
			filepath.Join(home, ".zshrc"),
			filepath.Join(home, ".zsh", ".zshrc"),
		}, nil
	case ShellBash:
		return []string{
			filepath.Join(home, ".bashrc"),
			filepath.Join(home, ".bash_profile"),
			filepath.Join(home, ".profile"),
		}, nil
	case ShellFish:
		return []string{
			filepath.Join(home, ".config", "fish", "config.fish"),
		}, nil
	case ShellPowerShell:
		if runtime.GOOS == "windows" {
			return []string{
				filepath.Join(home, "Documents", "PowerShell", "Microsoft.PowerShell_profile.ps1"),
				filepath.Join(home, "Documents", "WindowsPowerShell", "Microsoft.PowerShell_profile.ps1"),
			}, nil
		}
		return []string{
			filepath.Join(home, ".config", "powershell", "Microsoft.PowerShell_profile.ps1"),
		}, nil
	default:
		return nil, fmt.Errorf("unsupported shell: %s", shell)
	}
}

// FindExistingConfigFile finds the first existing configuration file from a list of paths
func FindExistingConfigFile(paths []string) string {
	for _, path := range paths {
		if FileExists(path) {
			return path
		}
	}
	return ""
}

// GetDefaultConfigFile returns the default (preferred) configuration file path for a shell
func GetDefaultConfigFile(shell SupportedShell) (string, error) {
	paths, err := GetShellConfigPaths(shell)
	if err != nil {
		return "", err
	}

	if len(paths) == 0 {
		return "", fmt.Errorf("no configuration paths defined for shell: %s", shell)
	}

	// Return the first (preferred) path
	return paths[0], nil
}

// DetectCurrentShell attempts to detect the current shell from environment variables and system information
func DetectCurrentShell() (string, error) {
	// Method 1: Check SHELL environment variable (Unix-like systems)
	if shell := os.Getenv("SHELL"); shell != "" {
		shellName := filepath.Base(shell)
		// Handle common shell executable names
		shellName = normalizeShellName(shellName)
		if IsValidShell(shellName) {
			return shellName, nil
		}
	}

	// Method 2: Check for PowerShell on Windows
	if runtime.GOOS == "windows" {
		if detectPowerShell() {
			return string(ShellPowerShell), nil
		}
	}

	// Method 3: Check shell-specific environment variables
	if shell := detectShellFromEnvVars(); shell != "" {
		return shell, nil
	}

	// Method 4: Check parent process (fallback)
	if shell := detectShellFromParentProcess(); shell != "" {
		return shell, nil
	}

	return "", fmt.Errorf("unable to detect shell automatically - please specify shell explicitly")
}

// normalizeShellName converts shell executable names to standard shell names
func normalizeShellName(shellName string) string {
	// Remove common prefixes/suffixes
	shellName = strings.TrimSuffix(shellName, ".exe")

	// Handle common variations
	switch strings.ToLower(shellName) {
	case "zsh", "zsh5":
		return string(ShellZsh)
	case "bash", "bash4", "bash5":
		return string(ShellBash)
	case "fish":
		return string(ShellFish)
	case "powershell", "pwsh", "powershell.exe", "pwsh.exe":
		return string(ShellPowerShell)
	default:
		return shellName
	}
}

// detectPowerShell checks for PowerShell-specific environment indicators
func detectPowerShell() bool {
	// Check for PowerShell-specific environment variables
	if os.Getenv("PSModulePath") != "" {
		return true
	}

	// Check for PowerShell in PATH
	if path := os.Getenv("PATH"); path != "" {
		if strings.Contains(strings.ToLower(path), "powershell") {
			return true
		}
	}

	// Check for PowerShell version variables
	if os.Getenv("PSVersionTable") != "" || os.Getenv("PSHOME") != "" {
		return true
	}

	return false
}

// detectShellFromEnvVars checks shell-specific environment variables
func detectShellFromEnvVars() string {
	// Zsh-specific variables
	if os.Getenv("ZSH_VERSION") != "" || os.Getenv("ZSH_NAME") != "" {
		return string(ShellZsh)
	}

	// Bash-specific variables
	if os.Getenv("BASH_VERSION") != "" || os.Getenv("BASH") != "" {
		return string(ShellBash)
	}

	// Fish-specific variables
	if os.Getenv("FISH_VERSION") != "" || os.Getenv("fish_greeting") != "" {
		return string(ShellFish)
	}

	// PowerShell-specific variables (additional check)
	if os.Getenv("PSVersionTable") != "" {
		return string(ShellPowerShell)
	}

	return ""
}

// detectShellFromParentProcess attempts to detect shell from parent process
// This is a simplified implementation - in production you might want more sophisticated process detection
func detectShellFromParentProcess() string {
	// This is a placeholder for more advanced parent process detection
	// On Unix systems, you could read /proc/self/stat or use ps commands
	// On Windows, you could use WMI or Windows API calls
	// For now, we'll return empty string as this requires platform-specific code
	return ""
}

// GetShellInstallPath returns the installation path for a wrapper function for the given shell
func GetShellInstallPath(shell SupportedShell) (string, error) {
	home, err := GetHomeDir()
	if err != nil {
		return "", err
	}

	switch shell {
	case ShellZsh:
		return filepath.Join(home, ".zsh", "functions", "aws"), nil
	case ShellBash:
		// For bash, the function is installed directly in the config file, not a separate file
		return GetDefaultConfigFile(shell)
	case ShellFish:
		return filepath.Join(home, ".config", "fish", "functions", "aws.fish"), nil
	case ShellPowerShell:
		// For PowerShell, the function is installed directly in the profile, not a separate file
		return GetDefaultConfigFile(shell)
	default:
		return "", fmt.Errorf("unsupported shell: %s", shell)
	}
}

// GetShellFunctionDirectory returns the directory where shell functions are stored
func GetShellFunctionDirectory(shell SupportedShell) (string, error) {
	home, err := GetHomeDir()
	if err != nil {
		return "", err
	}

	switch shell {
	case ShellZsh:
		return filepath.Join(home, ".zsh", "functions"), nil
	case ShellFish:
		return filepath.Join(home, ".config", "fish", "functions"), nil
	case ShellBash, ShellPowerShell:
		// Bash and PowerShell don't use separate function directories
		return "", fmt.Errorf("shell %s does not use a separate function directory", shell)
	default:
		return "", fmt.Errorf("unsupported shell: %s", shell)
	}
}

// CheckShellConfigAccess verifies that the shell configuration file is accessible
func CheckShellConfigAccess(shell SupportedShell) error {
	configFile, err := GetDefaultConfigFile(shell)
	if err != nil {
		return fmt.Errorf("failed to get config file path: %w", err)
	}

	// Check if config file exists
	if !FileExists(configFile) {
		// Check if parent directory exists and is writable
		parentDir := filepath.Dir(configFile)
		if !FileExists(parentDir) {
			return fmt.Errorf("configuration directory does not exist: %s", parentDir)
		}

		// Try to create the config file to test write permissions
		file, err := os.OpenFile(configFile, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0644)
		if err != nil {
			return fmt.Errorf("cannot create configuration file %s: %w", configFile, err)
		}
		file.Close()
		os.Remove(configFile) // Clean up test file

		return nil
	}

	// Check if existing config file is writable
	file, err := os.OpenFile(configFile, os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("configuration file %s is not writable: %w", configFile, err)
	}
	file.Close()

	return nil
}
