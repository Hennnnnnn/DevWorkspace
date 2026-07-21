package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Hennnnnnn/DevWorkspace/internal/client/actions"
	"github.com/Hennnnnnn/DevWorkspace/internal/protocol"
)

// --- teams list ---

type teamItem struct{ t protocol.Team }

func (i teamItem) Title() string       { return i.t.Name }
func (i teamItem) Description() string { return "id: " + i.t.ID }
func (i teamItem) FilterValue() string { return i.t.Name }

type teamsLoadedMsg struct {
	teams []protocol.Team
	err   error
}

func loadTeams() tea.Msg {
	ts, err := actions.ListTeams()
	return teamsLoadedMsg{teams: ts, err: err}
}

type teamsModel struct {
	list          list.Model
	width, height int
	loading       bool
	err           error
	status        string
	statusGen     int
	spinner       spinner.Model
}

func newTeamsView(width, height int) tea.Model {
	l := list.New(nil, list.NewDefaultDelegate(), width, height)
	l.Title = "Teams"
	l.SetShowHelp(true)
	l.DisableQuitKeybindings()
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{
			helpKey("enter", "members"),
			helpKey("c", "create"),
			helpKey("j", "join"),
			helpKey("d", "delete"),
			helpKey("r", "refresh"),
		}
	}
	return teamsModel{list: l, width: width, height: height, loading: true, spinner: newSpinner()}
}

func (m teamsModel) Init() tea.Cmd {
	return tea.Batch(loadTeams, m.spinner.Tick)
}

func (m teamsModel) isFiltering() bool {
	return m.list.FilterState() != list.Unfiltered
}

func (m teamsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

	case teamsLoadedMsg:
		m.loading = false
		m.err = msg.err
		items := make([]list.Item, len(msg.teams))
		for i, t := range msg.teams {
			items[i] = teamItem{t: t}
		}
		m.list.SetItems(items)
		return m, nil

	case actionDoneMsg:
		m.statusGen++
		if msg.err != nil {
			m.status = dangerStyle.Render("error: " + msg.err.Error())
		} else {
			m.status = successStyle.Render(msg.ok)
			cmds = append(cmds, loadTeams)
		}
		cmds = append(cmds, clearStatusCmd(4*time.Second, m.statusGen))
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
			if it, ok := m.list.SelectedItem().(teamItem); ok {
				return m, pushView(newMembersView(it.t.Name, m.width, m.height))
			}
		case "c":
			return m, pushView(newTeamInputView("create"))
		case "j":
			return m, pushView(newTeamInputView("join"))
		case "d":
			if it, ok := m.list.SelectedItem().(teamItem); ok {
				return m, pushView(newConfirmView("Delete team "+it.t.Name+"?", doDeleteTeam(it.t.Name)))
			}
		case "U":
			return m, pushView(newUnlockView())
		case "r":
			if m.list.FilterState() == list.Unfiltered {
				m.loading = true
				return m, tea.Batch(loadTeams, m.spinner.Tick)
			}
			break
		}
	}

	m.list, cmd = m.list.Update(msg)
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

func (m teamsModel) View() string {
	if m.loading {
		return fmt.Sprintf("\n  %s loading teams…\n", m.spinner.View())
	}
	if m.err != nil {
		return "\n" + dangerStyle.Render("  error: "+m.err.Error()) + "\n\n  esc: back\n"
	}
	body := m.list.View()
	if len(m.list.Items()) == 0 {
		body = "\n  " + warningStyle.Render("no teams") + " — press c to create or j to join\n\n  esc: back\n"
	}
	if !actions.IsUnlocked() {
		body += "\n  " + warningStyle.Render("locked") + " — press U to unlock (needed for create/join/delete)\n"
	}
	if m.status != "" {
		body += "\n  " + m.status + "\n"
	}
	return body
}

func doDeleteTeam(name string) tea.Cmd {
	return func() tea.Msg {
		if err := actions.DeleteTeam(name); err != nil {
			return actionDoneMsg{err: err}
		}
		return actionDoneMsg{ok: fmt.Sprintf("team %q deleted", name)}
	}
}

// --- team input (create / join) ---

type teamInputResultMsg struct {
	action string // "create" or "join"
	name   string
	err    error
}

