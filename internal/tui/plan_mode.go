package tui

import "github.com/charmbracelet/lipgloss"

func renderPlanBadge(theme Theme, on bool) string {
	if !on {
		return ""
	}
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#1c1c1c")).
		Background(lipgloss.Color("#f6c177")).
		Bold(true).
		Padding(0, 1)
	return style.Render(" PLAN MODE — write tools disabled ")
}
