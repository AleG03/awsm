package cmd

import (
	"awsm/internal/tool"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"awsm/internal/aws"
	"awsm/internal/awsini"
	"awsm/internal/tui"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

type checkStatus string

const (
	statusOK   checkStatus = "ok"
	statusWarn checkStatus = "warn"
	statusFail checkStatus = "fail"
	statusInfo checkStatus = "info"
)

type checkResult struct {
	Category string      `json:"category"`
	Name     string      `json:"name"`
	Status   checkStatus `json:"status"`
	Message  string      `json:"message,omitempty"`
}

var doctorJSON bool

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Run environment diagnostics for awsm",
	Long: `Inspect the local environment for common configuration issues:
- awsm/runtime versions
- AWS config and credentials files (existence, permissions)
- External tools (aws CLI, session-manager-plugin)
- awsm config validity
- Profiles and cache state
- Browsers configured for console integration

Use --json to get machine-readable output for bug reports.`,
	RunE: runDoctor,
}

func init() {
	doctorCmd.Flags().BoolVar(&doctorJSON, "json", false, "Output as JSON")
	rootCmd.AddCommand(doctorCmd)
}

func runDoctor(cmd *cobra.Command, args []string) error {
	results := collectDoctorResults()

	if doctorJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(results)
	}

	printDoctorResults(results)

	// Exit non-zero if any FAIL.
	for _, r := range results {
		if r.Status == statusFail {
			os.Exit(1)
		}
	}
	return nil
}

func collectDoctorResults() []checkResult {
	var out []checkResult

	out = append(out, checkVersions()...)
	out = append(out, checkAWSFiles()...)
	out = append(out, checkExternalTools()...)
	out = append(out, checkAwsmConfig()...)
	out = append(out, checkProfilesAndCache()...)
	out = append(out, checkBrowsers()...)

	return out
}

// ─── Checks ────────────────────────────────────────────────────────────────────

func checkVersions() []checkResult {
	return []checkResult{
		{Category: "Versions", Name: "awsm", Status: statusInfo, Message: rootCmd.Version},
		{Category: "Versions", Name: "go runtime", Status: statusInfo, Message: runtime.Version()},
		{Category: "Versions", Name: "os/arch", Status: statusInfo, Message: runtime.GOOS + "/" + runtime.GOARCH},
	}
}

func checkAWSFiles() []checkResult {
	var out []checkResult

	cfgPath, err := aws.GetAWSConfigPath()
	if err != nil {
		out = append(out, checkResult{Category: "AWS files", Name: "config path", Status: statusFail, Message: err.Error()})
	} else {
		out = append(out, fileCheck("AWS files", "~/.aws/config", cfgPath, 0o600))
	}

	credPath, err := aws.GetAWSCredentialsPath()
	if err != nil {
		out = append(out, checkResult{Category: "AWS files", Name: "credentials path", Status: statusFail, Message: err.Error()})
	} else {
		out = append(out, fileCheck("AWS files", "~/.aws/credentials", credPath, 0o600))
	}

	if cfgPath != "" {
		out = append(out, secretsInConfigCheck(cfgPath))
	}

	return out
}

// secretsInConfigCheck reports a config that holds static credentials while
// being readable by others. fileCheck already warns about the mode, but a
// config carrying access keys is a different severity from one carrying only
// regions, and awsm now writes IAM user keys there itself.
func secretsInConfigCheck(path string) checkResult {
	const name = "config secrets"

	permissive, mode, err := awsini.TooPermissive(path)
	if err != nil {
		return checkResult{Category: "AWS files", Name: name, Status: statusFail, Message: err.Error()}
	}

	cfg, err := awsini.Load(path)
	if err != nil {
		if os.IsNotExist(err) {
			return checkResult{Category: "AWS files", Name: name, Status: statusOK, Message: "no config file"}
		}
		return checkResult{Category: "AWS files", Name: name, Status: statusFail, Message: err.Error()}
	}

	var withKeys []string
	for _, section := range cfg.Sections() {
		if section.Key("aws_access_key_id").String() != "" {
			withKeys = append(withKeys, strings.TrimPrefix(section.Name(), "profile "))
		}
	}

	if len(withKeys) == 0 {
		return checkResult{Category: "AWS files", Name: name, Status: statusOK, Message: "no static keys in config"}
	}
	if runtime.GOOS != "windows" && permissive {
		return checkResult{
			Category: "AWS files", Name: name, Status: statusFail,
			Message: fmt.Sprintf("%d profile(s) keep access keys in the config, which is mode %o and readable by other users: %s — run chmod 600 %s",
				len(withKeys), mode, strings.Join(withKeys, ", "), path),
		}
	}
	return checkResult{
		Category: "AWS files", Name: name, Status: statusOK,
		Message: fmt.Sprintf("%d profile(s) keep access keys in the config, which is owner-only", len(withKeys)),
	}
}

