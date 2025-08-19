package wrapper

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// FileSystem interface for file operations (allows mocking in tests)
type FileSystem interface {
	Stat(name string) (os.FileInfo, error)
	MkdirAll(path string, perm os.FileMode) error
	ReadFile(filename string) ([]byte, error)
	WriteFile(filename string, data []byte, perm os.FileMode) error
	Remove(name string) error
	Rename(oldpath, newpath string) error
	Create(name string) (*os.File, error)
	Open(name string) (*os.File, error)
}

// OSFileSystem implements FileSystem using standard os package
type OSFileSystem struct{}

func (fs *OSFileSystem) Stat(name string) (os.FileInfo, error) {
	return os.Stat(name)
}

func (fs *OSFileSystem) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

func (fs *OSFileSystem) ReadFile(filename string) ([]byte, error) {
	return os.ReadFile(filename)
}

func (fs *OSFileSystem) WriteFile(filename string, data []byte, perm os.FileMode) error {
	return os.WriteFile(filename, data, perm)
}

func (fs *OSFileSystem) Remove(name string) error {
	return os.Remove(name)
}

func (fs *OSFileSystem) Rename(oldpath, newpath string) error {
	return os.Rename(oldpath, newpath)
}

func (fs *OSFileSystem) Create(name string) (*os.File, error) {
	return os.Create(name)
}

func (fs *OSFileSystem) Open(name string) (*os.File, error) {
	return os.Open(name)
}

// InstallationManager coordinates wrapper installation across shells
type InstallationManager struct {
	factory WrapperFactory
	fs      FileSystem
}

// NewInstallationManager creates a new InstallationManager
func NewInstallationManager() *InstallationManager {
	return &InstallationManager{
		factory: NewFactory(),
		fs:      &OSFileSystem{},
	}
}

// NewInstallationManagerWithFS creates a new InstallationManager with custom FileSystem
func NewInstallationManagerWithFS(fs FileSystem) *InstallationManager {
	return &InstallationManager{
		factory: NewFactory(),
		fs:      fs,
	}
}

// NewInstallationManagerWithFactoryAndFS creates a new InstallationManager with custom factory and filesystem
func NewInstallationManagerWithFactoryAndFS(factory WrapperFactory, fs FileSystem) *InstallationManager {
	return &InstallationManager{
		factory: factory,
		fs:      fs,
	}
}

// InstallOptions contains options for wrapper installation
type InstallOptions struct {
	Shell          string
	Force          bool // Force installation even if already installed
	BackupOriginal bool // Create backup of original files
}

// UninstallOptions contains options for wrapper uninstallation
type UninstallOptions struct {
	Shell           string
	RemoveBackups   bool // Remove backup files during uninstallation
	RestoreOriginal bool // Restore original files from backup
}

// Install installs the wrapper for the specified shell
func (im *InstallationManager) Install(opts InstallOptions) error {
	wrapper, err := im.factory.CreateWrapper(opts.Shell)
	if err != nil {
		return NewWrapperError(opts.Shell, "install", err, "failed to create wrapper")
	}

	// Check if already installed
	installed, err := wrapper.IsInstalled()
	if err != nil {
		return NewWrapperError(opts.Shell, "install", err, "failed to check installation status")
	}

	if installed {
		if !opts.Force {
			return NewWrapperError(opts.Shell, "install", ErrAlreadyInstalled, "wrapper is already installed")
		}

		// Force installation: remove existing installation first
		if err := wrapper.Uninstall(); err != nil {
			return NewWrapperError(opts.Shell, "install", err, "failed to remove existing installation for force reinstall")
		}
	}

	// Validate installation path accessibility
	installPath, err := wrapper.GetInstallPath()
	if err != nil {
		return NewWrapperError(opts.Shell, "install", err, "failed to get install path")
	}

	// Ensure installation directory exists and is writable
	installDir := filepath.Dir(installPath)
	if err := im.ensureDirectoryWithValidation(installDir, opts.Shell); err != nil {
		return err // Already wrapped with proper error context
	}

	// Create backup if requested
	if opts.BackupOriginal {
		if err := im.createBackupWithValidation(wrapper, opts.Shell); err != nil {
			return err // Already wrapped with proper error context
		}
	}

	// Generate and install wrapper function
	functionContent := wrapper.GenerateFunction()
	if err := wrapper.Install(functionContent); err != nil {
		// Attempt rollback if installation fails
		if opts.BackupOriginal {
			if rollbackErr := im.rollback(wrapper, opts.Shell); rollbackErr != nil {
				// Log rollback failure but return original error
				fmt.Fprintf(os.Stderr, "Warning: Failed to rollback after installation failure: %v\n", rollbackErr)
			}
		}
		return NewWrapperError(opts.Shell, "install", err, "failed to install wrapper function")
	}

	return nil
}

