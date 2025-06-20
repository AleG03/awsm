package tui

import (
	"fmt"
	"strings"

	"awsm/internal/aws"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

type ProfileItem struct {
	profile aws.ProfileInfo
}

func (i ProfileItem) FilterValue() string { return i.profile.Name }
func (i ProfileItem) Title() string       { return i.profile.Name }
func (i ProfileItem) Description() string {
	var parts []string

	// Add type with color
	switch i.profile.Type {
	case aws.ProfileTypeSSO:
		parts = append(parts, ProfileSSO.Render("SSO"))
	case aws.ProfileTypeIAM:
		parts = append(parts, ProfileIAM.Render("IAM"))
	case aws.ProfileTypeKey:
		parts = append(parts, ProfileKey.Render("Key"))
	}

	// Add region
	if i.profile.Region != "" {
		parts = append(parts, MutedStyle.Render(i.profile.Region))
	}

	// Add account ID for SSO
	if i.profile.SSOAccountID != "" {
		parts = append(parts, MutedStyle.Render("("+i.profile.SSOAccountID+")"))
	}

	return strings.Join(parts, " • ")
}

type ProfileSelectorModel struct {
	list     list.Model
	choice   string
	quitting bool
}

func NewProfileSelector(profiles []aws.ProfileInfo) ProfileSelectorModel {
	items := make([]list.Item, len(profiles))
	for i, profile := range profiles {
		items[i] = ProfileItem{profile: profile}
	}

	const defaultWidth = 80
	const listHeight = 14

	l := list.New(items, list.NewDefaultDelegate(), defaultWidth, listHeight)
	l.Title = HeaderStyle.Render("🚀 Select AWS Profile")
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.Styles.Title = HeaderStyle
	l.Styles.PaginationStyle = MutedStyle
	l.Styles.HelpStyle = MutedStyle

	return ProfileSelectorModel{list: l}
}

func (m ProfileSelectorModel) Init() tea.Cmd {
	return nil
}

func (m ProfileSelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// Use most of the terminal space with minimal padding
		width := msg.Width - 2
		height := msg.Height - 4

		// Set minimum constraints only
		if width < 40 {
			width = 40
		}
		if height < 10 {
			height = 10
		}

		m.list.SetWidth(width)
		m.list.SetHeight(height)
		return m, nil

	case tea.KeyMsg:
		switch keypress := msg.String(); keypress {
		case "ctrl+c":
			m.quitting = true
			return m, tea.Quit

		case "enter":
			i, ok := m.list.SelectedItem().(ProfileItem)
			if ok {
				m.choice = i.profile.Name
			}
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m ProfileSelectorModel) View() string {
	if m.choice != "" {
		return SuccessStyle.Render(fmt.Sprintf("✓ Selected profile: %s", m.choice))
	}
	if m.quitting {
		return MutedStyle.Render("Operation cancelled.")
	}
	return "\n" + m.list.View()
}

// SelectProfile shows an interactive profile selector
func SelectProfile() (string, error) {
	profiles, err := aws.ListProfilesDetailed()
	if err != nil {
		return "", err
	}

	if len(profiles) == 0 {
		return "", fmt.Errorf("no profiles found")
	}

	model := NewProfileSelector(profiles)
	program := tea.NewProgram(model, tea.WithAltScreen())

	finalModel, err := program.Run()
	if err != nil {
		return "", err
	}

	if m, ok := finalModel.(ProfileSelectorModel); ok {
		return m.choice, nil
	}

	return "", fmt.Errorf("unexpected model type")
}
