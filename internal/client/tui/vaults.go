package tui

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
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
}

func newVaultsView(width, height int) tea.Model {
	l := list.New(nil, list.NewDefaultDelegate(), width, height)
	l.Title = "Vaults"
	l.SetShowHelp(true)
	l.DisableQuitKeybindings()
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{helpKey("enter", "files")}
	}
	return vaultsModel{list: l, width: width, height: height, loading: true}
}

func (m vaultsModel) Init() tea.Cmd { return loadVaults }

func (m vaultsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

	case tea.KeyMsg:
		if m.loading {
			return m, nil
		}
		if msg.String() == "enter" {
			if it, ok := m.list.SelectedItem().(vaultItem); ok {
				return m, pushView(newFilesView(it.v.Name, m.width, m.height))
			}
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m vaultsModel) View() string {
	if m.loading {
		return "\n  loading vaults…\n"
	}
	if m.err != nil {
		return "\n" + dangerStyle.Render("  error: "+m.err.Error()) + "\n\n  esc: back\n"
	}
	if len(m.list.Items()) == 0 {
		return "\n  " + warningStyle.Render("no vaults") + " — create one with `devsync create-vault`\n\n  esc: back\n"
	}
	return m.list.View()
}
