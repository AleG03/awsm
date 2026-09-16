package daemon

import (
	"os/exec"
	"strings"
	"testing"
)

func TestOsaQuoteProducesValidAppleScript(t *testing.T) {
	// A literal newline used to slip through and make osascript fail with a
	// syntax error, which the dialog code then swallowed: the alert simply
	// never appeared. Running the real interpreter is the only check that
	// would have caught it.
	for _, in := range []string{
		"plain",
		"two\n\nlines",
		`quote " inside`,
		`back \ slash`,
		"tab\there",
		`profile "prod" needs MFA` + "\n" + `awsm profile set prod`,
	} {
		t.Run(in, func(t *testing.T) {
			script := "return " + osaQuote(in)
			out, err := exec.Command("osascript", "-e", script).CombinedOutput()
			if err != nil {
				t.Fatalf("osascript rejected %q: %v\n%s", in, err, out)
			}
			// osascript prints the string back with real newlines turned into
			// its own line separators, so compare on the first line only.
			got := strings.SplitN(strings.TrimRight(string(out), "\n"), "\n", 2)[0]
			want := strings.SplitN(in, "\n", 2)[0]
			if got != want {
				t.Errorf("round trip gave %q, want %q", got, want)
			}
		})
	}
}
