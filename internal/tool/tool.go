// Package tool finds the external programs awsm runs.
//
// PATH alone is not enough, because awsm is not always started from a shell.
// Anything launched by launchd -- a status bar application, a login item, the
// refresh daemon's own launch agent -- is given /usr/bin:/bin:/usr/sbin:/sbin
// and nothing a shell profile would have added. The AWS CLI installs to
// /usr/local/bin, which is not on that list.
//
// The symptom was a renewal failing with
//
//	aws sso login failed: exec: "aws": executable file not found in $PATH
//
// from a program that had itself started perfectly well. Resolving the tool
// here rather than leaving it to PATH fixes that for every caller at once,
// including launch agents already installed: there is nothing to re-run.
package tool

import (
	"os"
	"os/exec"
	"path/filepath"
)

// searchDirs are the usual install locations, tried when PATH comes up empty.
//
// Only directories that exist are used, which is what keeps this harmless on
// the platforms where these names mean nothing.
var searchDirs = []string{
	"/usr/local/bin",
	"/opt/homebrew/bin",
	"/opt/local/bin",
	"/usr/bin",
	"/bin",
}

// Look returns the path to an external program.
//
// PATH is asked first: a PATH that already names the tool is one somebody
// arranged deliberately, and this is here to fill a gap rather than overrule
// them.
func Look(name string) (string, error) {
	path, err := exec.LookPath(name)
	if err == nil {
		return path, nil
	}

	for _, dir := range candidates() {
		candidate := filepath.Join(dir, name)
		if info, statErr := os.Stat(candidate); statErr == nil &&
			!info.IsDir() && info.Mode()&0111 != 0 {
			return candidate, nil
		}
	}
	// The original error, so the message a caller prints is the familiar one.
	return "", err
}

// candidates is where to look, with awsm's own directory first: whatever
// installed awsm is a reasonable guess at where its companions are.
func candidates() []string {
	dirs := searchDirs
	if self, err := os.Executable(); err == nil {
		if resolved, err := filepath.EvalSymlinks(self); err == nil {
			self = resolved
		}
		dirs = append([]string{filepath.Dir(self)}, dirs...)
	}
	return dirs
}

// Command builds a command for an external program, found the same way.
//
// A program that cannot be found is left as the bare name, so the failure is
// the same one PATH would have produced rather than something new to explain.
func Command(name string, args ...string) *exec.Cmd {
	if path, err := Look(name); err == nil {
		name = path
	}
	return exec.Command(name, args...)
}