func doTeamInput(action, name string) tea.Cmd {
	return func() tea.Msg {
		switch action {
		case "create":
			t, err := actions.CreateTeam(name)
			if err != nil {
				return teamInputResultMsg{action: action, err: err}
			}
			return teamInputResultMsg{action: action, name: t.Name}
		case "join":
			if err := actions.Join(name); err != nil {
				return teamInputResultMsg{action: action, err: err}
			}
			return teamInputResultMsg{action: action, name: name}
		}
		return teamInputResultMsg{err: fmt.Errorf("unknown action: %s", action)}
	}
}

type teamInputModel struct {
	action  string // "create" or "join"
	input   textinput.Model
	err     error
	working bool
}

func newTeamInputView(action string) tea.Model {
	ti := textinput.New()
	switch action {
	case "create":
		ti.Placeholder = "team name"
	case "join":
		ti.Placeholder = "team name to join"
	}
	ti.Focus()
	return teamInputModel{action: action, input: ti}
}

func (m teamInputModel) Init() tea.Cmd { return textinput.Blink }

func (m teamInputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case teamInputResultMsg:
		m.working = false
		if msg.err != nil {
			m.err = msg.err
			m.input.Reset()
			return m, nil
		}
		status := fmt.Sprintf("team %q %sd", msg.name, m.action)
		return m, tea.Sequence(
			func() tea.Msg { return popMsg{} },
			func() tea.Msg { return actionDoneMsg{ok: status} },
			loadTeams,
		)

	case tea.KeyMsg:
		if msg.String() == "enter" && !m.working {
			if m.input.Value() == "" {
				return m, nil
			}
			m.working = true
			m.err = nil
			return m, doTeamInput(m.action, m.input.Value())
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m teamInputModel) View() string {
	var s string
	switch m.action {
	case "create":
		s = "\n  " + infoStyle.Render("Create team") + "\n\n  " + m.input.View() + "\n"
	case "join":
		s = "\n  " + infoStyle.Render("Join team") + "\n\n  " + m.input.View() + "\n"
	}
	if m.working {
		s += "\n  working…\n"
	}
	if m.err != nil {
		s += "\n  " + dangerStyle.Render("error: "+m.err.Error()) + "\n"
	}
	s += "\n  enter: confirm   esc: cancel\n"
	return s
}

// --- members view ---

type memberItem struct {
	m       protocol.Member
	pending bool
}

func (i memberItem) Title() string {
	if i.pending {
		return i.m.Username + " " + warningStyle.Render("(pending)")
	}
	return i.m.Username
}

func (i memberItem) Description() string {
	return fmt.Sprintf("%-8s %s", i.m.Status, i.m.Fingerprint)
}

func (i memberItem) FilterValue() string { return i.m.Username }

type membersLoadedMsg struct {
	team    string
	members []protocol.Member
	err     error
}

func loadMembers(team string) tea.Cmd {
	return func() tea.Msg {
		ms, err := actions.ListMembers(team, false)
		return membersLoadedMsg{team: team, members: ms, err: err}
	}
}

type membersModel struct {
	team          string
	list          list.Model
	width, height int
	loading       bool
	err           error
	status        string
	statusGen     int
	spinner       spinner.Model
	pendingOnly   bool
	allMembers    []protocol.Member
}

func newMembersView(team string, width, height int) tea.Model {
	l := list.New(nil, list.NewDefaultDelegate(), width, height)
	l.Title = "Members — " + team
	l.SetShowHelp(true)
	l.DisableQuitKeybindings()
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{
			helpKey("a", "approve selected"),
			helpKey("p", "toggle pending"),
			helpKey("U", "unlock"),
			helpKey("r", "refresh"),
		}
	}
	return membersModel{team: team, list: l, width: width, height: height, loading: true, spinner: newSpinner()}
}

func (m membersModel) filterAndSetItems() {
	items := make([]list.Item, 0, len(m.allMembers))
	for _, mem := range m.allMembers {
		isPending := mem.Status == "pending"
		if m.pendingOnly && !isPending {
			continue
		}
		items = append(items, memberItem{m: mem, pending: isPending})
	}
	m.list.SetItems(items)
}

func (m membersModel) Init() tea.Cmd {
	return tea.Batch(loadMembers(m.team), m.spinner.Tick)
}

