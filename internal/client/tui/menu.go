package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Hennnnnnn/DevWorkspace/internal/client/actions"
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

// pendingCountMsg carries the number of pending members across the caller's
// teams, for the badge on the Teams menu entry.
type pendingCountMsg struct{ count int }

func loadPendingCount() tea.Msg {
	if !actions.IsUnlocked() {
		return pendingCountMsg{}
	}
	teams, err := actions.ListTeams()
	if err != nil {
		return pendingCountMsg{}
	}
	count := 0
	for _, t := range teams {
		ms, err := actions.ListMembers(t.Name, true)
		if err != nil {
			continue
		}
		count += len(ms)
	}
	return pendingCountMsg{count: count}
}

func (m menuModel) Init() tea.Cmd { return loadPendingCount }

func (m menuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case pendingCountMsg:
		if msg.count > 0 {
			items := m.list.Items()
			for i, it := range items {
				mi, ok := it.(menuItem)
				if ok && mi.title == "Teams" {
					mi.desc = fmt.Sprintf("teams, members — %s", warningStyle.Render(fmt.Sprintf("%d pending invites", msg.count)))
					items[i] = mi
				}
			}
			m.list.SetItems(items)
		}
		return m, nil

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
