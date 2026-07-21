package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Hennnnnnn/DevWorkspace/internal/client/actions"
	"github.com/Hennnnnnn/DevWorkspace/internal/protocol"
)

// --- audit vault picker ---

type auditVaultItem struct{ v protocol.Vault }

func (i auditVaultItem) Title() string       { return i.v.Name }
func (i auditVaultItem) Description() string { return "view audit log" }
func (i auditVaultItem) FilterValue() string { return i.v.Name }

type auditVaultsLoadedMsg struct {
	vaults []protocol.Vault
	err    error
}

func loadAuditVaults() tea.Msg {
	vs, err := actions.ListVaults()
	return auditVaultsLoadedMsg{vaults: vs, err: err}
}

type auditVaultPickerModel struct {
	list          list.Model
	width, height int
	loading       bool
	err           error
	spinner       spinner.Model
}

func newAuditPickerView(width, height int) tea.Model {
	l := list.New(nil, list.NewDefaultDelegate(), width, height)
	l.Title = "Audit — select vault"
	l.SetShowHelp(true)
	l.DisableQuitKeybindings()
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{
			helpKey("enter", "view audit log"),
			helpKey("r", "refresh"),
		}
	}
	return auditVaultPickerModel{list: l, width: width, height: height, loading: true, spinner: newSpinner()}
}

func (m auditVaultPickerModel) Init() tea.Cmd {
	return tea.Batch(loadAuditVaults, m.spinner.Tick)
}

func (m auditVaultPickerModel) isFiltering() bool {
	return m.list.FilterState() != list.Unfiltered
}

func (m auditVaultPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

	case auditVaultsLoadedMsg:
		m.loading = false
		m.err = msg.err
		items := make([]list.Item, len(msg.vaults))
		for i, v := range msg.vaults {
			items[i] = auditVaultItem{v: v}
		}
		m.list.SetItems(items)
		return m, nil

	case tea.KeyMsg:
		if m.loading {
			break
		}
		switch msg.String() {
		case "enter":
			if it, ok := m.list.SelectedItem().(auditVaultItem); ok {
				return m, pushView(newAuditLogView(it.v.Name, m.width, m.height))
			}
		case "r":
			if m.list.FilterState() == list.Unfiltered {
				m.loading = true
				return m, tea.Batch(loadAuditVaults, m.spinner.Tick)
			}
			break
		}
	}

	m.list, cmd = m.list.Update(msg)
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

func (m auditVaultPickerModel) View() string {
	if m.loading {
		return fmt.Sprintf("\n  %s loading vaults…\n", m.spinner.View())
	}
	if m.err != nil {
		return "\n" + dangerStyle.Render("  error: "+m.err.Error()) + "\n\n  esc: back\n"
	}
	if len(m.list.Items()) == 0 {
		return "\n  " + warningStyle.Render("no vaults") + "\n\n  esc: back\n"
	}
	return m.list.View()
}

// --- audit log view ---

type auditEntryItem struct{ e protocol.AuditEntry }

func (i auditEntryItem) Title() string       { return i.e.CreatedAt + "  " + i.e.Action }
func (i auditEntryItem) Description() string { return fmt.Sprintf("%-20s %s", i.e.Username, i.e.Target) }
func (i auditEntryItem) FilterValue() string { return i.e.Action }

type auditLogLoadedMsg struct {
	vault   string
	entries []protocol.AuditEntry
	err     error
}

func loadAuditLog(vault string) tea.Cmd {
	return func() tea.Msg {
		es, err := actions.Audit(vault)
		return auditLogLoadedMsg{vault: vault, entries: es, err: err}
	}
}

type auditLogModel struct {
	vault         string
	list          list.Model
	width, height int
	loading       bool
	err           error
	spinner       spinner.Model
}

func newAuditLogView(vault string, width, height int) tea.Model {
	l := list.New(nil, list.NewDefaultDelegate(), width, height)
	l.Title = "Audit — " + vault
	l.SetShowHelp(true)
	l.DisableQuitKeybindings()
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{helpKey("r", "refresh")}
	}
	return auditLogModel{vault: vault, list: l, width: width, height: height, loading: true, spinner: newSpinner()}
}

func (m auditLogModel) Init() tea.Cmd {
	return tea.Batch(loadAuditLog(m.vault), m.spinner.Tick)
}

func (m auditLogModel) isFiltering() bool {
	return m.list.FilterState() != list.Unfiltered
}

func (m auditLogModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

	case auditLogLoadedMsg:
		m.loading = false
		m.err = msg.err
		items := make([]list.Item, len(msg.entries))
		for i, e := range msg.entries {
			items[i] = auditEntryItem{e: e}
		}
		m.list.SetItems(items)
		return m, nil

	case tea.KeyMsg:
		if m.loading {
			break
		}
		switch msg.String() {
		case "r":
			if m.list.FilterState() == list.Unfiltered {
				m.loading = true
				return m, tea.Batch(loadAuditLog(m.vault), m.spinner.Tick)
			}
			break
		}
	}

	m.list, cmd = m.list.Update(msg)
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

func (m auditLogModel) View() string {
	if m.loading {
		return fmt.Sprintf("\n  %s loading audit log…\n", m.spinner.View())
	}
	if m.err != nil {
		return "\n" + dangerStyle.Render("  error: "+m.err.Error()) + "\n\n  esc: back\n"
	}
	if len(m.list.Items()) == 0 {
		return "\n  " + warningStyle.Render("no audit entries") + " for this vault\n\n  esc: back\n"
	}
	return m.list.View()
}
