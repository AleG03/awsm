package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// maxLogBytes is when the log is rotated. One line per cycle is tiny, but a
// daemon that runs for months should not grow without bound on its own.
const maxLogBytes = 1 << 20 // 1 MiB

// LogPath returns the daemon log file.
func LogPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "daemon.log"), nil
}

// Log appends one line. Failing to log never fails a cycle: the work of
// renewing credentials matters more than the record of it.
func Log(format string, args ...any) {
	path, err := LogPath()
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return
	}
	rotateIfLarge(path)

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return
	}
	defer f.Close()
	line := fmt.Sprintf(format, args...)
	fmt.Fprintf(f, "%s %s\n", time.Now().Format(time.RFC3339), line)
}

// rotateIfLarge keeps a single previous file rather than a numbered series:
// the log is a recent-history aid, not an audit trail.
func rotateIfLarge(path string) {
	info, err := os.Stat(path)
	if err != nil || info.Size() < maxLogBytes {
		return
	}
	_ = os.Rename(path, path+".1")
}
