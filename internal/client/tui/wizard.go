package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Hennnnnnn/DevWorkspace/internal/client/actions"
	"github.com/Hennnnnnn/DevWorkspace/internal/client/config"
	"github.com/Hennnnnnn/DevWorkspace/internal/client/keystore"
)

// The first-run wizard: asks username + passphrase, then runs
// init → register → unlock → auto team/vault.
// Beginners never see those commands or the team/vault concepts.

type wizardStep int

const (
	stepInputs   wizardStep = iota // username + passphrase + confirm
	stepRecovery                   // show 24-word phrase, explicit confirm
	stepBusy                       // register → bootstrap → unlock → auto-create
	stepRetry                      // username taken: re-ask username only
)

type wizardInitMsg struct {
	res *actions.InitResult
	err error
}

// wizardSetupMsg is the outcome of register→bootstrap→unlock→auto-create.
type wizardSetupMsg struct {
	usernameTaken bool
	fingerprint   string
	err           error
}

type wizardModel struct {
	step      wizardStep
	inputs    []textinput.Model // 0=username 1=passphrase 2=confirm
	focus     int
	mnemonic  string
	fp        string
	serverURL string
	errText   string
	confirmed bool // recovery phrase "I saved it" state
}

func newWizardView() tea.Model {
	user := textinput.New()
	user.Placeholder = "username"
	user.Focus()
	pass := textinput.New()
	pass.Placeholder = "passphrase (min 8 chars)"
	pass.EchoMode = textinput.EchoPassword
	confirm := textinput.New()
	confirm.Placeholder = "confirm passphrase"
	confirm.EchoMode = textinput.EchoPassword

	cfg, _ := config.Load()
	url := ""
	if cfg != nil {
		url = cfg.ServerURL
	}
	return wizardModel{
		inputs:    []textinput.Model{user, pass, confirm},
		serverURL: url,
	}
}

func (m wizardModel) Init() tea.Cmd { return textinput.Blink }

func doWizardInit(passphrase string) tea.Cmd {
	return func() tea.Msg {
		res, err := actions.InitDevice(passphrase)
		return wizardInitMsg{res: res, err: err}
	}
}

// doWizardSetup runs everything after the recovery-phrase confirm in one go.
func doWizardSetup(username, passphrase string) tea.Cmd {
	return func() tea.Msg {
		res, err := actions.Register(username, "", passphrase)
		if err != nil {
			if strings.Contains(err.Error(), "(409)") || strings.Contains(err.Error(), "register failed") {
				return wizardSetupMsg{usernameTaken: true, err: err}
			}
			return wizardSetupMsg{err: err}
		}
		if err := actions.Unlock(passphrase, unlockTTL); err != nil {
			return wizardSetupMsg{err: err}
		}
		// Auto-create a starter team + vault so push/pull work instantly.
		if _, err := actions.CreateTeam("personal"); err != nil {
			return wizardSetupMsg{fingerprint: res.Fingerprint, err: err}
		}
		if _, err := actions.CreateVault("main", "personal"); err != nil {
			return wizardSetupMsg{fingerprint: res.Fingerprint, err: err}
		}
		return wizardSetupMsg{fingerprint: res.Fingerprint}
	}
}

func (m wizardModel) submitInputs() (tea.Model, tea.Cmd) {
	user := strings.TrimSpace(m.inputs[0].Value())
	pass := m.inputs[1].Value()
	if user == "" {
		m.errText = "username is required"
		return m, nil
	}
	if len(pass) < 8 {
		m.errText = "passphrase must be at least 8 characters"
		return m, nil
	}
	if pass != m.inputs[2].Value() {
		m.errText = "passphrases do not match"
		return m, nil
	}
	m.errText = ""
	if keystore.Exists() {
		// Key already generated (interrupted earlier run) — skip init and
		// the recovery screen (the phrase can only be shown at creation).
		m.step = stepBusy
		return m, doWizardSetup(user, pass)
	}
	m.step = stepBusy
	return m, doWizardInit(pass)
}

