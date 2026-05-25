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

const paletteVisible = 7

func (p *palette) view() string {
	if !p.visible {
		return ""
	}
	// Show at most `paletteVisible` rows; window slides to keep the selected
	// row visible (Claude Code style). For total ≤ 7, nothing scrolls.
	start := 0
	if len(p.matches) > paletteVisible {
		// Anchor the selection roughly in the middle of the visible window
		// when possible; clamp to edges.
		half := paletteVisible / 2
		start = p.selected - half
		if start < 0 {
			start = 0
		}
		if start+paletteVisible > len(p.matches) {
			start = len(p.matches) - paletteVisible
		}
	}
	end := start + paletteVisible
	if end > len(p.matches) {
		end = len(p.matches)
	}

	var b strings.Builder
	for i := start; i < end; i++ {
		c := p.matches[i]
		var line string
		if i == p.selected {
			line = p.theme.UserPrompt.Render("  "+c.Name()) + p.theme.StatusLine.Render("  "+c.Description())
		} else {
			line = p.theme.StatusLine.Render("  " + c.Name() + "  " + c.Description())
		}
		b.WriteString(line)
		if i < end-1 {
			b.WriteByte('\n')
		}
	}
	// Hidden-count footer when scrolled
	if len(p.matches) > paletteVisible {
		b.WriteByte('\n')
		b.WriteString(p.theme.StatusLine.Render(fmt.Sprintf("  · %d/%d ·  Tab to cycle", p.selected+1, len(p.matches))))
	}
	return b.String()
}