func fileCheck(category, label, path string, maxMode fs.FileMode) checkResult {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return checkResult{Category: category, Name: label, Status: statusWarn, Message: "missing: " + path}
		}
		return checkResult{Category: category, Name: label, Status: statusFail, Message: err.Error()}
	}
	mode := info.Mode().Perm()
	if runtime.GOOS != "windows" && mode&0o077 != 0 {
		return checkResult{
			Category: category, Name: label, Status: statusWarn,
			Message: fmt.Sprintf("permissions too open (%o), recommended %o: %s", mode, maxMode, path),
		}
	}
	return checkResult{Category: category, Name: label, Status: statusOK, Message: path}
}

func checkExternalTools() []checkResult {
	tools := []struct {
		name, bin, hint string
		required        bool
	}{
		{"aws CLI", "aws", "needed for SSO login flows; install from https://aws.amazon.com/cli/", true},
		{"session-manager-plugin", "session-manager-plugin", "needed by `awsm connect`; see https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-working-with-install-plugin.html", false},
	}
	var out []checkResult
	for _, t := range tools {
		path, err := tool.Look(t.bin)
		if err != nil {
			status := statusWarn
			msg := "not found in PATH — " + t.hint
			if t.required {
				status = statusFail
			}
			out = append(out, checkResult{Category: "External tools", Name: t.name, Status: status, Message: msg})
			continue
		}
		// Try to capture a version string but don't fail if it doesn't support --version.
		ver := strings.TrimSpace(captureFirstLine(exec.Command(t.bin, "--version")))
		if ver == "" {
			ver = path
		}
		out = append(out, checkResult{Category: "External tools", Name: t.name, Status: statusOK, Message: ver})
	}
	return out
}

func captureFirstLine(cmd *exec.Cmd) string {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd2 := exec.CommandContext(ctx, cmd.Path, cmd.Args[1:]...)
	b, err := cmd2.CombinedOutput()
	if err != nil {
		return ""
	}
	line := string(b)
	if idx := strings.IndexByte(line, '\n'); idx >= 0 {
		line = line[:idx]
	}
	return strings.TrimSpace(line)
}

func checkAwsmConfig() []checkResult {
	var out []checkResult
	home, err := os.UserHomeDir()
	if err != nil {
		return []checkResult{{Category: "awsm config", Name: "home dir", Status: statusFail, Message: err.Error()}}
	}
	path := filepath.Join(home, ".config", "awsm", "config.toml")
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			out = append(out, checkResult{Category: "awsm config", Name: "config.toml", Status: statusInfo, Message: "not present (optional): " + path})
			return out
		}
		return []checkResult{{Category: "awsm config", Name: "config.toml", Status: statusFail, Message: err.Error()}}
	}
	out = append(out, checkResult{Category: "awsm config", Name: "config.toml", Status: statusOK, Message: fmt.Sprintf("%s (%d bytes)", path, info.Size())})
	return out
}

