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

func helpKey(k, desc string) key.Binding {
	return key.NewBinding(key.WithKeys(k), key.WithHelp(k, desc))
}

// --- files list ---

type fileItem struct{ f protocol.FileMeta }

func (i fileItem) Title() string {
	if i.f.Deleted {
		return i.f.Path + " " + warningStyle.Render("(deleted)")
	}
	return i.f.Path
}
func (i fileItem) Description() string { return fmt.Sprintf("v%d", i.f.LatestVersion) }
func (i fileItem) FilterValue() string { return i.f.Path }

type filesLoadedMsg struct {
	files []protocol.FileMeta
	err   error
}

func loadFiles(vault string) tea.Cmd {
	return func() tea.Msg {
		fs, err := actions.ListFiles(vault)
		return filesLoadedMsg{files: fs, err: err}
	}
}

// actionDoneMsg reports the result of a pull/checkout (no step spinner in TUI yet).
type actionDoneMsg struct {
	ok  string
	err error
}

func pullFile(vault, path string) tea.Cmd {
	return func() tea.Msg {
		res, err := actions.Pull(vault, path, "", func(string) actions.DoneFunc { return func(string) {} })
		if err != nil {
			return actionDoneMsg{err: err}
		}
		return actionDoneMsg{ok: fmt.Sprintf("pulled %s (v%d) -> %s", path, res.Version, res.OutPath)}
	}
}

func doRmFile(vault, path string) tea.Cmd {
	return func() tea.Msg {
		ver, err := actions.Rm(vault, path)
		if err != nil {
			return actionDoneMsg{err: err}
		}
		return actionDoneMsg{ok: fmt.Sprintf("deleted %s (tombstone v%d)", path, ver)}
	}
}

type filesModel struct {
	vault         string
	list          list.Model
	width, height int
	loading       bool
	err           error
	status        string
	statusGen     int
	spinner       spinner.Model
}

func newFilesView(vault string, width, height int) tea.Model {
	l := list.New(nil, list.NewDefaultDelegate(), width, height)
	l.Title = "Files — " + vault
	l.SetShowHelp(true)
	l.DisableQuitKeybindings()
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{
			helpKey("enter", "history"),
			helpKey("p", "pull (download & decrypt)"),
			helpKey("u", "push (encrypt & upload)"),
			helpKey("d", "delete"),
			helpKey("U", "unlock"),
			helpKey("r", "refresh"),
		}
	}
	return filesModel{vault: vault, list: l, width: width, height: height, loading: true, spinner: newSpinner()}
}

func (m filesModel) Init() tea.Cmd {
	return tea.Batch(loadFiles(m.vault), m.spinner.Tick)
}

func (m filesModel) isFiltering() bool {
	return m.list.FilterState() != list.Unfiltered
}

func (m filesModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

	case filesLoadedMsg:
		m.loading = false
		m.err = msg.err
		items := make([]list.Item, len(msg.files))
		for i, f := range msg.files {
			items[i] = fileItem{f: f}
		}
		m.list.SetItems(items)
		return m, nil

	case actionDoneMsg:
		m.statusGen++
		if msg.err != nil {
			m.status = dangerStyle.Render("error: " + msg.err.Error())
		} else {
			m.status = successStyle.Render(msg.ok)
			cmds = append(cmds, loadFiles(m.vault))
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
		case "U":
			return m, pushView(newUnlockView())
		case "enter":
			if it, ok := m.list.SelectedItem().(fileItem); ok {
				return m, pushView(newHistoryView(m.vault, it.f.Path, m.width, m.height))
			}
		case "p":
			if it, ok := m.list.SelectedItem().(fileItem); ok {
				if !actions.IsUnlocked() {
					return m, pushView(newUnlockView())
				}
				m.statusGen++
				m.status = "pulling " + it.f.Path + "…"
				return m, tea.Batch(pullFile(m.vault, it.f.Path), clearStatusCmd(4*time.Second, m.statusGen))
			}
		case "u":
			if !actions.IsUnlocked() {
				return m, pushView(newUnlockView())
			}
			return m, pushView(newFilePickerView(m.vault))
		case "d":
			if it, ok := m.list.SelectedItem().(fileItem); ok {
				if it.f.Deleted {
					return m, nil
				}
				return m, pushView(newConfirmView("Delete "+it.f.Path+"?", doRmFile(m.vault, it.f.Path)))
			}
		case "r":
			if m.list.FilterState() == list.Unfiltered {
				m.loading = true
				return m, tea.Batch(loadFiles(m.vault), m.spinner.Tick)
			}
			break
		}
	}

	m.list, cmd = m.list.Update(msg)
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

