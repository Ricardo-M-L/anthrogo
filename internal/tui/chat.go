package tui

import (
	"fmt"
	"strings"
	"sync"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/glamour"
	tea "github.com/charmbracelet/bubbletea"
)

// chatLine holds both the rendered (ANSI) string that the viewport displays
// and the raw markdown source for assistant lines so finishAssistant can
// re-render with glamour once streaming ends.
type chatLine struct {
	rendered string // what view() shows
	rawText  string // non-empty only for assistant lines
}

type chat struct {
	mu        sync.Mutex
	vp        viewport.Model
	theme     Theme
	lines     []chatLine
	streaming bool // true while receiving assistant deltas for the current turn
	md        *glamour.TermRenderer
}

func newChat(theme Theme) chat {
	vp := viewport.New(80, 20)
	vp.YPosition = 0
	// Best-effort renderer; if construction fails (rare) md stays nil and
	// finishAssistant falls back to plain text.
	md, _ := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(0), // 0 = no extra wrapping; viewport handles scroll
	)
	return chat{vp: vp, theme: theme, md: md}
}

func (c *chat) resize(w, h int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.vp.Width = w
	c.vp.Height = h
	c.refresh()
}

func (c *chat) appendUser(text string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lines = append(c.lines, chatLine{
		rendered: c.theme.UserPrompt.Render("you > ") + text,
	})
	c.refresh()
}

func (c *chat) appendAssistantDelta(text string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.streaming || len(c.lines) == 0 {
		c.lines = append(c.lines, chatLine{
			rendered: c.theme.Assistant.Render("assistant > ") + text,
			rawText:  text,
		})
		c.streaming = true
	} else {
		last := &c.lines[len(c.lines)-1]
		last.rawText += text
		last.rendered = c.theme.Assistant.Render("assistant > ") + last.rawText
	}
	c.refresh()
}

// finishAssistant marks the current assistant turn as complete and re-renders
// the accumulated raw markdown through glamour.
func (c *chat) finishAssistant() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.streaming && len(c.lines) > 0 && c.md != nil {
		last := &c.lines[len(c.lines)-1]
		if last.rawText != "" {
			rendered, err := c.md.Render(last.rawText)
			if err == nil {
				last.rendered = c.theme.Assistant.Render("assistant > ") + "\n" + strings.TrimRight(rendered, "\n")
				c.refresh()
			}
		}
	}
	c.streaming = false
}

func (c *chat) appendTool(toolName, summary string, isError bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.streaming = false
	header := c.theme.ToolHeader.Render(fmt.Sprintf("[tool] %s", toolName))
	body := c.theme.ToolBody.Render(summary)
	if isError {
		body = c.theme.Error.Render(summary)
	}
	c.lines = append(c.lines, chatLine{rendered: header + " " + body})
	c.refresh()
}

func (c *chat) appendError(msg string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.streaming = false
	c.lines = append(c.lines, chatLine{rendered: c.theme.Error.Render("error: " + msg)})
	c.refresh()
}

func (c *chat) appendServerLog(server, msg string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lines = append(c.lines, chatLine{
		rendered: c.theme.StatusLine.Render(fmt.Sprintf("[mcp:%s] %s", server, msg)),
	})
	c.refresh()
}

func (c *chat) appendHookLog(event, msg string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lines = append(c.lines, chatLine{
		rendered: c.theme.StatusLine.Render(fmt.Sprintf("[hook:%s] %s", event, msg)),
	})
	c.refresh()
}

func (c *chat) refresh() {
	var b strings.Builder
	for i, ln := range c.lines {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(ln.rendered)
	}
	c.vp.SetContent(b.String())
	c.vp.GotoBottom()
}

func (c *chat) update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	c.vp, cmd = c.vp.Update(msg)
	return cmd
}

func (c *chat) view() string {
	return c.theme.Border.Render(c.vp.View())
}
