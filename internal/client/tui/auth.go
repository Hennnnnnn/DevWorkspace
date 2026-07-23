package tui

import (
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Hennnnnnn/DevWorkspace/internal/client/config"
	"github.com/Hennnnnnn/DevWorkspace/internal/client/keystore"
)

// authItem is one entry on the startup authentication screen.
type authItem struct {
	title, desc string
	open        func() tea.Model
}

func (i authItem) Title() string       { return i.title }
func (i authItem) Description() string { return i.desc }
func (i authItem) FilterValue() string { return i.title }

// newAuthView is the RFC §"Authentication Screen": a 3-button picker for the
// three account-centric user journeys — Login, Register, Recover. Device
// management (link device) is intentionally NOT here; it lives post-auth.
func newAuthView(width, height int) authModel {
	items := []list.Item{
		authItem{title: "Login", desc: "unlock this device with your passphrase", open: func() tea.Model { return newLoginUnlockView() }},
		authItem{title: "Register", desc: "create a new account and device key", open: func() tea.Model { return newWizardView() }},
		authItem{title: "Recover Account", desc: "restore access from a 24-word recovery phrase", open: func() tea.Model { return newRecoverView() }},
	}
	l := list.New(items, list.NewDefaultDelegate(), width, height)
	l.Title = "DevWorkspace"
	l.SetShowHelp(true)
	l.DisableQuitKeybindings()
	return authModel{list: l}
}

type authModel struct {
	list list.Model
}

func (m authModel) Init() tea.Cmd { return nil }

func (m authModel) isFiltering() bool { return m.list.FilterState() != list.Unfiltered }

func (m authModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetSize(msg.Width, msg.Height)
		return m, nil
	case tea.KeyMsg:
		if msg.String() == "enter" {
			if it, ok := m.list.SelectedItem().(authItem); ok {
				return m, pushView(it.open())
			}
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m authModel) View() string {
	body := m.list.View()
	// Hint when the local keystore is missing: Login is a no-op, pick
	// Register or Recover instead.
	if !keystore.Exists() {
		body += "\n  " + warningStyle.Render("no device key on this machine — Login needs an existing key, pick Register or Recover") + "\n"
	}
	if cfg, _ := config.Load(); cfg != nil && cfg.ServerURL != "" {
		body += "\n  " + infoStyle.Render("Server: "+cfg.ServerURL+"  (change: devsync config set server_url <url>)") + "\n"
	} else {
		body += "\n  " + warningStyle.Render("No server set — run: devsync config set server_url <url>") + "\n"
	}
	return body
}