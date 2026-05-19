package tui

import (
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
	require.Contains(t, strings.Join(m.(*App).chat.lines, "\n"), "hi there")
}
