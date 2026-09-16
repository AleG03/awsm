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

// osaQuote renders a Go string as an AppleScript string literal.
//
// A literal newline inside an AppleScript string is a syntax error, not a line
// break, so it has to become the two-character escape. Same for quotes, which
// a profile name is free to contain.
func osaQuote(s string) string {
	r := strings.NewReplacer(
		`\`, `\\`,
		`"`, `\"`,
		"\n", `\n`,
		"\r", `\r`,
		"\t", `\t`,
	)
	return `"` + r.Replace(s) + `"`
}
