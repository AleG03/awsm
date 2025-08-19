/*
AWSM - AWS Manager
Copyright (c) 2024 Alessandro Gallo. All rights reserved.

Licensed under the Business Source License 1.1.
See LICENSE file for full terms.
*/

package cmd

import (
	"awsm/internal/util"
	"awsm/internal/wrapper"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var (
	uninstallFlag bool
	statusFlag    bool
	forceFlag     bool
	jsonOutput    bool
	allShells     bool
)

var wrapperCmd = &cobra.Command{
	Use:   "wrapper [shell]",
	Short: "Install AWS CLI wrapper for automatic credential refresh",
	Long: `Install a shell wrapper function that automatically refreshes expired AWS credentials
before executing AWS CLI commands. The wrapper intercepts 'aws' commands and checks
credential validity using 'aws sts get-caller-identity'. If credentials are expired,
it automatically runs 'awsm refresh' before executing the original command.

Supported shells: zsh, bash, fish, powershell

Examples:
  # Install wrapper for current shell (auto-detected)
  awsm wrapper

  # Install wrapper for specific shell
  awsm wrapper zsh
  awsm wrapper bash
  awsm wrapper fish
  awsm wrapper powershell

  # Show installation status for all shells
  awsm wrapper --status

  # Uninstall wrapper from specific shell
  awsm wrapper --uninstall zsh

  # Force reinstall (overwrite existing installation)
  awsm wrapper --force zsh

  # Install for all supported shells
  awsm wrapper --all`,
	RunE: runWrapperCommand,
}

func runWrapperCommand(cmd *cobra.Command, args []string) error {
	manager := wrapper.NewInstallationManager()

	// Handle status flag
	if statusFlag {
		return handleStatusCommand(manager)
	}

	// Handle uninstall flag
	if uninstallFlag {
		return handleUninstallCommand(manager, args)
	}

	// Handle install command
	return handleInstallCommand(manager, args)
}

func handleStatusCommand(manager *wrapper.InstallationManager) error {
	status := manager.GetStatus()

	if jsonOutput {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(status)
	}

	// Print formatted status
	fmt.Println()
	util.InfoColor.Println("AWS CLI Wrapper Installation Status")
	util.InfoColor.Println("═══════════════════════════════════")
	fmt.Println()

	for _, shellStatus := range status.Shells {
		fmt.Printf("Shell: %s\n", shellStatus.Shell)
		if shellStatus.Error != "" {
			fmt.Printf("  Status: %s\n", util.ErrorColor.Sprint("Error"))
			fmt.Printf("  Error:  %s\n", shellStatus.Error)
		} else if shellStatus.Installed {
			fmt.Printf("  Status: %s\n", util.SuccessColor.Sprint("Installed"))
			fmt.Printf("  Path:   %s\n", shellStatus.Path)
		} else {
			fmt.Printf("  Status: %s\n", util.WarnColor.Sprint("Not Installed"))
			fmt.Printf("  Path:   %s\n", shellStatus.Path)
		}
		fmt.Println()
	}

	return nil
}

func handleUninstallCommand(manager *wrapper.InstallationManager, args []string) error {
	if allShells {
		return uninstallAllShells(manager)
	}

	shell, err := determineShell(args)
	if err != nil {
		return err
	}

	opts := wrapper.UninstallOptions{
		Shell:           shell,
		RemoveBackups:   true,
		RestoreOriginal: false,
	}

	if err := manager.Uninstall(opts); err != nil {
		return handleWrapperError(err, "uninstallation failed")
	}

	// Print success message with reload instructions
	printUninstallationSuccess(shell)
	return nil
}

func handleInstallCommand(manager *wrapper.InstallationManager, args []string) error {
	if allShells {
		return installAllShells(manager)
	}

	shell, err := determineShell(args)
	if err != nil {
		return err
	}

	// Validate installation environment before proceeding
	if err := wrapper.ValidateInstallationEnvironment(shell); err != nil {
		return handleWrapperError(err, "installation validation failed")
	}

	opts := wrapper.InstallOptions{
		Shell:          shell,
		Force:          forceFlag,
		BackupOriginal: true,
	}

	if err := manager.Install(opts); err != nil {
		return handleWrapperError(err, "installation failed")
	}

	// Print success message with detailed instructions
	printInstallationSuccess(shell)
	return nil
}

func installAllShells(manager *wrapper.InstallationManager) error {
	var errors []string
	var installed []string

	for _, shell := range wrapper.GetSupportedShells() {
		shellName := string(shell)
		opts := wrapper.InstallOptions{
			Shell:          shellName,
			Force:          forceFlag,
			BackupOriginal: true,
		}

		if err := manager.Install(opts); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", shellName, err))
		} else {
			installed = append(installed, shellName)
		}
	}

	// Report results
	if len(installed) > 0 {
		util.SuccessColor.Printf("✓ Successfully installed wrapper for: %s\n", strings.Join(installed, ", "))
	}

	if len(errors) > 0 {
		fmt.Println()
		util.WarnColor.Println("Some installations failed:")
		for _, err := range errors {
			fmt.Printf("  • %s\n", err)
		}
	}

	if len(installed) > 0 {
		fmt.Println()
		fmt.Println("Please restart your shells or source their configuration files to apply changes.")
	}

	return nil
}

