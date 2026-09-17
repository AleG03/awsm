package cmd

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// tarGz builds an archive in memory. entries maps a name inside the archive to
// its contents, so a test can describe a hostile archive as easily as a good one.
func tarGz(t *testing.T, entries map[string]string) string {
	t.Helper()
	var buffer bytes.Buffer
	gzipped := gzip.NewWriter(&buffer)
	archive := tar.NewWriter(gzipped)

	for name, content := range entries {
		header := &tar.Header{
			Name:     name,
			Mode:     0755,
			Size:     int64(len(content)),
			Typeflag: tar.TypeReg,
		}
		if err := archive.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if _, err := archive.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipped.Close(); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(t.TempDir(), "release.tar.gz")
	if err := os.WriteFile(path, buffer.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func zipArchive(t *testing.T, entries map[string]string) string {
	t.Helper()
	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	for name, content := range entries {
		writer, err := archive.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(t.TempDir(), "release.zip")
	if err := os.WriteFile(path, buffer.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestExtractBinaryFromTarGz(t *testing.T) {
	archive := tarGz(t, map[string]string{
		"README.md": "not the binary",
		"awsm":      "#!/bin/sh\necho new version\n",
	})
	destDir := t.TempDir()

	path, err := extractBinary(archive, destDir, "darwin")
	if err != nil {
		t.Fatalf("extractBinary() = %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "new version") {
		t.Errorf("extracted the wrong entry: %q", content)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0111 == 0 {
		t.Errorf("mode = %o, want the executable bit set", info.Mode().Perm())
	}
}

func TestExtractBinaryFromZip(t *testing.T) {
	archive := zipArchive(t, map[string]string{"awsm.exe": "windows build"})
	destDir := t.TempDir()

	path, err := extractBinary(archive, destDir, "windows")
	if err != nil {
		t.Fatalf("extractBinary() = %v", err)
	}
	if filepath.Base(path) != "awsm.exe" {
		t.Errorf("extracted %q, want awsm.exe", filepath.Base(path))
	}
	content, _ := os.ReadFile(path)
	if string(content) != "windows build" {
		t.Errorf("content = %q", content)
	}
}

// TestExtractBinaryIgnoresPathsThatEscape is the check tar(1) used to make for
// us. An archive is downloaded from the network, and an entry naming its way
// out of the extraction directory must not be able to overwrite anything.
//
// The entry is named ../../awsm on purpose. An archive holding ../../evil would
// prove nothing: that name is skipped for not being the binary we want, so the
// destination is never computed from it. Only an escaping path whose base name
// *does* match exercises the rule that matters -- the destination is ours, not
// the archive's.
func TestExtractBinaryIgnoresPathsThatEscape(t *testing.T) {
	archive := tarGz(t, map[string]string{"../../awsm": "should not escape"})

	parent := t.TempDir()
	destDir := filepath.Join(parent, "a", "b")
	if err := os.MkdirAll(destDir, 0700); err != nil {
		t.Fatal(err)
	}

	path, err := extractBinary(archive, destDir, "darwin")
	if err != nil {
		t.Fatalf("extractBinary() = %v", err)
	}
	if dir := filepath.Dir(path); dir != destDir {
		t.Errorf("wrote to %q, want everything inside %q", dir, destDir)
	}
	if content, _ := os.ReadFile(path); string(content) != "should not escape" {
		t.Errorf("content = %q", content)
	}
	// Where ../../awsm would land if the archive's path were trusted.
	if _, err := os.Stat(filepath.Join(parent, "awsm")); err == nil {
		t.Errorf("an entry escaped to %s", filepath.Join(parent, "awsm"))
	}
}

// countingReader records how much was actually read from it.
type countingReader struct {
	source io.Reader
	read   int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.source.Read(p)
	c.read += int64(n)
	return n, err
}

// TestWriteBinaryStopsReadingAtTheLimit checks the bound is enforced while
// reading, not discovered afterwards.
//
// Rejecting an oversized binary after copying it is not a protection: the point
// of the limit is that a hostile or broken archive cannot fill the disk first.
// Only a test that watches how much was consumed can tell those two apart.
func TestWriteBinaryStopsReadingAtTheLimit(t *testing.T) {
	original := maxBinarySize
	maxBinarySize = 64
	t.Cleanup(func() { maxBinarySize = original })

	source := &countingReader{source: strings.NewReader(strings.Repeat("x", 1<<20))}
	dest := filepath.Join(t.TempDir(), "awsm")

	if err := writeBinary(dest, source); err == nil {
		t.Fatal("writeBinary() = nil, want a refusal past the size limit")
	}
	if source.read > maxBinarySize+1 {
		t.Errorf("read %d bytes before refusing, want no more than %d", source.read, maxBinarySize+1)
	}
	if _, err := os.Stat(dest); err == nil {
		t.Error("the oversized file was left on disk")
	}
}

func TestExtractBinaryWithoutTheBinary(t *testing.T) {
	archive := tarGz(t, map[string]string{"README.md": "just docs"})

	if _, err := extractBinary(archive, t.TempDir(), "darwin"); err == nil {
		t.Fatal("extractBinary() = nil, want an error when the archive has no awsm")
	}
}

func TestExtractBinaryRefusesAnImplausibleSize(t *testing.T) {
	original := maxBinarySize
	maxBinarySize = 16
	t.Cleanup(func() { maxBinarySize = original })

	archive := tarGz(t, map[string]string{"awsm": strings.Repeat("x", 4096)})
	destDir := t.TempDir()

	if _, err := extractBinary(archive, destDir, "darwin"); err == nil {
		t.Fatal("extractBinary() = nil, want a refusal past the size limit")
	}
	if entries, _ := os.ReadDir(destDir); len(entries) != 0 {
		t.Errorf("left %d file(s) behind after refusing", len(entries))
	}
}

func TestReplaceBinary(t *testing.T) {
	dir := t.TempDir()
	current := filepath.Join(dir, "awsm")
	if err := os.WriteFile(current, []byte("old"), 0755); err != nil {
		t.Fatal(err)
	}
	extracted := filepath.Join(t.TempDir(), "awsm")
	if err := os.WriteFile(extracted, []byte("new"), 0755); err != nil {
		t.Fatal(err)
	}

	if err := replaceBinary(extracted, current); err != nil {
		t.Fatalf("replaceBinary() = %v", err)
	}

	content, err := os.ReadFile(current)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "new" {
		t.Errorf("content = %q, want the new binary", content)
	}
	if _, err := os.Stat(current + ".old"); err == nil {
		t.Error("the moved-aside binary was left behind on a platform that can remove it")
	}
}

// TestReplaceBinaryLeavesAWorkingBinary states the property that matters: an
// update that cannot install must leave a working awsm, not a hole where one
// used to be. Whether that is achieved by refusing early or by putting the old
// binary back is the implementation's business.
func TestReplaceBinaryLeavesAWorkingBinary(t *testing.T) {
	dir := t.TempDir()
	current := filepath.Join(dir, "awsm")
	if err := os.WriteFile(current, []byte("old but working"), 0755); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(t.TempDir(), "never-extracted")

	if err := replaceBinary(missing, current); err == nil {
		t.Fatal("replaceBinary() = nil, want an error when there is nothing to install")
	}

	content, err := os.ReadFile(current)
	if err != nil {
		t.Fatalf("the previous binary is gone: %v", err)
	}
	if string(content) != "old but working" {
		t.Errorf("content = %q, want the previous binary back", content)
	}
	if _, err := os.Stat(current + ".old"); err == nil {
		t.Error("left a stray .old file behind after a failed update")
	}
}
