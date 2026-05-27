package tui

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

// Diff colours — red dim for removals, green dim for additions. Match the
// muted palette Claude Code uses on dark terminals (and a bit lighter for
// light terminals). Defined as package vars to avoid re-allocating styles
// on every Edit call.
var (
	lipglossDelRed        = lipgloss.NewStyle().Foreground(lipgloss.Color("#eb6f92"))
	lipglossAddGreen      = lipgloss.NewStyle().Foreground(lipgloss.Color("#9ccfd8"))
	lipglossDelRedLight   = lipgloss.NewStyle().Foreground(lipgloss.Color("#a02b4a"))
	lipglossAddGreenLight = lipgloss.NewStyle().Foreground(lipgloss.Color("#2c7e8b"))
)

// chatLine holds both the rendered (ANSI) string that the viewport displays
// and the raw markdown source for assistant lines so finishAssistant can
// re-render with glamour once streaming ends.
type chatLine struct {
	rendered  string // what view() shows
	rawText   string // non-empty only for assistant lines
	toolUseID string // non-empty for the placeholder line of a pending tool call
}

type chat struct {
	mu         sync.Mutex
	vp         viewport.Model
	theme      Theme
	lines      []chatLine
	streaming  bool // true while receiving assistant deltas for the current turn
	thinking   bool // user submitted, waiting for first delta or tool call
	thinkTick  int  // animation frame counter
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
	c.thinking = true
	c.refresh()
}

// tickThinking advances the spinner frame. Called from the app's tick loop
// while c.thinking is true; cheap (refresh re-renders the viewport buffer).
func (c *chat) tickThinking() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.thinking {
		return
	}
	c.thinkTick++
	c.refresh()
}

func (c *chat) appendAssistantDelta(text string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.thinking = false
	if !c.streaming || len(c.lines) == 0 {
		// New assistant turn — start a single line that will accumulate
		// raw markdown. No 'assistant > ' label; content speaks for itself.
		c.lines = append(c.lines, chatLine{
			rendered: text,
			rawText:  text,
		})
		c.streaming = true
	} else {
		last := &c.lines[len(c.lines)-1]
		last.rawText += text
		// Streaming-safe markdown: render through glamour ONLY when the
		// accumulated raw text ends at a safe boundary (paragraph break
		// or closed fenced code block). For mid-paragraph deltas keep
		// the raw text so the partial sentence reads naturally.
		if c.md != nil && safeMarkdownBoundary(last.rawText) {
			if r, err := c.md.Render(last.rawText); err == nil {
				last.rendered = strings.TrimRight(r, "\n")
			} else {
				last.rendered = last.rawText
			}
		} else {
			last.rendered = last.rawText
		}
	}
	c.refresh()
}

