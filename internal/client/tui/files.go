package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
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
}

func newFilesView(vault string, width, height int) tea.Model {
	l := list.New(nil, list.NewDefaultDelegate(), width, height)
	l.Title = "Files — " + vault
	l.SetShowHelp(true)
	l.DisableQuitKeybindings()
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{helpKey("enter", "history"), helpKey("p", "pull"), helpKey("u", "push"), helpKey("d", "delete"), helpKey("U", "unlock")}
	}
	return filesModel{vault: vault, list: l, width: width, height: height, loading: true}
}

func (m filesModel) Init() tea.Cmd { return loadFiles(m.vault) }

func (m filesModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
		if msg.err != nil {
			m.status = dangerStyle.Render("error: " + msg.err.Error())
		} else {
			m.status = successStyle.Render(msg.ok)
			return m, loadFiles(m.vault)
		}
		return m, nil

	case tea.KeyMsg:
		if m.loading {
			return m, nil
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
				m.status = "pulling " + it.f.Path + "…"
				return m, pullFile(m.vault, it.f.Path)
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
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m filesModel) View() string {
	if m.loading {
		return "\n  loading files…\n"
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
}

func newHistoryView(vault, path string, width, height int) tea.Model {
	l := list.New(nil, list.NewDefaultDelegate(), width, height)
	l.Title = "History — " + path
	l.SetShowHelp(true)
	l.DisableQuitKeybindings()
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{helpKey("c", "checkout version"), helpKey("U", "unlock")}
	}
	return historyModel{vault: vault, path: path, list: l, width: width, height: height, loading: true}
}

func (m historyModel) Init() tea.Cmd { return loadHistory(m.vault, m.path) }

func (m historyModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
		if msg.err != nil {
			m.status = dangerStyle.Render("error: " + msg.err.Error())
		} else {
			m.status = successStyle.Render(msg.ok)
		}
		return m, nil

	case tea.KeyMsg:
		if m.loading {
			return m, nil
		}
		switch msg.String() {
		case "U":
			return m, pushView(newUnlockView())
		case "c":
			if it, ok := m.list.SelectedItem().(historyItem); ok {
				if !actions.IsUnlocked() {
					return m, pushView(newUnlockView())
				}
				m.status = fmt.Sprintf("checking out v%d…", it.e.Version)
				return m, checkoutFile(m.vault, m.path, it.e.Version)
			}
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m historyModel) View() string {
	if m.loading {
		return "\n  loading history…\n"
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
