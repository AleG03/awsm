package wrapper

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

var (
	// ErrShellNotSupported is returned when an unsupported shell is specified
	ErrShellNotSupported = errors.New("shell not supported")

	// ErrAlreadyInstalled is returned when trying to install a wrapper that's already installed
	ErrAlreadyInstalled = errors.New("wrapper already installed")

	// ErrNotInstalled is returned when trying to uninstall a wrapper that's not installed
	ErrNotInstalled = errors.New("wrapper not installed")

	// ErrPermissionDenied is returned when there are insufficient permissions for installation
	ErrPermissionDenied = errors.New("permission denied")

	// ErrConfigFileNotFound is returned when the shell configuration file cannot be found
	ErrConfigFileNotFound = errors.New("shell configuration file not found")

	// ErrDirectoryNotWritable is returned when a required directory is not writable
	ErrDirectoryNotWritable = errors.New("directory not writable")

	// ErrConfigFileNotWritable is returned when the shell configuration file is not writable
	ErrConfigFileNotWritable = errors.New("configuration file not writable")

	// ErrShellDetectionFailed is returned when automatic shell detection fails
	ErrShellDetectionFailed = errors.New("shell detection failed")

	// ErrBackupFailed is returned when creating a backup fails
	ErrBackupFailed = errors.New("backup creation failed")

	// ErrInvalidShellConfig is returned when shell configuration is invalid or corrupted
	ErrInvalidShellConfig = errors.New("invalid shell configuration")
)

// WrapperError represents a wrapper-specific error with additional context
type WrapperError struct {
	Shell       string
	Op          string // operation being performed
	Err         error
	Message     string
	Suggestions []string // user-friendly suggestions for resolving the error
}

func (e *WrapperError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("wrapper %s for %s: %s (%v)", e.Op, e.Shell, e.Message, e.Err)
	}
	return fmt.Sprintf("wrapper %s for %s: %v", e.Op, e.Shell, e.Err)
}

func (e *WrapperError) Unwrap() error {
	return e.Err
}

// GetUserFriendlyMessage returns a user-friendly error message with suggestions
func (e *WrapperError) GetUserFriendlyMessage() string {
	var msg strings.Builder

	// Main error message
	msg.WriteString(fmt.Sprintf("Failed to %s wrapper for %s shell", e.Op, e.Shell))

	// Add specific error details
	if e.Message != "" {
		msg.WriteString(fmt.Sprintf(": %s", e.Message))
	}

	// Add suggestions if available
	if len(e.Suggestions) > 0 {
		msg.WriteString("\n\nSuggested solutions:")
		for i, suggestion := range e.Suggestions {
			msg.WriteString(fmt.Sprintf("\n  %d. %s", i+1, suggestion))
		}
	}

	return msg.String()
}

// NewWrapperError creates a new WrapperError
func NewWrapperError(shell, op string, err error, message string) *WrapperError {
	wrapperErr := &WrapperError{
		Shell:   shell,
		Op:      op,
		Err:     err,
		Message: message,
	}

	// Add context-specific suggestions
	wrapperErr.Suggestions = generateSuggestions(shell, op, err, message)

	return wrapperErr
}

// generateSuggestions creates helpful suggestions based on the error context
func generateSuggestions(shell, op string, err error, message string) []string {
	var suggestions []string

	// Permission-related errors
	if isPermissionError(err) {
		suggestions = append(suggestions, getPermissionSuggestions(shell, op)...)
	}

	// Directory-related errors
	if isDirectoryError(err, message) {
		suggestions = append(suggestions, getDirectorySuggestions(shell)...)
	}

	// Configuration file errors
	if isConfigFileError(err, message) {
		suggestions = append(suggestions, getConfigFileSuggestions(shell)...)
	}

	// Shell detection errors
	if errors.Is(err, ErrShellDetectionFailed) {
		suggestions = append(suggestions, getShellDetectionSuggestions()...)
	}

	// Already installed errors
	if errors.Is(err, ErrAlreadyInstalled) {
		suggestions = append(suggestions, getAlreadyInstalledSuggestions(shell)...)
	}

	// Not installed errors
	if errors.Is(err, ErrNotInstalled) {
		suggestions = append(suggestions, getNotInstalledSuggestions(shell)...)
	}

	// Add general troubleshooting if no specific suggestions
	if len(suggestions) == 0 {
		suggestions = append(suggestions, getGeneralSuggestions(shell, op)...)
	}

	return suggestions
}

// isPermissionError checks if the error is permission-related
func isPermissionError(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "permission denied") ||
		strings.Contains(errStr, "access denied") ||
		strings.Contains(errStr, "operation not permitted") ||
		errors.Is(err, ErrPermissionDenied)
}

// isDirectoryError checks if the error is directory-related
func isDirectoryError(err error, message string) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())
	msgStr := strings.ToLower(message)

	return strings.Contains(errStr, "directory") ||
		strings.Contains(errStr, "mkdir") ||
		strings.Contains(msgStr, "directory") ||
		strings.Contains(msgStr, "create") ||
		errors.Is(err, ErrDirectoryNotWritable)
}

