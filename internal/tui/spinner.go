package tui

import (
	"context"
	"fmt"
	"os"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-isatty"
)

type SpinnerModel struct {
	spinner  spinner.Model
	message  string
	quitting bool
	err      error
}

func NewSpinner(message string) SpinnerModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = SpinnerStyle
	return SpinnerModel{
		spinner: s,
		message: message,
	}
}

func (m SpinnerModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m SpinnerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case error:
		m.err = msg
		return m, tea.Quit
	case string:
		if msg == "done" {
			m.quitting = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m SpinnerModel) View() string {
	if m.err != nil {
		return ErrorStyle.Render("✗ " + m.err.Error())
	}
	if m.quitting {
		return SuccessStyle.Render("✓ " + m.message + " completed")
	}
	return fmt.Sprintf("%s %s", m.spinner.View(), InfoStyle.Render(m.message))
}

// ShowSpinner displays a spinner for a long-running operation
func ShowSpinner(ctx context.Context, message string, fn func() error) error {
	// Check if we have a TTY, if not, just run the function with simple output
	if !isatty.IsTerminal(os.Stdout.Fd()) {
		fmt.Fprintf(os.Stderr, "%s %s...\n", "⏳", InfoStyle.Render(message))
		err := fn()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s %s\n", ErrorStyle.Render("✗"), err.Error())
		} else {
			fmt.Fprintf(os.Stderr, "%s %s completed\n", SuccessStyle.Render("✓"), message)
		}
		return err
	}

	model := NewSpinner(message)

	program := tea.NewProgram(model)

	go func() {
		defer program.Send("done")
		if err := fn(); err != nil {
			program.Send(err)
		}
	}()

	m, err := program.Run()
	if err != nil {
		return err
	}

	if model, ok := m.(SpinnerModel); ok {
		return model.err
	}

	return nil
}
