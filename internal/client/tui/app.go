// Package tui is the devsync interactive terminal UI, built on Bubble Tea.
// It calls the same internal/client/actions functions as the CLI.
package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Hennnnnnn/DevWorkspace/internal/client/actions"
)

// rootModel drives a lazygit/k9s-style drill-down stack: top menu -> list ->
// detail -> actions. Esc pops the stack; the top menu is never popped.
type rootModel struct {
	stack  []tea.Model
	width  int
	height int
}

func newRootModel() rootModel {
	return rootModel{stack: []tea.Model{newMenuModel(0, 0)}}
}

func (m rootModel) Init() tea.Cmd {
	if actions.IsUnlocked() {
		return nil
	}
	return pushView(newUnlockView())
}

func (m rootModel) top() tea.Model {
	return m.stack[len(m.stack)-1]
}

func (m rootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		// Propagate the size to every view on the stack, not just the top
		// one, so views resize correctly when popped back into view.
		for i, v := range m.stack {
			updated, _ := v.Update(msg)
			m.stack[i] = updated
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			if len(m.stack) > 1 {
				m.stack = m.stack[:len(m.stack)-1]
				return m, nil
			}
			return m, tea.Quit
		}

	case pushMsg:
		m.stack = append(m.stack, msg.model)
		return m, msg.model.Init()

	case popMsg:
		if len(m.stack) > 1 {
			m.stack = m.stack[:len(m.stack)-1]
		}
		return m, nil
	}

	updated, cmd := m.top().Update(msg)
	m.stack[len(m.stack)-1] = updated
	return m, cmd
}

func (m rootModel) View() string {
	return m.top().View()
}

// Run starts the TUI.
func Run() error {
	_, err := tea.NewProgram(newRootModel(), tea.WithAltScreen()).Run()
	return err
}
