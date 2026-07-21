package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Hennnnnnn/DevWorkspace/internal/client/actions"
	"github.com/Hennnnnnn/DevWorkspace/internal/protocol"
)

// --- team picker for vault creation ---

type vaultTeamItem struct{ t protocol.Team }

func (i vaultTeamItem) Title() string       { return i.t.Name }
func (i vaultTeamItem) Description() string { return "id: " + i.t.ID }
func (i vaultTeamItem) FilterValue() string { return i.t.Name }

type vaultTeamPickerModel struct {
	list          list.Model
	width, height int
	loading       bool
	err           error
	spinner       spinner.Model
}

func newVaultCreateTeamPicker(width, height int) tea.Model {
	l := list.New(nil, list.NewDefaultDelegate(), width, height)
	l.Title = "Create Vault — select team"
	l.SetShowHelp(true)
	l.DisableQuitKeybindings()
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{helpKey("enter", "select team")}
	}
	return vaultTeamPickerModel{
		list: l, width: width, height: height,
		loading: true, spinner: newSpinner(),
	}
}

func (m vaultTeamPickerModel) Init() tea.Cmd {
	return tea.Batch(loadTeams, m.spinner.Tick)
}

func (m vaultTeamPickerModel) isFiltering() bool {
	return m.list.FilterState() != list.Unfiltered
}

func (m vaultTeamPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

	case teamsLoadedMsg:
		m.loading = false
		m.err = msg.err
		items := make([]list.Item, len(msg.joined))
		for i, t := range msg.joined {
			items[i] = vaultTeamItem{t: t}
		}
		m.list.SetItems(items)

	case tea.KeyMsg:
		if m.loading {
			break
		}
		if msg.String() == "enter" {
			if it, ok := m.list.SelectedItem().(vaultTeamItem); ok {
				return m, pushView(newVaultCreateNameInput(it.t.Name, m.width, m.height))
			}
		}
	}

	m.list, cmd = m.list.Update(msg)
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

func (m vaultTeamPickerModel) View() string {
	if m.loading {
		return "\n  " + m.spinner.View() + " loading teams…\n"
	}
	if m.err != nil {
		return "\n" + dangerStyle.Render("  error: "+m.err.Error()) + "\n\n  esc: back\n"
	}
	if len(m.list.Items()) == 0 {
		return "\n  " + warningStyle.Render("no teams") + " — create a team first\n\n  esc: back\n"
	}
	return m.list.View()
}

// --- vault name input ---

type vaultCreateResultMsg struct {
	name string
	err  error
}

func doCreateVault(name, team string) tea.Cmd {
	return func() tea.Msg {
		_, err := actions.CreateVault(name, team)
		if err != nil {
			return vaultCreateResultMsg{name: name, err: err}
		}
		return vaultCreateResultMsg{name: name}
	}
}

type vaultCreateNameModel struct {
	team         string
	input        textinput.Model
	err          error
	working      bool
	width, height int
}

func newVaultCreateNameInput(team string, width, height int) tea.Model {
	ti := textinput.New()
	ti.Placeholder = "vault name"
	ti.Focus()
	return vaultCreateNameModel{team: team, input: ti, width: width, height: height}
}

func (m vaultCreateNameModel) Init() tea.Cmd { return textinput.Blink }

func (m vaultCreateNameModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case vaultCreateResultMsg:
		m.working = false
		if msg.err != nil {
			m.err = msg.err
			m.input.Reset()
			return m, nil
		}
		return m, tea.Sequence(
			func() tea.Msg { return popMsg{} },
			func() tea.Msg { return popMsg{} },
			func() tea.Msg { return actionDoneMsg{ok: fmt.Sprintf("vault %q created", msg.name)} },
			loadVaults,
		)

	case tea.KeyMsg:
		if msg.String() == "enter" && !m.working {
			if m.input.Value() == "" {
				return m, nil
			}
			m.working = true
			m.err = nil
			return m, doCreateVault(m.input.Value(), m.team)
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m vaultCreateNameModel) View() string {
	s := "\n  " + infoStyle.Render("Create vault in team "+m.team) + "\n\n  " + m.input.View() + "\n"
	if m.working {
		s += "\n  creating…\n"
	}
	if m.err != nil {
		s += "\n  " + dangerStyle.Render("error: "+m.err.Error()) + "\n"
	}
	s += "\n  enter: create   esc: cancel\n"
	return s
}
