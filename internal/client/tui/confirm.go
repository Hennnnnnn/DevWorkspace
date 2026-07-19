package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// confirmModel is a generic yes/no confirmation dialog.
// When confirmed it pops itself and runs cmd, whose result message
// (typically actionDoneMsg) is received by the parent view.
type confirmModel struct {
	prompt string
	cmd    tea.Cmd
}

func newConfirmView(prompt string, cmd tea.Cmd) tea.Model {
	return confirmModel{prompt: prompt, cmd: cmd}
}

func (m confirmModel) Init() tea.Cmd { return nil }

func (m confirmModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "y", "Y":
			return m, tea.Sequence(
				func() tea.Msg { return popMsg{} },
				m.cmd,
			)
		case "n", "N", "esc":
			return m, func() tea.Msg { return popMsg{} }
		}
	}
	return m, nil
}

func (m confirmModel) View() string {
	return "\n  " + dangerStyle.Render(m.prompt) + "\n\n  " + dangerStyle.Render("y") + " yes   " + infoStyle.Render("n") + " no\n"
}
