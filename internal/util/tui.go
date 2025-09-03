package util

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"
)

var (
	InfoColor    = color.New(color.FgCyan)
	SuccessColor = color.New(color.FgGreen)
	ErrorColor   = color.New(color.FgRed)
	WarnColor    = color.New(color.FgYellow)
	BoldColor    = color.New(color.Bold)
	quietMode    = false
)

func PrintTable(headers []string, data [][]string) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader(headers)
	table.SetAutoWrapText(false)
	table.SetAutoFormatHeaders(true)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetCenterSeparator("")
	table.SetColumnSeparator("")
	table.SetRowSeparator("")
	table.SetHeaderLine(false)
	table.SetBorder(false)
	table.SetTablePadding("  ")
	table.SetNoWhiteSpace(true)
	table.AppendBulk(data)
	table.Render()
}

// PrintTableWithBorders prints a table with borders and more compact formatting
func PrintTableWithBorders(headers []string, data [][]string) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader(headers)

	// Configure table style
	table.SetAutoWrapText(false)
	table.SetAutoFormatHeaders(true)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)

	// Set border characters for better alignment
	table.SetCenterSeparator("│")
	table.SetColumnSeparator("│")
	table.SetRowSeparator("─")

	// Enable borders
	table.SetHeaderLine(true)
	table.SetBorder(true)
	table.SetRowLine(true)

	// Critical: set minimal padding to ensure alignment
	table.SetTablePadding(" ")  // Single space between content and border
	table.SetNoWhiteSpace(true) // Remove any extra spacing

	// Add data and render
	table.AppendBulk(data)
	table.Render()
}

func PromptForInput(prompt string) (string, error) {
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
	for i := range slice {
		for j := i + 1; j < len(slice); j++ {
			if less(slice[j], slice[i]) {
				slice[i], slice[j] = slice[j], slice[i]
			}
		}
	}
}

// SetQuietMode enables or disables quiet mode for suppressing info messages
func SetQuietMode(quiet bool) {
	quietMode = quiet
	if quiet {
		// Disable color output when in quiet mode
		InfoColor.DisableColor()
		SuccessColor.DisableColor()
		WarnColor.DisableColor()
	}
}

// CreateCommand creates an exec.Cmd with the given command and arguments
func CreateCommand(name string, args ...string) *exec.Cmd {
	return exec.Command(name, args...)
}
