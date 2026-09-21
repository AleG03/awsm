// Package awsini centralizes reading and writing the INI files owned by the AWS
// CLI (~/.aws/config and ~/.aws/credentials).
//
// The AWS CLI parses those files with Python's RawConfigParser, which differs
// from the gopkg.in/ini.v1 defaults in two ways that silently corrupt the file
// on a read/modify/write cycle:
//
//   - A key with an empty value followed by indented lines is a nested
//     sub-section, e.g. "s3 =" with "addressing_style = path" beneath it.
//     Without AllowNestedValues go-ini reads the children as sibling keys of
//     the profile and writes them back flattened, leaving "s3" with an empty
//     value. botocore then aborts in _validate_s3_configuration with
//     "'str' object has no attribute 'get'", which breaks every AWS CLI call
//     including "aws sso login".
//   - RawConfigParser is built with inline_comment_prefixes=None, so "#" and
//     ";" are ordinary characters inside a value. go-ini treats them as inline
//     comment delimiters by default and truncates the value on write.
//
// Every read of an AWS INI file must therefore go through Load or LoadOrEmpty,
// and every write through Save or WriteFile, which additionally back the file
// up and replace it atomically.
package awsini

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	ini "gopkg.in/ini.v1"
)

// BackupsKept is the number of timestamped backups retained per file.
const BackupsKept = 10

// backupTimeFormat is used for backup file names; it sorts lexicographically.
const backupTimeFormat = "20060102T150405Z"

// Options returns the go-ini load options that make awsm agree with the AWS
// CLI on the meaning of an AWS INI file. See the package comment.
func Options() ini.LoadOptions {
	return ini.LoadOptions{
		AllowNestedValues:       true,
		IgnoreInlineComment:     true,
		PreserveSurroundedQuote: true,
	}
}

// Load reads an AWS INI file.
//
// A missing file is reported exactly as the filesystem reported it, because
// callers test the result with os.IsNotExist, which does not unwrap. Anything
// else is wrapped, since go-ini's parse errors do not name the file.
func Load(path string) (*ini.File, error) {
	cfg, err := ini.LoadSources(Options(), path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, err
		}
		return nil, fmt.Errorf("failed to load %s: %w", path, err)
	}
	return cfg, nil
}

// LoadOrEmpty reads an AWS INI file, returning an empty one if it is missing.
func LoadOrEmpty(path string) (*ini.File, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return Empty(), nil
	}
	return Load(path)
}

// Empty returns a new AWS INI file with the correct options applied.
func Empty() *ini.File {
	return ini.Empty(Options())
}

// Save writes cfg to path, backing up the previous contents and replacing the
// file atomically.
func Save(cfg *ini.File, path string) error {
	var buf bytes.Buffer
	if _, err := cfg.WriteTo(&buf); err != nil {
		return fmt.Errorf("failed to render %s: %w", path, err)
	}
	return WriteFile(path, buf.Bytes())
}

var (
	backedUpMu sync.Mutex
	backedUp   = map[string]bool{}
)

// WriteFile replaces path with content, backing up the previous contents and
// writing atomically so an interrupted run cannot leave a truncated file.
//
// A single command can write the same file more than once (awsm sso update
// rewrites the config and then reorganizes it). Only the first write backs it
// up, so the backup holds the state from before the command ran and one
// command consumes one slot of history.
func WriteFile(path string, content []byte) error {
	if firstWriteInProcess(path) {
		// Best effort: never block a requested change because the backup
		// failed.
		_, _ = Backup(path)
	}
	return writeAtomic(path, content)
}

// firstWriteInProcess reports whether path has not been written yet by this
// process, marking it as written.
func firstWriteInProcess(path string) bool {
	key, err := filepath.Abs(path)
	if err != nil {
		key = path
	}
	backedUpMu.Lock()
	defer backedUpMu.Unlock()
	if backedUp[key] {
		return false
	}
	backedUp[key] = true
	return true
}

// Backup copies path into the backup directory under a timestamped name and
// prunes old backups of the same file. It reports the backup path, or an empty
// path when there was nothing to back up.
func Backup(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}

	dir, err := BackupDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}

	base := filepath.Base(path)
	dest := filepath.Join(dir, fmt.Sprintf("%s.%s.bak", base, time.Now().UTC().Format(backupTimeFormat)))
	// These files can hold long-lived credentials: keep them owner-only.
	if err := os.WriteFile(dest, data, 0600); err != nil {
		return "", err
	}
	pruneBackups(dir, base, BackupsKept)
	return dest, nil
}

// BackupDir returns the directory holding backups of the AWS INI files.
func BackupDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not get user home directory: %w", err)
	}
	return filepath.Join(home, ".awsm", "backups"), nil
}

// pruneBackups keeps only the newest keep backups of base.
func pruneBackups(dir, base string, keep int) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	prefix := base + "."
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasPrefix(e.Name(), prefix) && strings.HasSuffix(e.Name(), ".bak") {
			names = append(names, e.Name())
		}
	}
	if len(names) <= keep {
		return
	}
	// The timestamp format sorts lexicographically, so the oldest sort first.
	sort.Strings(names)
	for _, name := range names[:len(names)-keep] {
		_ = os.Remove(filepath.Join(dir, name))
	}
}

// writeAtomic writes content to a temporary file in the destination directory
// and renames it over path, preserving the existing file mode.
func writeAtomic(path string, content []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create %s: %w", dir, err)
	}

	// New files are owner-only; an existing file keeps whatever mode the user
	// chose, so we never loosen nor tighten it behind their back.
	mode := os.FileMode(0600)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}

	return writeAtomicMode(path, content, mode)
}

// WritePrivateFile atomically replaces a secret file with owner-only permissions.
// It makes no backup, so exporting secrets does not create extra secret copies.
func WritePrivateFile(path string, content []byte) error {
	return writeAtomicMode(path, content, 0600)
}

func writeAtomicMode(path string, content []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	// os.CreateTemp creates with 0600, so the content is never briefly
	// world-readable.
	tmp, err := os.CreateTemp(dir, ".awsm-"+filepath.Base(path)+"-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary file in %s: %w", dir, err)
	}
	tmpName := tmp.Name()
	defer func() {
		tmp.Close()
		os.Remove(tmpName)
	}()

	if _, err := tmp.Write(content); err != nil {
		return fmt.Errorf("failed to write %s: %w", tmpName, err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("failed to flush %s: %w", tmpName, err)
	}
	if err := tmp.Chmod(mode); err != nil {
		return fmt.Errorf("failed to set permissions on %s: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("failed to close %s: %w", tmpName, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("failed to replace %s: %w", path, err)
	}
	return nil
}

// TooPermissive reports whether path can be read by anyone other than its
// owner, along with the mode it currently has.
//
// The AWS config file is conventionally 0644, which is fine while it only holds
// profile settings. Once static credentials are written into a profile the same
// mode makes them readable by every account on the machine, so callers that
// write secrets should check and offer to tighten it.
//
// A file that does not exist, or that cannot be stated, is not reported as a
// problem: there is nothing there to expose.
func TooPermissive(path string) (bool, os.FileMode, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, 0, nil
		}
		return false, 0, err
	}
	mode := info.Mode().Perm()
	return mode&0o077 != 0, mode, nil
}

// Restrict tightens path to owner-only access.
func Restrict(path string) error {
	if err := os.Chmod(path, 0600); err != nil {
		return fmt.Errorf("failed to restrict permissions on %s: %w", path, err)
	}
	return nil
}
