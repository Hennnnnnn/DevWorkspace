package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/Hennnnnnn/DevWorkspace/internal/client/commands"
)

const headerHeight = 12

func RenderHeader(width int) string {
	logo := logoStyle.Render(devsyncLogo)
	subtitle := taglineStyle.Render("DevWorkspaceSync CLI")
	tagline := taglineStyle.Render("Secure Secret Management for Developers")
	version := taglineStyle.Render("Version: " + commands.Version)
	sep := separatorStyle.Render(strings.Repeat("─", width))

	b := "\n"
	b += center(width, logo) + "\n\n"
	b += center(width, subtitle) + "\n"
	b += center(width, tagline) + "\n"
	b += center(width, version) + "\n\n"
	b += sep + "\n"
	return b
}

func center(width int, s string) string {
	if width <= 0 {
		return s
	}
	return lipgloss.NewStyle().Width(width).Align(lipgloss.Center).Render(s)
}

const devsyncLogo = `██████╗ ███████╗██╗   ██╗███████╗██╗   ██╗███╗   ██╗ ██████╗
██╔══██╗██╔════╝██║   ██║██╔════╝╚██╗ ██╔╝████╗  ██║██╔════╝
██║  ██║█████╗  ██║   ██║███████╗ ╚████╔╝ ██╔██╗ ██║██║     
██║  ██║██╔══╝  ╚██╗ ██╔╝╚════██║  ╚██╔╝  ██║╚██╗██║██║     
██████╔╝███████╗ ╚████╔╝ ███████║   ██║   ██║ ╚████║╚██████╗
╚═════╝ ╚══════╝  ╚═══╝  ╚══════╝   ╚═╝   ╚═╝  ╚═══╝ ╚═════╝`
