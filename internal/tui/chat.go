package tui

import (
	"fmt"
	"strconv"
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
	// Render user message with subtle '> ' prefix in accent colour. Each
	// input line is shown verbatim — no 'you > ' chat-room label.
	prefix := c.theme.UserPrompt.Render("> ")
	c.lines = append(c.lines, chatLine{rendered: prefix + text})
	c.refresh()
}

func (c *chat) appendAssistantDelta(text string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.streaming || len(c.lines) == 0 {
		// New assistant turn — start a single line that will accumulate
		// raw markdown until finishAssistant() re-renders through glamour.
		// No 'assistant > ' label; the content speaks for itself.
		c.lines = append(c.lines, chatLine{
			rendered: text,
			rawText:  text,
		})
		c.streaming = true
	} else {
		last := &c.lines[len(c.lines)-1]
		last.rawText += text
		last.rendered = last.rawText
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
				last.rendered = strings.TrimRight(rendered, "\n")
				c.refresh()
			}
		}
	}
	c.streaming = false
}

// appendToolCall renders a tool dispatch in Claude Code style:
//
//	⏺ Bash(command: "uname -a")
//
// Multi-arg tools show every arg comma-separated. Long string values are
// truncated. Use appendToolResult to attach the result line under it.
func (c *chat) appendToolCall(toolName string, input map[string]any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.streaming = false
	bullet := c.theme.ToolHeader.Render("⏺ ")
	name := c.theme.ToolHeader.Render(toolName)
	args := c.theme.ToolBody.Render(formatToolArgs(input))
	c.lines = append(c.lines, chatLine{rendered: bullet + name + args})
	c.refresh()
}

// appendToolResult renders the tool's result indented under the call with
// the tree connector '⎿'. Multi-line results are kept up to 4 lines then
// summarised with '… +N lines'. Errors render in the error colour.
func (c *chat) appendToolResult(summary string, isError bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	connector := c.theme.StatusLine.Render("  ⎿  ")
	body := summary
	if isError {
		body = c.theme.Error.Render(body)
	} else {
		body = c.theme.ToolBody.Render(body)
	}
	// Re-indent multi-line bodies so subsequent lines align under the
	// first character after '⎿  '.
	indent := "     "
	if strings.Contains(body, "\n") {
		first, rest, _ := strings.Cut(body, "\n")
		body = first + "\n" + indentLines(rest, indent)
	}
	c.lines = append(c.lines, chatLine{rendered: connector + body})
	c.refresh()
}

// formatToolArgs renders a tool's input map as a parenthesised arg list.
//   - empty map: "()"
//   - 1 arg with short string: ("uname -a")
//   - 1 arg, long string: ("…truncated to 60 chars…")
//   - N args: (key: "val", key2: "val2")
func formatToolArgs(input map[string]any) string {
	if len(input) == 0 {
		return "()"
	}
	keys := make([]string, 0, len(input))
	for k := range input {
		keys = append(keys, k)
	}
	// Stable order — alphabetical by key keeps the same tool always
	// rendering identically across turns.
	sortStringsAsc(keys)
	if len(keys) == 1 {
		v := input[keys[0]]
		s := truncateValue(formatValue(v), 80)
		// Common single-arg shortcut: Bash(command), Read(file_path)
		// look better when the key is implicit if the value is a string.
		if _, isStr := v.(string); isStr {
			return "(" + s + ")"
		}
		return fmt.Sprintf("(%s: %s)", keys[0], s)
	}
	var parts []string
	for _, k := range keys {
		v := input[k]
		parts = append(parts, fmt.Sprintf("%s: %s", k, truncateValue(formatValue(v), 50)))
	}
	return "(" + strings.Join(parts, ", ") + ")"
}

func formatValue(v any) string {
	switch t := v.(type) {
	case string:
		return strconv.Quote(t)
	case bool:
		return strconv.FormatBool(t)
	case int, int64, float64:
		return fmt.Sprintf("%v", t)
	default:
		return fmt.Sprintf("%v", t)
	}
}

func truncateValue(s string, max int) string {
	if len([]rune(s)) <= max {
		return s
	}
	r := []rune(s)
	// Account for the closing quote (if any) when truncating quoted strings.
	if len(r) > 1 && r[0] == '"' {
		return string(r[:max-2]) + `…"`
	}
	return string(r[:max-1]) + "…"
}

func indentLines(s, prefix string) string {
	if s == "" {
		return s
	}
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = prefix + lines[i]
	}
	return strings.Join(lines, "\n")
}

// sortStringsAsc — local alpha sort without pulling sort import where it's
// the only use. Keep allocation-free for small slices.
func sortStringsAsc(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
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
	// Pad the top of the viewport so transcript content sits at the BOTTOM
	// (next to the input row) — chat-style anchoring. Without this, short
	// transcripts hover at the top of the pane leaving a long gap above
	// the prompt, which looks awkward.
	content := b.String()
	used := strings.Count(content, "\n") + 1
	if c.vp.Height > used {
		content = strings.Repeat("\n", c.vp.Height-used) + content
	}
	c.vp.SetContent(content)
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
