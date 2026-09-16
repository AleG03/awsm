package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const unitName = "awsm-refresh"

func unitDir() (string, error) {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "systemd", "user"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "systemd", "user"), nil
}

// Install writes a service plus a timer.
//
// A user timer, not a system one: the credentials belong to this user's home
// directory, and a user unit also starts at login without any further
// arrangement.
func Install(cfg InstallConfig) error {
	exe, err := executablePath()
	if err != nil {
		return err
	}
	dir, err := unitDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("could not create %s: %w", dir, err)
	}

	command := exe
	for _, a := range tickArgs(cfg.AutoLogin) {
		command += " " + a
	}

	service := fmt.Sprintf(`[Unit]
Description=Renew the active AWS credentials

[Service]
Type=oneshot
ExecStart=%s
`, command)

	// OnUnitActiveSec measures from the end of the previous run, so a slow run
	// cannot pile up against the next one. Persistent catches up a run missed
	// while the machine was off, which is exactly when credentials go stale.
	timer := fmt.Sprintf(`[Unit]
Description=Renew the active AWS credentials periodically

[Timer]
OnStartupSec=30
OnUnitActiveSec=%d
AccuracySec=5s
Persistent=true

[Install]
WantedBy=timers.target
`, int(cfg.Interval.Seconds()))

	if err := os.WriteFile(filepath.Join(dir, unitName+".service"), []byte(service), 0644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, unitName+".timer"), []byte(timer), 0644); err != nil {
		return err
	}

	if out, err := exec.Command("systemctl", "--user", "daemon-reload").CombinedOutput(); err != nil {
		return fmt.Errorf("systemctl daemon-reload failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	if out, err := exec.Command("systemctl", "--user", "enable", "--now", unitName+".timer").CombinedOutput(); err != nil {
		return fmt.Errorf("could not enable the timer: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func Uninstall() error {
	dir, err := unitDir()
	if err != nil {
		return err
	}
	_ = exec.Command("systemctl", "--user", "disable", "--now", unitName+".timer").Run()
	for _, name := range []string{unitName + ".timer", unitName + ".service"} {
		if err := os.Remove(filepath.Join(dir, name)); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("could not remove %s: %w", name, err)
		}
	}
	_ = exec.Command("systemctl", "--user", "daemon-reload").Run()
	return nil
}

func Status() (ServiceStatus, error) {
	dir, err := unitDir()
	if err != nil {
		return ServiceStatus{}, err
	}
	path := filepath.Join(dir, unitName+".timer")
	status := ServiceStatus{UnitPath: path}

	if data, err := os.ReadFile(path); err == nil {
		status.Installed = true
		status.Interval = intervalFromTimer(string(data))
	}

	out, err := exec.Command("systemctl", "--user", "is-active", unitName+".timer").Output()
	status.Loaded = err == nil && strings.TrimSpace(string(out)) == "active"
	return status, nil
}

func intervalFromTimer(unit string) time.Duration {
	for _, line := range strings.Split(unit, "\n") {
		if !strings.HasPrefix(line, "OnUnitActiveSec=") {
			continue
		}
		var seconds int
		if _, err := fmt.Sscanf(strings.TrimPrefix(line, "OnUnitActiveSec="), "%d", &seconds); err == nil {
			return time.Duration(seconds) * time.Second
		}
	}
	return 0
}