func uninstallAllShells(manager *wrapper.InstallationManager) error {
	var errors []string
	var uninstalled []string

	for _, shell := range wrapper.GetSupportedShells() {
		shellName := string(shell)

		// Check if installed first
		status := manager.GetShellStatus(shellName)
		if !status.Installed {
			continue
		}

		opts := wrapper.UninstallOptions{
			Shell:           shellName,
			RemoveBackups:   true,
			RestoreOriginal: false,
		}

		if err := manager.Uninstall(opts); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", shellName, err))
		} else {
			uninstalled = append(uninstalled, shellName)
		}
	}

	// Report results
	if len(uninstalled) > 0 {
		util.SuccessColor.Printf("✓ Successfully uninstalled wrapper from: %s\n", strings.Join(uninstalled, ", "))
	}

	if len(errors) > 0 {
		fmt.Println()
		util.WarnColor.Println("Some uninstallations failed:")
		for _, err := range errors {
			fmt.Printf("  • %s\n", err)
		}
	}

	if len(uninstalled) == 0 && len(errors) == 0 {
		util.InfoColor.Println("No wrapper installations found to remove.")
	}

	return nil
}

func determineShell(args []string) (string, error) {
	// If shell specified as argument, use it
	if len(args) > 0 {
		shell := strings.ToLower(args[0])
		if err := wrapper.ValidateShell(shell); err != nil {
			// Create a wrapper error with helpful suggestions
			wrapperErr := wrapper.NewWrapperError(shell, "validate", err, "invalid shell specified")
			return "", wrapperErr
		}
		return shell, nil
	}

	// Auto-detect shell
	factory := wrapper.NewFactory()
	shell, err := factory.DetectShell()
	if err != nil {
		// Create a wrapper error for shell detection failure
		detectionErr := wrapper.NewWrapperError("unknown", "detect", wrapper.ErrShellDetectionFailed, "automatic shell detection failed")
		return "", detectionErr
	}

	return shell, nil
}

func getSupportedShellNames() []string {
	shells := wrapper.GetSupportedShells()
	names := make([]string, len(shells))
	for i, shell := range shells {
		names[i] = string(shell)
	}
	return names
}

func getShellConfigHint(shell string) string {
	switch shell {
	case "zsh":
		return "~/.zshrc"
	case "bash":
		return "~/.bashrc"
	case "fish":
		return "~/.config/fish/config.fish"
	case "powershell":
		return "$PROFILE"
	default:
		return "your shell configuration file"
	}
}