func checkProfilesAndCache() []checkResult {
	var out []checkResult

	profiles, err := aws.ListProfiles()
	if err != nil {
		out = append(out, checkResult{Category: "Profiles", Name: "list", Status: statusFail, Message: err.Error()})
	} else {
		status := statusOK
		if len(profiles) == 0 {
			status = statusWarn
		}
		out = append(out, checkResult{Category: "Profiles", Name: "count", Status: status, Message: fmt.Sprintf("%d profile(s) discovered", len(profiles))})
	}

	if cur := aws.GetCurrentProfileName(); cur != "" {
		out = append(out, checkResult{Category: "Profiles", Name: "active", Status: statusOK, Message: cur})
	} else {
		out = append(out, checkResult{Category: "Profiles", Name: "active", Status: statusInfo, Message: "no active profile set"})
	}

	home, err := os.UserHomeDir()
	if err == nil {
		cachePath := filepath.Join(home, ".awsm", "cache")
		if info, statErr := os.Stat(cachePath); statErr == nil && info.IsDir() {
			files, _ := os.ReadDir(cachePath)
			out = append(out, checkResult{Category: "Profiles", Name: "cache", Status: statusOK, Message: fmt.Sprintf("%d cached credential(s) at %s", len(files), cachePath)})
		} else {
			out = append(out, checkResult{Category: "Profiles", Name: "cache", Status: statusInfo, Message: "no cache directory yet"})
		}
	}

	// Every awsm write to ~/.aws/config or ~/.aws/credentials leaves a
	// timestamped copy here, which is worth surfacing: it is the way back if a
	// sync produced something unexpected.
	if backupDir, dirErr := awsini.BackupDir(); dirErr == nil {
		if info, statErr := os.Stat(backupDir); statErr == nil && info.IsDir() {
			files, _ := os.ReadDir(backupDir)
			out = append(out, checkResult{Category: "Profiles", Name: "backups", Status: statusOK, Message: fmt.Sprintf("%d backup(s) of your AWS files at %s", len(files), backupDir)})
		} else {
			out = append(out, checkResult{Category: "Profiles", Name: "backups", Status: statusInfo, Message: "no backups yet; awsm creates one before it writes your AWS files"})
		}
	}

	return out
}

func checkBrowsers() []checkResult {
	candidates := map[string][]string{
		"Google Chrome": {"google-chrome", "google-chrome-stable", "chrome", "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"},
		"Firefox":       {"firefox", "/Applications/Firefox.app/Contents/MacOS/firefox"},
		"Zen Browser":   {"zen-browser", "zen", "/Applications/Zen Browser.app/Contents/MacOS/zen"},
	}
	var out []checkResult
	for name, paths := range candidates {
		found := ""
		for _, p := range paths {
			if strings.ContainsRune(p, '/') {
				if _, err := os.Stat(p); err == nil {
					found = p
					break
				}
				continue
			}
			if path, err := exec.LookPath(p); err == nil {
				found = path
				break
			}
		}
		if found != "" {
			out = append(out, checkResult{Category: "Browsers", Name: name, Status: statusOK, Message: found})
		} else {
			out = append(out, checkResult{Category: "Browsers", Name: name, Status: statusInfo, Message: "not found (optional)"})
		}
	}
	return out
}

// ─── Pretty printing ───────────────────────────────────────────────────────────

func printDoctorResults(results []checkResult) {
	tui.PrintHeader("🩺  awsm doctor")

	currentCat := ""
	for _, r := range results {
		if r.Category != currentCat {
			currentCat = r.Category
			fmt.Fprintln(os.Stderr)
			fmt.Fprintln(os.Stderr, tui.SubheaderStyle.Render(currentCat))
		}
		fmt.Fprintf(os.Stderr, "  %s %s %s\n",
			statusBadge(r.Status),
			lipgloss.NewStyle().Bold(true).Render(r.Name),
			tui.MutedStyle.Render(r.Message),
		)
	}
	fmt.Fprintln(os.Stderr)

	summary := summarize(results)
	tui.PrintMuted(summary)
}

func statusBadge(s checkStatus) string {
	switch s {
	case statusOK:
		return tui.SuccessStyle.Render("[ OK ]")
	case statusWarn:
		return lipgloss.NewStyle().Foreground(tui.Warning).Bold(true).Render("[WARN]")
	case statusFail:
		return tui.ErrorStyle.Render("[FAIL]")
	default:
		return lipgloss.NewStyle().Foreground(tui.Muted).Bold(true).Render("[INFO]")
	}
}

func summarize(results []checkResult) string {
	var ok, warn, fail, info int
	for _, r := range results {
		switch r.Status {
		case statusOK:
			ok++
		case statusWarn:
			warn++
		case statusFail:
			fail++
		default:
			info++
		}
	}
	return fmt.Sprintf("Summary: %d ok, %d warn, %d fail, %d info", ok, warn, fail, info)
}
