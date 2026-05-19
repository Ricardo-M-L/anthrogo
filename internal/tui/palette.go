package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ricardo/anthrogo/pkg/command"
)

// palette is the overlay shown while the user is typing a slash command.
type palette struct {
	theme    Theme
	reg      *command.Registry
	visible  bool
	matches  []command.Command
	selected int
}

func newPalette(theme Theme, reg *command.Registry) palette {
	return palette{theme: theme, reg: reg}
}

func (p *palette) updateForInput(input string) {
	if !strings.HasPrefix(input, "/") {
		p.visible = false
		p.matches = nil
		p.selected = 0
		return
	}
	if p.reg == nil {
		return
	}
	p.matches = p.reg.Fuzzy(input)
	p.visible = len(p.matches) > 0
	if p.selected >= len(p.matches) {
		p.selected = 0
	}
}

// handleKey returns (consumed, newInputValue). newInputValue == "" means input unchanged.
func (p *palette) handleKey(msg tea.KeyMsg) (bool, string) {
	if !p.visible {
		return false, ""
	}
	switch msg.String() {
	case "tab":
		next := (p.selected + 1) % len(p.matches)
		p.selected = next
		return true, p.matches[next].Name() + " "
	case "shift+tab":
		next := (p.selected - 1 + len(p.matches)) % len(p.matches)
		p.selected = next
		return true, p.matches[next].Name() + " "
	case "esc":
		p.visible = false
		return true, ""
	}
	return false, ""
}

func (p *palette) view() string {
	if !p.visible {
		return ""
	}
	var b strings.Builder
	for i, c := range p.matches {
		marker := "  "
		if i == p.selected {
			marker = "▶ "
		}
		fmt.Fprintf(&b, "%s%s  %s\n", marker, c.Name(), c.Description())
	}
	return p.theme.ModalBorder.Padding(0, 1).Render(strings.TrimRight(b.String(), "\n"))
}