// handleWrapperError provides enhanced error handling with user-friendly messages
func handleWrapperError(err error, context string) error {
	if wrapperErr, ok := err.(*wrapper.WrapperError); ok {
		// Print user-friendly error message
		util.ErrorColor.Printf("✗ %s\n", context)
		fmt.Println()
		fmt.Println(wrapperErr.GetUserFriendlyMessage())
		fmt.Println()

		// Add general troubleshooting footer
		fmt.Println("For additional help:")
		fmt.Println("  • Check the awsm documentation")
		fmt.Println("  • Run 'awsm wrapper --status' to check current installation status")
		fmt.Println("  • Use 'awsm wrapper --help' for usage information")

		return fmt.Errorf("%s", context)
	}

	// Fallback for non-wrapper errors
	return fmt.Errorf("%s: %w", context, err)
}

// printInstallationSuccess prints a comprehensive success message with instructions
func printInstallationSuccess(shell string) {
	util.SuccessColor.Printf("✓ Successfully installed AWS CLI wrapper for %s\n", shell)
	fmt.Println()

	// Print what the wrapper does
	util.InfoColor.Println("What the wrapper does:")
	fmt.Println("  • Intercepts all 'aws' commands")
	fmt.Println("  • Checks credential validity before execution")
	fmt.Println("  • Automatically refreshes expired credentials using 'awsm refresh'")
	fmt.Println("  • Passes through all arguments transparently")
	fmt.Println("  • Adds minimal overhead (< 200ms when credentials are valid)")
	fmt.Println()

	// Shell-specific activation instructions
	util.InfoColor.Println("To activate the wrapper:")
	switch shell {
	case "zsh":
		fmt.Printf("  %s\n", util.BoldColor.Sprint("source ~/.zshrc"))
		fmt.Println("  or restart your terminal")
		fmt.Println()
		fmt.Println("Installation details:")
		fmt.Println("  • Function installed to: ~/.zsh/functions/aws")
		fmt.Println("  • Function directory added to fpath in ~/.zshrc")
		fmt.Println("  • Zsh will autoload the function when 'aws' is called")

	case "bash":
		fmt.Printf("  %s\n", util.BoldColor.Sprint("source ~/.bashrc"))
		fmt.Println("  or restart your terminal")
		fmt.Println()
		fmt.Println("Installation details:")
		fmt.Println("  • Function added to: ~/.bashrc")
		fmt.Println("  • Function will be available in new bash sessions")

	case "fish":
		fmt.Printf("  %s\n", util.BoldColor.Sprint("exec fish"))
		fmt.Println("  or restart your terminal")
		fmt.Println()
		fmt.Println("Installation details:")
		fmt.Println("  • Function installed to: ~/.config/fish/functions/aws.fish")
		fmt.Println("  • Fish will automatically load the function when needed")

	case "powershell":
		fmt.Printf("  %s\n", util.BoldColor.Sprint(". $PROFILE"))
		fmt.Println("  or restart PowerShell")
		fmt.Println()
		fmt.Println("Installation details:")
		fmt.Println("  • Function added to your PowerShell profile")
		fmt.Println("  • Function will be available in new PowerShell sessions")
	}

	fmt.Println()
	util.InfoColor.Println("Usage:")
	fmt.Println("  • Use 'aws' commands normally - the wrapper is completely transparent")
	fmt.Println("  • Example: aws s3 ls (will auto-refresh credentials if needed)")
	fmt.Println("  • To bypass the wrapper temporarily: command aws <args>")
	fmt.Println()

	util.InfoColor.Println("Management commands:")
	fmt.Printf("  • Check status: %s\n", util.BoldColor.Sprint("awsm wrapper --status"))
	fmt.Printf("  • Uninstall: %s\n", util.BoldColor.Sprint("awsm wrapper --uninstall "+shell))
	fmt.Printf("  • Reinstall: %s\n", util.BoldColor.Sprint("awsm wrapper --force "+shell))
	fmt.Println()

	// Add verification step
	util.InfoColor.Println("Verification:")
	fmt.Println("After reloading your shell, test the wrapper with:")
	fmt.Printf("  %s\n", util.BoldColor.Sprint("aws sts get-caller-identity"))
	fmt.Println("The wrapper should automatically refresh credentials if needed.")
}

