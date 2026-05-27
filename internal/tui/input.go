package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
)

type submitMsg struct{ text string }

const maxInputHistory = 1000

// promptInput is the bottom-of-screen prompt. It accepts multi-line text:
//
//	Enter        → submit
//	Alt+Enter    → insert newline (most terminals)
//	Ctrl+J       → insert newline (universal fallback; same key permission
//	                elicit uses for the multi-line text field)
//
// Single-row default; auto-grows up to 6 rows when the value spans lines.
type promptInput struct {
	ta          textarea.Model
	theme       Theme
	enabled     bool
	history     []string // newest at index len-1
	historyIdx  int      // -1 means "current draft" (not in history)
	draft       string   // saved when starting to navigate history
	historyPath string
}

func newPromptInput(theme Theme, historyPath string) promptInput {
	ta := textarea.New()
	ta.Prompt = "> "
	ta.Placeholder = "Type a message, / for commands, ? for help…"
	ta.CharLimit = 8000
	ta.SetWidth(80)
	ta.SetHeight(1)
	// Override the default Enter-inserts-newline binding so plain Enter
	// submits the prompt. Newline insertion moves to Alt+Enter / Ctrl+J.
	ta.KeyMap.InsertNewline = key.NewBinding(
		key.WithKeys("alt+enter", "ctrl+j"),
		key.WithHelp("alt+enter", "insert newline"),
	)
	// Hide the line-number gutter and decorative cursor line — looks
	// cleaner for a chat prompt than a code editor.
	ta.ShowLineNumbers = false
	ta.Focus()
	p := promptInput{ta: ta, theme: theme, enabled: true, historyPath: historyPath, historyIdx: -1}
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
		f, err := os.OpenFile(p.historyPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
		if err == nil {
			// History stores ONE physical line per prompt. Multi-line
			// prompts use \\n escapes so the file stays line-oriented.
			esc := strings.ReplaceAll(text, "\n", `\n`)
			fmt.Fprintln(f, esc)
			f.Close()
		}
	}
}

// value returns the current draft (helper used by callers that need
// read-only access without poking at the textarea field).
func (p promptInput) value() string { return p.ta.Value() }

// setValue replaces the draft. Used by palette command completion.
func (p *promptInput) setValue(s string) {
	p.ta.SetValue(s)
	p.growToFit()
}

func (p *promptInput) setWidth(w int) { p.ta.SetWidth(w) }
func (p *promptInput) setEnabled(on bool) {
	p.enabled = on
	if on {
		p.ta.Focus()
	} else {
		p.ta.Blur()
	}
}

// growToFit resizes the textarea between 1 and 6 rows so multi-line drafts
// stay visible without permanently reserving screen real estate.
func (p *promptInput) growToFit() {
	lines := strings.Count(p.ta.Value(), "\n") + 1
	if lines < 1 {
		lines = 1
	}
	if lines > 6 {
		lines = 6
	}
	if p.ta.Height() != lines {
		p.ta.SetHeight(lines)
	}
}

func (p promptInput) update(msg tea.Msg) (promptInput, tea.Cmd) {
	if !p.enabled {
		return p, nil
	}
	switch m := msg.(type) {
	case tea.KeyMsg:
		// Plain Enter (no modifiers) submits. Alt+Enter / Ctrl+J fall
		// through to textarea's InsertNewline binding for a literal \n.
		if m.Type == tea.KeyEnter && p.ta.Value() != "" {
			text := p.ta.Value()
			p.appendHistory(text)
			p.historyIdx = -1
			p.draft = ""
			p.ta.SetValue("")
			p.ta.SetHeight(1)
			return p, func() tea.Msg { return submitMsg{text: text} }
		}
		// History up/down only when on the first/last line of input (so
		// multi-line cursor navigation still works inside the textarea).
		if m.Type == tea.KeyUp && p.ta.Line() == 0 {
			if len(p.history) == 0 {
				return p, nil
			}
			if p.historyIdx == -1 {
				p.draft = p.ta.Value()
				p.historyIdx = len(p.history) - 1
			} else if p.historyIdx > 0 {
				p.historyIdx--
			}
			p.ta.SetValue(historyDecode(p.history[p.historyIdx]))
			p.growToFit()
			return p, nil
		}
		if m.Type == tea.KeyDown && p.historyIdx != -1 {
			p.historyIdx++
			if p.historyIdx >= len(p.history) {
				p.historyIdx = -1
				p.ta.SetValue(p.draft)
			} else {
				p.ta.SetValue(historyDecode(p.history[p.historyIdx]))
			}
			p.growToFit()
			return p, nil
		}
		if m.Type == tea.KeyCtrlE {
			if len(p.history) == 0 {
				return p, nil
			}
			p.ta.SetValue(historyDecode(p.history[len(p.history)-1]))
			p.historyIdx = -1
			p.growToFit()
			return p, nil
		}
	}
	var cmd tea.Cmd
	p.ta, cmd = p.ta.Update(msg)
	p.growToFit()
	return p, cmd
}

// historyDecode reverses the \n escaping done by appendHistory so multi-line
// drafts round-trip through the on-disk history file.
func historyDecode(s string) string {
	return strings.ReplaceAll(s, `\n`, "\n")
}

func (p promptInput) view() string {
	hint := ""
	if p.ta.Value() == "" && len(p.history) > 0 {
		hint = "  " + p.theme.StatusLine.Render("[Ctrl+E: edit last]")
	}
	sep := p.theme.StatusLine.Render(strings.Repeat("─", maxIntInput(40, p.ta.Width())))
	return sep + "\n" + p.ta.View() + hint
}

func maxIntInput(a, b int) int {
	if a > b {
		return a
	}
	return b
}
