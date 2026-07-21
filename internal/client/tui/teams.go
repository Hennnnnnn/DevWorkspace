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

// --- list items ---

type teamItem struct {
	t       protocol.Team
	joined  bool
	pending bool
}

func (i teamItem) Title() string { return i.t.Name }
func (i teamItem) Description() string {
	creator := ""
	if i.t.Creator != "" {
		creator = " by " + i.t.Creator
	}
	if i.joined {
		return "[joined]" + creator
	}
	if i.pending {
		return "[pending approval]" + creator
	}
	return "[not joined]" + creator
}
func (i teamItem) FilterValue() string { return i.t.Name }

type teamSectionItem struct{ title string }

func (i teamSectionItem) Title() string       { return i.title }
func (i teamSectionItem) Description() string { return "" }
func (i teamSectionItem) FilterValue() string { return "" }

// --- load ---

type teamsLoadedMsg struct {
	joined  []protocol.Team
	pending []protocol.Team
	all     []protocol.Team
	err     error
}

func loadTeams() tea.Msg {
	joined, err1 := actions.ListTeams()
	pending, err2 := actions.ListPendingTeams()
	all, err3 := actions.ListAllTeams()
	if err1 != nil {
		return teamsLoadedMsg{err: err1}
	}
	if err2 != nil {
		return teamsLoadedMsg{err: err2}
	}
	if err3 != nil {
		return teamsLoadedMsg{err: err3}
	}
	return teamsLoadedMsg{joined: joined, pending: pending, all: all}
}

func buildTeamItems(joined, pending, all []protocol.Team) []list.Item {
	joinedSet := make(map[string]bool, len(joined))
	for _, t := range joined {
		joinedSet[t.ID] = true
	}
	pendingSet := make(map[string]bool, len(pending))
	for _, t := range pending {
		pendingSet[t.ID] = true
	}
	items := make([]list.Item, 0, len(joined)+len(pending)+len(all)+3)
	items = append(items, teamSectionItem{title: fmt.Sprintf("── Joined (%d) ──", len(joined))})
	for _, t := range joined {
		items = append(items, teamItem{t: t, joined: true, pending: false})
	}
	items = append(items, teamSectionItem{title: fmt.Sprintf("── Pending (%d) ──", len(pending))})
	for _, t := range pending {
		items = append(items, teamItem{t: t, joined: false, pending: true})
	}
	notJoined := 0
	start := len(items)
	items = append(items, teamSectionItem{title: "── Not Joined ──"})
	for _, t := range all {
		if !joinedSet[t.ID] && !pendingSet[t.ID] {
			items = append(items, teamItem{t: t, joined: false, pending: false})
			notJoined++
		}
	}
	items[start] = teamSectionItem{title: fmt.Sprintf("── Not Joined (%d) ──", notJoined)}
	return items
}

// --- teams model ---

type teamsModel struct {
	list          list.Model
	width, height int
	loading       bool
	err           error
	status        string
	statusGen     int
	spinner       spinner.Model
	joinedTeams   []protocol.Team
	pendingTeams  []protocol.Team
	allTeams      []protocol.Team
}

