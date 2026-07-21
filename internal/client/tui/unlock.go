package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Hennnnnnn/DevWorkspace/internal/client/actions"
)

const unlockTTL = 8 * time.Hour

// unlockedMsg tells the root to pop the unlock view after a successful unlock.
type unlockResultMsg struct{ err error }

func doUnlock(passphrase string) tea.Cmd {
	return func() tea.Msg {
		return unlockResultMsg{err: actions.Unlock(passphrase, unlockTTL)}
	}
}

type unlockModel struct {
	input   textinput.Model
	err     error
	working bool
}

func newUnlockView() tea.Model {
	ti := textinput.New()
	ti.Placeholder = "device passphrase"
	ti.EchoMode = textinput.EchoPassword
	ti.Focus()
	return unlockModel{input: ti}
}

func (m unlockModel) Init() tea.Cmd { return textinput.Blink }

func (m unlockModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case unlockResultMsg:
		m.working = false
		if msg.err != nil {
			m.err = msg.err
			m.input.Reset()
			return m, nil
		}
		return m, popView // unlocked — back to the view that needed it

	case tea.KeyMsg:
		if msg.String() == "enter" && !m.working {
			if m.input.Value() == "" {
				return m, nil
			}
			m.working = true
			m.err = nil
			return m, doUnlock(m.input.Value())
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m unlockModel) View() string {
	s := "\n  " + warningStyle.Render("Locked") + " — enter your passphrase to unlock this device\n\n  " + m.input.View() + "\n"
	if m.working {
		s += "\n  unlocking…\n"
	}
	if m.err != nil {
		s += "\n  " + dangerStyle.Render("error: "+m.err.Error()) + "\n"
	}
	s += "\n  enter: unlock   esc: cancel\n"
	return s
}