// isConfigFileError checks if the error is configuration file-related
func isConfigFileError(err error, message string) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())
	msgStr := strings.ToLower(message)

	return strings.Contains(errStr, "config") ||
		strings.Contains(errStr, ".bashrc") ||
		strings.Contains(errStr, ".zshrc") ||
		strings.Contains(errStr, "profile") ||
		strings.Contains(msgStr, "config") ||
		errors.Is(err, ErrConfigFileNotFound) ||
		errors.Is(err, ErrConfigFileNotWritable)
}

// getPermissionSuggestions returns suggestions for permission-related errors
func getPermissionSuggestions(shell, op string) []string {
	suggestions := []string{
		"Check that you have write permissions to your home directory",
		fmt.Sprintf("Ensure the %s configuration directory exists and is writable", shell),
	}

	if runtime.GOOS != "windows" {
		suggestions = append(suggestions, "Try running: chmod 755 ~ && chmod 644 ~/."+getConfigFileName(shell))
	}

	suggestions = append(suggestions, "If using a shared system, contact your administrator for assistance")

	return suggestions
}

// getDirectorySuggestions returns suggestions for directory-related errors
func getDirectorySuggestions(shell string) []string {
	suggestions := []string{
		"Ensure your home directory is accessible",
	}

	switch shell {
	case "zsh":
		suggestions = append(suggestions,
			"Create the Zsh functions directory: mkdir -p ~/.zsh/functions",
			"Verify ~/.zsh directory permissions: ls -la ~/.zsh")
	case "fish":
		suggestions = append(suggestions,
			"Create the Fish functions directory: mkdir -p ~/.config/fish/functions",
			"Verify Fish configuration directory: ls -la ~/.config/fish")
	case "powershell":
		if runtime.GOOS == "windows" {
			suggestions = append(suggestions, "Create PowerShell profile directory: New-Item -ItemType Directory -Force -Path (Split-Path $PROFILE)")
		} else {
			suggestions = append(suggestions, "Create PowerShell profile directory: mkdir -p ~/.config/powershell")
		}
	}

	return suggestions
}

// getConfigFileSuggestions returns suggestions for configuration file errors
func getConfigFileSuggestions(shell string) []string {
	configFile := getConfigFileName(shell)

	suggestions := []string{
		fmt.Sprintf("Check if %s exists and is writable", configFile),
		fmt.Sprintf("Create the configuration file if it doesn't exist: touch %s", configFile),
	}

	if runtime.GOOS != "windows" {
		suggestions = append(suggestions, fmt.Sprintf("Fix file permissions: chmod 644 %s", configFile))
	}

	return suggestions
}

// getShellDetectionSuggestions returns suggestions for shell detection failures
func getShellDetectionSuggestions() []string {
	supportedShells := []string{"zsh", "bash", "fish", "powershell"}

	return []string{
		"Specify the shell explicitly: awsm wrapper <shell>",
		fmt.Sprintf("Supported shells: %s", strings.Join(supportedShells, ", ")),
		"Check your SHELL environment variable: echo $SHELL",
		"Ensure you're running the command from your target shell",
	}
}

// getAlreadyInstalledSuggestions returns suggestions for already installed errors
func getAlreadyInstalledSuggestions(shell string) []string {
	return []string{
		"Use --force flag to reinstall: awsm wrapper --force " + shell,
		"Check installation status: awsm wrapper --status",
		"Uninstall first, then reinstall: awsm wrapper --uninstall " + shell,
	}
}

// getNotInstalledSuggestions returns suggestions for not installed errors
func getNotInstalledSuggestions(shell string) []string {
	return []string{
		"Install the wrapper first: awsm wrapper " + shell,
		"Check installation status: awsm wrapper --status",
		"Verify you're using the correct shell name",
	}
}

// getGeneralSuggestions returns general troubleshooting suggestions
func getGeneralSuggestions(shell, op string) []string {
	return []string{
		"Check that awsm is properly installed and in your PATH",
		"Verify your shell configuration files are not corrupted",
		"Try running the command with elevated privileges if necessary",
		"Check the awsm documentation for troubleshooting tips",
		"Report this issue if the problem persists: https://github.com/your-repo/awsm/issues",
	}
}

// getConfigFileName returns the primary configuration file name for a shell
func getConfigFileName(shell string) string {
	switch shell {
	case "zsh":
		return "~/.zshrc"
	case "bash":
		return "~/.bashrc"
	case "fish":
		return "~/.config/fish/config.fish"
	case "powershell":
		if runtime.GOOS == "windows" {
			return "$PROFILE"
		}
		return "~/.config/powershell/Microsoft.PowerShell_profile.ps1"
	default:
		return "shell configuration file"
	}
}

