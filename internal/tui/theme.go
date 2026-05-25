package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type Theme struct {
	Name        string
	UserPrompt  lipgloss.Style
	Assistant   lipgloss.Style
	ToolHeader  lipgloss.Style
	ToolBody    lipgloss.Style
	Error       lipgloss.Style
	StatusLine  lipgloss.Style
	Border      lipgloss.Style
	ModalBorder lipgloss.Style
}

func DarkTheme() Theme {
	return Theme{
		Name:        "dark",
		UserPrompt:  lipgloss.NewStyle().Foreground(lipgloss.Color("#7c7cff")).Bold(true),
		Assistant:   lipgloss.NewStyle().Foreground(lipgloss.Color("#eaeaea")),
		ToolHeader:  lipgloss.NewStyle().Foreground(lipgloss.Color("#f6c177")).Bold(true),
		ToolBody:    lipgloss.NewStyle().Foreground(lipgloss.Color("#9ccfd8")),
		Error:       lipgloss.NewStyle().Foreground(lipgloss.Color("#eb6f92")),
		StatusLine:  lipgloss.NewStyle().Foreground(lipgloss.Color("#6e6a86")),
		Border:      lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#403d52")),
		// Modal: rounded single-line border in the accent purple to match
		// the welcome banner. Double-line + alarmist orange replaced as
		// part of the Claude-Code-style polish pass.
		ModalBorder: lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#7c7cff")),
	}
}

func LightTheme() Theme {
	return Theme{
		Name:        "light",
		UserPrompt:  lipgloss.NewStyle().Foreground(lipgloss.Color("#3b3bff")).Bold(true),
		Assistant:   lipgloss.NewStyle().Foreground(lipgloss.Color("#1a1a1a")),
		ToolHeader:  lipgloss.NewStyle().Foreground(lipgloss.Color("#aa6300")).Bold(true),
		ToolBody:    lipgloss.NewStyle().Foreground(lipgloss.Color("#2c7e8b")),
		Error:       lipgloss.NewStyle().Foreground(lipgloss.Color("#a02b4a")),
		StatusLine:  lipgloss.NewStyle().Foreground(lipgloss.Color("#5a5670")),
		Border:      lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#a8a4c5")),
		ModalBorder: lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#3b3bff")),
	}
}

// DefaultTheme returns the dark theme (back-compat alias).
func DefaultTheme() Theme { return DarkTheme() }

// ThemeByName looks up a built-in theme by name (case-insensitive).
// Empty string or "dark" both resolve to the dark theme.
func ThemeByName(name string) (Theme, bool) {
	switch strings.ToLower(name) {
	case "", "dark":
		return DarkTheme(), true
	case "light":
		return LightTheme(), true
	}
	return Theme{}, false
}

// ThemeNames returns the list of all built-in theme names.
func ThemeNames() []string { return []string{"dark", "light"} }

// ThemeFromConfig resolves the active theme from a config name and optional
// per-field hex colour overrides. When name is "custom" and overrides is
// non-empty the dark theme is used as a base and the supplied colours are
// applied on top.
func ThemeFromConfig(name string, overrides map[string]string) Theme {
	if name == "custom" && len(overrides) > 0 {
		base := DarkTheme()
		for field, color := range overrides {
			c := lipgloss.Color(color)
			switch field {
			case "user_prompt":
				base.UserPrompt = base.UserPrompt.Foreground(c)
			case "assistant":
				base.Assistant = base.Assistant.Foreground(c)
			case "tool_header":
				base.ToolHeader = base.ToolHeader.Foreground(c)
			case "tool_body":
				base.ToolBody = base.ToolBody.Foreground(c)
			case "error":
				base.Error = base.Error.Foreground(c)
			case "status_line":
				base.StatusLine = base.StatusLine.Foreground(c)
			case "border":
				base.Border = base.Border.BorderForeground(c)
			case "modal_border":
				base.ModalBorder = base.ModalBorder.BorderForeground(c)
			}
		}
		base.Name = "custom"
		return base
	}
	t, _ := ThemeByName(name)
	return t
}
