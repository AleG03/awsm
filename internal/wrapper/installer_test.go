package wrapper

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// MockFileSystem implements FileSystem interface for testing
type MockFileSystem struct {
	files       map[string][]byte
	directories map[string]bool
	errors      map[string]error // Map of path -> error to simulate failures
}

func NewMockFileSystem() *MockFileSystem {
	return &MockFileSystem{
		files:       make(map[string][]byte),
		directories: make(map[string]bool),
		errors:      make(map[string]error),
	}
}

func (mfs *MockFileSystem) SetError(path string, err error) {
	mfs.errors[path] = err
}

func (mfs *MockFileSystem) AddFile(path string, content []byte) {
	mfs.files[path] = content
	// Ensure parent directories exist
	dir := filepath.Dir(path)
	mfs.directories[dir] = true
}

func (mfs *MockFileSystem) AddDirectory(path string) {
	mfs.directories[path] = true
}

func (mfs *MockFileSystem) Stat(name string) (os.FileInfo, error) {
	if err, exists := mfs.errors[name]; exists {
		return nil, err
	}

	if _, exists := mfs.files[name]; exists {
		return &mockFileInfo{name: filepath.Base(name), isDir: false}, nil
	}

	if _, exists := mfs.directories[name]; exists {
		return &mockFileInfo{name: filepath.Base(name), isDir: true}, nil
	}

	return nil, os.ErrNotExist
}

func (mfs *MockFileSystem) MkdirAll(path string, perm os.FileMode) error {
	if err, exists := mfs.errors[path]; exists {
		return err
	}

	mfs.directories[path] = true
	return nil
}

func (mfs *MockFileSystem) ReadFile(filename string) ([]byte, error) {
	if err, exists := mfs.errors[filename]; exists {
		return nil, err
	}

	if content, exists := mfs.files[filename]; exists {
		return content, nil
	}

	return nil, os.ErrNotExist
}

func (mfs *MockFileSystem) WriteFile(filename string, data []byte, perm os.FileMode) error {
	if err, exists := mfs.errors[filename]; exists {
		return err
	}

	mfs.files[filename] = data
	// Ensure parent directory exists
	dir := filepath.Dir(filename)
	mfs.directories[dir] = true
	return nil
}

func (mfs *MockFileSystem) Remove(name string) error {
	if err, exists := mfs.errors[name]; exists {
		return err
	}

	delete(mfs.files, name)
	delete(mfs.directories, name)
	return nil
}

// SetPermissionError sets a permission error for a specific path
func (mfs *MockFileSystem) SetPermissionError(path string, enable bool) {
	if enable {
		mfs.errors[path] = os.ErrPermission
	} else {
		delete(mfs.errors, path)
	}
}

// SetDiskSpaceError simulates disk space errors
func (mfs *MockFileSystem) SetDiskSpaceError(enable bool) {
	if enable {
		// Set error for all write operations
		mfs.errors["__disk_full__"] = fmt.Errorf("no space left on device")
	} else {
		delete(mfs.errors, "__disk_full__")
	}
}

// Override WriteFile to check for disk space error
func (mfs *MockFileSystem) WriteFileWithDiskCheck(filename string, data []byte, perm os.FileMode) error {
	if _, exists := mfs.errors["__disk_full__"]; exists {
		return fmt.Errorf("no space left on device")
	}
	return mfs.WriteFile(filename, data, perm)
}

func (mfs *MockFileSystem) Rename(oldpath, newpath string) error {
	if err, exists := mfs.errors[oldpath]; exists {
		return err
	}
	if err, exists := mfs.errors[newpath]; exists {
		return err
	}

	if content, exists := mfs.files[oldpath]; exists {
		mfs.files[newpath] = content
		delete(mfs.files, oldpath)
		return nil
	}

	return os.ErrNotExist
}