func (m wizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case wizardInitMsg:
		if msg.err != nil {
			m.step = stepInputs
			m.errText = msg.err.Error()
			return m, nil
		}
		m.mnemonic = msg.res.Mnemonic
		m.fp = msg.res.Fingerprint
		m.step = stepRecovery
		m.confirmed = false
		return m, nil

	case wizardSetupMsg:
		if msg.usernameTaken {
			m.step = stepRetry
			m.errText = "username already taken — pick another"
			m.inputs[0].SetValue("")
			m.inputs[0].Focus()
			return m, textinput.Blink
		}
		if msg.err != nil {
			m.step = stepRetry
			m.errText = msg.err.Error()
			m.inputs[0].Focus()
			return m, textinput.Blink
		}
		if msg.fingerprint != "" {
			m.fp = msg.fingerprint
		}
		return m, func() tea.Msg { return replaceViewMsg{model: newMenuModel(0, 0)} }

	case tea.KeyMsg:
		switch m.step {
		case stepInputs:
			switch msg.String() {
			case "tab", "down", "enter":
				if msg.String() == "enter" && m.focus == len(m.inputs)-1 {
					return m.submitInputs()
				}
				m.inputs[m.focus].Blur()
				m.focus = (m.focus + 1) % len(m.inputs)
				m.inputs[m.focus].Focus()
				return m, textinput.Blink
			case "shift+tab", "up":
				m.inputs[m.focus].Blur()
				m.focus = (m.focus - 1 + len(m.inputs)) % len(m.inputs)
				m.inputs[m.focus].Focus()
				return m, textinput.Blink
			}
			var cmd tea.Cmd
			m.inputs[m.focus], cmd = m.inputs[m.focus].Update(msg)
			return m, cmd

		case stepRecovery:
			switch msg.String() {
			case "y", "Y":
				m.confirmed = true
				return m, nil
			case "enter":
				if m.confirmed {
					m.step = stepBusy
					return m, doWizardSetup(strings.TrimSpace(m.inputs[0].Value()), m.inputs[1].Value())
				}
			}
			return m, nil

		case stepRetry:
			if msg.String() == "enter" {
				user := strings.TrimSpace(m.inputs[0].Value())
				if user == "" {
					return m, nil
				}
				m.errText = ""
				m.step = stepBusy
				return m, doWizardSetup(user, m.inputs[1].Value())
			}
			var cmd tea.Cmd
			m.inputs[0], cmd = m.inputs[0].Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

func (m wizardModel) View() string {
	var b strings.Builder
	b.WriteString("\n  " + selectionStyle.Render("Welcome to devsync") + "\n")
	if m.serverURL != "" {
		b.WriteString("  " + infoStyle.Render("Server: "+m.serverURL+"  (change: devsync config set server_url <url>)") + "\n")
	}
	b.WriteString("\n")

	switch m.step {
	case stepInputs:
		b.WriteString("  Set up this device — pick a username and a passphrase.\n\n")
		labels := []string{"Username:  ", "Passphrase:", "Confirm:   "}
		for i, in := range m.inputs {
			b.WriteString("  " + labels[i] + " " + in.View() + "\n")
		}
		b.WriteString("\n  tab: next field   enter (on last field): continue\n")

	case stepRecovery:
		b.WriteString("  " + successStyle.Render("Device key created") + "  fingerprint: " + m.fp + "\n\n")
		b.WriteString("  " + dangerStyle.Render("RECOVERY PHRASE — shown only ONCE.") + "\n")
		b.WriteString("  These 24 words are the ONLY way to recover access if you lose\n")
		b.WriteString("  your passphrase or this device. Write them down, store offline.\n\n")
		b.WriteString("  " + warningStyle.Render(m.mnemonic) + "\n\n")
		if m.confirmed {
			b.WriteString("  " + successStyle.Render("✓ saved") + " — press enter to continue\n")
		} else {
			b.WriteString("  press " + selectionStyle.Render("y") + " to confirm \"I have saved my recovery phrase\"\n")
		}

	case stepBusy:
		b.WriteString("  setting up your account…\n")

	case stepRetry:
		b.WriteString("  " + dangerStyle.Render(m.errText) + "\n\n")
		b.WriteString("  Username:   " + m.inputs[0].View() + "\n")
		b.WriteString("\n  enter: retry\n")
		return b.String()
	}

	if m.errText != "" && m.step == stepInputs {
		b.WriteString("\n  " + dangerStyle.Render(m.errText) + "\n")
	}
	return b.String()
}
