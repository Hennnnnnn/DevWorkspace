package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Hennnnnnn/DevWorkspace/internal/client/actions"
	"github.com/Hennnnnnn/DevWorkspace/internal/protocol"
)

// vaultItem is one vault in the list.
type vaultItem struct{ v protocol.Vault }

func (i vaultItem) Title() string       { return i.v.Name }
func (i vaultItem) Description() string { return "team: " + i.v.Team }
func (i vaultItem) FilterValue() string { return i.v.Name }

// vaultsLoadedMsg carries the async vault-list result.
type vaultsLoadedMsg struct {
	vaults []protocol.Vault
	err    error
}

func loadVaults() tea.Msg {
	vs, err := actions.ListVaults()
	return vaultsLoadedMsg{vaults: vs, err: err}
}

type vaultsModel struct {
	list          list.Model
	width, height int
	loading       bool
	err           error
	status        string
	statusGen     int
	spinner       spinner.Model
}

func newVaultsView(width, height int) tea.Model {
	l := list.New(nil, list.NewDefaultDelegate(), width, height)
	l.Title = "Vaults"
	l.SetShowHelp(true)
	l.DisableQuitKeybindings()
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{
			helpKey("enter", "files"),
			helpKey("c", "create"),
			helpKey("r", "refresh"),
		}
	}
	return vaultsModel{list: l, width: width, height: height, loading: true, spinner: newSpinner()}
}

func (m vaultsModel) Init() tea.Cmd {
	return tea.Batch(loadVaults, m.spinner.Tick)
}

func (m vaultsModel) isFiltering() bool {
	return m.list.FilterState() != list.Unfiltered
}

func (m vaultsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd
	if m.loading {
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.list.SetSize(msg.Width, msg.Height)
		return m, nil

	case vaultsLoadedMsg:
		m.loading = false
		m.err = msg.err
		items := make([]list.Item, len(msg.vaults))
		for i, v := range msg.vaults {
			items[i] = vaultItem{v: v}
		}
		m.list.SetItems(items)
		return m, nil

	case actionDoneMsg:
		m.statusGen++
		if msg.err != nil {
			m.status = dangerStyle.Render("error: " + msg.err.Error())
		} else {
			m.status = successStyle.Render(msg.ok)
		}
		cmds = append(cmds, clearStatusCmd(4*time.Second, m.statusGen))
		if msg.err == nil {
			cmds = append(cmds, loadVaults)
		}
		return m, tea.Batch(cmds...)

	case statusMsg:
		if msg.gen == m.statusGen {
			m.status = ""
		}
		return m, nil

	case tea.KeyMsg:
		if m.loading {
			break
		}
		switch msg.String() {
		case "enter":
			if it, ok := m.list.SelectedItem().(vaultItem); ok {
				return m, pushView(newFilesView(it.v.Name, m.width, m.height))
			}
		case "c":
			return m, pushView(newVaultCreateTeamPicker(m.width, m.height))
		case "r":
			if m.list.FilterState() == list.Unfiltered {
				m.loading = true
				return m, tea.Batch(loadVaults, m.spinner.Tick)
			}
			break
		}
	}

	m.list, cmd = m.list.Update(msg)
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

func (m vaultsModel) View() string {
	if m.loading {
		return fmt.Sprintf("\n  %s loading vaults…\n", m.spinner.View())
	}
	if m.err != nil {
		return "\n" + dangerStyle.Render("  error: "+m.err.Error()) + "\n\n  esc: back\n"
	}
	body := m.list.View()
	if len(m.list.Items()) == 0 {
		body = "\n  " + warningStyle.Render("no vaults") + " — press c to create\n\n  esc: back\n"
	}
	if m.status != "" {
		body += "\n  " + m.status + "\n"
	}
	return body
}
