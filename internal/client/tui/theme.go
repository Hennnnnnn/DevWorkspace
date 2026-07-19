package tui

import "github.com/charmbracelet/lipgloss"

// Semantic colors, per PLAN.md — hardcoded, not user-configurable.
var (
	successColor   = lipgloss.Color("2") // green
	dangerColor    = lipgloss.Color("1") // red — destructive confirm
	warningColor   = lipgloss.Color("3") // yellow — locked state
	infoColor      = lipgloss.Color("4") // blue
	selectionColor = lipgloss.Color("6") // cyan — active list item

	successStyle   = lipgloss.NewStyle().Foreground(successColor)
	dangerStyle    = lipgloss.NewStyle().Foreground(dangerColor)
	warningStyle   = lipgloss.NewStyle().Foreground(warningColor)
	infoStyle      = lipgloss.NewStyle().Foreground(infoColor)
	selectionStyle = lipgloss.NewStyle().Foreground(selectionColor).Bold(true)
)