// printUninstallationSuccess prints a success message for uninstallation
func printUninstallationSuccess(shell string) {
	util.SuccessColor.Printf("✓ Successfully uninstalled AWS CLI wrapper for %s\n", shell)
	fmt.Println()

	util.InfoColor.Println("To complete the removal:")
	switch shell {
	case "zsh":
		fmt.Printf("  %s\n", util.BoldColor.Sprint("source ~/.zshrc"))
		fmt.Println("  or restart your terminal")
		fmt.Println()
		fmt.Println("What was removed:")
		fmt.Println("  • Function file: ~/.zsh/functions/aws")
		fmt.Println("  • Note: fpath entry remains in ~/.zshrc for other functions")

	case "bash":
		fmt.Printf("  %s\n", util.BoldColor.Sprint("source ~/.bashrc"))
		fmt.Println("  or restart your terminal")
		fmt.Println()
		fmt.Println("What was removed:")
		fmt.Println("  • Wrapper function from ~/.bashrc")

	case "fish":
		fmt.Printf("  %s\n", util.BoldColor.Sprint("exec fish"))
		fmt.Println("  or restart your terminal")
		fmt.Println()
		fmt.Println("What was removed:")
		fmt.Println("  • Function file: ~/.config/fish/functions/aws.fish")

	case "powershell":
		fmt.Printf("  %s\n", util.BoldColor.Sprint(". $PROFILE"))
		fmt.Println("  or restart PowerShell")
		fmt.Println()
		fmt.Println("What was removed:")
		fmt.Println("  • Wrapper function from PowerShell profile")
	}

	fmt.Println()
	fmt.Println("The original AWS CLI will now work normally without automatic credential refresh.")
	fmt.Printf("To reinstall the wrapper: %s\n", util.BoldColor.Sprint("awsm wrapper "+shell))
}

func printInstallationInstructions(shell string) error {
	// This function is kept for backward compatibility but now calls the enhanced version
	printInstallationSuccess(shell)
	return nil
}

func init() {
	wrapperCmd.Flags().BoolVar(&uninstallFlag, "uninstall", false, "Uninstall the wrapper from specified shell")
	wrapperCmd.Flags().BoolVar(&statusFlag, "status", false, "Show installation status for all shells")
	wrapperCmd.Flags().BoolVar(&forceFlag, "force", false, "Force installation even if wrapper already exists")
	wrapperCmd.Flags().BoolVarP(&jsonOutput, "json", "j", false, "Output status in JSON format (only with --status)")
	wrapperCmd.Flags().BoolVar(&allShells, "all", false, "Install/uninstall wrapper for all supported shells")

	// Add mutual exclusivity validation
	wrapperCmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		// Count active flags
		flagCount := 0
		if uninstallFlag {
			flagCount++
		}
		if statusFlag {
			flagCount++
		}
		if flagCount > 1 {
			return fmt.Errorf("cannot use --uninstall and --status flags together")
		}

		// JSON output only makes sense with status
		if jsonOutput && !statusFlag {
			return fmt.Errorf("--json flag can only be used with --status")
		}

		// Validate shell argument if provided
		if len(args) > 1 {
			return fmt.Errorf("too many arguments - expected at most one shell name")
		}

		// If --all is used with a shell argument, that's an error
		if allShells && len(args) > 0 {
			return fmt.Errorf("cannot specify shell when using --all flag")
		}

		return nil
	}

	rootCmd.AddCommand(wrapperCmd)
}