func newTeamsView(width, height int) tea.Model {
	l := list.New(nil, list.NewDefaultDelegate(), width, height)
	l.Title = "Teams"
	l.SetShowHelp(true)
	l.DisableQuitKeybindings()
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{
			helpKey("enter", "members / join"),
			helpKey("c", "create"),
			helpKey("j", "join selected"),
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
		m.joinedTeams = msg.joined
		m.pendingTeams = msg.pending
		m.allTeams = msg.all
		m.list.SetItems(buildTeamItems(msg.joined, msg.pending, msg.all))
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
			it, ok := m.list.SelectedItem().(teamItem)
			if !ok {
				break
			}
			if it.joined {
				return m, pushView(newMembersView(it.t.Name, m.width, m.height))
			}
			return m, doJoinTeam(it.t.Name)
		case "c":
			return m, pushView(newTeamInputView("create"))
		case "j":
			it, ok := m.list.SelectedItem().(teamItem)
			if !ok {
				break
			}
			if it.joined {
				break
			}
			return m, doJoinTeam(it.t.Name)
		case "d":
			it, ok := m.list.SelectedItem().(teamItem)
			if !ok || !it.joined {
				break
			}
			return m, pushView(newConfirmView("Delete team "+it.t.Name+"?", doDeleteTeam(it.t.Name)))
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
	if len(m.joinedTeams) == 0 && len(m.pendingTeams) == 0 && len(m.allTeams) == 0 {
		body = "\n  " + warningStyle.Render("no teams") + " — press c to create\n\n  esc: back\n"
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

func doJoinTeam(name string) tea.Cmd {
	return func() tea.Msg {
		if err := actions.Join(name); err != nil {
			return actionDoneMsg{err: err}
		}
		return actionDoneMsg{ok: fmt.Sprintf("joined team %q (pending approval)", name)}
	}
}

// --- team input (create) ---

type teamInputResultMsg struct {
	name string
	err  error
}

func doTeamInput(name string) tea.Cmd {
	return func() tea.Msg {
		_, err := actions.CreateTeam(name)
		if err != nil {
			return teamInputResultMsg{err: err}
		}
		return teamInputResultMsg{name: name}
	}
}

type teamInputModel struct {
	input   textinput.Model
	err     error
	working bool
}

func newTeamInputView(action string) tea.Model {
	ti := textinput.New()
	ti.Placeholder = "team name"
	ti.Focus()
	return teamInputModel{input: ti}
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
		status := fmt.Sprintf("team %q created", msg.name)
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
			return m, doTeamInput(m.input.Value())
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m teamInputModel) View() string {
	s := "\n  " + infoStyle.Render("Create team") + "\n\n  " + m.input.View() + "\n"
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
		case "a", "enter":
			it, ok := m.list.SelectedItem().(memberItem)
			if !ok || !it.pending {
				return m, nil
			}
			return m, pushView(newApproveFlowView(it.m, m.team))
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

// --- approve flow: confirm identity → pick vaults → approve+grant in one go ---

type approveResultMsg struct {
	ok  string
	err error
}

func doApproveGrant(user, fingerprint string, vaults []string) tea.Cmd {
	return func() tea.Msg {
		res, err := actions.Approve(user, fingerprint, vaults)
		if err != nil {
			return approveResultMsg{err: err}
		}
		note := ""
		if res.ShareNote != "" {
			note = " (" + res.ShareNote + ")"
		} else if res.VaultsShared > 0 {
			note = fmt.Sprintf(", granted %d vault(s)", res.VaultsShared)
		}
		return approveResultMsg{ok: fmt.Sprintf("approved %s%s", user, note)}
	}
}

type approveVaultsLoadedMsg struct {
	vaults []protocol.Vault
	err    error
}

type approveFlowStep int

const (
	approveConfirm approveFlowStep = iota // visual check: username + fingerprint
	approvePick                           // multi-select vaults to grant
	approveBusy
)

type approveFlowModel struct {
	member   protocol.Member
	team     string
	step     approveFlowStep
	vaults   []protocol.Vault
	selected map[int]bool
	cursor   int
	err      error
}

func newApproveFlowView(member protocol.Member, team string) tea.Model {
	return approveFlowModel{member: member, team: team, selected: map[int]bool{}}
}

func (m approveFlowModel) Init() tea.Cmd {
	return func() tea.Msg {
		vs, err := actions.ListVaults()
		return approveVaultsLoadedMsg{vaults: vs, err: err}
	}
}

func (m approveFlowModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case approveVaultsLoadedMsg:
		if msg.err == nil {
			// Default: grant every team vault (changeable in the picker).
			m.vaults = msg.vaults
			for i := range m.vaults {
				m.selected[i] = true
			}
		}
		return m, nil

	case approveResultMsg:
		if msg.err != nil {
			m.step = approveConfirm
			m.err = msg.err
			return m, nil
		}
		return m, tea.Sequence(
			func() tea.Msg { return popMsg{} },
			func() tea.Msg { return actionDoneMsg{ok: msg.ok} },
			loadMembers(m.team),
		)

	case tea.KeyMsg:
		switch m.step {
		case approveConfirm:
			if msg.String() == "y" || msg.String() == "enter" {
				m.err = nil
				if len(m.vaults) == 0 {
					m.step = approveBusy
					return m, doApproveGrant(m.member.Username, m.member.Fingerprint, nil)
				}
				m.step = approvePick
			}
			return m, nil

		case approvePick:
			switch msg.String() {
			case "up", "k":
				if m.cursor > 0 {
					m.cursor--
				}
			case "down", "j":
				if m.cursor < len(m.vaults)-1 {
					m.cursor++
				}
			case " ":
				m.selected[m.cursor] = !m.selected[m.cursor]
			case "enter":
				var names []string
				for i, v := range m.vaults {
					if m.selected[i] {
						names = append(names, v.Name)
					}
				}
				if names == nil {
					names = []string{} // approve without granting anything
				}
				m.step = approveBusy
				return m, doApproveGrant(m.member.Username, m.member.Fingerprint, names)
			}
			return m, nil
		}
	}
	return m, nil
}

func (m approveFlowModel) View() string {
	switch m.step {
	case approveConfirm:
		s := "\n  " + infoStyle.Render("Approve pending user") + "\n\n"
		s += "  username:    " + selectionStyle.Render(m.member.Username) + "\n"
		s += "  fingerprint: " + selectionStyle.Render(m.member.Fingerprint) + "\n\n"
		s += "  " + warningStyle.Render("Verify this fingerprint with the user out-of-band") + "\n"
		s += "  (call, chat, in person) before approving.\n"
		if m.err != nil {
			s += "\n  " + dangerStyle.Render("error: "+m.err.Error()) + "\n"
		}
		s += "\n  y/enter: it matches — continue   esc: cancel\n"
		return s

	case approvePick:
		s := "\n  " + infoStyle.Render("Grant vault access to "+m.member.Username) + "\n\n"
		for i, v := range m.vaults {
			cursor := "  "
			if i == m.cursor {
				cursor = selectionStyle.Render("> ")
			}
			check := "[ ]"
			if m.selected[i] {
				check = successStyle.Render("[x]")
			}
			s += "  " + cursor + check + " " + v.Name + " (" + v.Team + ")\n"
		}
		s += "\n  space: toggle   enter: approve + grant   esc: cancel\n"
		return s

	default:
		return "\n  approving…\n"
	}
}
