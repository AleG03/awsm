package daemon

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const taskName = "awsm-refresh"

// Install registers a scheduled task.
//
// /SC MINUTE with /MO is the only sub-hourly repetition schtasks exposes
// directly, so the interval is rounded up to whole minutes; /RL LIMITED keeps
// the task at the user's own privileges, which is all it needs.
func Install(cfg InstallConfig) error {
	exe, err := executablePath()
	if err != nil {
		return err
	}

	minutes := int(cfg.Interval.Minutes())
	if minutes < 1 {
		minutes = 1
	}

	command := `"` + exe + `"`
	for _, a := range tickArgs(cfg.AutoLogin) {
		command += " " + a
	}

	out, err := exec.Command("schtasks", "/Create",
		"/TN", taskName,
		"/TR", command,
		"/SC", "MINUTE",
		"/MO", fmt.Sprint(minutes),
		"/RL", "LIMITED",
		"/F",
	).CombinedOutput()
	if err != nil {
		return fmt.Errorf("schtasks /Create failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func Uninstall() error {
	out, err := exec.Command("schtasks", "/Delete", "/TN", taskName, "/F").CombinedOutput()
	if err != nil {
		text := strings.TrimSpace(string(out))
		// Deleting a task that is not there is the desired end state.
		if strings.Contains(strings.ToLower(text), "cannot find") {
			return nil
		}
		return fmt.Errorf("schtasks /Delete failed: %w: %s", err, text)
	}
	return nil
}

func Status() (ServiceStatus, error) {
	status := ServiceStatus{UnitPath: `Task Scheduler\` + taskName}

	out, err := exec.Command("schtasks", "/Query", "/TN", taskName, "/FO", "LIST").CombinedOutput()
	if err != nil {
		return status, nil
	}
	status.Installed = true
	text := string(out)
	status.Loaded = !strings.Contains(text, "Disabled")
	status.Interval = intervalFromTask(text)
	return status, nil
}

// intervalFromTask is best effort: schtasks reports the repetition in a
// locale-dependent form, so status falls back to reporting nothing rather than
// guessing wrong.
func intervalFromTask(text string) time.Duration {
	for _, line := range strings.Split(text, "\n") {
		if !strings.Contains(strings.ToLower(line), "repeat: every") {
			continue
		}
		var hours, minutes int
		if _, err := fmt.Sscanf(strings.TrimSpace(line), "Repeat: Every: %d:%d", &hours, &minutes); err == nil {
			return time.Duration(hours)*time.Hour + time.Duration(minutes)*time.Minute
		}
	}
	return 0
}
