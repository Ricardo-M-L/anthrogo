package tui

import (
	"fmt"
	"strings"
	"sync"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

type chat struct {
	mu        sync.Mutex
	vp        viewport.Model
	theme     Theme
	lines     []string
	streaming bool // true while we are receiving assistant deltas for the current turn
}

func newChat(theme Theme) chat {
	vp := viewport.New(80, 20)
	vp.YPosition = 0
	return chat{vp: vp, theme: theme}
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
	c.lines = append(c.lines, c.theme.UserPrompt.Render("you > ")+text)
	c.refresh()
}

func (c *chat) appendAssistantDelta(text string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.streaming || len(c.lines) == 0 {
		c.lines = append(c.lines, c.theme.Assistant.Render("assistant > ")+text)
		c.streaming = true
	} else {
		c.lines[len(c.lines)-1] += text
	}
	c.refresh()
}

// finishAssistant marks the current assistant turn as complete so the next
// delta starts a new line.
func (c *chat) finishAssistant() {
	c.mu.Lock()
	defer c.mu.Unlock()
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
	c.lines = append(c.lines, header+" "+body)
	c.refresh()
}

func (c *chat) appendError(msg string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.streaming = false
	c.lines = append(c.lines, c.theme.Error.Render("error: "+msg))
	c.refresh()
}

func (c *chat) appendServerLog(server, msg string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lines = append(c.lines, c.theme.StatusLine.Render(fmt.Sprintf("[mcp:%s] %s", server, msg)))
	c.refresh()
}

func (c *chat) appendHookLog(event, msg string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lines = append(c.lines, c.theme.StatusLine.Render(fmt.Sprintf("[hook:%s] %s", event, msg)))
	c.refresh()
}

func (c *chat) refresh() {
	c.vp.SetContent(strings.Join(c.lines, "\n"))
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
