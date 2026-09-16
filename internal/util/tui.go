package util

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/fatih/color"
)

var (
	InfoColor    = color.New(color.FgCyan)
	SuccessColor = color.New(color.FgGreen)
	ErrorColor   = color.New(color.FgRed)
	WarnColor    = color.New(color.FgYellow)
	BoldColor    = color.New(color.Bold)
)

// ErrNonInteractive is returned when input is requested from a process that
// has declared it has nobody to ask.
var ErrNonInteractive = errors.New("input required but this process is running unattended")

// nonInteractive is set by processes that must never block on a prompt, such
// as the scheduled credential refresh.
//
// Relying on stdin being closed is not enough: it makes the failure depend on
// how the process happened to be started, and under a service manager a prompt
// can hang rather than fail. Declaring it turns a hang into an immediate,
// explainable error.
var nonInteractive bool

// SetNonInteractive declares that no user is present to answer prompts.
func SetNonInteractive(v bool) { nonInteractive = v }

// IsNonInteractive reports whether prompting has been disabled.
func IsNonInteractive() bool { return nonInteractive }

func PromptForInput(prompt string) (string, error) {
	if nonInteractive {
		return "", ErrNonInteractive
	}
	reader := bufio.NewReader(os.Stdin)
	fmt.Print(prompt)
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(input), nil
}

// SortBy sorts a slice using the provided less function
func SortBy[T any](slice []T, less func(T, T) bool) {
	sort.Slice(slice, func(i, j int) bool {
		return less(slice[i], slice[j])
	})
}
