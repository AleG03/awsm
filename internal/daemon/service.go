package daemon

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// Label identifies the scheduled job to the operating system. Reverse-DNS
// because launchd requires it and the other two do not mind.
const Label = "io.github.aleg03.awsm.refresh"

// DefaultInterval is how often the job runs.
//
// One minute, because the threshold and not the interval is what protects the
// credentials: halving it to thirty seconds would move the retry budget from
// nine minutes to nine and a half while doubling the wake-ups. At roughly eight
// milliseconds a run this is about half a second of CPU per hour.
const DefaultInterval = time.Minute

// InstallConfig describes the job to register.
type InstallConfig struct {
	Interval  time.Duration
	AutoLogin bool
}

// ServiceStatus is what Status reports back.
type ServiceStatus struct {
	Installed bool
	Loaded    bool
	UnitPath  string
	Interval  time.Duration
	Details   string
}

// executablePath returns the awsm binary to schedule.
//
// The resolved path is stored rather than the name, so the job does not depend
// on what PATH looks like in a service manager's environment, which is not the
// shell's.
func executablePath() (string, error) {
	path, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("could not determine the awsm executable path: %w", err)
	}
	return path, nil
}

// AwsmCommand renders a command line that runs this very binary.
//
// Suggesting a bare "awsm" would resolve through PATH, which may hold a
// different build than the one raising the alert -- exactly the confusion that
// makes a fix appear not to work. The scheduled job already records the
// absolute path for the same reason.
func AwsmCommand(args ...string) string {
	exe, err := executablePath()
	if err != nil {
		exe = "awsm"
	}
	if strings.ContainsAny(exe, " \t") {
		exe = "'" + exe + "'"
	}
	return exe + " " + strings.Join(args, " ")
}

// tickArgs are the arguments the scheduled job runs with.
func tickArgs(autoLogin bool) []string {
	args := []string{"daemon", "tick"}
	if autoLogin {
		args = append(args, "--auto-login")
	}
	return args
}
