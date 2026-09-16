package daemon

import (
	"fmt"
	"os/exec"
)

// notify uses notify-send, present wherever a notification daemon is.
// A headless session has neither, so its absence is reported and ignored.
func notify(title, message string) error {
	path, err := exec.LookPath("notify-send")
	if err != nil {
		return fmt.Errorf("notify-send not found: %w", err)
	}
	return exec.Command(path, "--app-name=awsm", title, message).Run()
}
