package tool

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAToolOnThePathIsUsedAsIs(t *testing.T) {
	// PATH first, always: a PATH that names the tool is one somebody arranged
	// deliberately, and this package is here to fill a gap rather than to
	// second-guess them.
	dir := t.TempDir()
	write(t, filepath.Join(dir, "pretend-tool"))
	t.Setenv("PATH", dir)

	got, err := Look("pretend-tool")
	if err != nil {
		t.Fatalf("Look: %v", err)
	}
	if want := filepath.Join(dir, "pretend-tool"); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestAToolOffThePathIsStillFound(t *testing.T) {
	// The bug this package exists for. Anything launchd starts gets
	// /usr/bin:/bin:/usr/sbin:/sbin, and the AWS CLI installs to
	// /usr/local/bin -- so `awsm sso login` failed reporting that aws could
	// not be found, from a program that had started perfectly well.
	elsewhere := t.TempDir()
	write(t, filepath.Join(elsewhere, "pretend-tool"))

	// A PATH that cannot see it, as launchd's cannot see /usr/local/bin.
	t.Setenv("PATH", t.TempDir())

	original := searchDirs
	searchDirs = []string{elsewhere}
	t.Cleanup(func() { searchDirs = original })

	got, err := Look("pretend-tool")
	if err != nil {
		t.Fatalf("Look: %v", err)
	}
	if want := filepath.Join(elsewhere, "pretend-tool"); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSomethingThatIsNotThereFailsTheFamiliarWay(t *testing.T) {
	// The caller prints this. It should be the message PATH would have given,
	// not a new one to explain.
	t.Setenv("PATH", t.TempDir())
	original := searchDirs
	searchDirs = []string{t.TempDir()}
	t.Cleanup(func() { searchDirs = original })

	if _, err := Look("no-such-tool-anywhere"); err == nil {
		t.Fatal("expected an error")
	} else if got := err.Error(); got == "" {
		t.Error("the error says nothing")
	}
}

func TestADirectoryIsNotATool(t *testing.T) {
	// A directory named like the tool, and a file that is not executable, both
	// have to be stepped over rather than returned.
	elsewhere := t.TempDir()
	if err := os.Mkdir(filepath.Join(elsewhere, "pretend-tool"), 0755); err != nil {
		t.Fatal(err)
	}
	notExecutable := filepath.Join(t.TempDir(), "pretend-tool")
	if err := os.WriteFile(notExecutable, []byte("#!/bin/sh\n"), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("PATH", t.TempDir())
	original := searchDirs
	searchDirs = []string{elsewhere, filepath.Dir(notExecutable)}
	t.Cleanup(func() { searchDirs = original })

	if got, err := Look("pretend-tool"); err == nil {
		t.Errorf("returned %q, which is not something that can be run", got)
	}
}

func TestCommandFallsBackToTheBareName(t *testing.T) {
	// When nothing can be found, the command has to fail the way it always
	// did rather than with something new.
	t.Setenv("PATH", t.TempDir())
	original := searchDirs
	searchDirs = nil
	t.Cleanup(func() { searchDirs = original })

	if got := Command("no-such-tool-anywhere").Path; got != "no-such-tool-anywhere" {
		t.Errorf("got %q, want the bare name", got)
	}
}

func write(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0700); err != nil {
		t.Fatal(err)
	}
}