// ValidateInstallationEnvironment performs comprehensive validation before installation
func ValidateInstallationEnvironment(shell string) error {
	// Validate shell support
	if err := ValidateShell(shell); err != nil {
		return NewWrapperError(shell, "validate", err, "unsupported shell")
	}

	// Check home directory access
	home, err := os.UserHomeDir()
	if err != nil {
		return NewWrapperError(shell, "validate", err, "cannot access home directory")
	}

	// Test home directory write permissions
	testFile := filepath.Join(home, ".awsm-test-write")
	if file, err := os.Create(testFile); err != nil {
		return NewWrapperError(shell, "validate", err, "home directory is not writable")
	} else {
		file.Close()
		os.Remove(testFile)
	}

	// Validate shell-specific requirements
	switch shell {
	case "zsh":
		return validateZshEnvironment()
	case "bash":
		return validateBashEnvironment()
	case "fish":
		return validateFishEnvironment()
	case "powershell":
		return validatePowerShellEnvironment()
	}

	return nil
}

// validateZshEnvironment validates Zsh-specific requirements
func validateZshEnvironment() error {
	home, _ := os.UserHomeDir()

	// Check if .zsh directory can be created
	zshDir := filepath.Join(home, ".zsh")
	if err := os.MkdirAll(zshDir, 0755); err != nil {
		return NewWrapperError("zsh", "validate", err, "cannot create .zsh directory")
	}

	// Check if functions directory can be created
	functionsDir := filepath.Join(zshDir, "functions")
	if err := os.MkdirAll(functionsDir, 0755); err != nil {
		return NewWrapperError("zsh", "validate", err, "cannot create functions directory")
	}

	return nil
}

// validateBashEnvironment validates Bash-specific requirements
func validateBashEnvironment() error {
	home, _ := os.UserHomeDir()
	bashrc := filepath.Join(home, ".bashrc")

	// Check if .bashrc can be created or is writable
	if _, err := os.Stat(bashrc); os.IsNotExist(err) {
		// Try to create it
		if file, err := os.Create(bashrc); err != nil {
			return NewWrapperError("bash", "validate", err, "cannot create .bashrc file")
		} else {
			file.Close()
		}
	} else {
		// Check if it's writable
		if file, err := os.OpenFile(bashrc, os.O_WRONLY|os.O_APPEND, 0644); err != nil {
			return NewWrapperError("bash", "validate", err, ".bashrc file is not writable")
		} else {
			file.Close()
		}
	}

	return nil
}

// validateFishEnvironment validates Fish-specific requirements
func validateFishEnvironment() error {
	home, _ := os.UserHomeDir()

	// Check if Fish config directory can be created
	fishConfigDir := filepath.Join(home, ".config", "fish")
	if err := os.MkdirAll(fishConfigDir, 0755); err != nil {
		return NewWrapperError("fish", "validate", err, "cannot create Fish configuration directory")
	}

	// Check if functions directory can be created
	functionsDir := filepath.Join(fishConfigDir, "functions")
	if err := os.MkdirAll(functionsDir, 0755); err != nil {
		return NewWrapperError("fish", "validate", err, "cannot create Fish functions directory")
	}

	return nil
}

// validatePowerShellEnvironment validates PowerShell-specific requirements
func validatePowerShellEnvironment() error {
	home, _ := os.UserHomeDir()

	var profilePath string
	if runtime.GOOS == "windows" {
		profilePath = filepath.Join(home, "Documents", "PowerShell", "Microsoft.PowerShell_profile.ps1")
	} else {
		profilePath = filepath.Join(home, ".config", "powershell", "Microsoft.PowerShell_profile.ps1")
	}

	// Check if profile directory can be created
	profileDir := filepath.Dir(profilePath)
	if err := os.MkdirAll(profileDir, 0755); err != nil {
		return NewWrapperError("powershell", "validate", err, "cannot create PowerShell profile directory")
	}

	// Check if profile can be created or is writable
	if _, err := os.Stat(profilePath); os.IsNotExist(err) {
		// Try to create it
		if file, err := os.Create(profilePath); err != nil {
			return NewWrapperError("powershell", "validate", err, "cannot create PowerShell profile")
		} else {
			file.Close()
		}
	} else {
		// Check if it's writable
		if file, err := os.OpenFile(profilePath, os.O_WRONLY|os.O_APPEND, 0644); err != nil {
			return NewWrapperError("powershell", "validate", err, "PowerShell profile is not writable")
		} else {
			file.Close()
		}
	}

	return nil
}

// ValidateShell checks if a shell name is valid and provides helpful error messages
func ValidateShell(shell string) error {
	if shell == "" {
		return fmt.Errorf("shell cannot be empty")
	}

	supportedShells := []string{"zsh", "bash", "fish", "powershell"}
	for _, supported := range supportedShells {
		if strings.ToLower(shell) == supported {
			return nil
		}
	}

	return fmt.Errorf("shell '%s' is not supported. Supported shells: %s",
		shell, strings.Join(supportedShells, ", "))
}
