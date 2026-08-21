package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
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

func installUpdate(archivePath, goos string) error {
	// Get current executable path
	currentExe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get current executable path: %w", err)
	}

	if goos == "windows" {
		// For Windows, we'd need to handle zip extraction
		return fmt.Errorf("Windows auto-update not yet supported. Please download manually from GitHub")
	}

	// Extract into a private temp dir (avoids a predictable shared path)
	extractDir, err := os.MkdirTemp("", "awsm-extract-*")
	if err != nil {
		return fmt.Errorf("failed to create extraction dir: %w", err)
	}
	defer os.RemoveAll(extractDir)

	if err := exec.Command("tar", "-xzf", archivePath, "-C", extractDir).Run(); err != nil {
		return fmt.Errorf("failed to extract archive: %w", err)
	}

	extractedBinary := filepath.Join(extractDir, "awsm")

	if err := os.Chmod(extractedBinary, 0755); err != nil {
		return fmt.Errorf("failed to make binary executable: %w", err)
	}

	// Replace current binary. os.Rename fails with EXDEV when the temp dir
	// and install path are on different filesystems, so fall back to a copy.
	if err := os.Rename(extractedBinary, currentExe); err != nil {
		if err := copyFile(extractedBinary, currentExe); err != nil {
			return fmt.Errorf("failed to replace binary: %w", err)
		}
	}

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
