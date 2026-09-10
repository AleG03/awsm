package awsini

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// isolateHome points os.UserHomeDir at a scratch directory so tests never
// write backups into the real ~/.awsm.
func isolateHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	resetBackupTracking()
	return home
}

// resetBackupTracking clears the once-per-process backup bookkeeping so tests
// do not interfere with each other.
func resetBackupTracking() {
	backedUpMu.Lock()
	defer backedUpMu.Unlock()
	backedUp = map[string]bool{}
}

// The AWS CLI reads a key with an empty value followed by indented lines as a
// nested sub-section. Flattening it leaves "s3 =" with an empty string value,
// which makes botocore abort in _validate_s3_configuration with
// "'str' object has no attribute 'get'" - the bug this package exists for.
func TestSavePreservesNestedSubSections(t *testing.T) {
	isolateHome(t)
	path := filepath.Join(t.TempDir(), "config")
	original := "[default]\nregion = eu-west-1\ns3 =\n  max_concurrent_requests = 20\n  addressing_style = path\n"
	if err := os.WriteFile(path, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Section("default").Key("s3").NestedValues(); len(got) != 2 {
		t.Fatalf("nested values lost on load: %q", got)
	}
	if err := Save(cfg, path); err != nil {
		t.Fatal(err)
	}

	saved := readFile(t, path)
	for _, want := range []string{"max_concurrent_requests = 20", "addressing_style = path"} {
		if !strings.Contains(saved, want) {
			t.Errorf("saved config lost %q:\n%s", want, saved)
		}
	}
	for _, line := range strings.Split(saved, "\n") {
		if strings.HasPrefix(line, "max_concurrent_requests") || strings.HasPrefix(line, "addressing_style") {
			t.Errorf("nested key flattened to top level:\n%s", saved)
		}
	}

	// The children must still be readable after the round trip.
	reloaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := reloaded.Section("default").Key("s3").NestedValues(); len(got) != 2 {
		t.Errorf("nested values lost on round trip: %q", got)
	}
}

// botocore builds its parser with inline_comment_prefixes=None, so "#" and ";"
// are ordinary characters inside a value.
func TestLoadAndSaveKeepHashAndSemicolonInValues(t *testing.T) {
	isolateHome(t)
	path := filepath.Join(t.TempDir(), "config")
	original := "[profile p]\ncredential_process = /usr/bin/tok --key=a#b;c\ncli_pager                = \"less -R\"\n"
	if err := os.WriteFile(path, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Section("profile p").Key("credential_process").Value(); got != "/usr/bin/tok --key=a#b;c" {
		t.Errorf("value truncated on load: %q", got)
	}
	if got := cfg.Section("profile p").Key("cli_pager").Value(); got != `"less -R"` {
		t.Errorf("quotes stripped on load: %q", got)
	}

	if err := Save(cfg, path); err != nil {
		t.Fatal(err)
	}
	saved := readFile(t, path)
	if !strings.Contains(saved, "/usr/bin/tok --key=a#b;c") {
		t.Errorf("value truncated on save:\n%s", saved)
	}
	if !strings.Contains(saved, `"less -R"`) {
		t.Errorf("quotes stripped on save:\n%s", saved)
	}
}

func TestWriteFileBacksUpAndReplacesAtomically(t *testing.T) {
	home := isolateHome(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "credentials")
	if err := os.WriteFile(path, []byte("[old]\n"), 0600); err != nil {
		t.Fatal(err)
	}

	if err := WriteFile(path, []byte("[new]\n")); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, path); got != "[new]\n" {
		t.Errorf("content = %q", got)
	}

	backups := listBackups(t, home, "credentials")
	if len(backups) != 1 {
		t.Fatalf("expected 1 backup, got %v", backups)
	}
	backupDir, _ := BackupDir()
	if got := readFile(t, filepath.Join(backupDir, backups[0])); got != "[old]\n" {
		t.Errorf("backup content = %q", got)
	}

	// Credentials end up in the backup directory, so it must stay owner-only.
	assertMode(t, backupDir, 0700)
	assertMode(t, filepath.Join(backupDir, backups[0]), 0600)

	// No temporary file may survive a successful write.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".awsm-") {
			t.Errorf("temporary file left behind: %s", e.Name())
		}
	}
}

