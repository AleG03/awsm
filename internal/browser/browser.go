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
	// If no profile/container is requested, use the default simple method
	if chromeProfileAlias == "" && firefoxContainer == "" {
		return browser.OpenURL(targetURL)
	}

	// Handle Chrome profile if specified
	if chromeProfileAlias != "" {
		return openURLInChromeProfile(targetURL, chromeProfileAlias)
	}

	// Handle Firefox container if specified
	if firefoxContainer != "" {
		return openURLInFirefoxContainer(targetURL, firefoxContainer)
	}

	return nil
}

// openURLInChromeProfile opens a URL in a specific Chrome profile
func openURLInChromeProfile(targetURL, chromeProfileAlias string) error {
	// Look up the alias to get the real directory name from the config file
	profileDirectory := config.GetChromeProfileDirectory(chromeProfileAlias)

	var cmd *exec.Cmd
	profileArg := fmt.Sprintf("--profile-directory=%s", profileDirectory)

	// Build the command based on the operating system
	switch runtime.GOOS {
	case "darwin": // macOS
		cmd = exec.Command("/Applications/Google Chrome.app/Contents/MacOS/Google Chrome", profileArg, targetURL)
	case "windows":
		cmd = exec.Command("C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe", profileArg, targetURL)
	case "linux":
		// Assumes 'google-chrome' is in the user's PATH
		cmd = exec.Command("google-chrome", profileArg, targetURL)
	default:
		// For unsupported OS, fall back to the default browser
		fmt.Printf("Unsupported OS for Chrome profiles. Opening in default browser.\n")
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

	// Look for the default profile
	for _, entry := range entries {
		if entry.IsDir() {
			name := entry.Name()
			if filepath.Ext(name) == ".default-release" {
				return filepath.Join(firefoxDir, name), nil
			}
		}
	}

	// Try .default if .default-release is not found
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

	// Read existing containers file
	data, err := os.ReadFile(containersFile)
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("error reading containers file: %w", err)
	}

	if err == nil {
		if err := json.Unmarshal(data, &containers); err != nil {
			return "", fmt.Errorf("error parsing containers file: %w", err)
		}
	} else {
		// Initialize new containers file
		containers = FirefoxContainers{
			Version:           5,
			LastUserContextId: 1,
			Identities:        []FirefoxContainer{},
		}
	}

	// Check if container already exists
	for _, container := range containers.Identities {
		if container.Name == name {
			return name, nil
		}
	}

	// Create new container
	containers.LastUserContextId++
	newContainer := FirefoxContainer{
		Icon:          "fingerprint",
		Color:         "blue",
		Name:          name,
		Public:        true,
		UserContextID: containers.LastUserContextId,
	}
	containers.Identities = append(containers.Identities, newContainer)

	// Write updated containers file
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
	fmt.Printf("Debug: Ensuring Firefox container exists: %s\n", containerName)

	// Create or get container
	containerName, err := createFirefoxContainer(containerName)
	if err != nil {
		fmt.Printf("Warning: Could not create/get container: %v\nOpening in default context.\n", err)
		return browser.OpenURL(targetURL)
	}

	fmt.Printf("Debug: Opening URL in container: %s\n", containerName)

	var cmd *exec.Cmd

	// Format container URL with proper encoding
	containerURL := fmt.Sprintf("ext+container:name=%s&url=%s",
		url.QueryEscape(containerName),
		url.QueryEscape(targetURL))

	// Build the command based on the operating system
	switch runtime.GOOS {
	case "darwin": // macOS
		cmd = exec.Command("/Applications/Firefox.app/Contents/MacOS/firefox",
			"--new-tab",
			containerURL)
		fmt.Printf("Debug: Running command: %v\n", cmd.Args)
	case "windows":
		cmd = exec.Command("C:\\Program Files\\Mozilla Firefox\\firefox.exe",
			"--new-tab",
			containerURL)
	case "linux":
		cmd = exec.Command("firefox",
			"--new-tab",
			containerURL)
	default:
		fmt.Printf("Unsupported OS for Firefox containers. Opening in default browser.\n")
		return browser.OpenURL(targetURL)
	}

	if err := cmd.Start(); err != nil {
		fmt.Printf("Warning: Could not start Firefox: %v\nOpening in default browser.\n", err)
		return browser.OpenURL(targetURL)
	}

	return nil
}