func (mfs *MockFileSystem) Create(name string) (*os.File, error) {
	if err, exists := mfs.errors[name]; exists {
		return nil, err
	}

	// For testing purposes, we'll simulate file creation
	mfs.files[name] = []byte{}
	return nil, nil // Return nil file for simplicity in tests
}

func (mfs *MockFileSystem) Open(name string) (*os.File, error) {
	if err, exists := mfs.errors[name]; exists {
		return nil, err
	}

	if _, exists := mfs.files[name]; exists {
		return nil, nil // Return nil file for simplicity in tests
	}

	return nil, os.ErrNotExist
}

// mockFileInfo implements os.FileInfo for testing
type mockFileInfo struct {
	name  string
	isDir bool
}

func (mfi *mockFileInfo) Name() string       { return mfi.name }
func (mfi *mockFileInfo) Size() int64        { return 0 }
func (mfi *mockFileInfo) Mode() os.FileMode  { return 0644 }
func (mfi *mockFileInfo) ModTime() time.Time { return time.Now() }
func (mfi *mockFileInfo) IsDir() bool        { return mfi.isDir }
func (mfi *mockFileInfo) Sys() interface{}   { return nil }

// MockWrapperGenerator implements WrapperGenerator for testing
type MockWrapperGenerator struct {
	shell            string
	installPath      string
	configFile       string
	functionContent  string
	installed        bool
	installError     error
	uninstallError   error
	isInstalledError error
}

func NewMockWrapperGenerator(shell string) *MockWrapperGenerator {
	return &MockWrapperGenerator{
		shell:           shell,
		installPath:     "/mock/path/" + shell,
		configFile:      "/mock/config/" + shell,
		functionContent: "mock function for " + shell,
	}
}

func (mwg *MockWrapperGenerator) GenerateFunction() string {
	return mwg.functionContent
}

func (mwg *MockWrapperGenerator) GetInstallPath() (string, error) {
	return mwg.installPath, nil
}

func (mwg *MockWrapperGenerator) GetConfigFile() (string, error) {
	return mwg.configFile, nil
}

func (mwg *MockWrapperGenerator) Install(functionContent string) error {
	if mwg.installError != nil {
		return mwg.installError
	}
	mwg.installed = true
	return nil
}

func (mwg *MockWrapperGenerator) Uninstall() error {
	if mwg.uninstallError != nil {
		return mwg.uninstallError
	}
	mwg.installed = false
	return nil
}

func (mwg *MockWrapperGenerator) IsInstalled() (bool, error) {
	if mwg.isInstalledError != nil {
		return false, mwg.isInstalledError
	}
	return mwg.installed, nil
}

// MockFactory implements Factory for testing
type MockFactory struct {
	wrappers map[string]*MockWrapperGenerator
	errors   map[string]error
}

func NewMockFactory() *MockFactory {
	return &MockFactory{
		wrappers: make(map[string]*MockWrapperGenerator),
		errors:   make(map[string]error),
	}
}

func (mf *MockFactory) AddWrapper(shell string, wrapper *MockWrapperGenerator) {
	mf.wrappers[shell] = wrapper
}

func (mf *MockFactory) SetError(shell string, err error) {
	mf.errors[shell] = err
}

func (mf *MockFactory) CreateWrapper(shell string) (WrapperGenerator, error) {
	if err, exists := mf.errors[shell]; exists {
		return nil, err
	}

	if wrapper, exists := mf.wrappers[shell]; exists {
		return wrapper, nil
	}

	return NewMockWrapperGenerator(shell), nil
}

