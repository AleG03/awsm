package filelock

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestLockSerializesProcesses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "counter")
	if err := os.WriteFile(path, []byte("0"), 0600); err != nil {
		t.Fatal(err)
	}
	var children []*exec.Cmd
	for i := 0; i < 4; i++ {
		child := exec.Command(os.Args[0], "-test.run=^TestLockProcessHelper$")
		child.Env = append(os.Environ(), "AWSM_TEST_LOCK_PATH="+path)
		if err := child.Start(); err != nil {
			t.Fatal(err)
		}
		children = append(children, child)
	}
	for _, child := range children {
		if err := child.Wait(); err != nil {
			t.Fatal(err)
		}
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "80" {
		t.Fatalf("lost concurrent writes: got %s, want 80", raw)
	}
	// The inode must remain stable after release, including across invocations.
	if _, err := os.Stat(path + ".awsm.lock"); err != nil {
		t.Fatal(err)
	}
}

func TestLockProcessHelper(t *testing.T) {
	path := os.Getenv("AWSM_TEST_LOCK_PATH")
	if path == "" {
		return
	}
	for i := 0; i < 20; i++ {
		err := With(path, func() error {
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			n, err := strconv.Atoi(strings.TrimSpace(string(raw)))
			if err != nil {
				return err
			}
			time.Sleep(time.Millisecond)
			// Replace the data inode, as AWS INI writes do. The lock is a sidecar.
			tmp, err := os.CreateTemp(filepath.Dir(path), "counter-*")
			if err != nil {
				return err
			}
			if _, err = fmt.Fprint(tmp, n+1); err != nil {
				tmp.Close()
				return err
			}
			if err = tmp.Close(); err != nil {
				return err
			}
			return os.Rename(tmp.Name(), path)
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestLockReleasedOnError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data")
	expected := fmt.Errorf("test failure")
	if err := With(path, func() error { return expected }); err != expected {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- With(path, func() error { return nil }) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("lock was not released on error")
	}
}
