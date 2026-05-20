package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"
)

func TestApp_Layout_F2Cycles(t *testing.T) {
	a := New(Options{})
	require.Equal(t, layoutSingle, a.layout)
	_, _ = a.Update(tea.KeyMsg{Type: tea.KeyF2})
	require.Equal(t, layoutSplit, a.layout)
	_, _ = a.Update(tea.KeyMsg{Type: tea.KeyF2})
	require.Equal(t, layoutTriple, a.layout)
	_, _ = a.Update(tea.KeyMsg{Type: tea.KeyF2})
	require.Equal(t, layoutSingle, a.layout)
}

func TestApp_AppendServerLog_AlwaysHitsLogPane(t *testing.T) {
	a := New(Options{})
	a.AppendServerLog("srv", "hello")
	require.Contains(t, strings.Join(a.logPane.lines, "\n"), "hello")
}

func TestApp_View_LayoutSplit_IncludesLogPane(t *testing.T) {
	a := New(Options{})
	a.width = 80
	a.height = 30
	a.layout = layoutSplit
	a.applyLayout()
	a.AppendServerLog("srv", "test-marker")
	out := a.View()
	require.Contains(t, out, "test-marker")
}

func TestApp_Layout_StatusLineShowsF2Hint(t *testing.T) {
	a := New(Options{})
	a.width = 80
	a.height = 24
	a.applyLayout()

	// Single layout
	out := testStripANSI(a.View())
	require.Contains(t, out, "[F2: single]")

	// Split layout
	_, _ = a.Update(tea.KeyMsg{Type: tea.KeyF2})
	out = testStripANSI(a.View())
	require.Contains(t, out, "[F2: split]")

	// Triple layout
	_, _ = a.Update(tea.KeyMsg{Type: tea.KeyF2})
	out = testStripANSI(a.View())
	require.Contains(t, out, "[F2: triple]")
}

func TestApp_ApplyLayout_PaneSizing(t *testing.T) {
	a := New(Options{})
	a.width = 100
	a.height = 40
	contentH := 40 - 3 // 37

	// split: chat=70%, log=30%
	a.layout = layoutSplit
	a.applyLayout()
	topH := contentH * 7 / 10 // 25
	botH := contentH - topH   // 12
	require.Equal(t, topH, a.chat.vp.Height)
	require.Equal(t, botH, a.logPane.vp.Height)

	// triple: side=30%, main=70%
	a.layout = layoutTriple
	a.applyLayout()
	sideW := 100 * 3 / 10 // 30
	mainW := 100 - sideW  // 70
	require.Equal(t, mainW, a.chat.vp.Width)
	require.Equal(t, sideW, a.statusPane.width)
}

func TestLogPane_Append_RollingCap(t *testing.T) {
	lp := newLogPane(DefaultTheme())
	for i := 0; i < 250; i++ {
		lp.append("line")
	}
	require.Equal(t, 200, len(lp.lines), "log pane should cap at 200 lines")
}

func TestTruncCwd(t *testing.T) {
	require.Equal(t, "/short", truncCwd("/short", 20))
	long := "/very/long/path/that/exceeds/limit"
	result := truncCwd(long, 10)
	// Check rune/display width: "…" counts as 1 rune, so total rune length <= maxw.
	require.True(t, len([]rune(result)) <= 10, "truncated cwd must be within maxw runes")
	require.True(t, strings.HasPrefix(result, "…"), "truncated cwd must start with ellipsis")
}
