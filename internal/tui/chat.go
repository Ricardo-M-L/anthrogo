package tui

import (
	"fmt"
	"strings"
	"sync"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
)

// chatLine holds both the rendered (ANSI) string that the viewport displays
// and the raw markdown source for assistant lines so finishAssistant can
// re-render with glamour once streaming ends.
type chatLine struct {
	rendered string // what view() shows
	rawText  string // non-empty only for assistant lines
}

type chat struct {
	mu         sync.Mutex
	vp         viewport.Model
	theme      Theme
	lines      []chatLine
	streaming  bool // true while receiving assistant deltas for the current turn
	md         *glamour.TermRenderer
	welcomeMsg string // shown when lines is empty (model + cwd + hints)
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
	if len(c.lines) == 0 && c.welcomeMsg != "" {
		// Put welcome text in viewport too so the viewport height isn't
		// painted as a giant black box on first render. view() short-
		// circuits this with the raw welcome message for cleaner output.
		c.vp.SetContent(c.welcomeMsg)
		return
	}
	for i, ln := range c.lines {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(ln.rendered)
	}
	c.vp.SetContent(b.String())
	c.vp.GotoBottom()
}

func (c *chat) scrollUp() tea.Cmd {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.vp.ScrollUp(3)
	return nil
}

func (c *chat) scrollDown() tea.Cmd {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.vp.ScrollDown(3)
	return nil
}

func (c *chat) update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	c.vp, cmd = c.vp.Update(msg)
	return cmd
}

// buildWelcomeBanner returns the static greeting shown when the chat
// transcript is empty. Keeps the screen quiet on startup instead of
// reserving a giant empty bordered box (the old behaviour). Style is
// intentionally minimal — title in accent, body in default fg, hints
// in the dim status colour.
func buildWelcomeBanner(theme Theme, model, cwd string) string {
	title := theme.UserPrompt.Render("✱ anthrogo")
	greeting := "  Connected to " + theme.ToolHeader.Render(model)
	if cwd != "" {
		greeting += theme.StatusLine.Render("  ·  ") + theme.StatusLine.Render(cwd)
	}
	hints := []string{
		"  " + theme.StatusLine.Render("/  ") + "list slash commands",
		"  " + theme.StatusLine.Render("?  ") + "help",
		"  " + theme.StatusLine.Render("↑  ") + "recall a previous prompt",
		"  " + theme.StatusLine.Render("F2 ") + "toggle layouts (single / split / triple)",
		"  " + theme.StatusLine.Render("⌃C ") + "interrupt or quit",
	}
	return title + "\n\n" + greeting + "\n\n" + strings.Join(hints, "\n")
}

func (c *chat) setWelcome(msg string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.welcomeMsg = msg
	c.refresh()
}

func (c *chat) view() string {
	c.mu.Lock()
	empty := len(c.lines) == 0 && c.welcomeMsg != ""
	c.mu.Unlock()
	if empty {
		// No transcript yet — render the welcome banner inside the viewport
		// area without a bordered frame. Keeps the screen quiet on startup.
		return c.welcomeMsg
	}
	return c.vp.View()
}
