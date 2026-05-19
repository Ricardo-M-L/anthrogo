package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/ricardo/anthrogo/pkg/query"
)

type engineEventMsg struct{ ev query.Event }

func pumpStream(ch <-chan query.Event) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return streamClosedMsg{}
		}
		return engineEventMsg{ev: ev}
	}
}

type streamClosedMsg struct{}
