package browser

import (
	"awsm/internal/config"
	"fmt"
	"os/exec"
	"runtime"

	"github.com/pkg/browser"
)

// It takes the friendly alias name as input.
func OpenURL(url, chromeProfileAlias string) error {
	// If no profile is requested, use the default simple method.
	if chromeProfileAlias == "" {
		return browser.OpenURL(url)
	}

	// Look up the alias to get the real directory name from the config file.
	profileDirectory := config.GetChromeProfileDirectory(chromeProfileAlias)

	var cmd *exec.Cmd
	profileArg := fmt.Sprintf("--profile-directory=%s", profileDirectory)

	// Build the command based on the operating system.
	switch runtime.GOOS {
	case "darwin": // macOS
		cmd = exec.Command("/Applications/Google Chrome.app/Contents/MacOS/Google Chrome", profileArg, url)
	case "windows":

		cmd = exec.Command("C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe", profileArg, url)
	case "linux":
		// Assumes 'google-chrome' is in the user's PATH
		cmd = exec.Command("google-chrome", profileArg, url)
	default:
		// For unsupported OS, fall back to the default browser.
		fmt.Printf("Unsupported OS for Chrome profiles. Opening in default browser.\n")
		return browser.OpenURL(url)
	}

	return cmd.Start()
}
