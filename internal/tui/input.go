package tui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type submitMsg struct{ text string }

type promptInput struct {
	ti      textinput.Model
	theme   Theme
	enabled bool
}

func newPromptInput(theme Theme) promptInput {
	ti := textinput.New()
	ti.Prompt = "> "
	ti.Placeholder = "Ask anthrogo anything... (Ctrl+C to quit)"
	ti.CharLimit = 4000
	ti.Width = 80
	ti.Focus()
	return promptInput{ti: ti, theme: theme, enabled: true}
}

func (p *promptInput) setWidth(w int) { p.ti.Width = w }
func (p *promptInput) setEnabled(on bool) {
	p.enabled = on
	if on {
		p.ti.Focus()
	} else {
		p.ti.Blur()
	}
}

func (p promptInput) update(msg tea.Msg) (promptInput, tea.Cmd) {
	if !p.enabled {
		return p, nil
	}
	switch m := msg.(type) {
	case tea.KeyMsg:
		if m.Type == tea.KeyEnter && p.ti.Value() != "" {
			text := p.ti.Value()
			p.ti.SetValue("")
			return p, func() tea.Msg { return submitMsg{text: text} }
		}
	}
	var cmd tea.Cmd
	p.ti, cmd = p.ti.Update(msg)
	return p, cmd
}

func (p promptInput) view() string {
	return p.theme.Border.Render(p.ti.View())
}
