package awsini

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTooPermissive(t *testing.T) {
	dir := t.TempDir()

	for _, tc := range []struct {
		name string
		mode os.FileMode
		want bool
	}{
		{"owner only", 0o600, false},
		{"owner read only", 0o400, false},
		{"group readable", 0o640, true},
		{"world readable", 0o644, true},
		{"world writable", 0o606, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(dir, tc.name)
			if err := os.WriteFile(path, []byte("[profile a]\n"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(path, tc.mode); err != nil {
				t.Fatal(err)
			}

			permissive, mode, err := TooPermissive(path)
			if err != nil {
				t.Fatalf("TooPermissive: %v", err)
			}
			if permissive != tc.want {
				t.Errorf("TooPermissive(%o) = %v, want %v", tc.mode, permissive, tc.want)
			}
			if mode != tc.mode {
				t.Errorf("reported mode %o, want %o", mode, tc.mode)
			}
		})
	}
}

func TestTooPermissiveIgnoresAMissingFile(t *testing.T) {
	// Nothing is exposed by a file that does not exist, so this must not be
	// reported as a problem nor as an error.
	permissive, _, err := TooPermissive(filepath.Join(t.TempDir(), "absent"))
	if err != nil {
		t.Fatalf("TooPermissive: %v", err)
	}
	if permissive {
		t.Error("a missing file should not be reported as permissive")
	}
}

func TestRestrict(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(path, []byte("[profile a]\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := Restrict(path); err != nil {
		t.Fatalf("Restrict: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("mode = %o, want 600", got)
	}
}
