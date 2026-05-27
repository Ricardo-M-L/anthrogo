package tui

import (
	"context"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"

	"github.com/ricardo/anthrogo/pkg/command"
)

type stubCmd struct {
	name string
	desc string
}

func (s stubCmd) Name() string        { return s.name }
func (s stubCmd) Aliases() []string   { return nil }
func (s stubCmd) Description() string { return s.desc }
func (s stubCmd) Type() command.Type  { return command.TypeLocal }
func (s stubCmd) Run(context.Context, string, command.Host) (command.Result, error) {
	return command.Result{Text: "ok"}, nil
}

func newTestRegistry(names ...string) *command.Registry {
	r := command.NewRegistry()
	for _, n := range names {
		r.Register(stubCmd{name: n, desc: "stub " + n})
	}
	return r
}

func TestPalette_HiddenWhenInputDoesntStartWithSlash(t *testing.T) {
	p := newPalette(DefaultTheme(), newTestRegistry("/help", "/clear"))
	p.updateForInput("hello")
	require.False(t, p.visible())
	require.Equal(t, "", p.view())
}

func TestPalette_VisibleWithFuzzyMatches(t *testing.T) {
	p := newPalette(DefaultTheme(), newTestRegistry("/help", "/clear", "/cwd", "/compact"))
	p.updateForInput("/c")
	require.True(t, p.visible())
	require.Len(t, p.matches, 3) // /clear /cwd /compact
}

func TestPalette_TabCyclesForward(t *testing.T) {
	p := newPalette(DefaultTheme(), newTestRegistry("/clear", "/cwd", "/compact"))
	p.updateForInput("/c")
	require.Equal(t, 0, p.selected)

	consumed, input := p.handleKey(tea.KeyMsg{Type: tea.KeyTab})
	require.True(t, consumed)
	require.Equal(t, 1, p.selected)
	require.Equal(t, "/cwd ", input)

	consumed, input = p.handleKey(tea.KeyMsg{Type: tea.KeyTab})
	require.True(t, consumed)
	require.Equal(t, 2, p.selected)
	require.Equal(t, "/compact ", input)

	// Wrap around
	consumed, input = p.handleKey(tea.KeyMsg{Type: tea.KeyTab})
	require.True(t, consumed)
	require.Equal(t, 0, p.selected)
	require.Equal(t, "/clear ", input)
}

func TestPalette_ShiftTabCyclesBackward(t *testing.T) {
	p := newPalette(DefaultTheme(), newTestRegistry("/clear", "/cwd"))
	p.updateForInput("/c")

	consumed, input := p.handleKey(tea.KeyMsg{Type: tea.KeyShiftTab})
	require.True(t, consumed)
	require.Equal(t, 1, p.selected)
	require.Equal(t, "/cwd ", input)
}

func TestPalette_EscHidesPalette(t *testing.T) {
	p := newPalette(DefaultTheme(), newTestRegistry("/help"))
	p.updateForInput("/h")
	require.True(t, p.visible())

	consumed, input := p.handleKey(tea.KeyMsg{Type: tea.KeyEsc})
	require.True(t, consumed)
	require.Equal(t, "", input)
	require.False(t, p.visible())
}

func TestPalette_NonNavigationKey_NotConsumed(t *testing.T) {
	p := newPalette(DefaultTheme(), newTestRegistry("/help"))
	p.updateForInput("/h")

	consumed, input := p.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	require.False(t, consumed)
	require.Equal(t, "", input)
}

func TestPalette_HiddenWhenNoFuzzyMatches(t *testing.T) {
	p := newPalette(DefaultTheme(), newTestRegistry("/help", "/clear"))
	p.updateForInput("/zzz")
	require.False(t, p.visible())
	require.Empty(t, p.matches)
}
