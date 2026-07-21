package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

type filterableView interface {
	isFiltering() bool
}

type statusMsg struct{ gen int }

func clearStatusCmd(d time.Duration, gen int) tea.Cmd {
	return tea.Tick(d, func(_ time.Time) tea.Msg {
		return statusMsg{gen: gen}
	})
}

func newSpinner() spinner.Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = infoStyle
	return s
}