// Uninstall removes the wrapper for the specified shell
func (im *InstallationManager) Uninstall(opts UninstallOptions) error {
	wrapper, err := im.factory.CreateWrapper(opts.Shell)
	if err != nil {
		return NewWrapperError(opts.Shell, "uninstall", err, "failed to create wrapper")
	}

	// Check if installed
	installed, err := wrapper.IsInstalled()
	if err != nil {
		return NewWrapperError(opts.Shell, "uninstall", err, "failed to check installation status")
	}
	if !installed {
		return NewWrapperError(opts.Shell, "uninstall", ErrNotInstalled, "wrapper is not installed")
	}

	// Restore original files if requested
	if opts.RestoreOriginal {
		if err := im.restoreBackup(wrapper, opts.Shell); err != nil {
			return NewWrapperError(opts.Shell, "uninstall", err, "failed to restore backup")
		}
	} else {
		// Standard uninstall
		if err := wrapper.Uninstall(); err != nil {
			return NewWrapperError(opts.Shell, "uninstall", err, "failed to uninstall wrapper")
		}
	}

	// Remove backup files if requested
	if opts.RemoveBackups {
		im.removeBackups(wrapper, opts.Shell)
	}

	return nil
}

// GetStatus returns the installation status for all supported shells
func (im *InstallationManager) GetStatus() *WrapperStatus {
	status := &WrapperStatus{
		Shells: make([]InstallationStatus, 0, len(GetSupportedShells())),
	}

	for _, shell := range GetSupportedShells() {
		shellStatus := im.getShellStatus(string(shell))
		status.Shells = append(status.Shells, shellStatus)
	}

	return status
}

// GetShellStatus returns the installation status for a specific shell
func (im *InstallationManager) GetShellStatus(shell string) InstallationStatus {
	return im.getShellStatus(shell)
}

// getShellStatus returns the installation status for a specific shell
func (im *InstallationManager) getShellStatus(shell string) InstallationStatus {
	status := InstallationStatus{
		Shell: shell,
	}

	wrapper, err := im.factory.CreateWrapper(shell)
	if err != nil {
		status.Error = fmt.Sprintf("failed to create wrapper: %v", err)
		return status
	}

	installPath, err := wrapper.GetInstallPath()
	if err != nil {
		status.Error = fmt.Sprintf("failed to get install path: %v", err)
		return status
	}
	status.Path = installPath

	installed, err := wrapper.IsInstalled()
	if err != nil {
		status.Error = fmt.Sprintf("failed to check installation status: %v", err)
		return status
	}
	status.Installed = installed

	return status
}

// createBackup creates a backup of files that will be modified during installation
func (im *InstallationManager) createBackup(wrapper WrapperGenerator, shell string) error {
	installPath, err := wrapper.GetInstallPath()
	if err != nil {
		return err
	}

	// Check if install path exists
	if _, err := im.fs.Stat(installPath); os.IsNotExist(err) {
		// No existing file to backup
		return nil
	} else if err != nil {
		return err
	}

	// Create backup with timestamp
	backupPath := im.getBackupPath(installPath)
	return im.copyFile(installPath, backupPath)
}

// rollback restores files from backup after a failed installation
func (im *InstallationManager) rollback(wrapper WrapperGenerator, shell string) error {
	installPath, err := wrapper.GetInstallPath()
	if err != nil {
		return err
	}

	backupPath := im.getBackupPath(installPath)
	if _, err := im.fs.Stat(backupPath); os.IsNotExist(err) {
		// No backup to restore
		return nil
	}

	return im.copyFile(backupPath, installPath)
}

