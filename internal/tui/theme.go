package tui

import "github.com/charmbracelet/lipgloss"

type Theme struct {
	UserPrompt  lipgloss.Style
	Assistant   lipgloss.Style
	ToolHeader  lipgloss.Style
	ToolBody    lipgloss.Style
	Error       lipgloss.Style
	StatusLine  lipgloss.Style
	Border      lipgloss.Style
	ModalBorder lipgloss.Style
}

func DefaultTheme() Theme {
	return Theme{
		UserPrompt:  lipgloss.NewStyle().Foreground(lipgloss.Color("#7c7cff")).Bold(true),
		Assistant:   lipgloss.NewStyle().Foreground(lipgloss.Color("#eaeaea")),
		ToolHeader:  lipgloss.NewStyle().Foreground(lipgloss.Color("#f6c177")).Bold(true),
		ToolBody:    lipgloss.NewStyle().Foreground(lipgloss.Color("#9ccfd8")),
		Error:       lipgloss.NewStyle().Foreground(lipgloss.Color("#eb6f92")),
		StatusLine:  lipgloss.NewStyle().Foreground(lipgloss.Color("#6e6a86")),
		Border:      lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#403d52")),
		ModalBorder: lipgloss.NewStyle().BorderStyle(lipgloss.DoubleBorder()).BorderForeground(lipgloss.Color("#f6c177")),
	}
}
