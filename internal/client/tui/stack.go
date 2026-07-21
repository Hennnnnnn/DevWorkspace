package tui

import tea "github.com/charmbracelet/bubbletea"

// pushMsg asks the root model to push a new view onto the drill-down stack.
type pushMsg struct{ model tea.Model }

// popMsg asks the root model to pop the current view (go back).
type popMsg struct{}

// pushView returns a Cmd that navigates forward into model.
func pushView(model tea.Model) tea.Cmd {
	return func() tea.Msg { return pushMsg{model: model} }
}

// popView is a Cmd that navigates back to the previous view.
func popView() tea.Msg { return popMsg{} }

// replaceViewMsg asks the root model to replace the ENTIRE stack with model.
// Used by the onboarding flow (wizard → waiting → menu) where "back" makes
// no sense.
type replaceViewMsg struct{ model tea.Model }
