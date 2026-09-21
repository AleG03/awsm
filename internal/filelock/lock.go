// Package filelock serializes cooperating processes using a stable sidecar file.
package filelock

import (
	"fmt"
	"os"
	"path/filepath"
)

// With holds an exclusive OS lock for the entire read/modify/write operation.
// The sidecar must not be removed: replacing its inode would split the lock.
// Closing the descriptor also releases the lock if the process exits abruptly.
func With(path string, fn func() error) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.OpenFile(path+".awsm.lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return fmt.Errorf("open lock for %s: %w", path, err)
	}
	defer f.Close()
	if err := lock(f); err != nil {
		return fmt.Errorf("lock %s: %w", path, err)
	}
	return fn()
}
