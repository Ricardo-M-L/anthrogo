package tui

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChat_AppendAssistantDelta_AccumulatesRawText(t *testing.T) {
	c := newChat(DefaultTheme())
	c.appendAssistantDelta("Hello ")
	c.appendAssistantDelta("world")
	require.Equal(t, "Hello world", c.lines[0].rawText)
}

func TestChat_FinishAssistant_RendersMarkdown(t *testing.T) {
	c := newChat(DefaultTheme())
	c.appendAssistantDelta("# title\n\nbody **bold**")
	c.finishAssistant()
	// After render, the line should NOT just contain "**bold**" literally
	// (glamour transforms it). Check rendered string differs from rawText.
	require.NotEqual(t, c.lines[0].rawText, stripAssistantPrefix(c.lines[0].rendered))
	// glamour preserves text content even when styling it
	require.Contains(t, c.lines[0].rendered, "title")
}

func TestChat_AppendUser_NoMarkdownRendering(t *testing.T) {
	c := newChat(DefaultTheme())
	c.appendUser("**not bold**")
	require.Contains(t, c.lines[0].rendered, "**not bold**")
}

func TestChat_AppendAssistantDelta_StartsNewLineAfterFinish(t *testing.T) {
	c := newChat(DefaultTheme())
	c.appendAssistantDelta("first turn")
	c.finishAssistant()
	c.appendAssistantDelta("second turn")
	require.Len(t, c.lines, 2)
	require.Equal(t, "second turn", c.lines[1].rawText)
}

// stripAssistantPrefix removes the lipgloss-rendered "assistant > " prefix so
// tests can compare the body content only.
func stripAssistantPrefix(s string) string {
	// The prefix is rendered with ANSI codes; find the plain-text marker after
	// any ANSI escape sequences.
	const marker = "assistant > "
	idx := strings.Index(s, marker)
	if idx == -1 {
		return s
	}
	return s[idx+len(marker):]
}
