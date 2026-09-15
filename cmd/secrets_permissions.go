package cmd

import (
	"fmt"
	"os"

	"awsm/internal/aws"
	"awsm/internal/awsini"
	"awsm/internal/tui"

	"github.com/mattn/go-isatty"
)

// offerToRestrictConfig warns when ~/.aws/config is readable by more than its
// owner, and offers to tighten it.
//
// awsm now writes IAM user keys into the profile itself, and awsini
// deliberately preserves the mode of an existing file, so a config that was the
// conventional 0644 stays 0644 with secrets in it. Tightening silently would be
// the wrong call too: the mode is the user's, and something on their machine
// may rely on it.
//
// Never fails the command that called it. The keys are already written; a
// refused or impossible chmod is a warning, not a reason to report failure.
func offerToRestrictConfig() {
	configPath, err := aws.GetAWSConfigPath()
	if err != nil {
		return
	}

	permissive, mode, err := awsini.TooPermissive(configPath)
	if err != nil || !permissive {
		return
	}

	tui.PrintWarning(fmt.Sprintf(
		"%s now holds access keys but is mode %#o, so any user on this machine can read them.",
		configPath, mode))

	// Without a terminal, tui.Confirm falls back to reading stdin, which would
	// hang a script or a pipeline. Say the thing and move on.
	if !isatty.IsTerminal(os.Stdin.Fd()) {
		tui.PrintMuted(fmt.Sprintf("Restrict it with: chmod 600 %s", configPath))
		return
	}

	confirmed, err := tui.Confirm("Restrict it to your user only (chmod 600)?")
	if err != nil || !confirmed {
		tui.PrintMuted(fmt.Sprintf("Left as is. Restrict it later with: chmod 600 %s", configPath))
		return
	}

	if err := awsini.Restrict(configPath); err != nil {
		tui.PrintError(fmt.Sprintf("Could not change permissions: %v", err))
		return
	}
	tui.PrintSuccess(fmt.Sprintf("%s is now owner-only.", configPath))
}
