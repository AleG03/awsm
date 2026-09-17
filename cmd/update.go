package cmd

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"awsm/internal/tui"

	"github.com/spf13/cobra"
)

type GitHubRelease struct {
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update AWSM to the latest version",
	Long:  `Downloads and installs the latest version of AWSM from GitHub releases.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		tui.PrintInfo("Checking for updates...")

		// Get latest release info
		resp, err := http.Get("https://api.github.com/repos/AleG03/awsm/releases/latest")
		if err != nil {
			return fmt.Errorf("failed to check for updates: %w", err)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read response: %w", err)
		}

		var release GitHubRelease
		if err := json.Unmarshal(body, &release); err != nil {
			return fmt.Errorf("failed to parse release info: %w", err)
		}

		// Check if we're already on the latest version
		if version != "dev" && release.TagName == "v"+version {
			tui.PrintSuccess("Already on the latest version!")
			return nil
		}

		tui.PrintInfo(fmt.Sprintf("Latest version: %s", release.TagName))
		tui.PrintInfo(fmt.Sprintf("Current version: %s", version))

		// Find the appropriate asset for current OS/arch
		assetName := fmt.Sprintf("awsm_%s_%s_%s",
			strings.TrimPrefix(release.TagName, "v"),
			runtime.GOOS,
			runtime.GOARCH)

		if runtime.GOOS == "windows" {
			assetName += ".zip"
		} else {
			assetName += ".tar.gz"
		}

		var downloadURL, checksumsURL string
		for _, asset := range release.Assets {
			if asset.Name == assetName {
				downloadURL = asset.BrowserDownloadURL
			}
			if asset.Name == "checksums.txt" {
				checksumsURL = asset.BrowserDownloadURL
			}
		}

		if downloadURL == "" {
			return fmt.Errorf("no compatible release found for %s/%s", runtime.GOOS, runtime.GOARCH)
		}

		tui.PrintStep(fmt.Sprintf("Downloading %s...", assetName))

		// Download the release
		resp, err = http.Get(downloadURL)
		if err != nil {
			return fmt.Errorf("failed to download update: %w", err)
		}
		defer resp.Body.Close()

		// Create temp file
		tmpFile, err := os.CreateTemp("", "awsm-update-*")
		if err != nil {
			return fmt.Errorf("failed to create temp file: %w", err)
		}
		defer os.Remove(tmpFile.Name())

		// Write downloaded content while hashing it
		hasher := sha256.New()
		_, err = io.Copy(io.MultiWriter(tmpFile, hasher), resp.Body)
		if err != nil {
			return fmt.Errorf("failed to write update: %w", err)
		}
		tmpFile.Close()

		if checksumsURL == "" {
			return fmt.Errorf("release is missing checksums.txt, refusing to install unverified binary")
		}
		expectedSum, err := fetchExpectedChecksum(checksumsURL, assetName)
		if err != nil {
			return fmt.Errorf("failed to verify checksum: %w", err)
		}
		if actualSum := hex.EncodeToString(hasher.Sum(nil)); actualSum != expectedSum {
			return fmt.Errorf("checksum mismatch for %s: expected %s, got %s", assetName, expectedSum, actualSum)
		}
		tui.PrintSuccess("Checksum verified.")

		// Extract and install
		if err := installUpdate(tmpFile.Name(), runtime.GOOS); err != nil {
			return fmt.Errorf("failed to install update: %w", err)
		}

		tui.PrintSuccess(fmt.Sprintf("Successfully updated to %s!", release.TagName))
		tui.PrintMuted("Please restart the command to use the new version.")
		return nil
	},
}

func fetchExpectedChecksum(checksumsURL, assetName string) (string, error) {
	resp, err := http.Get(checksumsURL)
	if err != nil {
		return "", fmt.Errorf("failed to download checksums.txt: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read checksums.txt: %w", err)
	}

	for _, line := range strings.Split(string(body), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == assetName {
			return fields[0], nil
		}
	}
	return "", fmt.Errorf("no checksum entry found for %s", assetName)
}

// maxBinarySize bounds what is copied out of a release archive.
//
// A var rather than a const so the tests can lower it: io.Copy on its own is
// happy to fill a disk, and an archive that claims to hold a gigabyte is a
// broken download, not a new version.
var maxBinarySize int64 = 256 << 20

func installUpdate(archivePath, goos string) error {
	currentExe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get current executable path: %w", err)
	}

	// Extract into a private temp dir (avoids a predictable shared path)
	extractDir, err := os.MkdirTemp("", "awsm-extract-*")
	if err != nil {
		return fmt.Errorf("failed to create extraction dir: %w", err)
	}
	defer os.RemoveAll(extractDir)

	extracted, err := extractBinary(archivePath, extractDir, goos)
	if err != nil {
		return err
	}
	return replaceBinary(extracted, currentExe)
}

// extractBinary pulls the awsm executable out of a release archive.
//
// Done with archive/tar and archive/zip rather than by running tar(1): one less
// external program to find, and it is what lets the Windows archive be handled
// at all. The destination path is fixed rather than taken from the archive, so
// an entry named ../../something has nowhere to escape to -- the protection
// tar(1) used to provide is now this line.
func extractBinary(archivePath, destDir, goos string) (string, error) {
	wanted := "awsm"
	if goos == "windows" {
		wanted = "awsm.exe"
	}
	dest := filepath.Join(destDir, wanted)

	var err error
	if goos == "windows" {
		err = extractFromZip(archivePath, wanted, dest)
	} else {
		err = extractFromTarGz(archivePath, wanted, dest)
	}
	if err != nil {
		return "", err
	}
	return dest, nil
}

func extractFromTarGz(archivePath, wanted, dest string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("failed to open the downloaded archive: %w", err)
	}
	defer file.Close()

	gzipped, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("the download is not a gzip archive: %w", err)
	}
	defer gzipped.Close()

	archive := tar.NewReader(gzipped)
	for {
		header, err := archive.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read the archive: %w", err)
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}
		if filepath.Base(filepath.Clean(header.Name)) != wanted {
			continue
		}
		return writeBinary(dest, archive)
	}
	return fmt.Errorf("the archive contains no %s", wanted)
}

func extractFromZip(archivePath, wanted, dest string) error {
	archive, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("the download is not a zip archive: %w", err)
	}
	defer archive.Close()

	for _, entry := range archive.File {
		if entry.FileInfo().IsDir() {
			continue
		}
		if filepath.Base(filepath.Clean(entry.Name)) != wanted {
			continue
		}
		content, err := entry.Open()
		if err != nil {
			return fmt.Errorf("failed to read %s from the archive: %w", wanted, err)
		}
		defer content.Close()
		return writeBinary(dest, content)
	}
	return fmt.Errorf("the archive contains no %s", wanted)
}

// writeBinary copies one executable out of an archive, refusing an implausible
// size instead of writing until the disk is full.
func writeBinary(dest string, source io.Reader) error {
	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return fmt.Errorf("failed to create the extracted binary: %w", err)
	}

	written, err := io.Copy(out, io.LimitReader(source, maxBinarySize+1))
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(dest)
		return fmt.Errorf("failed to extract the binary: %w", err)
	}
	if written > maxBinarySize {
		_ = os.Remove(dest)
		return fmt.Errorf("the binary in the archive is larger than %d bytes; refusing it", maxBinarySize)
	}
	if written == 0 {
		_ = os.Remove(dest)
		return fmt.Errorf("the binary in the archive is empty")
	}
	return nil
}

// replaceBinary puts the new executable where the running one is.
//
// The old binary is moved aside rather than overwritten, because Windows
// refuses to replace a file that is executing while allowing it to be renamed.
// Unix would accept a plain rename, but doing it the same way everywhere leaves
// one path to reason about and buys the same thing on both: if installing the
// new binary fails, the old one goes back, and the user is never left without
// a working awsm.
//
// Note that the Windows half of this has never been run on Windows.
func replaceBinary(extracted, currentExe string) error {
	// Checked before anything is moved: there is no reason to open a window
	// where the user has no awsm at all, however briefly, for an update that
	// was never going to happen. The restore below still covers the case that
	// remains -- a move that fails partway, on a full disk or across a mount.
	if _, err := os.Stat(extracted); err != nil {
		return fmt.Errorf("there is no extracted binary to install: %w", err)
	}

	aside := currentExe + ".old"
	_ = os.Remove(aside) // a leftover from a previous update

	if err := os.Rename(currentExe, aside); err != nil {
		return fmt.Errorf("failed to move the current binary aside: %w", err)
	}

	// os.Rename fails with EXDEV when the temp dir and the install path are on
	// different filesystems, so fall back to a copy.
	if err := os.Rename(extracted, currentExe); err != nil {
		if err := copyFile(extracted, currentExe); err != nil {
			if restoreErr := os.Rename(aside, currentExe); restoreErr != nil {
				return fmt.Errorf("failed to install the new binary (%w), and the previous one is left at %s: %v", err, aside, restoreErr)
			}
			return fmt.Errorf("failed to install the new binary, the previous one is back in place: %w", err)
		}
	}

	// Still executing from this file on Unix and still locked on Windows, so
	// removing it is best effort; the next update clears what is left.
	_ = os.Remove(aside)
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.CreateTemp(filepath.Dir(dst), ".awsm-update-*")
	if err != nil {
		return err
	}
	defer os.Remove(out.Name())

	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Chmod(0755); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Rename(out.Name(), dst)
}

func init() {
	rootCmd.AddCommand(updateCmd)
}
