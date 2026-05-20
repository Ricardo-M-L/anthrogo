package tui

import (
	"regexp"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"

	"github.com/ricardo/anthrogo/pkg/permissions"
	"github.com/ricardo/anthrogo/pkg/provider"
	"github.com/ricardo/anthrogo/pkg/provider/fake"
	"github.com/ricardo/anthrogo/pkg/tool"
)

// testStripANSI removes ANSI escape sequences from a string (test-local helper).
var ansiEscape = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

func testStripANSI(s string) string {
	return ansiEscape.ReplaceAllString(s, "")
}

func TestApp_Init_ReturnsTickCmd(t *testing.T) {
	a := New(Options{})
	cmd := a.Init()
	require.NotNil(t, cmd)
	// The tea.Batch with the tick should at minimum produce a non-nil cmd.
	// We don't drive the bubbletea loop here; just smoke-test that Init wires the tick.
}

func TestApp_Update_TickReturnsAnotherTick(t *testing.T) {
	a := New(Options{})
	_, cmd := a.Update(tickMsg(time.Now()))
	require.NotNil(t, cmd, "tick handler must re-schedule")
}

func TestApp_Mouse_WheelUpScrolls(t *testing.T) {
	a := New(Options{})
	_, cmd := a.Update(tea.MouseMsg{Button: tea.MouseButtonWheelUp})
	_ = cmd // hard to assert scroll state without driving the bubbletea loop;
	// smoke test that it doesn't panic.
}

func TestApp_Mouse_LeftClickOnURL(t *testing.T) {
	a := New(Options{})
	a.chat.lines = append(a.chat.lines, chatLine{
		rendered: "see https://example.com here",
		rawText:  "see https://example.com here",
	})
	url := a.urlAtPosition(8, 0) // x=8 → in "see https" region
	require.Equal(t, "https://example.com", url)
}

func TestApp_Mouse_NoURL_ReturnsEmpty(t *testing.T) {
	a := New(Options{})
	a.chat.lines = append(a.chat.lines, chatLine{rendered: "no link here", rawText: "no link here"})
	require.Equal(t, "", a.urlAtPosition(5, 0))
}

func TestURLRegex_ParsesHTTP(t *testing.T) {
	urls := urlRegex.FindAllString("see http://foo.com and https://bar.com", -1)
	require.Len(t, urls, 2)
}

func TestApp_ScriptedTurn_RendersAssistantText(t *testing.T) {
	fp := fake.New([]provider.Event{
		{Kind: provider.EventTextDelta, Text: "hi"},
		{Kind: provider.EventTextDelta, Text: " there"},
		{Kind: provider.EventMessageStop, StopReason: "end_turn"},
	})
	app := New(Options{
		Provider:    fp,
		Tools:       tool.NewRegistry(),
		Permissions: permissions.Empty(),
		Model:       "x",
	})

	// Drive Update synchronously: window size, then a submit, then pump
	// until the stream closes.
	m, _ := app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m, _ = m.(*App).Update(submitMsg{text: "hello"})

	// Pull events until streamClosedMsg.
	deadline := time.After(2 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timeout waiting for stream close")
		default:
		}
		cmd := m.(*App).stream
		ev, ok := <-cmd
		if !ok {
			break
		}
		m, _ = m.(*App).Update(engineEventMsg{ev: ev})
	}
	var renderedLines []string
	for _, ln := range m.(*App).chat.lines {
		renderedLines = append(renderedLines, ln.rendered)
	}
	plain := testStripANSI(strings.Join(renderedLines, "\n"))
	require.Contains(t, plain, "hi there")
}
