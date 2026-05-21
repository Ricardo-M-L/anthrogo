package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type submitMsg struct{ text string }

const maxInputHistory = 1000

type promptInput struct {
	ti          textinput.Model
	theme       Theme
	enabled     bool
	history     []string // newest at index len-1
	historyIdx  int      // -1 means "current draft" (not in history)
	draft       string   // saved when starting to navigate history
	historyPath string
}

func newPromptInput(theme Theme, historyPath string) promptInput {
	ti := textinput.New()
	ti.Prompt = "> "
	ti.Placeholder = "Ask anthrogo anything... (Ctrl+C to quit)"
	ti.CharLimit = 4000
	ti.Width = 80
	ti.Focus()
	p := promptInput{ti: ti, theme: theme, enabled: true, historyPath: historyPath, historyIdx: -1}
	p.loadHistory()
	return p
}

func (p *promptInput) loadHistory() {
	if p.historyPath == "" {
		return
	}
	raw, err := os.ReadFile(p.historyPath)
	if err != nil {
		return
	}
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return
	}
	if len(lines) > maxInputHistory {
		lines = lines[len(lines)-maxInputHistory:]
	}
	p.history = lines
}

func (p *promptInput) appendHistory(text string) {
	if text == "" {
		return
	}
	// Skip consecutive duplicates.
	if len(p.history) > 0 && p.history[len(p.history)-1] == text {
		return
	}
	p.history = append(p.history, text)
	if len(p.history) > maxInputHistory {
		p.history = p.history[len(p.history)-maxInputHistory:]
	}
	if p.historyPath != "" {
		_ = os.MkdirAll(filepath.Dir(p.historyPath), 0o755)
		f, err := os.OpenFile(p.historyPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
		if err == nil {
			fmt.Fprintln(f, text)
			f.Close()
		}
	}
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
			p.appendHistory(text)
			p.historyIdx = -1
			p.draft = ""
			p.ti.SetValue("")
			return p, func() tea.Msg { return submitMsg{text: text} }
		}
		if m.Type == tea.KeyUp {
			if len(p.history) == 0 {
				return p, nil
			}
			if p.historyIdx == -1 {
				p.draft = p.ti.Value()
				p.historyIdx = len(p.history) - 1
			} else if p.historyIdx > 0 {
				p.historyIdx--
			}
			p.ti.SetValue(p.history[p.historyIdx])
			return p, nil
		}
		if m.Type == tea.KeyDown {
			if p.historyIdx == -1 {
				return p, nil
			}
			p.historyIdx++
			if p.historyIdx >= len(p.history) {
				p.historyIdx = -1
				p.ti.SetValue(p.draft)
			} else {
				p.ti.SetValue(p.history[p.historyIdx])
			}
			return p, nil
		}
		if m.Type == tea.KeyCtrlE {
			if len(p.history) == 0 {
				return p, nil
			}
			// Populate the input with the last submitted prompt for editing + re-submission.
			p.ti.SetValue(p.history[len(p.history)-1])
			p.historyIdx = -1
			return p, nil
		}
	}
	var cmd tea.Cmd
	p.ti, cmd = p.ti.Update(msg)
	return p, cmd
}

func (p promptInput) view() string {
	hint := ""
	if p.ti.Value() == "" && len(p.history) > 0 {
		hint = "  [Ctrl+E: edit last]"
	}
	return p.theme.Border.Render(p.ti.View() + hint)
}
