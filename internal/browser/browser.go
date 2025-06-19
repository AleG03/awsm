package browser

import (
	"awsm/internal/config"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/pkg/browser"
)

type FirefoxContainer struct {
	Icon          string `json:"icon"`
	Color         string `json:"color"`
	Name          string `json:"name"`
	Public        bool   `json:"public"`
	UserContextID int    `json:"userContextId"`
}

type FirefoxContainers struct {
	Version           int                `json:"version"`
	LastUserContextId int                `json:"lastUserContextId"`
	Identities        []FirefoxContainer `json:"identities"`
}

// OpenURL opens a URL in the specified browser profile/container.
// It takes the URL and either a Chrome profile alias or Firefox container name.
func OpenURL(targetURL, chromeProfileAlias string, firefoxContainer string) error {
	if chromeProfileAlias == "" && firefoxContainer == "" {
		return browser.OpenURL(targetURL)
	}

	if chromeProfileAlias != "" {
		return openURLInChromeProfile(targetURL, chromeProfileAlias)
	}

	if firefoxContainer != "" {
		return openURLInFirefoxContainer(targetURL, firefoxContainer)
	}

	return nil
}

// openURLInChromeProfile opens a URL in a specific Chrome profile
func openURLInChromeProfile(targetURL, chromeProfileAlias string) error {
	profileDirectory := config.GetChromeProfileDirectory(chromeProfileAlias)
	var cmd *exec.Cmd
	profileArg := fmt.Sprintf("--profile-directory=%s", profileDirectory)
	switch runtime.GOOS {
	case "darwin": // macOS
		cmd = exec.Command("/Applications/Google Chrome.app/Contents/MacOS/Google Chrome", profileArg, targetURL)
	case "windows":
		cmd = exec.Command("C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe", profileArg, targetURL)
	case "linux":
		cmd = exec.Command("google-chrome", profileArg, targetURL)
	default:
		return browser.OpenURL(targetURL)
	}

	return cmd.Start()
}

// getFirefoxProfileDir returns the path to the default Firefox profile directory
func getFirefoxProfileDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not get user home directory: %w", err)
	}

	var firefoxDir string
	switch runtime.GOOS {
	case "darwin":
		firefoxDir = filepath.Join(homeDir, "Library/Application Support/Firefox/Profiles")
	case "linux":
		firefoxDir = filepath.Join(homeDir, ".mozilla/firefox")
	case "windows":
		firefoxDir = filepath.Join(homeDir, "AppData/Roaming/Mozilla/Firefox/Profiles")
	default:
		return "", fmt.Errorf("unsupported operating system")
	}

	entries, err := os.ReadDir(firefoxDir)
	if err != nil {
		return "", fmt.Errorf("could not read Firefox profiles directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			name := entry.Name()
			if filepath.Ext(name) == ".default-release" {
				return filepath.Join(firefoxDir, name), nil
			}
		}
	}

	for _, entry := range entries {
		if entry.IsDir() {
			name := entry.Name()
			if filepath.Ext(name) == ".default" {
				return filepath.Join(firefoxDir, name), nil
			}
		}
	}

	return "", fmt.Errorf("could not find Firefox default profile")
}

// createFirefoxContainer creates a new Firefox container or returns an existing one
func createFirefoxContainer(name string) (string, error) {
	profileDir, err := getFirefoxProfileDir()
	if err != nil {
		return "", err
	}

	containersFile := filepath.Join(profileDir, "containers.json")
	var containers FirefoxContainers

	data, err := os.ReadFile(containersFile)
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("error reading containers file: %w", err)
	}

	if err == nil {
		if err := json.Unmarshal(data, &containers); err != nil {
			return "", fmt.Errorf("error parsing containers file: %w", err)
		}
	} else {
		containers = FirefoxContainers{
			Version:           5,
			LastUserContextId: 1,
			Identities:        []FirefoxContainer{},
		}
	}

	for _, container := range containers.Identities {
		if container.Name == name {
			return name, nil
		}
	}

	containers.LastUserContextId++
	newContainer := FirefoxContainer{
		Icon:          "fingerprint",
		Color:         "blue",
		Name:          name,
		Public:        true,
		UserContextID: containers.LastUserContextId,
	}
	containers.Identities = append(containers.Identities, newContainer)

	updatedData, err := json.MarshalIndent(containers, "", "  ")
	if err != nil {
		return "", fmt.Errorf("error marshaling containers: %w", err)
	}

	if err := os.WriteFile(containersFile, updatedData, 0644); err != nil {
		return "", fmt.Errorf("error writing containers file: %w", err)
	}

	return name, nil
}

// openURLInFirefoxContainer opens a URL in a Firefox container
func openURLInFirefoxContainer(targetURL, containerName string) error {
	containerName, err := createFirefoxContainer(containerName)
	if err != nil {
		return browser.OpenURL(targetURL)
	}

	var cmd *exec.Cmd
	containerURL := fmt.Sprintf("ext+container:name=%s&url=%s",
		url.QueryEscape(containerName),
		url.QueryEscape(targetURL))
	switch runtime.GOOS {
	case "darwin": // macOS
		cmd = exec.Command("/Applications/Firefox.app/Contents/MacOS/firefox",
			"--new-tab",
			containerURL)
	case "windows":
		cmd = exec.Command("C:\\Program Files\\Mozilla Firefox\\firefox.exe",
			"--new-tab",
			containerURL)
	case "linux":
		cmd = exec.Command("firefox",
			"--new-tab",
			containerURL)
	default:
		return browser.OpenURL(targetURL)
	}

	if err := cmd.Start(); err != nil {
		return browser.OpenURL(targetURL)
	}

	return nil
}
