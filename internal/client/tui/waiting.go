package tui

import (
	"time"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Hennnnnnn/DevWorkspace/internal/client/actions"
)

// waitingModel is the "Waiting for Approval" screen for teammates whose
// account is still pending. Polls /whoami and switches to the normal menu
// automatically once an admin approves.

const pollInterval = 5 * time.Second

type pollTickMsg struct{}

type copyResetMsg struct{}

func clearCopiedAfter(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg { return copyResetMsg{} })
}

type pollResultMsg struct {
	status string
	err    error
}

func pollTick() tea.Cmd {
	return tea.Tick(pollInterval, func(time.Time) tea.Msg { return pollTickMsg{} })
}

func doPoll() tea.Cmd {
	return func() tea.Msg {
		who, err := actions.WhoAmI()
		if err != nil {
			return pollResultMsg{err: err}
		}
		return pollResultMsg{status: who.Status}
	}
}

type waitingModel struct {
	username    string
	fingerprint string
	lastErr     error
	copied      bool
}

func newWaitingView(username, fingerprint string) tea.Model {
	return waitingModel{username: username, fingerprint: fingerprint}
}

func (m waitingModel) Init() tea.Cmd { return doPoll() }

func (m waitingModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "c" {
			if err := clipboard.WriteAll(m.fingerprint); err == nil {
				m.copied = true
				return m, clearCopiedAfter(2 * time.Second)
			}
		}
		return m, nil
	case copyResetMsg:
		m.copied = false
		return m, nil
	case pollTickMsg:
		return m, doPoll()
	case pollResultMsg:
		// Pending devices can't auth /whoami on some servers — treat errors
		// as "still pending" and keep polling.
		m.lastErr = msg.err
		if msg.err == nil && msg.status == "active" {
			return m, func() tea.Msg { return replaceViewMsg{model: newMenuModel(0, 0)} }
		}
		return m, pollTick()
	}
	return m, nil
}

func (m waitingModel) View() string {
	s := "\n  " + warningStyle.Render("Waiting for Approval") + "\n\n"
	s += "  Your account is registered but needs approval from your team admin.\n\n"
	s += "  Send these to your admin:\n"
	s += "    username:    " + selectionStyle.Render(m.username) + "\n"
	s += "    fingerprint: " + selectionStyle.Render(m.fingerprint)
	if m.copied {
		s += "  " + selectionStyle.Render("copied!")
	}
	s += "\n\n"
	s += "  checking every 5 seconds…  (this screen advances automatically)\n"
	s += "\n  c: copy fingerprint   ctrl+c: quit\n"
	return s
}
