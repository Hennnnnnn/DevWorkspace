package tui

import (
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

// menuItem is a top-level menu entry. It satisfies list.Item and knows how
// to build the view it opens.
type menuItem struct {
	title, desc string
	open        func() tea.Model
}

func (i menuItem) Title() string       { return i.title }
func (i menuItem) Description() string { return i.desc }
func (i menuItem) FilterValue() string { return i.title }

func newTopMenu(width, height int) list.Model {
	items := []list.Item{
		menuItem{title: "Vaults", desc: "browse vaults and files", open: func() tea.Model { return newVaultsView(width, height) }},
		menuItem{title: "Teams", desc: "teams, members, approvals", open: func() tea.Model { return newTeamsView(width, height) }},
		menuItem{title: "Devices", desc: "your registered devices", open: func() tea.Model { return newDevicesView(width, height) }},
		menuItem{title: "Audit", desc: "vault audit log", open: func() tea.Model { return newAuditPickerView(width, height) }},
	}
	l := list.New(items, list.NewDefaultDelegate(), width, height)
	l.Title = "devsync"
	l.SetShowHelp(true)
	// rootModel owns quit (ctrl+c) and back (esc) globally.
	l.DisableQuitKeybindings()
	return l
}

type menuModel struct {
	list list.Model
}

func newMenuModel(width, height int) menuModel {
	return menuModel{list: newTopMenu(width, height)}
}

func (m menuModel) Init() tea.Cmd { return nil }

func (m menuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetSize(msg.Width, msg.Height)
		return m, nil
	case tea.KeyMsg:
		if msg.String() == "enter" {
			if it, ok := m.list.SelectedItem().(menuItem); ok {
				return m, pushView(it.open())
			}
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m menuModel) View() string {
	return m.list.View()
}
