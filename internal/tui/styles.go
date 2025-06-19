package tui

import (
	"github.com/charmbracelet/lipgloss"
)

var (
	// Colors
	Primary   = lipgloss.Color("#00D9FF")
	Secondary = lipgloss.Color("#7C3AED")
	Success   = lipgloss.Color("#10B981")
	Warning   = lipgloss.Color("#F59E0B")
	Error     = lipgloss.Color("#EF4444")
	Muted     = lipgloss.Color("#6B7280")

	// Base styles
	BaseStyle = lipgloss.NewStyle().
			Padding(0, 1)

	HeaderStyle = lipgloss.NewStyle().
			Foreground(Primary).
			Bold(true).
			Padding(1, 0)

	SuccessStyle = lipgloss.NewStyle().
			Foreground(Success).
			Bold(true)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(Error).
			Bold(true)

	WarningStyle = lipgloss.NewStyle().
			Foreground(Warning).
			Bold(true)

	InfoStyle = lipgloss.NewStyle().
			Foreground(Primary)

	MutedStyle = lipgloss.NewStyle().
			Foreground(Muted)

	// Profile styles
	ProfileActiveStyle = lipgloss.NewStyle().
				Foreground(Success).
				Bold(true)

	ProfileSSO = lipgloss.NewStyle().
			Foreground(Primary)

	ProfileIAM = lipgloss.NewStyle().
			Foreground(Secondary)

	ProfileKey = lipgloss.NewStyle().
			Foreground(Warning)

	// Box styles
	BoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(Primary).
			Padding(1, 2).
			Margin(1, 0)

	// Spinner style
	SpinnerStyle = lipgloss.NewStyle().
			Foreground(Primary)
)
