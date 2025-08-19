package wrapper

// WrapperFactory interface for creating wrapper generators
type WrapperFactory interface {
	CreateWrapper(shell string) (WrapperGenerator, error)
}

// WrapperGenerator defines the interface for shell-specific wrapper implementations
type WrapperGenerator interface {
	// GenerateFunction returns the shell-specific wrapper function content
	GenerateFunction() string

	// GetInstallPath returns the path where the wrapper function should be installed
	GetInstallPath() (string, error)

	// GetConfigFile returns the path to the shell's configuration file
	GetConfigFile() (string, error)

	// Install installs the wrapper function with the given content
	Install(functionContent string) error

	// Uninstall removes the wrapper function
	Uninstall() error

	// IsInstalled checks if the wrapper function is currently installed
	IsInstalled() (bool, error)
}

// InstallationStatus represents the installation status for a specific shell
type InstallationStatus struct {
	Shell     string `json:"shell"`
	Installed bool   `json:"installed"`
	Path      string `json:"path"`
	Error     string `json:"error,omitempty"`
}

// WrapperStatus represents the overall wrapper installation status across all shells
type WrapperStatus struct {
	Shells []InstallationStatus `json:"shells"`
}

// WrapperConfig holds configuration for wrapper installation
type WrapperConfig struct {
	Shell           string
	FunctionContent string
	InstallPath     string
	ConfigFile      string
	BackupOriginal  bool
}

// SupportedShell represents a shell that the wrapper can be installed for
type SupportedShell string

const (
	ShellZsh        SupportedShell = "zsh"
	ShellBash       SupportedShell = "bash"
	ShellFish       SupportedShell = "fish"
	ShellPowerShell SupportedShell = "powershell"
)

// GetSupportedShells returns a list of all supported shells
func GetSupportedShells() []SupportedShell {
	return []SupportedShell{
		ShellZsh,
		ShellBash,
		ShellFish,
		ShellPowerShell,
	}
}

// IsValidShell checks if the given shell is supported
func IsValidShell(shell string) bool {
	for _, supported := range GetSupportedShells() {
		if string(supported) == shell {
			return true
		}
	}
	return false
}
