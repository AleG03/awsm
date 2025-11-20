package browser

import (
	"awsm/internal/config"
	"crypto/rand"
	"fmt"
	"math/big"
	"net/url"
	"os/exec"
	"runtime"

	"github.com/pkg/browser"
)

// Available Firefox container colors (from the extension documentation)
var firefoxColors = []string{
	"blue", "turquoise", "green", "yellow",
	"orange", "red", "pink", "purple",
}

// Available Firefox container icons (from the extension documentation)
var firefoxIcons = []string{
	"fingerprint", "briefcase", "dollar", "cart", "circle",
	"gift", "vacation", "food", "fruit", "pet", "tree", "chill",
}

// getRandomFirefoxColor returns a random color from the available Firefox container colors
func getRandomFirefoxColor() string {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(firefoxColors))))
	if err != nil {
		// Fallback to a default color if random generation fails
		return "blue"
	}
	return firefoxColors[n.Int64()]
}

// getRandomFirefoxIcon returns a random icon from the available Firefox container icons
func getRandomFirefoxIcon() string {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(firefoxIcons))))
	if err != nil {
		// Fallback to a default icon if random generation fails
		return "fingerprint"
	}
	return firefoxIcons[n.Int64()]
}

// OpenURL opens a URL in the specified browser profile/container.
// It takes the URL and either a Chrome profile alias, Firefox container name, or Zen container name.
func OpenURL(targetURL, chromeProfileAlias string, firefoxContainer string, zenContainer string) error {
	if chromeProfileAlias == "" && firefoxContainer == "" && zenContainer == "" {
		return browser.OpenURL(targetURL)
	}

	if chromeProfileAlias != "" {
		return openURLInChromeProfile(targetURL, chromeProfileAlias)
	}

	if firefoxContainer != "" {
		return openURLInFirefoxContainer(targetURL, firefoxContainer)
	}

	if zenContainer != "" {
		return openURLInZenContainer(targetURL, zenContainer)
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

// openURLInFirefoxContainer opens a URL in a Firefox container with random color and icon
func openURLInFirefoxContainer(targetURL, containerName string) error {
	// Generate random color and icon for the container
	randomColor := getRandomFirefoxColor()
	randomIcon := getRandomFirefoxIcon()

	var cmd *exec.Cmd
	// Use the extension's URL format with color and icon parameters
	// This will create the container with random color/icon if it doesn't exist
	containerURL := fmt.Sprintf("ext+container:name=%s&color=%s&icon=%s&url=%s",
		url.QueryEscape(containerName),
		url.QueryEscape(randomColor),
		url.QueryEscape(randomIcon),
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

// openURLInZenContainer opens a URL in a Zen Browser container with random color and icon
func openURLInZenContainer(targetURL, containerName string) error {
	// Generate random color and icon for the container
	randomColor := getRandomFirefoxColor()
	randomIcon := getRandomFirefoxIcon()

	var cmd *exec.Cmd
	// Use the extension's URL format with color and icon parameters
	// Zen Browser supports the same ext+container: URL scheme as Firefox
	containerURL := fmt.Sprintf("ext+container:name=%s&color=%s&icon=%s&url=%s",
		url.QueryEscape(containerName),
		url.QueryEscape(randomColor),
		url.QueryEscape(randomIcon),
		url.QueryEscape(targetURL))

	switch runtime.GOOS {
	case "darwin": // macOS
		cmd = exec.Command("/Applications/Zen.app/Contents/MacOS/zen",
			"--new-tab",
			containerURL)
	case "windows":
		cmd = exec.Command("C:\\Program Files\\Zen\\zen.exe",
			"--new-tab",
			containerURL)
	case "linux":
		cmd = exec.Command("zen",
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