// restoreBackup restores original files from backup
func (im *InstallationManager) restoreBackup(wrapper WrapperGenerator, shell string) error {
	installPath, err := wrapper.GetInstallPath()
	if err != nil {
		return err
	}

	backupPath := im.getBackupPath(installPath)
	if _, err := im.fs.Stat(backupPath); os.IsNotExist(err) {
		// No backup to restore, just uninstall normally
		return wrapper.Uninstall()
	}

	// Restore from backup
	if err := im.copyFile(backupPath, installPath); err != nil {
		return err
	}

	// Remove the backup file
	return im.fs.Remove(backupPath)
}

// removeBackups removes backup files for the specified shell
func (im *InstallationManager) removeBackups(wrapper WrapperGenerator, shell string) error {
	installPath, err := wrapper.GetInstallPath()
	if err != nil {
		return err
	}

	backupPath := im.getBackupPath(installPath)
	if _, err := im.fs.Stat(backupPath); os.IsNotExist(err) {
		// No backup file to remove
		return nil
	}

	return im.fs.Remove(backupPath)
}

// getBackupPath generates a backup file path with timestamp
func (im *InstallationManager) getBackupPath(originalPath string) string {
	timestamp := time.Now().Format("20060102-150405")
	return fmt.Sprintf("%s.awsm-backup-%s", originalPath, timestamp)
}

// ensureDirectory creates a directory and all necessary parent directories
func (im *InstallationManager) ensureDirectory(path string) error {
	if _, err := im.fs.Stat(path); os.IsNotExist(err) {
		return im.fs.MkdirAll(path, 0755)
	}
	return nil
}

// ensureDirectoryWithValidation creates a directory with comprehensive validation
func (im *InstallationManager) ensureDirectoryWithValidation(path, shell string) error {
	// Check if directory already exists
	if stat, err := im.fs.Stat(path); err == nil {
		// Directory exists, check if it's writable
		if !stat.IsDir() {
			return NewWrapperError(shell, "install", fmt.Errorf("path exists but is not a directory"),
				fmt.Sprintf("path %s exists but is not a directory", path))
		}

		// Test write permissions
		testFile := filepath.Join(path, ".awsm-write-test")
		if file, err := im.fs.Create(testFile); err != nil {
			return NewWrapperError(shell, "install", ErrDirectoryNotWritable,
				fmt.Sprintf("directory %s is not writable", path))
		} else {
			file.Close()
			im.fs.Remove(testFile)
		}

		return nil
	} else if !os.IsNotExist(err) {
		// Some other error occurred
		return NewWrapperError(shell, "install", err,
			fmt.Sprintf("failed to check directory status: %s", path))
	}

	// Directory doesn't exist, try to create it
	if err := im.fs.MkdirAll(path, 0755); err != nil {
		if os.IsPermission(err) {
			return NewWrapperError(shell, "install", ErrPermissionDenied,
				fmt.Sprintf("permission denied creating directory: %s", path))
		}
		return NewWrapperError(shell, "install", err,
			fmt.Sprintf("failed to create directory: %s", path))
	}

	return nil
}

// createBackupWithValidation creates a backup with enhanced error handling
func (im *InstallationManager) createBackupWithValidation(wrapper WrapperGenerator, shell string) error {
	installPath, err := wrapper.GetInstallPath()
	if err != nil {
		return NewWrapperError(shell, "install", err, "failed to get install path for backup")
	}

	// Check if install path exists
	if _, err := im.fs.Stat(installPath); os.IsNotExist(err) {
		// No existing file to backup
		return nil
	} else if err != nil {
		return NewWrapperError(shell, "install", err,
			fmt.Sprintf("failed to check file status for backup: %s", installPath))
	}

	// Create backup with timestamp
	backupPath := im.getBackupPath(installPath)

	// Ensure backup directory exists
	backupDir := filepath.Dir(backupPath)
	if err := im.ensureDirectory(backupDir); err != nil {
		return NewWrapperError(shell, "install", err, "failed to create backup directory")
	}

	if err := im.copyFile(installPath, backupPath); err != nil {
		return NewWrapperError(shell, "install", ErrBackupFailed,
			fmt.Sprintf("failed to create backup of %s", installPath))
	}

	return nil
}

// copyFile copies a file from src to dst
func (im *InstallationManager) copyFile(src, dst string) error {
	// Read source file
	data, err := im.fs.ReadFile(src)
	if err != nil {
		return err
	}

	// Write to destination
	return im.fs.WriteFile(dst, data, 0644)
}
