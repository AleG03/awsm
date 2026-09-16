package daemon

import (
	"os/exec"
	"strings"
)

// notify uses osascript, which ships with macOS.
//
// The notification is deliberately not clickable: making it run a command would
// mean shipping an .app bundle or depending on terminal-notifier, and the
// alternative without dependencies is a modal dialog that steals focus. A
// daemon should not take over the screen to tell you a token expired.
func notify(title, message string) error {
	script := "display notification " + osaQuote(message) +
		" with title " + osaQuote(title)
	return exec.Command("osascript", "-e", script).Run()
}

// osaQuote renders a Go string as an AppleScript string literal. AppleScript
// escapes with backslashes, and an unescaped quote in a profile name would turn
// the message into a syntax error.
func osaQuote(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`)
	return `"` + r.Replace(s) + `"`
}