// awsm sso update writes the config and then reorganizes it; both writes in
// one command must share a single backup, or the retained history covers half
// as many commands.
func TestWriteFileBacksUpOnlyOncePerProcess(t *testing.T) {
	home := isolateHome(t)
	resetBackupTracking()
	path := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(path, []byte("[before]\n"), 0600); err != nil {
		t.Fatal(err)
	}

	for _, content := range []string{"[first]\n", "[second]\n", "[third]\n"} {
		if err := WriteFile(path, []byte(content)); err != nil {
			t.Fatal(err)
		}
	}

	backups := listBackups(t, home, "config")
	if len(backups) != 1 {
		t.Fatalf("expected 1 backup for 3 writes, got %v", backups)
	}
	backupDir, _ := BackupDir()
	if got := readFile(t, filepath.Join(backupDir, backups[0])); got != "[before]\n" {
		t.Errorf("backup holds %q, want the state from before the command", got)
	}
	if got := readFile(t, path); got != "[third]\n" {
		t.Errorf("final content = %q", got)
	}
}

func TestWriteFileKeepsExistingModeAndDefaultsToOwnerOnly(t *testing.T) {
	isolateHome(t)
	dir := t.TempDir()

	existing := filepath.Join(dir, "config")
	if err := os.WriteFile(existing, []byte("[a]\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := WriteFile(existing, []byte("[b]\n")); err != nil {
		t.Fatal(err)
	}
	assertMode(t, existing, 0644)

	fresh := filepath.Join(dir, "new-config")
	if err := WriteFile(fresh, []byte("[c]\n")); err != nil {
		t.Fatal(err)
	}
	assertMode(t, fresh, 0600)
}

func TestBackupPrunesToBackupsKept(t *testing.T) {
	home := isolateHome(t)
	path := filepath.Join(t.TempDir(), "config")

	backupDir, err := BackupDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(backupDir, 0700); err != nil {
		t.Fatal(err)
	}
	// Backups are named with a one second resolution timestamp, so seed the
	// directory directly instead of sleeping.
	for i := 0; i < BackupsKept+5; i++ {
		name := filepath.Join(backupDir, "config.2020010"+string(rune('0'+i%10))+"T00000"+string(rune('0'+i%10))+"Z.bak")
		if err := os.WriteFile(name, []byte("old"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(path, []byte("[current]\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Backup(path); err != nil {
		t.Fatal(err)
	}

	if got := listBackups(t, home, "config"); len(got) != BackupsKept {
		t.Errorf("expected %d backups after pruning, got %d: %v", BackupsKept, len(got), got)
	}
}

func TestBackupOfMissingFileIsNoOp(t *testing.T) {
	isolateHome(t)
	dest, err := Backup(filepath.Join(t.TempDir(), "does-not-exist"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dest != "" {
		t.Errorf("expected no backup path, got %q", dest)
	}
}

func TestLoadOrEmptyOnMissingFile(t *testing.T) {
	isolateHome(t)
	path := filepath.Join(t.TempDir(), "absent")

	cfg, err := LoadOrEmpty(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The file it returns must carry the same options as a loaded one, or a
	// nested sub-section written into a brand new config is flattened again.
	section, err := cfg.NewSection("default")
	if err != nil {
		t.Fatal(err)
	}
	section.Key("s3").SetValue("")
	if err := Save(cfg, path); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err != nil {
		t.Errorf("saved file does not load back: %v", err)
	}
}

// Callers branch on os.IsNotExist, which does not unwrap, so a missing file
// has to come back as a plain filesystem error. Wrapping it made
// "awsm profile list" fail on a machine with no ~/.aws/credentials.
func TestLoadReportsMissingFileAsNotExist(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "absent"))
	if err == nil {
		t.Fatal("expected an error")
	}
	if !os.IsNotExist(err) {
		t.Errorf("os.IsNotExist(err) = false for %v", err)
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("errors.Is(err, os.ErrNotExist) = false for %v", err)
	}
}

// A malformed file still has to say which file it was.
func TestLoadNamesTheFileOnParseError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(path, []byte("this is not ini\n"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected a parse error")
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("error does not name the file: %v", err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func listBackups(t *testing.T, home, base string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(home, ".awsm", "backups"))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), base+".") {
			names = append(names, e.Name())
		}
	}
	return names
}

func assertMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Errorf("%s mode = %o, want %o", path, got, want)
	}
}
