// Package tui is the devsync interactive terminal UI, built on Bubble Tea.
// It calls the same internal/client/actions functions as the CLI.
package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Hennnnnnn/DevWorkspace/internal/client/actions"
	"github.com/Hennnnnnn/DevWorkspace/internal/client/config"
	"github.com/Hennnnnnn/DevWorkspace/internal/client/keystore"
)

// rootModel drives a lazygit/k9s-style drill-down stack: top menu -> list ->
// detail -> actions. Esc pops the stack; the top menu is never popped.
type rootModel struct {
	stack  []tea.Model
	width  int
	height int
}

func newRootModel() rootModel {
	// First run (no keystore or never registered) → onboarding wizard.
	cfg, _ := config.Load()
	if !keystore.Exists() || cfg == nil || cfg.Username == "" {
		return rootModel{stack: []tea.Model{newWizardView()}}
	}
	return rootModel{stack: []tea.Model{newMenuModel(0, 0)}}
}

func (m rootModel) Init() tea.Cmd {
	base := m.stack[0]
	if _, onboarding := base.(wizardModel); onboarding {
		return base.Init()
	}
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
		h := msg.Height - headerHeight
		if h < 1 {
			h = 1
		}
		adjusted := tea.WindowSizeMsg{Width: msg.Width, Height: h}
		for i, v := range m.stack {
			updated, _ := v.Update(adjusted)
			m.stack[i] = updated
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			if fv, ok := m.top().(filterableView); ok && fv.isFiltering() {
				break // forward to child, list clears filter
			}
			if len(m.stack) > 1 {
				m.stack = m.stack[:len(m.stack)-1]
				return m, nil
			}
			return m, tea.Quit
		}

	case pushMsg:
		if m.width > 0 && m.height > 0 {
			h := m.height - headerHeight
			if h < 1 {
				h = 1
			}
			updated, _ := msg.model.Update(tea.WindowSizeMsg{Width: m.width, Height: h})
			msg.model = updated
		}
		m.stack = append(m.stack, msg.model)
		return m, msg.model.Init()

	case popMsg:
		if len(m.stack) > 1 {
			m.stack = m.stack[:len(m.stack)-1]
		}
		return m, nil

	case replaceViewMsg:
		if m.width > 0 && m.height > 0 {
			h := m.height - headerHeight
			if h < 1 {
				h = 1
			}
			updated, _ := msg.model.Update(tea.WindowSizeMsg{Width: m.width, Height: h})
			msg.model = updated
		}
		m.stack = []tea.Model{msg.model}
		return m, msg.model.Init()
	}

	updated, cmd := m.top().Update(msg)
	m.stack[len(m.stack)-1] = updated
	return m, cmd
}

func (m rootModel) View() string {
	return RenderHeader(m.width) + m.top().View()
}

// Run starts the TUI.
func Run() error {
	_, err := tea.NewProgram(newRootModel(), tea.WithAltScreen()).Run()
	return err
}
