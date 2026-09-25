package browser

import (
	"awsm/internal/config"
	"awsm/internal/tool"
	"fmt"
	"net/url"
	"os/exec"
	"runtime"

	"github.com/pkg/browser"
)

// buildContainerURL constructs the ext+container: URL handled by the
// AWSM Container Opener extension (see extensions/firefox-container).
// Color and icon are intentionally omitted: the extension derives them
// deterministically from the container name so the same profile always
// renders with the same visual identity.
func buildContainerURL(containerName, targetURL string) string {
	return fmt.Sprintf("ext+container:name=%s&url=%s",
		url.QueryEscape(containerName),
		url.QueryEscape(targetURL))
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
		cmd = tool.Command("google-chrome", profileArg, targetURL)
	default:
		return browser.OpenURL(targetURL)
	}

	return cmd.Start()
}

// openURLInFirefoxContainer opens a URL in the named Firefox container via
// the AWSM Container Opener extension.
func openURLInFirefoxContainer(targetURL, containerName string) error {
	cmd, err := firefoxContainerCommand(runtime.GOOS, buildContainerURL(containerName, targetURL))
	if err != nil {
		return err
	}
	return launchFirefoxContainer(cmd, runtime.GOOS)
}

func firefoxContainerCommand(platform, containerURL string) (*exec.Cmd, error) {
	switch platform {
	case "darwin":
		// Use macOS application dispatch, just like opening a link from another app.
		// Launching the bundle's executable directly can fail to load the Firefox
		// profile even while the same profile works in the normally launched app.
		return exec.Command("/usr/bin/open", "-a", "Firefox", containerURL), nil
	case "windows":
		return exec.Command(`C:\Program Files\Mozilla Firefox\firefox.exe`, "--new-tab", containerURL), nil
	case "linux":
		return tool.Command("firefox", "--new-tab", containerURL), nil
	default:
		return nil, fmt.Errorf("Firefox containers are not supported on %s", platform)
	}
}

func launchFirefoxContainer(cmd *exec.Cmd, platform string) error {
	var err error
	if platform == "darwin" {
		// open exits after handing off the URL, so wait for dispatch errors (for
		// example Firefox not being installed), without waiting for Firefox to quit.
		err = cmd.Run()
	} else {
		err = cmd.Start()
		if err == nil {
			go func() { _ = cmd.Wait() }()
		}
	}
	if err != nil {
		// A container request must not silently open an AWS session in the default
		// browser when the requested browser could not be launched.
		return fmt.Errorf("could not open Firefox container: %w", err)
	}
	return nil
}

// openURLInZenContainer opens a URL in the named Zen Browser container via
// the AWSM Container Opener extension (Zen reuses the same ext+container:
// URL scheme as Firefox).
func openURLInZenContainer(targetURL, containerName string) error {
	var cmd *exec.Cmd
	containerURL := buildContainerURL(containerName, targetURL)

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
		cmd = tool.Command("zen",
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
