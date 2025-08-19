package wrapper

import (
	"errors"
	"testing"
)

func TestFactory_CreateWrapper(t *testing.T) {
	factory := NewFactory()

	tests := []struct {
		shell       string
		expectError bool
		errorType   error
	}{
		{"zsh", false, nil},
		{"bash", false, nil},
		{"fish", false, nil},
		{"powershell", false, nil},
		{"invalid", true, ErrShellNotSupported},
		{"", true, ErrShellNotSupported},
	}

	for _, test := range tests {
		wrapper, err := factory.CreateWrapper(test.shell)

		if test.expectError {
			if err == nil {
				t.Errorf("Expected error for shell '%s', got nil", test.shell)
			}
			if test.errorType != nil && !errors.Is(err, test.errorType) {
				t.Errorf("Expected error type %v for shell '%s', got %v", test.errorType, test.shell, err)
			}
			if wrapper != nil {
				t.Errorf("Expected nil wrapper for invalid shell '%s', got %v", test.shell, wrapper)
			}
		} else {
			if err != nil {
				t.Errorf("Unexpected error for shell '%s': %v", test.shell, err)
			}
			// Note: wrapper will be nil since we haven't implemented the actual constructors yet
			// This is expected for this task - the constructors are placeholders
		}
	}
}

func TestFactory_DetectShell(t *testing.T) {
	factory := NewFactory()

	// Test that the method delegates to DetectCurrentShell
	// We can't easily test the actual detection without mocking environment
	_, err := factory.DetectShell()

	// We don't assert success/failure here since it depends on the test environment
	// Just ensure the method doesn't panic and returns the same result as DetectCurrentShell
	directResult, directErr := DetectCurrentShell()

	if (err == nil) != (directErr == nil) {
		t.Errorf("Factory.DetectShell() and DetectCurrentShell() should return same error status")
	}

	if err == nil && directErr == nil {
		factoryResult, _ := factory.DetectShell()
		if factoryResult != directResult {
			t.Errorf("Factory.DetectShell() = %q, DetectCurrentShell() = %q, should be equal", factoryResult, directResult)
		}
	}
}

func TestFactory_GetAllWrappers(t *testing.T) {
	factory := NewFactory()

	wrappers, err := factory.GetAllWrappers()

	// Since the constructors are placeholders returning nil, this will work
	// but the wrapper instances will be nil
	if err != nil {
		t.Errorf("Unexpected error getting all wrappers: %v", err)
	}

	expectedShells := []string{"zsh", "bash", "fish", "powershell"}

	if len(wrappers) != len(expectedShells) {
		t.Errorf("Expected %d wrappers, got %d", len(expectedShells), len(wrappers))
	}

	for _, shell := range expectedShells {
		if _, exists := wrappers[shell]; !exists {
			t.Errorf("Expected wrapper for shell '%s' to exist", shell)
		}
	}
}

func TestNewFactory(t *testing.T) {
	factory := NewFactory()

	if factory == nil {
		t.Error("Expected non-nil factory")
	}
}