func (m membersModel) isFiltering() bool {
	return m.list.FilterState() != list.Unfiltered
}

func (m membersModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

	case membersLoadedMsg:
		m.loading = false
		m.err = msg.err
		m.allMembers = msg.members
		m.filterAndSetItems()
		return m, nil

	case actionDoneMsg:
		m.statusGen++
		if msg.err != nil {
			m.status = dangerStyle.Render("error: " + msg.err.Error())
		} else {
			m.status = successStyle.Render(msg.ok)
			cmds = append(cmds, loadMembers(m.team))
		}
		cmds = append(cmds, clearStatusCmd(4*time.Second, m.statusGen))
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
		case "a":
			it, ok := m.list.SelectedItem().(memberItem)
			if !ok || !it.pending {
				return m, nil
			}
			return m, pushView(newApproveInputView(it.m.Username, m.team))
		case "p":
			m.pendingOnly = !m.pendingOnly
			m.filterAndSetItems()
			return m, nil
		case "U":
			return m, pushView(newUnlockView())
		case "r":
			if m.list.FilterState() == list.Unfiltered {
				m.loading = true
				return m, tea.Batch(loadMembers(m.team), m.spinner.Tick)
			}
			break
		}
	}

	m.list, cmd = m.list.Update(msg)
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

func (m membersModel) View() string {
	if m.loading {
		return fmt.Sprintf("\n  %s loading members…\n", m.spinner.View())
	}
	if m.err != nil {
		return "\n" + dangerStyle.Render("  error: "+m.err.Error()) + "\n\n  esc: back\n"
	}
	body := m.list.View()
	if len(m.list.Items()) == 0 {
		if m.pendingOnly {
			body = "\n  " + warningStyle.Render("no pending members") + "\n\n  esc: back\n"
		} else {
			body = "\n  " + warningStyle.Render("no members") + " in this team\n\n  esc: back\n"
		}
	}
	if !actions.IsUnlocked() {
		body += "\n  " + warningStyle.Render("locked") + " — press U to unlock (needed for approve)\n"
	}
	if m.status != "" {
		body += "\n  " + m.status + "\n"
	}
	return body
}

// --- approve input ---

type approveResultMsg struct {
	ok  string
	err error
}

func doApprove(user, fingerprint string) tea.Cmd {
	return func() tea.Msg {
		res, err := actions.Approve(user, fingerprint)
		if err != nil {
			return approveResultMsg{err: err}
		}
		note := ""
		if res.ShareNote != "" {
			note = " (" + res.ShareNote + ")"
		}
		return approveResultMsg{ok: fmt.Sprintf("approved %s%s", user, note)}
	}
}

type approveInputModel struct {
	user    string
	team    string
	input   textinput.Model
	err     error
	working bool
}

func newApproveInputView(user, team string) tea.Model {
	ti := textinput.New()
	ti.Placeholder = "device fingerprint (SHA256:...)"
	ti.Focus()
	return approveInputModel{user: user, team: team, input: ti}
}

func (m approveInputModel) Init() tea.Cmd { return textinput.Blink }

func (m approveInputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case approveResultMsg:
		m.working = false
		if msg.err != nil {
			m.err = msg.err
			m.input.Reset()
			return m, nil
		}
		return m, tea.Sequence(
			func() tea.Msg { return popMsg{} },
			func() tea.Msg { return actionDoneMsg{ok: msg.ok} },
			func() tea.Msg { return membersLoadedMsg{team: m.team} },
			loadMembers(m.team),
		)

	case tea.KeyMsg:
		if msg.String() == "enter" && !m.working {
			if m.input.Value() == "" {
				return m, nil
			}
			m.working = true
			m.err = nil
			return m, doApprove(m.user, m.input.Value())
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m approveInputModel) View() string {
	s := "\n  " + infoStyle.Render("Approve "+m.user) + "\n  Verify fingerprint out-of-band!\n\n  " + m.input.View() + "\n"
	if m.working {
		s += "\n  approving…\n"
	}
	if m.err != nil {
		s += "\n  " + dangerStyle.Render("error: "+m.err.Error()) + "\n"
	}
	s += "\n  enter: approve   esc: cancel\n"
	return s
}
