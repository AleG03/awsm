package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"awsm/internal/util"

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
		util.InfoColor.Println("Checking for updates...")

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
			util.SuccessColor.Println("✔ Already on the latest version!")
			return nil
		}

		util.InfoColor.Printf("Latest version: %s\n", release.TagName)
		util.InfoColor.Printf("Current version: %s\n", version)

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

		var downloadURL string
		for _, asset := range release.Assets {
			if asset.Name == assetName {
				downloadURL = asset.BrowserDownloadURL
				break
			}
		}

		if downloadURL == "" {
			return fmt.Errorf("no compatible release found for %s/%s", runtime.GOOS, runtime.GOARCH)
		}

		util.InfoColor.Printf("Downloading %s...\n", assetName)

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

		// Write downloaded content
		_, err = io.Copy(tmpFile, resp.Body)
		if err != nil {
			return fmt.Errorf("failed to write update: %w", err)
		}
		tmpFile.Close()

		// Extract and install
		if err := installUpdate(tmpFile.Name(), runtime.GOOS); err != nil {
			return fmt.Errorf("failed to install update: %w", err)
		}

		util.SuccessColor.Printf("✔ Successfully updated to %s!\n", release.TagName)
		util.InfoColor.Println("Please restart the command to use the new version.")
		return nil
	},
}

func installUpdate(archivePath, goos string) error {
	// Get current executable path
	currentExe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get current executable path: %w", err)
	}

	// Extract archive
	var cmd *exec.Cmd
	if goos == "windows" {
		// For Windows, we'd need to handle zip extraction
		return fmt.Errorf("Windows auto-update not yet supported. Please download manually from GitHub")
	} else {
		// Extract tar.gz
		cmd = exec.Command("tar", "-xzf", archivePath, "-C", "/tmp")
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to extract archive: %w", err)
	}

	// Find extracted binary
	extractedBinary := "/tmp/awsm"
	if goos == "windows" {
		extractedBinary = "/tmp/awsm.exe"
	}

	// Replace current binary
	if err := os.Rename(extractedBinary, currentExe); err != nil {
		return fmt.Errorf("failed to replace binary: %w", err)
	}

	// Make executable
	if err := os.Chmod(currentExe, 0755); err != nil {
		return fmt.Errorf("failed to make binary executable: %w", err)
	}

	return nil
}

func init() {
	rootCmd.AddCommand(updateCmd)
}
