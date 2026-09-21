//go:build darwin || linux

package filelock

import (
	"errors"
	"golang.org/x/sys/unix"
	"os"
)

func lock(f *os.File) error {
	for {
		err := unix.Flock(int(f.Fd()), unix.LOCK_EX)
		if !errors.Is(err, unix.EINTR) {
			return err
		}
	}
}