func (m filesModel) View() string {
	if m.loading {
		return fmt.Sprintf("\n  %s loading files…\n", m.spinner.View())
	}
	if m.err != nil {
		return "\n" + dangerStyle.Render("  error: "+m.err.Error()) + "\n\n  esc: back\n"
	}
	body := m.list.View()
	if len(m.list.Items()) == 0 {
		body = "\n  " + warningStyle.Render("no files") + " in this vault\n\n  esc: back\n"
	}
	if !actions.IsUnlocked() {
		body += "\n  " + warningStyle.Render("locked") + " — press U to unlock (needed for push/pull/delete)\n"
	}
	if m.status != "" {
		body += "\n  " + m.status + "\n"
	}
	return body
}

// --- file history detail ---

type historyItem struct{ e protocol.HistoryEntry }

func (i historyItem) Title() string {
	del := ""
	if i.e.Deleted {
		del = " " + warningStyle.Render("(deleted)")
	}
	return fmt.Sprintf("v%d%s", i.e.Version, del)
}
func (i historyItem) Description() string {
	return fmt.Sprintf("%s  key=v%d  %d bytes", i.e.CreatedAt, i.e.KeyVersion, i.e.SizeBytes)
}
func (i historyItem) FilterValue() string { return fmt.Sprintf("v%d", i.e.Version) }

type historyLoadedMsg struct {
	entries []protocol.HistoryEntry
	err     error
}

func loadHistory(vault, path string) tea.Cmd {
	return func() tea.Msg {
		es, err := actions.History(vault, path)
		return historyLoadedMsg{entries: es, err: err}
	}
}

func checkoutFile(vault, path string, version int) tea.Cmd {
	return func() tea.Msg {
		res, err := actions.Checkout(vault, path, "", version)
		if err != nil {
			return actionDoneMsg{err: err}
		}
		return actionDoneMsg{ok: fmt.Sprintf("checked out %s v%d -> %s", path, res.Version, res.OutPath)}
	}
}

type historyModel struct {
	vault, path   string
	list          list.Model
	width, height int
	loading       bool
	err           error
	status        string
	statusGen     int
	spinner       spinner.Model
}

func newHistoryView(vault, path string, width, height int) tea.Model {
	l := list.New(nil, list.NewDefaultDelegate(), width, height)
	l.Title = "History — " + path
	l.SetShowHelp(true)
	l.DisableQuitKeybindings()
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{helpKey("c", "checkout version"), helpKey("U", "unlock")}
	}
	return historyModel{vault: vault, path: path, list: l, width: width, height: height, loading: true, spinner: newSpinner()}
}

func (m historyModel) Init() tea.Cmd {
	return tea.Batch(loadHistory(m.vault, m.path), m.spinner.Tick)
}

func (m historyModel) isFiltering() bool {
	return m.list.FilterState() != list.Unfiltered
}

func (m historyModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

	case historyLoadedMsg:
		m.loading = false
		m.err = msg.err
		items := make([]list.Item, len(msg.entries))
		for i, e := range msg.entries {
			items[i] = historyItem{e: e}
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
		case "U":
			return m, pushView(newUnlockView())
		case "c":
			if it, ok := m.list.SelectedItem().(historyItem); ok {
				if !actions.IsUnlocked() {
					return m, pushView(newUnlockView())
				}
				m.statusGen++
				m.status = fmt.Sprintf("checking out v%d…", it.e.Version)
				return m, tea.Batch(checkoutFile(m.vault, m.path, it.e.Version), clearStatusCmd(4*time.Second, m.statusGen))
			}
		}
	}

	m.list, cmd = m.list.Update(msg)
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

func (m historyModel) View() string {
	if m.loading {
		return fmt.Sprintf("\n  %s loading history…\n", m.spinner.View())
	}
	if m.err != nil {
		return "\n" + dangerStyle.Render("  error: "+m.err.Error()) + "\n\n  esc: back\n"
	}
	body := m.list.View()
	if !actions.IsUnlocked() {
		body += "\n  " + warningStyle.Render("locked") + " — press U to unlock (needed for checkout)\n"
	}
	if m.status != "" {
		body += "\n  " + m.status + "\n"
	}
	return body
}
