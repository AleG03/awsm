package tui

import (
	"fmt"
	"os"
	"strings"

	"awsm/internal/aws"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-isatty"
)

type regionItem struct {
	name string
}

func (i regionItem) FilterValue() string { return i.name }
func (i regionItem) Title() string       { return i.name }
func (i regionItem) Description() string {
	return MutedStyle.Render(getRegionDescription(i.name))
}

type regionSelectorModel struct {
	list     list.Model
	choice   string
	quitting bool
}

func newRegionSelector(regions []string) regionSelectorModel {
	items := make([]list.Item, len(regions))
	for i, r := range regions {
		items[i] = regionItem{name: r}
	}

	const defaultWidth = 60
	const listHeight = 16

	l := list.New(items, list.NewDefaultDelegate(), defaultWidth, listHeight)
	l.Title = "🌍 Select AWS Region"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.Styles.Title = HeaderStyle
	l.Styles.PaginationStyle = MutedStyle
	l.Styles.HelpStyle = MutedStyle

	return regionSelectorModel{list: l}
}

func (m regionSelectorModel) Init() tea.Cmd {
	return nil
}

func (m regionSelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		width := msg.Width - 2
		height := msg.Height - 4
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
		switch msg.String() {
		case "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "enter":
			i, ok := m.list.SelectedItem().(regionItem)
			if ok {
				m.choice = i.name
			}
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m regionSelectorModel) View() string {
	if m.choice != "" {
		return fmt.Sprintf("%s %s\n",
			SuccessStyle.Render("✓ Region:"),
			InfoStyle.Render(m.choice))
	}
	if m.quitting {
		return MutedStyle.Render("  Cancelled.") + "\n"
	}
	return "\n" + m.list.View()
}

// SelectRegion shows an interactive region selector with fuzzy search
func SelectRegion() (string, error) {
	if !isatty.IsTerminal(os.Stdout.Fd()) {
		fmt.Fprint(os.Stderr, "Region: ")
		var input string
		_, err := fmt.Scanln(&input)
		return strings.TrimSpace(input), err
	}

	regions := aws.GetAllRegions()
	if len(regions) == 0 {
		return "", fmt.Errorf("no regions available")
	}

	model := newRegionSelector(regions)
	program := tea.NewProgram(model, tea.WithAltScreen())

	finalModel, err := program.Run()
	if err != nil {
		return "", err
	}

	if m, ok := finalModel.(regionSelectorModel); ok {
		if m.quitting && m.choice == "" {
			return "", fmt.Errorf("region selection cancelled")
		}
		return m.choice, nil
	}

	return "", fmt.Errorf("unexpected model type")
}

func getRegionDescription(region string) string {
	descriptions := map[string]string{
		"us-east-1":      "N. Virginia",
		"us-east-2":      "Ohio",
		"us-west-1":      "N. California",
		"us-west-2":      "Oregon",
		"af-south-1":     "Cape Town",
		"ap-east-1":      "Hong Kong",
		"ap-south-1":     "Mumbai",
		"ap-south-2":     "Hyderabad",
		"ap-southeast-1": "Singapore",
		"ap-southeast-2": "Sydney",
		"ap-southeast-3": "Jakarta",
		"ap-southeast-4": "Melbourne",
		"ap-northeast-1": "Tokyo",
		"ap-northeast-2": "Seoul",
		"ap-northeast-3": "Osaka",
		"ca-central-1":   "Canada",
		"ca-west-1":      "Calgary",
		"eu-central-1":   "Frankfurt",
		"eu-central-2":   "Zurich",
		"eu-west-1":      "Ireland",
		"eu-west-2":      "London",
		"eu-west-3":      "Paris",
		"eu-south-1":     "Milan",
		"eu-south-2":     "Spain",
		"eu-north-1":     "Stockholm",
		"il-central-1":   "Tel Aviv",
		"me-south-1":     "Bahrain",
		"me-central-1":   "UAE",
		"sa-east-1":      "São Paulo",
		"eusc-de-east-1": "EU Sovereign Cloud",
	}

	if desc, ok := descriptions[region]; ok {
		return desc
	}
	return ""
}
