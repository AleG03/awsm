package tui

import (
	"fmt"
	"strings"

	"awsm/internal/aws"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

type EC2Item struct {
	instance aws.EC2Instance
}

func (i EC2Item) FilterValue() string {
	return i.instance.Name + " " + i.instance.InstanceID
}

func (i EC2Item) Title() string {
	if i.instance.Name != "" {
		return fmt.Sprintf("%s (%s)", i.instance.Name, i.instance.InstanceID)
	}
	return i.instance.InstanceID
}

func (i EC2Item) Description() string {
	var parts []string

	// Add state
	state := i.instance.State
	switch state {
	case "running":
		parts = append(parts, SuccessStyle.Render(state))
	default:
		parts = append(parts, MutedStyle.Render(state))
	}

	// Add IPs
	if i.instance.PrivateIP != "" {
		parts = append(parts, MutedStyle.Render("Priv: "+i.instance.PrivateIP))
	}
	if i.instance.PublicIP != "" {
		parts = append(parts, MutedStyle.Render("Pub: "+i.instance.PublicIP))
	}

	return strings.Join(parts, " • ")
}

type EC2SelectorModel struct {
	list     list.Model
	choice   aws.EC2Instance
	quitting bool
}

func NewEC2Selector(instances []aws.EC2Instance) EC2SelectorModel {
	items := make([]list.Item, len(instances))
	for i, instance := range instances {
		items[i] = EC2Item{instance: instance}
	}

	const defaultWidth = 80
	const listHeight = 14

	l := list.New(items, list.NewDefaultDelegate(), defaultWidth, listHeight)
	l.Title = HeaderStyle.Render("🚀 Select EC2 Instance")
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.Styles.Title = HeaderStyle
	l.Styles.PaginationStyle = MutedStyle
	l.Styles.HelpStyle = MutedStyle

	return EC2SelectorModel{list: l}
}

func (m EC2SelectorModel) Init() tea.Cmd {
	return nil
}

func (m EC2SelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
		switch keypress := msg.String(); keypress {
		case "ctrl+c":
			m.quitting = true
			return m, tea.Quit

		case "enter":
			i, ok := m.list.SelectedItem().(EC2Item)
			if ok {
				m.choice = i.instance
			}
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m EC2SelectorModel) View() string {
	if m.choice.InstanceID != "" {
		name := m.choice.Name
		if name == "" {
			name = m.choice.InstanceID
		}
		return SuccessStyle.Render(fmt.Sprintf("✓ Selected instance: %s", name))
	}
	if m.quitting {
		return MutedStyle.Render("Operation cancelled.")
	}
	return "\n" + m.list.View()
}

// SelectEC2Instance shows an interactive EC2 selector
func SelectEC2Instance(profile, region string) (aws.EC2Instance, error) {
	instances, err := aws.ListRunningInstances(profile, region)
	if err != nil {
		return aws.EC2Instance{}, err
	}

	if len(instances) == 0 {
		return aws.EC2Instance{}, fmt.Errorf("no running instances found in region %s", region)
	}

	model := NewEC2Selector(instances)
	program := tea.NewProgram(model, tea.WithAltScreen())

	finalModel, err := program.Run()
	if err != nil {
		return aws.EC2Instance{}, err
	}

	if m, ok := finalModel.(EC2SelectorModel); ok {
		if m.choice.InstanceID == "" && !m.quitting {
			return aws.EC2Instance{}, fmt.Errorf("no instance selected")
		}
		return m.choice, nil
	}

	return aws.EC2Instance{}, fmt.Errorf("unexpected model type")
}
