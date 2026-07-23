package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Hennnnnnn/DevWorkspace/internal/client/actions"
)

// The Recover Account flow: enter username + 24-word recovery phrase + new
// passphrase → actions.Recover regenerates the keystore, the server
// reactivates the existing device row, then we unlock and go to the dashboard.
// No device activation step — recovery phrase IS the reset token.

type recoverStep int

const (
	recoverStepInputs recoverStep = iota
	recoverStepBusy
)

type recoverResultMsg struct {
	res *actions.RegisterResult
	err error
}

type recoverModel struct {
	step    recoverStep
	inputs  []textinput.Model // 0=username 1=mnemonic 2=passphrase 3=confirm
	focus   int
	errText string
}

func newRecoverView() tea.Model {
	user := textinput.New()
	user.Placeholder = "username"
	user.Focus()
	mnemonic := textinput.New()
	mnemonic.Placeholder = "24-word recovery phrase (space-separated)"
	pass := textinput.New()
	pass.Placeholder = "new passphrase (min 8 chars)"
	pass.EchoMode = textinput.EchoPassword
	confirm := textinput.New()
	confirm.Placeholder = "confirm passphrase"
	confirm.EchoMode = textinput.EchoPassword
	return recoverModel{inputs: []textinput.Model{user, mnemonic, pass, confirm}}
}

func (m recoverModel) Init() tea.Cmd { return textinput.Blink }

func doRecover(username, mnemonic, passphrase string) tea.Cmd {
	return func() tea.Msg {
		res, err := actions.Recover(username, mnemonic, passphrase)
		if err != nil {
			return recoverResultMsg{err: err}
		}
		// Unlock immediately so the dashboard is usable.
		if err := actions.Unlock(passphrase, unlockTTL); err != nil {
			return recoverResultMsg{res: res, err: err}
		}
		return recoverResultMsg{res: res}
	}
}

func (m recoverModel) submit() (tea.Model, tea.Cmd) {
	user := strings.TrimSpace(m.inputs[0].Value())
	mnemonic := strings.TrimSpace(m.inputs[1].Value())
	pass := m.inputs[2].Value()
	if user == "" {
		m.errText = "username is required"
		return m, nil
	}
	// ponytail: BIP39 is 24 words. Let crypto.MnemonicToSeed do the checksum
	// validation; this is just a UX guard so typos don't waste a network round-trip.
	if len(strings.Fields(mnemonic)) != 24 {
		m.errText = "recovery phrase must be exactly 24 words"
		return m, nil
	}
	if len(pass) < 8 {
		m.errText = "passphrase must be at least 8 characters"
		return m, nil
	}
	if pass != m.inputs[3].Value() {
		m.errText = "passphrases do not match"
		return m, nil
	}
	m.errText = ""
	m.step = recoverStepBusy
	return m, doRecover(user, mnemonic, pass)
}

func (m recoverModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case recoverResultMsg:
		if msg.err != nil {
			m.step = recoverStepInputs
			m.errText = msg.err.Error()
			// Wipe sensitive fields; keep username.
			m.inputs[1].Reset()
			m.inputs[2].Reset()
			m.inputs[3].Reset()
			m.focus = 1
			m.inputs[0].Blur()
			m.inputs[1].Focus()
			return m, textinput.Blink
		}
		// Recovery + unlock succeeded → dashboard.
		return m, func() tea.Msg { return replaceViewMsg{model: newMenuModel(0, 0)} }

	case tea.KeyMsg:
		if m.step == recoverStepBusy {
			return m, nil
		}
		switch msg.String() {
		case "tab", "down":
			m.inputs[m.focus].Blur()
			m.focus = (m.focus + 1) % len(m.inputs)
			m.inputs[m.focus].Focus()
			return m, textinput.Blink
		case "shift+tab", "up":
			m.inputs[m.focus].Blur()
			m.focus = (m.focus - 1 + len(m.inputs)) % len(m.inputs)
			m.inputs[m.focus].Focus()
			return m, textinput.Blink
		case "enter":
			if m.focus == len(m.inputs)-1 {
				return m.submit()
			}
			m.inputs[m.focus].Blur()
			m.focus++
			m.inputs[m.focus].Focus()
			return m, textinput.Blink
		}
		var cmd tea.Cmd
		m.inputs[m.focus], cmd = m.inputs[m.focus].Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m recoverModel) View() string {
	var b strings.Builder
	b.WriteString("\n  " + selectionStyle.Render("Recover Account") + "\n\n")
	b.WriteString("  Restore access from your 24-word recovery phrase. The server\n")
	b.WriteString("  recognises the fingerprint and reactivates this device immediately.\n\n")
	labels := []string{"Username:        ", "Recovery phrase: ", "New passphrase:  ", "Confirm:         "}
	for i, in := range m.inputs {
		b.WriteString("  " + labels[i] + " " + in.View() + "\n")
	}
	if m.step == recoverStepBusy {
		b.WriteString("\n  recovering…\n")
	}
	if m.errText != "" {
		b.WriteString("\n  " + dangerStyle.Render(m.errText) + "\n")
	}
	b.WriteString("\n  tab/enter: next field   enter (on last): submit   esc: back\n")
	return b.String()
}