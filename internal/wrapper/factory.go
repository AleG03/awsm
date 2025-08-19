package wrapper

import (
	"fmt"
)

// Factory creates WrapperGenerator instances for different shells
type Factory struct{}

// Ensure Factory implements WrapperFactory interface
var _ WrapperFactory = (*Factory)(nil)

// NewFactory creates a new Factory instance
func NewFactory() *Factory {
	return &Factory{}
}

// CreateWrapper creates a WrapperGenerator for the specified shell
func (f *Factory) CreateWrapper(shell string) (WrapperGenerator, error) {
	if !IsValidShell(shell) {
		return nil, fmt.Errorf("%w: %s (supported: %v)", ErrShellNotSupported, shell, GetSupportedShells())
	}

	switch SupportedShell(shell) {
	case ShellZsh:
		return NewZshWrapper(), nil
	case ShellBash:
		return NewBashWrapper(), nil
	case ShellFish:
		return NewFishWrapper(), nil
	case ShellPowerShell:
		return NewPowerShellWrapper(), nil
	default:
		return nil, fmt.Errorf("%w: %s", ErrShellNotSupported, shell)
	}
}

// DetectShell attempts to detect the current shell from environment variables
func (f *Factory) DetectShell() (string, error) {
	return DetectCurrentShell()
}

// GetAllWrappers returns WrapperGenerator instances for all supported shells
func (f *Factory) GetAllWrappers() (map[string]WrapperGenerator, error) {
	wrappers := make(map[string]WrapperGenerator)

	for _, shell := range GetSupportedShells() {
		wrapper, err := f.CreateWrapper(string(shell))
		if err != nil {
			return nil, fmt.Errorf("failed to create wrapper for %s: %w", shell, err)
		}
		wrappers[string(shell)] = wrapper
	}

	return wrappers, nil
}

// Placeholder functions for shell-specific wrapper constructors
// These will be implemented in subsequent tasks

func NewZshWrapper() WrapperGenerator {
	return &ZshWrapper{
		shell: ShellZsh,
	}
}

func NewBashWrapper() WrapperGenerator {
	return &BashWrapper{
		shell: ShellBash,
	}
}

func NewFishWrapper() WrapperGenerator {
	return &FishWrapper{
		shell: ShellFish,
	}
}

func NewPowerShellWrapper() WrapperGenerator {
	return &PowerShellWrapper{
		shell: ShellPowerShell,
	}
}