// safeMarkdownBoundary returns true when the buffer ends at a place where
// glamour can render without producing surprises mid-stream.
//   - ends with a blank line (\n\n): previous paragraph is closed
//   - even number of ``` fences: no unclosed code block
//
// Returning false keeps the buffer in raw-text mode until the next safe
// boundary; finishAssistant() always runs the final glamour pass.
func safeMarkdownBoundary(s string) bool {
	if !strings.HasSuffix(s, "\n\n") {
		return false
	}
	// Count code fences. Odd = unclosed; skip rendering.
	if strings.Count(s, "```")%2 != 0 {
		return false
	}
	return true
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

// appendToolPending renders a placeholder line for a tool whose args are
// still being composed by the model. The line is replaced when the full
// tool_use_request arrives (matched by tool_use_id). Eliminates the dead-
// time gap when the model is emitting a large JSON input blob (e.g. an
// Edit with multi-line old_string / new_string).
func (c *chat) appendToolPending(toolName, toolUseID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.streaming = false
	c.thinking = false
	bullet := c.theme.ToolHeader.Render("⏺ ")
	name := c.theme.ToolHeader.Render(toolName)
	args := c.theme.StatusLine.Render(" (composing args…)")
	suffix := ""
	if toolUseID != "" {
		s := toolUseID
		if len(s) > 8 {
			s = s[len(s)-8:]
		}
		suffix = c.theme.StatusLine.Render("  · " + s)
	}
	// Tag the chatLine with the tool_use_id so appendToolCall can find
	// and replace it when the full input arrives.
	c.lines = append(c.lines, chatLine{
		rendered:  bullet + name + args + suffix,
		toolUseID: toolUseID,
	})
	c.refresh()
}

// appendToolCall renders a tool dispatch in Claude Code style:
//
//	⏺ Bash(command: "uname -a")
//
// Multi-arg tools show every arg comma-separated. Long string values are
// truncated. Use appendToolResult to attach the result line under it.
func (c *chat) appendToolCall(toolName, toolUseID string, input map[string]any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.streaming = false
	c.thinking = false
	bullet := c.theme.ToolHeader.Render("⏺ ")
	name := c.theme.ToolHeader.Render(toolName)
	// Tool-specific compact formatters take precedence over the generic
	// arg-list. TodoWrite gets a checkbox list; other tools fall through.
	var args string
	if toolName == "TodoWrite" {
		args = c.theme.StatusLine.Render("(update)")
	} else {
		args = c.theme.ToolBody.Render(formatToolArgs(toolName, input))
	}
	header := bullet + name + args
	if toolUseID != "" {
		// Show last 8 chars of the tool_use_id for log correlation, in
		// the dim status colour so it doesn't compete with the call.
		suffix := toolUseID
		if len(suffix) > 8 {
			suffix = suffix[len(suffix)-8:]
		}
		header += c.theme.StatusLine.Render("  · " + suffix)
	}
	// If a pending placeholder for this tool_use_id exists, replace it in
	// place (rather than appending a second line). Otherwise append.
	pendingIdx := -1
	if toolUseID != "" {
		for i := range c.lines {
			if c.lines[i].toolUseID == toolUseID {
				pendingIdx = i
				break
			}
		}
	}
	full := chatLine{rendered: header, toolUseID: toolUseID}
	if pendingIdx >= 0 {
		c.lines[pendingIdx] = full
	} else {
		c.lines = append(c.lines, full)
	}
	// For Edit-style tools, also append a compact diff block immediately
	// after the call header so the user sees what's about to change.
	if diff := formatEditDiff(c.theme, toolName, input); diff != "" {
		c.lines = append(c.lines, chatLine{rendered: diff})
	}
	// TodoWrite: render the checkbox list inline so the user sees the
	// plan immediately, not as a JSON dump.
	if toolName == "TodoWrite" {
		if list := formatTodoList(c.theme, input); list != "" {
			c.lines = append(c.lines, chatLine{rendered: list})
		}
	}
	c.refresh()
}

// formatTodoList renders TodoWrite's todos array as a checkbox list:
//
//	☐ pending item
//	◑ in-progress item (highlighted)
//	☑ completed item (dimmed + strikethrough)
//
// Returns "" if input["todos"] is missing or unexpected.
func formatTodoList(theme Theme, input map[string]any) string {
	raw, ok := input["todos"]
	if !ok {
		return ""
	}
	arr, ok := raw.([]any)
	if !ok {
		return ""
	}
	var b strings.Builder
	indent := "     "
	for i, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		content, _ := m["content"].(string)
		status, _ := m["status"].(string)
		var glyph, line string
		switch status {
		case "completed":
			glyph = "☑"
			line = theme.StatusLine.Render(glyph+" "+content) + theme.StatusLine.Render("  (done)")
		case "in_progress":
			glyph = "◑"
			line = theme.ToolHeader.Render(glyph+" "+content) + theme.StatusLine.Render("  (in progress)")
		default: // pending or unknown
			glyph = "☐"
			line = theme.ToolBody.Render(glyph + " " + content)
		}
		b.WriteString(indent + line)
		if i < len(arr)-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
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
//
// For Edit-style tools the long old_string/new_string are elided here (their
// diff is rendered on a separate line by formatEditDiff).
func formatToolArgs(toolName string, input map[string]any) string {
	if len(input) == 0 {
		return "()"
	}
	keys := make([]string, 0, len(input))
	skip := editDiffKeys(toolName)
	for k := range input {
		if _, drop := skip[k]; drop {
			continue
		}
		keys = append(keys, k)
	}
	if len(keys) == 0 {
		// Edit with only old/new — show just the file_path. Fall through
		// to single-key branch.
		for k := range input {
			if k == "file_path" || k == "path" {
				keys = append(keys, k)
			}
		}
	}
	sortStringsAsc(keys)
	if len(keys) == 1 {
		v := input[keys[0]]
		s := truncateValue(formatValue(v), 80)
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

// editDiffKeys returns the set of input-map keys that should be omitted from
// the parenthesised arg list because they're rendered as a diff block below.
func editDiffKeys(toolName string) map[string]bool {
	switch toolName {
	case "Edit", "MultiEdit", "NotebookEdit":
		return map[string]bool{"old_string": true, "new_string": true}
	}
	return nil
}

// formatEditDiff renders a compact diff snippet for Edit-style tools so the
// user sees what's about to change before the result lands. Returns "" if
// the tool isn't an Edit variant or required keys are missing.
//
//   - old line 1
//   - old line 2
//   - new line 1
//   - new line 2
func formatEditDiff(theme Theme, toolName string, input map[string]any) string {
	if _, ok := editDiffKeys(toolName)[""]; ok || editDiffKeys(toolName) == nil {
		// editDiffKeys returns nil for non-Edit tools; check explicitly.
		if editDiffKeys(toolName) == nil {
			return ""
		}
	}
	oldS, _ := input["old_string"].(string)
	newS, _ := input["new_string"].(string)
	if oldS == "" && newS == "" {
		return ""
	}
	const maxLines = 6
	delStyle := lipglossDelRed
	addStyle := lipglossAddGreen
	if theme.Name == "light" {
		delStyle = lipglossDelRedLight
		addStyle = lipglossAddGreenLight
	}
	var b strings.Builder
	indent := "     "
	for _, ln := range linesCap(oldS, maxLines) {
		b.WriteString(indent + delStyle.Render("- "+ln) + "\n")
	}
	for _, ln := range linesCap(newS, maxLines) {
		b.WriteString(indent + addStyle.Render("+ "+ln) + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func linesCap(s string, n int) []string {
	if s == "" {
		return nil
	}
	all := strings.Split(s, "\n")
	if len(all) <= n {
		return all
	}
	return append(all[:n], fmt.Sprintf("… +%d lines", len(all)-n))
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

// appendCompactNotice renders a visible transcript line when context
// compaction has trimmed earlier turns. Without this users see earlier
// messages silently vanish from the model's apparent memory and lose
// trust in what the assistant remembers.
//
//	⚙ context compacted — 87,500 → 12,400 tokens (auto)
func (c *chat) appendCompactNotice(originalTok, newTok int, trigger string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.streaming = false
	c.thinking = false
	icon := c.theme.ToolHeader.Render("⚙ ")
	label := c.theme.ToolHeader.Render("context compacted")
	body := c.theme.StatusLine.Render(fmt.Sprintf(
		" — %s → %s tokens (%s)",
		formatTokensWithCommas(originalTok),
		formatTokensWithCommas(newTok),
		trigger,
	))
	c.lines = append(c.lines, chatLine{rendered: icon + label + body})
	c.refresh()
}

// formatTokensWithCommas is the same comma-thousands formatter app.go uses
// for the status line, inlined here to avoid an import cycle.
func formatTokensWithCommas(n int) string {
	if n == 0 {
		return "0"
	}
	s := fmt.Sprintf("%d", n)
	out := make([]byte, 0, len(s)+len(s)/3)
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, byte(c))
	}
	return string(out)
}

func (c *chat) appendError(msg string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.streaming = false
	c.thinking = false
	// Claude Code style: bold red 'Error:' label + dim red body, prefixed
	// with the same '⏺' bullet so errors visually align with tool blocks.
	bullet := c.theme.Error.Bold(true).Render("⏺ Error: ")
	body := c.theme.Error.Render(msg)
	c.lines = append(c.lines, chatLine{rendered: bullet + body})
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
	if c.thinking {
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(renderThinking(c.theme, c.thinkTick))
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

// spinnerFrames is the 8-frame braille spinner Claude Code uses (also seen
// in spin/cli-spinners packages). Cycles every 80ms = ~12fps.
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// renderThinking renders the 'Thinking…' indicator with an animated spinner.
// Style: dim accent for the spinner, dim foreground for the word.
func renderThinking(theme Theme, frame int) string {
	sp := spinnerFrames[frame%len(spinnerFrames)]
	return theme.ToolHeader.Render(sp) + theme.StatusLine.Render(" Thinking…")
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
