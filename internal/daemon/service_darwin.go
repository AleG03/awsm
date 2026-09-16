package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// plistPath is the LaunchAgent file. Living under ~/Library/LaunchAgents is
// what makes launchd load it at every login, which is the "start at login"
// half of the feature: no extra mechanism needed.
func plistPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents", Label+".plist"), nil
}

func Install(cfg InstallConfig) error {
	exe, err := executablePath()
	if err != nil {
		return err
	}
	path, err := plistPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("could not create %s: %w", filepath.Dir(path), err)
	}

	var args strings.Builder
	for _, a := range append([]string{exe}, tickArgs(cfg.AutoLogin)...) {
		fmt.Fprintf(&args, "\t\t<string>%s</string>\n", xmlEscape(a))
	}

	logPath, err := LogPath()
	if err != nil {
		return err
	}

	// RunAtLoad makes the first cycle happen at login rather than one interval
	// later, which matters: logging in is exactly when credentials tend to be
	// stale after the machine has been off.
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>
	<key>ProgramArguments</key>
	<array>
%s	</array>
	<key>StartInterval</key>
	<integer>%d</integer>
	<key>RunAtLoad</key>
	<true/>
	<key>ProcessType</key>
	<string>Background</string>
	<key>StandardErrorPath</key>
	<string>%s</string>
</dict>
</plist>
`, Label, args.String(), int(cfg.Interval.Seconds()), xmlEscape(logPath))

	if err := os.WriteFile(path, []byte(plist), 0644); err != nil {
		return fmt.Errorf("could not write %s: %w", path, err)
	}

	// Replace any previous registration; booting out something that is not
	// loaded is not an error worth reporting.
	target := fmt.Sprintf("gui/%d", os.Getuid())
	_ = exec.Command("launchctl", "bootout", target+"/"+Label).Run()
	if out, err := exec.Command("launchctl", "bootstrap", target, path).CombinedOutput(); err != nil {
		return fmt.Errorf("launchctl bootstrap failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func Uninstall() error {
	path, err := plistPath()
	if err != nil {
		return err
	}
	target := fmt.Sprintf("gui/%d/%s", os.Getuid(), Label)
	_ = exec.Command("launchctl", "bootout", target).Run()
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("could not remove %s: %w", path, err)
	}
	return nil
}

func Status() (ServiceStatus, error) {
	path, err := plistPath()
	if err != nil {
		return ServiceStatus{}, err
	}
	status := ServiceStatus{UnitPath: path}

	data, err := os.ReadFile(path)
	if err == nil {
		status.Installed = true
		status.Interval = intervalFromPlist(string(data))
	}

	out, err := exec.Command("launchctl", "list").Output()
	if err == nil {
		status.Loaded = strings.Contains(string(out), Label)
	}
	return status, nil
}

// intervalFromPlist recovers StartInterval so status can report the interval
// actually registered rather than the current default.
func intervalFromPlist(plist string) time.Duration {
	const key = "<key>StartInterval</key>"
	i := strings.Index(plist, key)
	if i < 0 {
		return 0
	}
	rest := plist[i+len(key):]
	start := strings.Index(rest, "<integer>")
	end := strings.Index(rest, "</integer>")
	if start < 0 || end < 0 || end < start {
		return 0
	}
	var seconds int
	if _, err := fmt.Sscanf(strings.TrimSpace(rest[start+len("<integer>"):end]), "%d", &seconds); err != nil {
		return 0
	}
	return time.Duration(seconds) * time.Second
}

// xmlEscape protects the plist from paths containing XML metacharacters.
func xmlEscape(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(s)
}