func TestInstallationManager_Install(t *testing.T) {
	tests := []struct {
		name        string
		opts        InstallOptions
		setupMocks  func(*MockFileSystem, *MockFactory)
		expectError bool
		errorType   error
	}{
		{
			name: "successful installation",
			opts: InstallOptions{
				Shell:          "zsh",
				Force:          false,
				BackupOriginal: false,
			},
			setupMocks: func(mfs *MockFileSystem, mf *MockFactory) {
				wrapper := NewMockWrapperGenerator("zsh")
				wrapper.installed = false
				mf.AddWrapper("zsh", wrapper)
			},
			expectError: false,
		},
		{
			name: "installation with backup",
			opts: InstallOptions{
				Shell:          "bash",
				Force:          false,
				BackupOriginal: true,
			},
			setupMocks: func(mfs *MockFileSystem, mf *MockFactory) {
				wrapper := NewMockWrapperGenerator("bash")
				wrapper.installed = false
				mf.AddWrapper("bash", wrapper)
				// Add existing file to backup
				mfs.AddFile("/mock/path/bash", []byte("existing content"))
			},
			expectError: false,
		},
		{
			name: "already installed without force",
			opts: InstallOptions{
				Shell:          "zsh",
				Force:          false,
				BackupOriginal: false,
			},
			setupMocks: func(mfs *MockFileSystem, mf *MockFactory) {
				wrapper := NewMockWrapperGenerator("zsh")
				wrapper.installed = true
				mf.AddWrapper("zsh", wrapper)
			},
			expectError: true,
			errorType:   ErrAlreadyInstalled,
		},
		{
			name: "force installation when already installed",
			opts: InstallOptions{
				Shell:          "zsh",
				Force:          true,
				BackupOriginal: false,
			},
			setupMocks: func(mfs *MockFileSystem, mf *MockFactory) {
				wrapper := NewMockWrapperGenerator("zsh")
				wrapper.installed = true
				mf.AddWrapper("zsh", wrapper)
			},
			expectError: false,
		},
		{
			name: "unsupported shell",
			opts: InstallOptions{
				Shell:          "unsupported",
				Force:          false,
				BackupOriginal: false,
			},
			setupMocks: func(mfs *MockFileSystem, mf *MockFactory) {
				mf.SetError("unsupported", ErrShellNotSupported)
			},
			expectError: true,
			errorType:   ErrShellNotSupported,
		},
		{
			name: "installation failure with rollback",
			opts: InstallOptions{
				Shell:          "fish",
				Force:          false,
				BackupOriginal: true,
			},
			setupMocks: func(mfs *MockFileSystem, mf *MockFactory) {
				wrapper := NewMockWrapperGenerator("fish")
				wrapper.installed = false
				wrapper.installError = errors.New("install failed")
				mf.AddWrapper("fish", wrapper)
				// Add existing file to backup
				mfs.AddFile("/mock/path/fish", []byte("existing content"))
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mfs := NewMockFileSystem()
			mf := NewMockFactory()
			tt.setupMocks(mfs, mf)

			im := NewInstallationManagerWithFactoryAndFS(mf, mfs)

			err := im.Install(tt.opts)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
					return
				}
				if tt.errorType != nil {
					var wrapperErr *WrapperError
					if errors.As(err, &wrapperErr) {
						if !errors.Is(wrapperErr.Err, tt.errorType) {
							t.Errorf("expected error type %v, got %v", tt.errorType, wrapperErr.Err)
						}
					} else if !errors.Is(err, tt.errorType) {
						t.Errorf("expected error type %v, got %v", tt.errorType, err)
					}
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestInstallationManager_Uninstall(t *testing.T) {
	tests := []struct {
		name        string
		opts        UninstallOptions
		setupMocks  func(*MockFileSystem, *MockFactory)
		expectError bool
		errorType   error
	}{
		{
			name: "successful uninstallation",
			opts: UninstallOptions{
				Shell:           "zsh",
				RemoveBackups:   false,
				RestoreOriginal: false,
			},
			setupMocks: func(mfs *MockFileSystem, mf *MockFactory) {
				wrapper := NewMockWrapperGenerator("zsh")
				wrapper.installed = true
				mf.AddWrapper("zsh", wrapper)
			},
			expectError: false,
		},
		{
			name: "uninstall with backup removal",
			opts: UninstallOptions{
				Shell:           "bash",
				RemoveBackups:   true,
				RestoreOriginal: false,
			},
			setupMocks: func(mfs *MockFileSystem, mf *MockFactory) {
				wrapper := NewMockWrapperGenerator("bash")
				wrapper.installed = true
				mf.AddWrapper("bash", wrapper)
			},
			expectError: false,
		},
		{
			name: "uninstall with restore original",
			opts: UninstallOptions{
				Shell:           "fish",
				RemoveBackups:   false,
				RestoreOriginal: true,
			},
			setupMocks: func(mfs *MockFileSystem, mf *MockFactory) {
				wrapper := NewMockWrapperGenerator("fish")
				wrapper.installed = true
				mf.AddWrapper("fish", wrapper)
				// Add backup file
				backupPath := "/mock/path/fish.awsm-backup-20240101-120000"
				mfs.AddFile(backupPath, []byte("original content"))
			},
			expectError: false,
		},
		{
			name: "not installed",
			opts: UninstallOptions{
				Shell:           "zsh",
				RemoveBackups:   false,
				RestoreOriginal: false,
			},
			setupMocks: func(mfs *MockFileSystem, mf *MockFactory) {
				wrapper := NewMockWrapperGenerator("zsh")
				wrapper.installed = false
				mf.AddWrapper("zsh", wrapper)
			},
			expectError: true,
			errorType:   ErrNotInstalled,
		},
		{
			name: "unsupported shell",
			opts: UninstallOptions{
				Shell:           "unsupported",
				RemoveBackups:   false,
				RestoreOriginal: false,
			},
			setupMocks: func(mfs *MockFileSystem, mf *MockFactory) {
				mf.SetError("unsupported", ErrShellNotSupported)
			},
			expectError: true,
			errorType:   ErrShellNotSupported,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mfs := NewMockFileSystem()
			mf := NewMockFactory()
			tt.setupMocks(mfs, mf)

			im := NewInstallationManagerWithFactoryAndFS(mf, mfs)

			err := im.Uninstall(tt.opts)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
					return
				}
				if tt.errorType != nil {
					var wrapperErr *WrapperError
					if errors.As(err, &wrapperErr) {
						if !errors.Is(wrapperErr.Err, tt.errorType) {
							t.Errorf("expected error type %v, got %v", tt.errorType, wrapperErr.Err)
						}
					} else if !errors.Is(err, tt.errorType) {
						t.Errorf("expected error type %v, got %v", tt.errorType, err)
					}
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestInstallationManager_GetStatus(t *testing.T) {
	mfs := NewMockFileSystem()
	mf := NewMockFactory()

	// Setup mock wrappers for all supported shells
	shells := []string{"zsh", "bash", "fish", "powershell"}
	for i, shell := range shells {
		wrapper := NewMockWrapperGenerator(shell)
		wrapper.installed = i%2 == 0 // Alternate installed status
		mf.AddWrapper(shell, wrapper)
	}

	im := NewInstallationManagerWithFactoryAndFS(mf, mfs)

	status := im.GetStatus()

	if len(status.Shells) != len(shells) {
		t.Errorf("expected %d shells, got %d", len(shells), len(status.Shells))
	}

	for i, shellStatus := range status.Shells {
		expectedInstalled := i%2 == 0
		if shellStatus.Installed != expectedInstalled {
			t.Errorf("shell %s: expected installed=%v, got %v", shellStatus.Shell, expectedInstalled, shellStatus.Installed)
		}
		if shellStatus.Error != "" {
			t.Errorf("shell %s: unexpected error: %s", shellStatus.Shell, shellStatus.Error)
		}
	}
}

func TestInstallationManager_GetShellStatus(t *testing.T) {
	tests := []struct {
		name           string
		shell          string
		setupMocks     func(*MockFileSystem, *MockFactory)
		expectedStatus InstallationStatus
	}{
		{
			name:  "installed shell",
			shell: "zsh",
			setupMocks: func(mfs *MockFileSystem, mf *MockFactory) {
				wrapper := NewMockWrapperGenerator("zsh")
				wrapper.installed = true
				mf.AddWrapper("zsh", wrapper)
			},
			expectedStatus: InstallationStatus{
				Shell:     "zsh",
				Installed: true,
				Path:      "/mock/path/zsh",
				Error:     "",
			},
		},
		{
			name:  "not installed shell",
			shell: "bash",
			setupMocks: func(mfs *MockFileSystem, mf *MockFactory) {
				wrapper := NewMockWrapperGenerator("bash")
				wrapper.installed = false
				mf.AddWrapper("bash", wrapper)
			},
			expectedStatus: InstallationStatus{
				Shell:     "bash",
				Installed: false,
				Path:      "/mock/path/bash",
				Error:     "",
			},
		},
		{
			name:  "error checking status",
			shell: "fish",
			setupMocks: func(mfs *MockFileSystem, mf *MockFactory) {
				wrapper := NewMockWrapperGenerator("fish")
				wrapper.isInstalledError = errors.New("check failed")
				mf.AddWrapper("fish", wrapper)
			},
			expectedStatus: InstallationStatus{
				Shell:     "fish",
				Installed: false,
				Path:      "/mock/path/fish",
				Error:     "failed to check installation status: check failed",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mfs := NewMockFileSystem()
			mf := NewMockFactory()
			tt.setupMocks(mfs, mf)

			im := NewInstallationManagerWithFactoryAndFS(mf, mfs)

			status := im.GetShellStatus(tt.shell)

			if status.Shell != tt.expectedStatus.Shell {
				t.Errorf("expected shell %s, got %s", tt.expectedStatus.Shell, status.Shell)
			}
			if status.Installed != tt.expectedStatus.Installed {
				t.Errorf("expected installed %v, got %v", tt.expectedStatus.Installed, status.Installed)
			}
			if status.Path != tt.expectedStatus.Path {
				t.Errorf("expected path %s, got %s", tt.expectedStatus.Path, status.Path)
			}
			if !strings.Contains(status.Error, tt.expectedStatus.Error) {
				t.Errorf("expected error to contain %s, got %s", tt.expectedStatus.Error, status.Error)
			}
		})
	}
}

func TestInstallationManager_BackupAndRestore(t *testing.T) {
	mfs := NewMockFileSystem()
	mf := NewMockFactory()

	wrapper := NewMockWrapperGenerator("zsh")
	wrapper.installed = false
	mf.AddWrapper("zsh", wrapper)

	// Add existing file
	originalContent := []byte("original wrapper content")
	mfs.AddFile("/mock/path/zsh", originalContent)

	im := NewInstallationManagerWithFactoryAndFS(mf, mfs)

	// Test installation with backup
	opts := InstallOptions{
		Shell:          "zsh",
		Force:          false,
		BackupOriginal: true,
	}

	err := im.Install(opts)
	if err != nil {
		t.Fatalf("installation failed: %v", err)
	}

	// Verify backup was created (we can't easily test the exact timestamp)
	backupFound := false
	for path := range mfs.files {
		if strings.Contains(path, ".awsm-backup-") {
			backupFound = true
			break
		}
	}
	if !backupFound {
		t.Error("backup file was not created")
	}

	// Test restore during uninstall
	uninstallOpts := UninstallOptions{
		Shell:           "zsh",
		RemoveBackups:   true,
		RestoreOriginal: true,
	}

	err = im.Uninstall(uninstallOpts)
	if err != nil {
		t.Fatalf("uninstallation failed: %v", err)
	}

	// Verify backup was removed
	backupFound = false
	for path := range mfs.files {
		if strings.Contains(path, ".awsm-backup-") {
			backupFound = true
			break
		}
	}
	if backupFound {
		t.Error("backup file was not removed")
	}
}
