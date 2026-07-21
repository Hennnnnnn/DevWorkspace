package tui

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/bubbles/filepicker"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Hennnnnnn/DevWorkspace/internal/client/actions"
)

func doPush(file, vault string) tea.Cmd {
	return func() tea.Msg {
		ver, err := actions.Push(file, vault, func(string) actions.DoneFunc {
			return func(string) {}
		})
		if err != nil {
			return actionDoneMsg{err: err}
		}
		return actionDoneMsg{ok: fmt.Sprintf("pushed %s (v%d)", filepath.Base(file), ver)}
	}
}

type filePickerModel struct {
	vault   string
	fp      filepicker.Model
	working bool
	err     error
	status  string
}

func newFilePickerView(vault string) tea.Model {
	fp := filepicker.New()
	fp.FileAllowed = true
	fp.DirAllowed = false
	fp.ShowHidden = false
	return filePickerModel{vault: vault, fp: fp}
}

func (m filePickerModel) Init() tea.Cmd { return m.fp.Init() }

func (m filePickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case actionDoneMsg:
		m.working = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		return m, tea.Sequence(
			func() tea.Msg { return popMsg{} },
			func() tea.Msg { return actionDoneMsg{ok: msg.ok} },
		)

	case tea.WindowSizeMsg:
		m.fp.SetHeight(msg.Height - 6)
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg { return popMsg{} }
		case "~":
			home, err := os.UserHomeDir()
			if err == nil {
				m.fp.CurrentDirectory = home
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.fp, cmd = m.fp.Update(msg)

	if didSelect, path := m.fp.DidSelectFile(msg); didSelect && !m.working {
		m.working = true
		m.status = "pushing " + filepath.Base(path) + "…"
		return m, tea.Batch(cmd, doPush(path, m.vault))
	}

	return m, cmd
}

func (m filePickerModel) View() string {
	if m.working {
		return "\n  " + infoStyle.Render(m.status) + "\n"
	}
	s := "\n  " + infoStyle.Render("Select file to push to "+m.vault) + "\n"
	s += "\n  " + selectionStyle.Render("path: "+m.fp.CurrentDirectory) + "\n\n"
	s += m.fp.View()
	if m.err != nil {
		s += "\n  " + dangerStyle.Render("error: "+m.err.Error()) + "\n"
	}
	s += "\n  esc: cancel   ~: home\n"
	return s
}
