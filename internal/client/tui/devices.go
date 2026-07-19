package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Hennnnnnn/DevWorkspace/internal/client/actions"
	"github.com/Hennnnnnn/DevWorkspace/internal/protocol"
)

// --- devices list ---

type deviceItem struct{ d protocol.Device }

func (i deviceItem) Title() string       { return i.d.Name }
func (i deviceItem) Description() string { return shortID(i.d.ID) + " " + i.d.Status + " " + i.d.Fingerprint }
func (i deviceItem) FilterValue() string { return i.d.Name }

func shortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

type devicesLoadedMsg struct {
	devices []protocol.Device
	err     error
}

func loadDevices() tea.Msg {
	ds, err := actions.ListDevices()
	return devicesLoadedMsg{devices: ds, err: err}
}

type devicesModel struct {
	list          list.Model
	width, height int
	loading       bool
	err           error
	status        string
}

func newDevicesView(width, height int) tea.Model {
	l := list.New(nil, list.NewDefaultDelegate(), width, height)
	l.Title = "Devices"
	l.SetShowHelp(true)
	l.DisableQuitKeybindings()
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{
			helpKey("enter", "revoke"),
			helpKey("U", "unlock"),
		}
	}
	return devicesModel{list: l, width: width, height: height, loading: true}
}

func (m devicesModel) Init() tea.Cmd { return loadDevices }

func (m devicesModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.list.SetSize(msg.Width, msg.Height)
		return m, nil

	case devicesLoadedMsg:
		m.loading = false
		m.err = msg.err
		items := make([]list.Item, len(msg.devices))
		for i, d := range msg.devices {
			items[i] = deviceItem{d: d}
		}
		m.list.SetItems(items)
		return m, nil

	case actionDoneMsg:
		if msg.err != nil {
			m.status = dangerStyle.Render("error: " + msg.err.Error())
		} else {
			m.status = successStyle.Render(msg.ok)
			return m, loadDevices
		}
		return m, nil

	case tea.KeyMsg:
		if m.loading {
			return m, nil
		}
		switch msg.String() {
		case "enter":
			it, ok := m.list.SelectedItem().(deviceItem)
			if !ok {
				return m, nil
			}
			return m, pushView(newConfirmView("Revoke device "+it.d.Name+"?", doRevoke(it.d.ID)))
		case "U":
			return m, pushView(newUnlockView())
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m devicesModel) View() string {
	if m.loading {
		return "\n  loading devices…\n"
	}
	if m.err != nil {
		return "\n" + dangerStyle.Render("  error: "+m.err.Error()) + "\n\n  esc: back\n"
	}
	body := m.list.View()
	if len(m.list.Items()) == 0 {
		body = "\n  " + warningStyle.Render("no devices") + "\n\n  esc: back\n"
	}
	if !actions.IsUnlocked() {
		body += "\n  " + warningStyle.Render("locked") + " — press U to unlock\n"
	}
	if m.status != "" {
		body += "\n  " + m.status + "\n"
	}
	return body
}

func doRevoke(deviceID string) tea.Cmd {
	return func() tea.Msg {
		if err := actions.RevokeDevice(deviceID); err != nil {
			return actionDoneMsg{err: err}
		}
		return actionDoneMsg{ok: fmt.Sprintf("revoked device %s", shortID(deviceID))}
	}
}
